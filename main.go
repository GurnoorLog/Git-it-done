package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"track1-agent/internal/classify"
	"track1-agent/internal/fireworks"
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

func run() error {
	fwConfig, err := fireworks.LoadConfig()
	if err != nil {
		return err
	}

	fwClient := fireworks.NewClient(fwConfig)
	taskRouter := router.New(fwClient, fwConfig.AllowedModels)

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

	// Phase 1: classify + deterministic solvers (fast, no API calls)
	for i, t := range tasks {
		cat := classify.Classify(t.Prompt)
		var resolved bool
		switch cat {
		case classify.CategoryMath:
			if res := solvers.SolveMath(t.Prompt); res.Solved {
				results[i] = task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}
				resolved = true
			}
		case classify.CategoryLogical:
			if res := solvers.SolveLogic(t.Prompt); res.Solved {
				results[i] = task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}
				resolved = true
			}
		case classify.CategorySentiment:
			text := solvers.ExtractQuotedOrTail(t.Prompt)
			if label, hits, confident := solvers.LexiconSentiment(text); confident {
				results[i] = task.Result{TaskID: t.TaskID, Answer: solvers.BuildSentimentAnswer(label, hits), ResolutionPath: "deterministic"}
				resolved = true
			}
		case classify.CategoryCodeGeneration:
			if code := solvers.LookupCodeTemplate(t.Prompt); code != "" {
				if ok, _ := solvers.VerifyCode(ctx, code); ok {
					results[i] = task.Result{TaskID: t.TaskID, Answer: code, ResolutionPath: "deterministic"}
					resolved = true
				}
			}
		}
		if resolved {
			log.Printf("[Task %s] Resolved: deterministic (%s)", t.TaskID, cat)
		} else {
			pending = append(pending, i)
		}
	}

	log.Printf("Phase 1 complete: %d deterministic, %d pending Fireworks", len(tasks)-len(pending), len(pending))

	if len(pending) > 0 {
		// Phase 2: group pending tasks by category and batch-process
		type pendingTask struct {
			idx  int
			task task.Task
			cat  classify.Category
		}

		pendingList := make([]pendingTask, 0, len(pending))
		for _, idx := range pending {
			pendingList = append(pendingList, pendingTask{
				idx:  idx,
				task: tasks[idx],
				cat:  classify.Classify(tasks[idx].Prompt),
			})
		}

		byCat := make(map[classify.Category][]pendingTask)
		for _, pt := range pendingList {
			byCat[pt.cat] = append(byCat[pt.cat], pt)
		}

		for cat, group := range byCat {
			select {
			case <-ctx.Done():
				log.Printf("Deadline reached; marking remaining tasks as emergency")
				for _, pt := range group {
					results[pt.idx] = emergencyResult(pt.task)
				}
				continue
			default:
			}

			taskList := make([]task.Task, len(group))
			for i, pt := range group {
				taskList[i] = pt.task
			}

			batchResults := taskRouter.BatchProcess(ctx, taskList, cat)
			for i, br := range batchResults {
				if i < len(group) {
					results[group[i].idx] = br
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
