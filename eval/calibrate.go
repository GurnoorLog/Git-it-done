package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"track1-agent/internal/fireworks"
)

func loadEnv() {
	f, err := os.Open(".env.testmodels")
	if err != nil {
		f, err = os.Open("../.env.testmodels")
	}
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

type evalTask struct {
	category string
	prompt   string
}

var tasks = []evalTask{
	{"math", "A store sells a jacket for $120. If there is a 25% discount, what is the final price?"},
	{"sentiment", "Classify the sentiment: 'The food was terrible and the service was rude.'"},
	{"summarization", "Summarize in one sentence: Artificial intelligence is transforming virtually every industry from healthcare to finance."},
	{"ner", "Extract all named entities: 'Apple Inc. was founded by Steve Jobs in Cupertino, California in 1976.'"},
	{"code_debugging", "Debug this Python code: \ndef divide(a, b):\n    return a / b\nresult = divide(10, 0)"},
	{"code_generation", "Write a Python function that returns the second largest value in a list of integers."},
	{"logical", "Five people — Alice, Bob, Carol, Dave, and Eve — are seated in a row. Alice is not next to Bob. Carol is immediately to the left of Dave. Eve is either first or last. Bob is not in position 3. Who is in position 2?"},
	{"factual", "What is the capital city of Australia?"},
}

type CalibrationData struct {
	Category       string  `json:"category"`
	Model          string  `json:"model"`
	LatencyMs      int64   `json:"latency_ms"`
	TotalTokens    int     `json:"total_tokens"`
	ConfidenceRec  float64 `json:"confidence_recommendation"` // For local sidecar fallback threshold
}

func main() {
	loadEnv()

	cfg, err := fireworks.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client := fireworks.NewClient(cfg)
	tierMap := fireworks.ClassifyTiers(cfg.AllowedModels)
	ctx := context.Background()

	fmt.Printf("Parsed Tiers: %+v\n", tierMap.Models)

	// Phase 1: Connectivity check on all available models.
	fmt.Println("\n--- Model Connectivity Check ---")
	for _, model := range cfg.AllowedModels {
		req := fireworks.GenerateRequest{
			Model:       model,
			System:      "Respond with 'ok'.",
			Prompt:      "Test",
			Temperature: 0.0,
			MaxTokens:   5,
		}
		resp, err := client.Generate(ctx, req)
		if err != nil {
			fmt.Printf("  %-55s ERROR: %v\n", model, err)
		} else {
			fmt.Printf("  %-55s OK (answer=%s, tokens=%d)\n", model, resp.Answer, resp.TotalTokens)
		}
	}

	fmt.Println("\nStarting calibration run across all 8 categories...")

	var results []CalibrationData

	for _, t := range tasks {
		targetModel := tierMap.SelectModel(t.category)
		fmt.Printf("\nTesting [%s] on model [%s]...\n", t.category, targetModel)
		
		prompts := fireworks.GetPrompts(t.category, "")
		req := fireworks.GenerateRequest{
			Model:       targetModel,
			System:      prompts.System,
			Prompt:      t.prompt,
			Prefill:     prompts.Prefill,
			Temperature: 0.0,
			MaxTokens:   500, // Safe default for calibration
		}

		start := time.Now()
		resp, err := client.Generate(ctx, req)
		latency := time.Since(start).Milliseconds()

		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		fmt.Printf("Answer: %s\nTokens: %d, Latency: %dms\n", resp.Answer, resp.TotalTokens, latency)
		
		// Recommend threshold based on task complexity
		conf := 0.9
		if t.category == "sentiment" || t.category == "factual" {
			conf = 0.6 // local can easily handle this
		} else if t.category == "math" || t.category == "logical" {
			conf = 1.0 // local struggles, high confidence needed
		}

		results = append(results, CalibrationData{
			Category:      t.category,
			Model:         targetModel,
			LatencyMs:     latency,
			TotalTokens:   resp.TotalTokens,
			ConfidenceRec: conf,
		})
	}

	b, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile("calibration_table.json", b, 0644); err != nil {
		log.Fatalf("Failed to write calibration_table.json: %v", err)
	}

	fmt.Println("\nCalibration complete! Results written to calibration_table.json.")
}
