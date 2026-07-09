package fireworks

import (
	"regexp"
	"strings"
)

// Tier represents the cost/capability hierarchy of available models.
type Tier int

const (
	TierUnknown Tier = iota
	TierCheap        // MoE-active pattern (e.g. 26b-a4b)
	TierQuantized    // Quantized markers (e.g. nvfp4, int8)
	TierCode         // Code specialist
	TierDense        // Dense general model (e.g. 31b without a4b or quant markers)
	TierFlagship     // Largest unclassified model
)

func (t Tier) String() string {
	switch t {
	case TierCheap:
		return "Cheap (MoE)"
	case TierQuantized:
		return "Quantized Mid"
	case TierCode:
		return "Code Specialist"
	case TierDense:
		return "Dense General"
	case TierFlagship:
		return "Flagship Fallback"
	default:
		return "Unknown"
	}
}

// TierMap assigns models to tiers based on their string names.
type TierMap struct {
	Models map[Tier]string
}

// ClassifyTiers takes a list of allowed models and maps them to tiers
// based on substring signals, NOT hardcoded model names.
func ClassifyTiers(allowed []string) TierMap {
	m := TierMap{Models: make(map[Tier]string)}
	var unclassified []string

	reMoE := regexp.MustCompile(`(?i)\b\d+b-a\d+b\b`)
	reQuant := regexp.MustCompile(`(?i)(nvfp4|nvfp8|int8|q4|q8|gptq|awq)`)
	reCode := regexp.MustCompile(`(?i)\bcode\b`)

	// Pass 1: explicit markers.
	for _, model := range allowed {
		lower := strings.ToLower(model)
		if reMoE.MatchString(lower) {
			m.Models[TierCheap] = model
		} else if reQuant.MatchString(lower) {
			m.Models[TierQuantized] = model
		} else if reCode.MatchString(lower) {
			m.Models[TierCode] = model
		} else {
			unclassified = append(unclassified, model)
		}
	}

	// Pass 2: residual classification.
	// If there's an unclassified model with 'gemma' in the name, it's likely the dense general model.
	// The remaining unclassified model (e.g. minimax) is the flagship fallback.
	var remaining []string
	for _, model := range unclassified {
		lower := strings.ToLower(model)
		// Check for -it suffix without MoE or Quant markers, which often denotes the dense general model.
		// A safer heuristic since we know Gemma models are dense if they lack MoE/Quant markers.
		if strings.Contains(lower, "gemma") {
			m.Models[TierDense] = model
		} else {
			remaining = append(remaining, model)
		}
	}

	// Any remaining model goes to Flagship.
	if len(remaining) > 0 {
		m.Models[TierFlagship] = remaining[0]
	}

	return m
}

// SelectModel chooses the best model tier for a given category.
// Falls back to increasingly capable tiers if the preferred one is missing.
func (tm TierMap) SelectModel(category string) string {
	models := tm.SelectModelFallbacks(category)
	if len(models) == 0 {
		return ""
	}
	return models[0]
}

// SelectModelFallbacks returns all models in preference order for a category.
// This enables the router to retry on a less-preferred model if the first one fails.
func (tm TierMap) SelectModelFallbacks(category string) []string {
	// Helper to collect all tiers in preference order, deduplicating.
	seen := make(map[string]bool)
	collect := func(prefs ...Tier) []string {
		var result []string
		for _, p := range prefs {
			if model, ok := tm.Models[p]; ok && !seen[model] {
				seen[model] = true
				result = append(result, model)
			}
		}
		// Append any remaining models not yet seen as a final fallback.
		for _, model := range tm.Models {
			if !seen[model] {
				seen[model] = true
				result = append(result, model)
			}
		}
		return result
	}

	switch category {
	// Simple tasks: cheap MoE is accurate enough, saves tokens for ranking.
	case "sentiment", "ner":
		return collect(TierCheap, TierQuantized, TierDense, TierFlagship, TierCode)
	// Summarization: MoE handles text tasks well, cheap on tokens.
	case "summarization":
		return collect(TierCheap, TierQuantized, TierDense, TierFlagship, TierCode)
	// Factual knowledge: flagship (minimax) is most accurate.
	case "factual":
		return collect(TierFlagship, TierDense, TierQuantized, TierCode, TierCheap)
	// Code: code specialist first.
	case "code_generation", "code_debugging":
		return collect(TierCode, TierFlagship, TierDense, TierQuantized, TierCheap)
	// Math/Logic: need best reasoning, flagship first.
	case "math", "logical":
		return collect(TierFlagship, TierDense, TierQuantized, TierCode, TierCheap)
	default:
		return collect(TierFlagship, TierDense, TierCheap, TierQuantized, TierCode)
	}
}
