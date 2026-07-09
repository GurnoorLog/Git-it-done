package router

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"track1-agent/internal/classify"
	"track1-agent/internal/fireworks"
	"track1-agent/internal/local"
	"track1-agent/internal/solvers"
	"track1-agent/internal/task"
	"track1-agent/internal/validate"
)

func forceFireworksOnly() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("FORCE_FIREWORKS_ONLY")))
	return v == "true" || v == "1" || v == "yes"
}

type Router struct {
	localClient     *local.Client
	fireworksClient *fireworks.Client
	tierMap         fireworks.TierMap
}

func New(lc *local.Client, fc *fireworks.Client, allowedModels []string) *Router {
	return &Router{
		localClient:     lc,
		fireworksClient: fc,
		tierMap:         fireworks.ClassifyTiers(allowedModels),
	}
}

// Process routes a task through: deterministic solvers → verified-local
// pipelines → Fireworks. Local answers are ONLY accepted when they pass a
// deterministic, non-LLM verification; everything else escalates.
func (r *Router) Process(ctx context.Context, t task.Task) (task.Result, error) {
	cat := classify.Classify(t.Prompt)
	log.Printf("[Task %s] Category: %s\n", t.TaskID, cat)

	if forceFireworksOnly() {
		return r.resolveViaFireworks(ctx, t, cat, "")
	}

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
		if r.localClient != nil {
			if res, ok := r.trySentimentLocal(ctx, t); ok {
				return res, nil
			}
		}

	case classify.CategoryNER:
		if r.localClient != nil {
			if res, ok := r.tryNERLocal(ctx, t); ok {
				return res, nil
			}
		}

	case classify.CategorySummarization:
		if r.localClient != nil {
			if res, ok := r.trySummaryLocal(ctx, t); ok {
				return res, nil
			}
		}

	case classify.CategoryCodeGeneration:
		if r.localClient != nil {
			if res, ok := r.tryCodeGenLocal(ctx, t); ok {
				return res, nil
			}
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
	}

	return r.resolveViaFireworks(ctx, t, cat, extraContext)
}

// ── Verified-local pipelines (zero Fireworks tokens when accepted) ────────

// trySentimentLocal accepts a local label ONLY when the deterministic polarity
// lexicon gives an unambiguous signal that matches the local model's label.
func (r *Router) trySentimentLocal(ctx context.Context, t task.Task) (task.Result, bool) {
	text := solvers.ExtractQuotedOrTail(t.Prompt)
	label, hits, confident := solvers.LexiconSentiment(text)
	if !confident {
		log.Printf("[Task %s] Sentiment lexicon not confident; escalating\n", t.TaskID)
		return task.Result{}, false
	}

	out, err := r.localClient.GenerateOpts(ctx, t.TaskID, 
		"Classify the sentiment of the text as positive, negative, or neutral. Reply with exactly one word.",
		text, local.GenOpts{Temperature: 0, NumPredict: 8})
	if err != nil {
		log.Printf("[Task %s] Sentiment local call failed: %v; escalating\n", t.TaskID, err)
		return task.Result{}, false
	}
	if solvers.ParseSentimentLabel(out) != label {
		log.Printf("[Task %s] Sentiment local label %q disagrees with lexicon %q; escalating\n", t.TaskID, strings.TrimSpace(out), label)
		return task.Result{}, false
	}

	log.Printf("[Task %s] Resolved: local sentiment verified (%s)\n", t.TaskID, label)
	return task.Result{TaskID: t.TaskID, Answer: solvers.BuildSentimentAnswer(label, hits), ResolutionPath: "local_verified"}, true
}

// tryNERLocal accepts local extraction ONLY when every entity appears verbatim
// in the source AND no capitalised candidate was obviously missed.
func (r *Router) tryNERLocal(ctx context.Context, t task.Task) (task.Result, bool) {
	if solvers.MentionsDates(t.Prompt) {
		return task.Result{}, false // fixed 3-field pipeline can't serve date entities
	}
	source := solvers.ExtractNERSource(t.Prompt)
	sys := `Extract all named entities from the text. Output ONLY a JSON object: {"persons":["..."],"organizations":["..."],"locations":["..."]}. Copy each entity exactly as it appears in the text. Use empty arrays for missing categories.`

	out, err := r.localClient.GenerateOpts(ctx, t.TaskID, sys, source,
		local.GenOpts{Temperature: 0, NumPredict: 200, JSONFormat: true})
	if err != nil {
		return task.Result{}, false
	}
	ents, ok := solvers.ParseNERJSON(out)
	if !ok || !solvers.VerifyNER(source, ents) {
		log.Printf("[Task %s] NER local extraction failed verification; escalating\n", t.TaskID)
		return task.Result{}, false
	}

	var answer string
	if solvers.WantsJSON(t.Prompt) {
		answer = solvers.FormatNERJSON(ents)
	} else {
		answer = solvers.FormatNERSentence(ents)
	}
	if answer == "" {
		return task.Result{}, false
	}
	log.Printf("[Task %s] Resolved: local NER verified\n", t.TaskID)
	return task.Result{TaskID: t.TaskID, Answer: answer, ResolutionPath: "local_verified"}, true
}

// trySummaryLocal accepts a local summary ONLY when it exactly satisfies the
// parsed format constraint and passes content-overlap/hallucination checks.
func (r *Router) trySummaryLocal(ctx context.Context, t task.Task) (task.Result, bool) {
	source := solvers.ExtractSummarySource(t.Prompt)
	constraint := solvers.ParseSummaryConstraint(t.Prompt)
	sys := fireworks.GetPrompts("summarization", t.Prompt).System

	out, err := r.localClient.GenerateOpts(ctx, t.TaskID, sys, t.Prompt,
		local.GenOpts{Temperature: 0, NumPredict: 220})
	if err != nil {
		return task.Result{}, false
	}
	out = strings.TrimSpace(out)
	if !solvers.VerifySummary(source, out, constraint) {
		log.Printf("[Task %s] Summary local draft failed verification; escalating\n", t.TaskID)
		return task.Result{}, false
	}
	log.Printf("[Task %s] Resolved: local summary verified\n", t.TaskID)
	return task.Result{TaskID: t.TaskID, Answer: out, ResolutionPath: "local_verified"}, true
}

// tryCodeGenLocal accepts local code ONLY when a deterministic smoke test can
// be derived from the spec AND the code passes it under real execution.
func (r *Router) tryCodeGenLocal(ctx context.Context, t task.Task) (task.Result, bool) {
	sys := fireworks.GetPrompts("code_generation", t.Prompt).System
	out, err := r.localClient.GenerateOpts(ctx, t.TaskID, sys, t.Prompt,
		local.GenOpts{Temperature: 0, NumPredict: 400})
	if err != nil {
		return task.Result{}, false
	}
	code := stripCodeFences(out)
	script, ok := solvers.BuildSmokeTest(t.Prompt, code)
	if !ok {
		log.Printf("[Task %s] No smoke test derivable for spec; escalating\n", t.TaskID)
		return task.Result{}, false
	}
	if passed, msg := solvers.RunPython(ctx, script); !passed {
		log.Printf("[Task %s] Local code failed smoke test (%s); escalating\n", t.TaskID, msg)
		return task.Result{}, false
	}
	log.Printf("[Task %s] Resolved: local codegen passed smoke tests\n", t.TaskID)
	return task.Result{TaskID: t.TaskID, Answer: code, ResolutionPath: "local_verified"}, true
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
				// Run derived smoke tests too, when available.
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

// Emergency produces a best-effort answer when the normal pipeline failed.
// It never returns an "ERROR:" string — a plausible answer always beats a
// guaranteed zero.
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
	// Last resort: a local draft is still better than nothing.
	if r.localClient != nil {
		if out, err := r.localClient.GenerateOpts(ctx, t.TaskID, "Answer the task directly and concisely.", t.Prompt, local.GenOpts{Temperature: 0, NumPredict: 250}); err == nil && strings.TrimSpace(out) != "" {
			return task.Result{TaskID: t.TaskID, Answer: strings.TrimSpace(out), ResolutionPath: "local"}
		}
	}
	return task.Result{TaskID: t.TaskID, Answer: "Unable to fully determine the answer within constraints.", ResolutionPath: "local"}
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

// getMaxTokens returns tight per-category completion budgets. Values are
// calibrated to never truncate a well-formed answer while keeping the token
// score minimal.
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
		return 200
	}
}
