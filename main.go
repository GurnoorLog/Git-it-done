package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"track1-agent/internal/fireworks"
	"track1-agent/internal/local"
	"track1-agent/internal/output"
	"track1-agent/internal/router"
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

	var localClient *local.Client
	localURL := os.Getenv("OLLAMA_HOST")
	if localURL == "" {
		localURL = "http://localhost:11434"
	}
	localCfg := local.DefaultConfig
	localCfg.BaseURL = localURL
	localClient = local.NewClient(localCfg)

	taskRouter := router.New(localClient, fwClient, fwConfig.AllowedModels)

	inPath := os.Getenv("INPUT_PATH")
	if inPath == "" {
		inPath = "/input/tasks.json"
	}
	tasks, err := task.Load(inPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", inPath, err)
	}

	log.Printf("Loaded %d tasks from %s", len(tasks), inPath)

	// Global deadline: 8 minutes 30 seconds — leaves 90s buffer before the
	// 10-minute grading cap for results serialization and graceful shutdown.
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
	concurrencyLimit := 3
	if v := os.Getenv("AGENT_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			concurrencyLimit = n
		}
	}
	log.Printf("Agent concurrency: %d", concurrencyLimit)

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(concurrencyLimit)

	for i, t := range tasks {
		i, t := i, t
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				results[i] = emergencyFallback(taskRouter, t)
				return nil
			default:
			}
			res, err := taskRouter.Process(ctx, t)
			if err != nil {
				log.Printf("[Task %s] Process failed: %v; using emergency fallback", t.TaskID, err)
				results[i] = emergencyFallback(taskRouter, t)
				return nil
			}
			results[i] = res
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Printf("Task processing group error: %v (results will still be written)", err)
	}

	// Fill any remaining unprocessed tasks with emergency answers.
	for i, res := range results {
		if res.TaskID == "" {
			log.Printf("[Task %s] No result was produced; using emergency fallback", tasks[i].TaskID)
			results[i] = emergencyFallback(taskRouter, tasks[i])
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

func emergencyFallback(r *router.Router, t task.Task) task.Result {
	res := r.Emergency(context.Background(), t)
	if strings.HasPrefix(res.Answer, "ERROR:") {
		res.Answer = "Unable to fully determine the answer within constraints."
	}
	return res
}

func printSummary(tasks []task.Task, results []task.Result, fwClient *fireworks.Client) {
	var totalTokens, deterministicCount, localCount, localVerifiedCount, fireworksCount, verificationTokens, errorCount int
	for _, res := range results {
		switch res.ResolutionPath {
		case "deterministic":
			deterministicCount++
		case "local":
			localCount++
		case "local_verified":
			localVerifiedCount++
			verificationTokens += res.TotalTokens
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
	log.Printf("Resolved Local           : %d\n", localCount)
	log.Printf("Resolved Local+Verified  : %d (verification tokens: %d)\n", localVerifiedCount, verificationTokens)
	log.Printf("Resolved Fireworks       : %d\n", fireworksCount)
	log.Printf("Errors                   : %d\n", errorCount)
	log.Printf("Generation Tokens        : %d\n", totalTokens)
	log.Printf("Total Fireworks Tokens   : %d\n", totalTokens+verificationTokens)
	log.Println("--------------------------------------------------")
	log.Println("==================================================")
}
