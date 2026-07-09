package router

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"track1-agent/internal/classify"
	"track1-agent/internal/fireworks"
	"track1-agent/internal/solvers"
	"track1-agent/internal/task"
	"track1-agent/internal/validate"
)

type Router struct {
	fireworksClient *fireworks.Client
	tierMap         fireworks.TierMap
}

func New(fc *fireworks.Client, allowedModels []string) *Router {
	return &Router{
		fireworksClient: fc,
		tierMap:         fireworks.ClassifyTiers(allowedModels),
	}
}

// Process routes a task through deterministic solvers, then Fireworks.
func (r *Router) Process(ctx context.Context, t task.Task) (task.Result, error) {
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
			MaxTokens:   getMaxTokens(cat),
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
			}
		}

		if valErr := validate.Validate(cat, ans); valErr != nil {
			log.Printf("[Task %s] Schema invalid on %s: %v (retrying)\n", t.TaskID, targetModel, valErr)
			req.Prompt = basePrompt + "\n\nIMPORTANT: Your previous response failed format validation: " + valErr.Error() + ". Respond in the exact required format."
			resp, err = r.fireworksClient.Generate(ctx, req)
			if err != nil {
				lastErr = err
				continue
			}
			ans = resp.Answer
			if isCode {
				ans = stripCodeFences(ans)
			}
			if valErr2 := validate.Validate(cat, ans); valErr2 != nil {
				lastErr = valErr2
				continue
			}
		}

		log.Printf("[Task %s] Resolved: fireworks (model=%s, tokens=%d)\n", t.TaskID, targetModel, resp.TotalTokens)
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
	batchSb.WriteString("Now provide your answers in this exact format (no extra text):\n")
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

var batchAnswerRe = regexp.MustCompile(`(?i)Task\s+(\d+)\s*:\s*(.*?)(?=\n\s*Task\s+\d+\s*:|\z)`)

// parseBatchAnswers returns answers indexed by task number (1-based).
// Missing answers are zero-valued strings.
func parseBatchAnswers(raw string) []string {
	numMap := make(map[int]string)
	for _, m := range batchAnswerRe.FindAllStringSubmatch(raw, -1) {
		if len(m) >= 3 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				numMap[n] = strings.TrimSpace(m[2])
			}
		}
	}
	// Find the highest task number so we know the slice length.
	maxN := 0
	for n := range numMap {
		if n > maxN {
			maxN = n
		}
	}
	result := make([]string, maxN)
	for n, ans := range numMap {
		result[n-1] = ans
	}
	return result
}

func getBatchMaxTokens(cat classify.Category, count int) int {
	perTask := getMaxTokens(cat)
	total := perTask * count
	// Add overhead for the combined prompt structure
	total += 200
	return total
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

func getMaxTokens(cat classify.Category) int {
	switch cat {
	case classify.CategorySentiment:
		return 80
	case classify.CategoryNER:
		return 250
	case classify.CategorySummarization:
		return 200
	case classify.CategoryCodeDebugging, classify.CategoryCodeGeneration:
		return 700
	case classify.CategoryLogical:
		return 800
	case classify.CategoryMath:
		return 300
	default: // factual
		return 250
	}
}
