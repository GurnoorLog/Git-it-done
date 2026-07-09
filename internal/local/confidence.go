package local

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
)

// ConfidenceResult holds the best answer and its confidence score.
type ConfidenceResult struct {
	Answer     string
	Confidence float64 // 0.0 to 1.0
	Method     string  // "field-match", "self-rated", or "single"
}

// isStructuredCategory returns true for categories where we can do semantic field comparison.
func isStructuredCategory(category string) bool {
	switch category {
	case "sentiment", "ner":
		return true
	}
	return false
}

// GenerateWithConfidence selects the right confidence strategy based on category.
//   - Structured (sentiment, ner): 2-sample field-comparison at temp 0.7.
//   - Free-text (everything else): 1 generation at temp 0.1, then a self-rating YES/NO call.
func (c *Client) GenerateWithConfidence(ctx context.Context, taskID, system, prompt, category string) (ConfidenceResult, error) {
	if isStructuredCategory(category) {
		return c.structuredFieldMatch(ctx, taskID, system, prompt, category)
	}
	return c.selfRatedConfidence(ctx, taskID, system, prompt, category)
}

// structuredFieldMatch generates 2 samples and compares semantically meaningful fields.
func (c *Client) structuredFieldMatch(ctx context.Context, taskID, system, prompt, category string) (ConfidenceResult, error) {
	// Generate 2 samples at temp 0.7.
	const numSamples = 2
	results := make([]string, numSamples)
	var firstErr error

	for i := 0; i < numSamples; i++ {
		ans, err := c.Generate(ctx, taskID, system, prompt, 0.7)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			results[i] = ""
		} else {
			results[i] = strings.TrimSpace(ans)
		}
	}

	// Filter valid samples.
	var valid []string
	for _, r := range results {
		if r != "" {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		return ConfidenceResult{}, firstErr
	}
	if len(valid) == 1 {
		// Only one sample; treat as low confidence.
		log.Printf("[Local Confidence] [Task %s] field-match: only 1 valid sample, conf=0.5", taskID)
		return ConfidenceResult{Answer: valid[0], Confidence: 0.5, Method: "field-match"}, nil
	}

	// Compare semantically.
	var conf float64
	switch category {
	case "sentiment":
		conf = compareSentiment(valid[0], valid[1])
	case "ner":
		conf = compareNER(valid[0], valid[1])
	default:
		conf = 0.5
	}

	// Use the first answer as the canonical answer.
	log.Printf("[Local Confidence] [Task %s] field-match category=%s conf=%.2f", taskID, category, conf)
	return ConfidenceResult{Answer: valid[0], Confidence: conf, Method: "field-match"}, nil
}

// selfRatedConfidence generates once at low temperature, then asks the model to rate its answer.
func (c *Client) selfRatedConfidence(ctx context.Context, taskID, system, prompt, category string) (ConfidenceResult, error) {
	// Step 1: Single generation at near-deterministic temperature.
	ans, err := c.Generate(ctx, taskID, system, prompt, 0.1)
	if err != nil {
		return ConfidenceResult{}, err
	}
	ans = strings.TrimSpace(ans)

	// Step 2: Self-rating verification call.
	verifyPrompt := "You were given the following question:\n\n" + prompt +
		"\n\nYou answered:\n\n" + ans +
		"\n\nIs your answer correct and complete? Reply ONLY with YES or NO."
	verdict, verifyErr := c.Generate(ctx, taskID, "", verifyPrompt, 0.0)
	if verifyErr != nil {
		// If self-rating fails, accept the answer at medium confidence rather than discarding.
		log.Printf("[Local Confidence] [Task %s] self-rated category=%s verification failed: %v; defaulting conf=0.3", taskID, category, verifyErr)
		return ConfidenceResult{Answer: ans, Confidence: 0.3, Method: "self-rated(verify-failed)"}, nil
	}

	var conf float64
	v := strings.ToUpper(strings.TrimSpace(verdict))
	if strings.HasPrefix(v, "YES") {
		conf = 0.9
	} else {
		conf = 0.2
	}

	log.Printf("[Local Confidence] [Task %s] self-rated category=%s verdict=%q conf=%.2f", taskID, category, v, conf)
	return ConfidenceResult{Answer: ans, Confidence: conf, Method: "self-rated"}, nil
}

// --- Semantic comparison helpers ---

func parseSentimentValue(raw string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	if v, ok := m["sentiment"]; ok {
		return strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v)))
	}
	return ""
}

func compareSentiment(a, b string) float64 {
	va := parseSentimentValue(a)
	vb := parseSentimentValue(b)
	if va == "" || vb == "" {
		// Fallback: full string comparison.
		if strings.ToLower(a) == strings.ToLower(b) {
			return 1.0
		}
		return 0.5
	}
	if va == vb {
		return 1.0
	}
	return 0.0
}

func parseNEREntitySet(raw string) map[string][]string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	result := make(map[string][]string)
	for _, key := range []string{"persons", "organizations", "locations"} {
		if v, ok := m[key]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", item)))
					if s != "" {
						result[key] = append(result[key], s)
					}
				}
				sort.Strings(result[key])
			}
		}
	}
	return result
}

func compareNER(a, b string) float64 {
	ma := parseNEREntitySet(a)
	mb := parseNEREntitySet(b)
	if ma == nil || mb == nil {
		if strings.ToLower(a) == strings.ToLower(b) {
			return 1.0
		}
		return 0.5
	}

	matches := 0
	total := 0
	for _, key := range []string{"persons", "organizations", "locations"} {
		setA := toSet(ma[key])
		setB := toSet(mb[key])
		for v := range setA {
			total++
			if setB[v] {
				matches++
			}
		}
		for v := range setB {
			if !setA[v] {
				total++
			}
		}
	}
	if total == 0 {
		return 1.0
	}
	return float64(matches) / float64(total)
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool)
	for _, v := range s {
		m[v] = true
	}
	return m
}
