package router

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"track1-agent/internal/cache"
	"track1-agent/internal/classify"
	"track1-agent/internal/fireworks"
	"track1-agent/internal/solvers"
	"track1-agent/internal/task"
	"track1-agent/internal/validate"
)

type Router struct {
	fireworksClient *fireworks.Client
	tierMap         fireworks.TierMap
	cache           *cache.Cache
}

func New(fc *fireworks.Client, allowedModels []string) *Router {
	return &Router{
		fireworksClient: fc,
		tierMap:         fireworks.ClassifyTiers(allowedModels),
		cache:           cache.New(),
	}
}

// Process routes a task through deterministic solvers, then Fireworks.
func (r *Router) Process(ctx context.Context, t task.Task) (task.Result, error) {
	// Check cache first
	if cached, ok := r.cache.Get(t.Prompt); ok {
		log.Printf("[Task %s] Cache hit\n", t.TaskID)
		return task.Result{TaskID: t.TaskID, Answer: cached, ResolutionPath: "deterministic"}, nil
	}

	cat := classify.Classify(t.Prompt)
	log.Printf("[Task %s] Category: %s\n", t.TaskID, cat)

	extraContext := ""
	switch cat {
	case classify.CategoryMath:
		if res := solvers.SolveMath(t.Prompt); res.Solved {
			log.Printf("[Task %s] Resolved: deterministic math\n", t.TaskID)
			return task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}, nil
		}

	case classify.CategoryLogical:
		if res := solvers.SolveLogic(t.Prompt); res.Solved {
			log.Printf("[Task %s] Resolved: deterministic logic (%s)\n", t.TaskID, res.Reasoning)
			return task.Result{TaskID: t.TaskID, Answer: res.Answer, ResolutionPath: "deterministic"}, nil
		}

	case classify.CategorySentiment:
		text := solvers.ExtractQuotedOrTail(t.Prompt)
		label, hits, confident := solvers.LexiconSentiment(text)
		if confident {
			log.Printf("[Task %s] Resolved: deterministic sentiment (%s)\n", t.TaskID, label)
			return task.Result{TaskID: t.TaskID, Answer: solvers.BuildSentimentAnswer(label, hits), ResolutionPath: "deterministic"}, nil
		}

	case classify.CategoryCodeDebugging:
		lintRes := solvers.StaticAnalyze(t.Prompt)
		if lintRes.AutoFixed && lintRes.FixedCode != "" {
			if ok, _ := solvers.VerifyCode(ctx, lintRes.FixedCode); ok {
				log.Printf("[Task %s] Resolved: static auto-fix\n", t.TaskID)
				return task.Result{TaskID: t.TaskID, Answer: lintRes.FixedCode, ResolutionPath: "deterministic"}, nil
			}
		}
		extraContext = lintRes.ExtraContext

	case classify.CategoryCodeGeneration:
		if code := solvers.LookupCodeTemplate(t.Prompt); code != "" {
			if ok, _ := solvers.VerifyCode(ctx, code); ok {
				log.Printf("[Task %s] Resolved: code catalog (%s)\n", t.TaskID, "matched template")
				return task.Result{TaskID: t.TaskID, Answer: code, ResolutionPath: "deterministic"}, nil
			}
		}
	}

	return r.resolveViaFireworks(ctx, t, cat, extraContext)
}

// ── Fireworks resolution ──────────────────────────────────────────────────

func (r *Router) resolveViaFireworks(ctx context.Context, t task.Task, cat classify.Category, extraContext string) (task.Result, error) {
	fallbacks := r.tierMap.SelectModelFallbacks(cat.String())
	if len(fallbacks) == 0 {
		return task.Result{}, fmt.Errorf("no suitable model found in ALLOWED_MODELS for category %s", cat)
	}

	prompts := fireworks.GetPrompts(cat.String(), t.Prompt)
	basePrompt := fireworks.BuildPrompt(t.Prompt, extraContext)
	isCode := cat == classify.CategoryCodeGeneration || cat == classify.CategoryCodeDebugging

	var lastErr error
	for _, targetModel := range fallbacks {
		req := fireworks.GenerateRequest{
			Model:       targetModel,
			System:      prompts.System,
			Prompt:      basePrompt,
			Prefill:     prompts.Prefill,
			Temperature: 0.0,
			MaxTokens:   prompts.MaxTokens,
		}

		resp, err := r.fireworksClient.Generate(ctx, req)
		if err != nil {
			log.Printf("[Task %s] Fireworks error on %s: %v (trying next)\n", t.TaskID, targetModel, err)
			lastErr = err
			continue
		}

		ans := resp.Answer
		if isCode {
			ans = stripCodeFences(ans)
		}

		// Quick quality check: reject empty, "I don't know", or error-like answers
		if isLowQualityAnswer(ans) {
			log.Printf("[Task %s] Low-quality answer from %s (trying next)\n", t.TaskID, targetModel)
			lastErr = fmt.Errorf("low quality answer: %q", ans)
			continue
		}

		// Code verification + test cases
		if isCode {
			ok, errMsg := solvers.VerifyCode(ctx, ans)
			if ok && cat == classify.CategoryCodeGeneration {
				if script, has := solvers.BuildSmokeTest(t.Prompt, ans); has {
					if passed, msg := solvers.RunPython(ctx, script); !passed {
						ok, errMsg = false, "failed test cases: "+msg
					}
				}
			}
			if !ok {
				log.Printf("[Task %s] Code verification failed on %s: %s (retrying)\n", t.TaskID, targetModel, errMsg)
				req.Prompt = basePrompt + "\n\nYour previous code failed: " + errMsg + ". Output the complete corrected code only."
				resp, err = r.fireworksClient.Generate(ctx, req)
				if err != nil {
					lastErr = err
					continue
				}
				ans = stripCodeFences(resp.Answer)
				if isLowQualityAnswer(ans) {
					lastErr = fmt.Errorf("low quality answer after retry: %q", ans)
					continue
				}
			}
		}

		// Schema validation: fail → try next model (not same model again)
		if valErr := validate.Validate(cat, ans); valErr != nil {
			log.Printf("[Task %s] Schema invalid on %s: %v (trying next model)\n", t.TaskID, targetModel, valErr)
			lastErr = valErr
			continue
		}

		log.Printf("[Task %s] Resolved: fireworks (model=%s, tokens=%d)\n", t.TaskID, targetModel, resp.TotalTokens)
		r.cache.Set(t.Prompt, ans)
		return task.Result{
			TaskID:         t.TaskID,
			Answer:         ans,
			ResolutionPath: "fireworks",
			ModelUsed:      targetModel,
			TotalTokens:    resp.TotalTokens,
		}, nil
	}

	return task.Result{}, fmt.Errorf("all models failed for category %s, last error: %w", cat, lastErr)
}

// ── Batching ─────────────────────────────────────────────────────────────────

// BatchResult holds the per-task outcome of a batch Fireworks call.
type BatchResult struct {
	TaskID string
	Result task.Result
	Error  error
}

// BatchProcess solves multiple same-category tasks in a single Fireworks call.
// It builds a numbered combined prompt, sends one request, and parses the
// numbered answers back into individual results.
func (r *Router) BatchProcess(ctx context.Context, tasks []task.Task, cat classify.Category) []task.Result {
	if len(tasks) == 0 {
		return nil
	}
	if len(tasks) == 1 {
		res, err := r.Process(ctx, tasks[0])
		if err != nil {
			return []task.Result{{TaskID: tasks[0].TaskID, Answer: emergencyText(), ResolutionPath: "emergency"}}
		}
		return []task.Result{res}
	}

	// Build a combined prompt with numbered sub-tasks.
	var batchSb strings.Builder
	batchSb.WriteString("You are solving ")
	batchSb.WriteString(strconv.Itoa(len(tasks)))
	batchSb.WriteString(" tasks. Answer each one, numbering your answers.\n\n")
	for i, t := range tasks {
		batchSb.WriteString("Task ")
		batchSb.WriteString(strconv.Itoa(i + 1))
		batchSb.WriteString(":\n")
		batchSb.WriteString(t.Prompt)
		batchSb.WriteString("\n\n")
	}
	batchSb.WriteString("Now provide your answers after this line in the exact format shown (no extra text):\n")
	batchSb.WriteString("=== ANSWERS ===\n")
	for i := range tasks {
		batchSb.WriteString("Task ")
		batchSb.WriteString(strconv.Itoa(i + 1))
		batchSb.WriteString(": [answer ")
		batchSb.WriteString(strconv.Itoa(i + 1))
		batchSb.WriteString("]\n")
	}

	combinedPrompt := batchSb.String()
	log.Printf("[Batch %s] Batched %d tasks into one prompt (%d chars)", cat, len(tasks), len(combinedPrompt))

	fallbacks := r.tierMap.SelectModelFallbacks(cat.String())
	catPrompts := fireworks.GetPrompts(cat.String(), tasks[0].Prompt)

	var lastErr error
	for _, targetModel := range fallbacks {
		req := fireworks.GenerateRequest{
			Model:       targetModel,
			System:      catPrompts.System,
			Prompt:      combinedPrompt,
			Prefill:     catPrompts.Prefill,
			Temperature: 0.0,
			MaxTokens:   getBatchMaxTokens(cat, len(tasks)),
		}

		resp, err := r.fireworksClient.Generate(ctx, req)
		if err != nil {
			log.Printf("[Batch %s] Fireworks error on %s: %v (trying next)", cat, targetModel, err)
			lastErr = err
			continue
		}

		parsed := parseBatchAnswers(resp.Answer)
		if len(parsed) < len(tasks) {
			log.Printf("[Batch %s] Parse warning: got %d answers, expected %d; falling back to individual", cat, len(parsed), len(tasks))
			break // fall through to individual fallback
		}

		results := make([]task.Result, len(tasks))
		tokensPerTask := resp.TotalTokens / max(1, len(tasks))
		for i, t := range tasks {
			ans := parsed[i]
			if cat == classify.CategoryCodeGeneration || cat == classify.CategoryCodeDebugging {
				ans = stripCodeFences(ans)
			}
			results[i] = task.Result{
				TaskID:         t.TaskID,
				Answer:         ans,
				ResolutionPath: "fireworks",
				ModelUsed:      targetModel,
				TotalTokens:    tokensPerTask,
			}
		}
		log.Printf("[Batch %s] Batch resolved on %s (total tokens=%d)", cat, targetModel, resp.TotalTokens)
		return results
	}

	log.Printf("[Batch %s] Batch failed all models; falling back to individual calls", cat)
	_ = lastErr
	results := make([]task.Result, len(tasks))
	for i, t := range tasks {
		res, err := r.Process(ctx, t)
		if err != nil {
			results[i] = task.Result{TaskID: t.TaskID, Answer: emergencyText(), ResolutionPath: "emergency"}
		} else {
			results[i] = res
		}
	}
	return results
}

var batchTaskRe = regexp.MustCompile(`(?i)^\s*Task\s+(\d+)\s*:\s*`) // matches "Task N: " at line start
var reAnswersDelim = regexp.MustCompile(`(?i)^\s*={3,}\s*ANSWERS\s*={3,}\s*`) // matches "=== ANSWERS ==="
var reNewline = regexp.MustCompile(`\r?\n`)

// parseBatchAnswers returns answers indexed by position (0-based).
// It finds the "=== ANSWERS ===" delimiter, then extracts content after each
// "Task N:" header until the next header or end of string.
func parseBatchAnswers(raw string) []string {
	lines := reNewline.Split(raw, -1)

	// Find the ANSWERS delimiter
	startIdx := -1
	for i, line := range lines {
		if reAnswersDelim.MatchString(line) {
			startIdx = i + 1
			break
		}
	}
	if startIdx < 0 {
		return nil
	}

	taskLines := make(map[int][]string)
	var currentTask int
	var inAnswer bool
	for _, line := range lines[startIdx:] {
		if m := batchTaskRe.FindStringSubmatch(line); len(m) >= 2 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				currentTask = n
				inAnswer = true
				rest := strings.TrimSpace(batchTaskRe.ReplaceAllString(line, ""))
				if rest != "" {
					taskLines[currentTask] = append(taskLines[currentTask], rest)
				}
				continue
			}
		}
		if inAnswer && currentTask > 0 {
			taskLines[currentTask] = append(taskLines[currentTask], line)
		}
	}
	if len(taskLines) == 0 {
		return nil
	}
	maxN := 0
	for n := range taskLines {
		if n > maxN {
			maxN = n
		}
	}
	result := make([]string, maxN)
	for n, lines := range taskLines {
		answer := strings.TrimSpace(strings.Join(lines, "\n"))
		result[n-1] = answer
	}
	// Validate all expected answers are non-empty
	for i := 0; i < len(result); i++ {
		if result[i] == "" {
			return nil
		}
	}
	return result
}

// isLowQualityAnswer catches empty, error-like, or refusal responses.
func isLowQualityAnswer(ans string) bool {
	trimmed := strings.TrimSpace(ans)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	refusals := []string{
		"i don't know", "i cannot", "i can't", "i'm not sure",
		"i am not sure", "unable to", "not able to", "error:",
		"an error occurred", "as an ai", "i apologize",
	}
	for _, r := range refusals {
		if strings.Contains(lower, r) {
			return true
		}
	}
	return false
}

func emergencyText() string {
	return "Unable to fully determine the answer within constraints."
}

// Emergency produces a best-effort answer when the normal pipeline failed.
func (r *Router) Emergency(ctx context.Context, t task.Task) task.Result {
	fallbacks := r.tierMap.SelectModelFallbacks("factual")
	for _, model := range fallbacks {
		resp, err := r.fireworksClient.Generate(ctx, fireworks.GenerateRequest{
			Model:       model,
			System:      "Answer the task directly and concisely. No preamble.",
			Prompt:      t.Prompt,
			Temperature: 0.0,
			MaxTokens:   300,
		})
		if err == nil && strings.TrimSpace(resp.Answer) != "" {
			log.Printf("[Task %s] Resolved: emergency fireworks (model=%s)\n", t.TaskID, model)
			return task.Result{TaskID: t.TaskID, Answer: resp.Answer, ResolutionPath: "fireworks", ModelUsed: model, TotalTokens: resp.TotalTokens}
		}
	}
	return task.Result{TaskID: t.TaskID, Answer: "Unable to fully determine the answer within constraints.", ResolutionPath: "emergency"}
}

// ── Helpers ───────────────────────────────────────────────────────────────

var reFence = regexp.MustCompile(`(?s)^\s*` + "```" + `[a-zA-Z]*\n?(.*?)\n?` + "```" + `\s*$`)
var reOpenFence = regexp.MustCompile(`^` + "```" + `[a-zA-Z]*\n`)

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if m := reFence.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if reOpenFence.MatchString(s) {
		s = reOpenFence.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}

func getBatchMaxTokens(cat classify.Category, count int) int {
	perTask := 250
	switch cat {
	case classify.CategorySentiment:
		perTask = 80
	case classify.CategoryNER, classify.CategorySummarization:
		perTask = 250
	case classify.CategoryCodeDebugging, classify.CategoryCodeGeneration:
		perTask = 700
	case classify.CategoryLogical:
		perTask = 800
	case classify.CategoryMath:
		perTask = 300
	}
	return perTask*count + 200
}
