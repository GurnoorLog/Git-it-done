package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"track1-agent/internal/classify"
	"track1-agent/internal/fireworks"
	"track1-agent/internal/local"
	"track1-agent/internal/output"
	"track1-agent/internal/router"
	"track1-agent/internal/solvers"
	"track1-agent/internal/task"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := run(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}

func loadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}
}

func run() error {
	loadEnv()
	fwConfig, err := fireworks.LoadConfig()
	if err != nil {
		return err
	}

	fwClient := fireworks.NewClient(fwConfig)
	taskRouter := router.New(fwClient, fwConfig.AllowedModels)

	localClient := local.New()
	if err := localClient.Start(context.Background()); err != nil {
		log.Printf("Warning: local model not available: %v (continuing without)", err)
	}
	taskRouter.SetLocalClient(localClient)
	defer localClient.Stop()

	inPath := os.Getenv("INPUT_PATH")
	if inPath == "" {
		inPath = "/input/tasks.json"
	}
	tasks, err := task.Load(inPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", inPath, err)
	}

	log.Printf("Loaded %d tasks from %s", len(tasks), inPath)

	deadline := 8*time.Minute + 30*time.Second
	if v := os.Getenv("AGENT_DEADLINE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			deadline = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	log.Printf("Agent deadline: %v", deadline)

	results := make([]task.Result, len(tasks))
	pending := make([]int, 0, len(tasks))

	// Phase 1: first try logic solver on ALL tasks (pre-classification pass,
	// matching the reference solution's approach)
	for i, t := range tasks {
		if res := solvers.SolveLogic(t.Prompt); res.Solved {
			results[i] = task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}
			log.Printf("[Task %s] Resolved: deterministic (logic pre-classify)", t.TaskID)
		} else {
			pending = append(pending, i)
		}
	}

	// Phase 1b: classify remaining tasks and try deterministic solvers
	var stillPending []int
	for _, idx := range pending {
		t := tasks[idx]
		cat := classify.Classify(t.Prompt)
		var resolved bool
		switch cat {
		case classify.CategoryMath:
			if res := solvers.SolveMath(t.Prompt); res.Solved {
				results[idx] = task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}
				resolved = true
			}
		case classify.CategorySentiment:
			text := solvers.ExtractQuotedOrTail(t.Prompt)
			if label, hits, confident := solvers.LexiconSentiment(text); confident {
				results[idx] = task.Result{TaskID: t.TaskID, Answer: solvers.BuildSentimentAnswer(label, hits), ResolutionPath: "deterministic"}
				resolved = true
			}
		case classify.CategoryCodeGeneration:
			if code := solvers.LookupCodeTemplate(t.Prompt); code != "" {
				if ok, _ := solvers.VerifyCode(ctx, code); ok {
					results[idx] = task.Result{TaskID: t.TaskID, Answer: code, ResolutionPath: "deterministic"}
					resolved = true
				}
			}
		}
		if resolved {
			log.Printf("[Task %s] Resolved: deterministic (%s)", t.TaskID, cat)
		} else {
			stillPending = append(stillPending, idx)
		}
	}
	pending = stillPending

	log.Printf("Phase 1 complete: %d deterministic, %d pending Fireworks", len(tasks)-len(pending), len(pending))

	if len(pending) > 0 {
		// Phase 2: batch Fireworks calls. Group same-category tasks into
		// batches (max 5 per batch), matching the reference solution.
		batches := groupPendingTasks(tasks, pending)
		log.Printf("Phase 2: %d tasks in %d batches", len(pending), len(batches))

		for _, batch := range batches {
			select {
			case <-ctx.Done():
				log.Printf("Deadline reached; marking remaining tasks as emergency")
				for _, ptIdx := range pending {
					if results[ptIdx].TaskID == "" {
						results[ptIdx] = emergencyResult(tasks[ptIdx])
					}
				}
				goto done
			default:
			}

			// Extract plain tasks for the router
			plain := make([]task.Task, len(batch))
			for i, it := range batch {
				plain[i] = it.Task
			}
			cat := classify.Classify(plain[0].Prompt)

			if len(batch) == 1 {
				res, err := taskRouter.Process(ctx, plain[0])
				if err != nil {
					log.Printf("[Task %s] Fireworks failed: %v; using emergency", batch[0].TaskID, err)
					results[batch[0].Index] = emergencyResult(batch[0].Task)
				} else {
					results[batch[0].Index] = res
				}
			} else {
				batchResults := taskRouter.BatchProcess(ctx, plain, cat)
				for _, br := range batchResults {
					for _, it := range batch {
						if it.TaskID == br.TaskID {
							results[it.Index] = br
							break
						}
					}
				}
			}
		}

		// Fill any remaining unprocessed tasks with emergency answers.
		for i, res := range results {
			if res.TaskID == "" {
				log.Printf("[Task %s] No result was produced; using emergency fallback", tasks[i].TaskID)
				results[i] = emergencyResult(tasks[i])
			}
		}
	}

done:
	outPath := os.Getenv("OUTPUT_PATH")
	if outPath == "" {
		outPath = "/output/results.json"
	}
	if err := output.Write(outPath, results); err != nil {
		return fmt.Errorf("failed to write to %s: %w", outPath, err)
	}

	printSummary(tasks, results, fwClient)
	log.Println("Successfully processed all tasks and wrote results.")
	return nil
}

func emergencyResult(t task.Task) task.Result {
	return task.Result{
		TaskID:         t.TaskID,
		Answer:         "Unable to fully determine the answer within constraints.",
		ResolutionPath: "emergency",
	}
}

// indexedTask pairs a task with its position in the originals array.
type indexedTask struct {
	task.Task
	Index int
}

// groupPendingTasks groups pending tasks by category, up to 5 per batch.
// Non-batchable categories (summarization, code_*) are kept as singles.
func groupPendingTasks(tasks []task.Task, pending []int) [][]indexedTask {
	nonBatchable := map[classify.Category]bool{
		classify.CategorySummarization:   true,
		classify.CategoryCodeDebugging:   true,
		classify.CategoryCodeGeneration: true,
	}
	groups := make(map[classify.Category][]indexedTask)
	for _, idx := range pending {
		cat := classify.Classify(tasks[idx].Prompt)
		groups[cat] = append(groups[cat], indexedTask{tasks[idx], idx})
	}
	var batches [][]indexedTask
	for cat, items := range groups {
		if nonBatchable[cat] || len(items) == 1 {
			for _, it := range items {
				batches = append(batches, []indexedTask{it})
			}
		} else {
			for i := 0; i < len(items); i += 5 {
				end := i + 5
				if end > len(items) {
					end = len(items)
				}
				batches = append(batches, items[i:end])
			}
		}
	}
	return batches
}

func printSummary(tasks []task.Task, results []task.Result, fwClient *fireworks.Client) {
	var totalTokens, deterministicCount, fireworksCount, errorCount int
	for _, res := range results {
		switch res.ResolutionPath {
		case "deterministic":
			deterministicCount++
		case "fireworks":
			fireworksCount++
			totalTokens += res.TotalTokens
		default:
			errorCount++
		}
	}

	log.Println("==================================================")
	log.Println("              FINAL SUMMARY REPORT                ")
	log.Println("==================================================")
	log.Printf("Total Tasks              : %d\n", len(tasks))
	log.Printf("Resolved Deterministic   : %d\n", deterministicCount)
	log.Printf("Resolved Fireworks       : %d\n", fireworksCount)
	log.Printf("Errors                   : %d\n", errorCount)
	log.Printf("Total Fireworks Tokens   : %d\n", totalTokens)
	log.Println("--------------------------------------------------")
	log.Println("==================================================")
}
