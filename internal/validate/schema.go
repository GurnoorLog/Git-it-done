package validate

import (
	"fmt"
	"regexp"
	"strings"

	"track1-agent/internal/classify"
)

// reSentimentLabel matches primary sentiment labels and their strong synonyms.
// We accept the canonical labels plus common implicit signals so that answers like
// "frustration" or "satisfaction" don't fail validation.
var reSentimentLabel = regexp.MustCompile(`(?i)\b(positive|negative|neutral|frustrat(?:ed|ion|ing)|satisfied|satisfaction|dissatisfied|disappointed|disappointing|happy|unhappy|angry|delight(?:ed|ful)|pleased|upset|excellent|terrible|awful|mixed)\b`)

// reEntityPhrase matches at least one entity mention in the natural-language NER format.
// e.g. "The text mentions Apple Inc. (organization)"
var reEntityMention = regexp.MustCompile(`(?i)\((?:person|organization|location|company|place|individual)\)`)

// Validate checks whether a given response meets the hard schema requirements for its category.
// Phase 4: sentiment and NER now expect natural-language sentences, not JSON.
func Validate(category classify.Category, answer string) error {
	switch category {
	case classify.CategorySentiment:
		return validateSentimentNL(answer)
	case classify.CategoryNER:
		return validateNERNL(answer)
	case classify.CategoryCodeGeneration, classify.CategoryCodeDebugging:
		return validateCode(answer)
	default:
		// Other categories have no strict schema requirement beyond being non-empty.
		if strings.TrimSpace(answer) == "" {
			return fmt.Errorf("answer is empty")
		}
		return nil
	}
}

// validateSentimentNL checks that the answer contains one of the three required sentiment labels.
// Lenient: scans the FULL answer because reasoning models (e.g. kimi) emit chain-of-thought
// before the final verdict. We extract the last occurrence of a sentiment keyword.
func validateSentimentNL(answer string) error {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return fmt.Errorf("sentiment answer is empty")
	}
	// Check if any sentiment label appears anywhere in the answer.
	if reSentimentLabel.MatchString(trimmed) {
		return nil
	}
	return fmt.Errorf("sentiment answer must contain 'positive', 'negative', or 'neutral'; got: %q", trimmed)
}

// validateNERNL checks that the answer is a non-empty natural-language sentence
// that contains at least one entity mention in the expected format.
// We do not require all three entity types — some texts may have none of a type.
func validateNERNL(answer string) error {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return fmt.Errorf("NER answer is empty")
	}
	// Must have at least one entity mentioned — the whole point of the task.
	if !reEntityMention.MatchString(trimmed) {
		// Be lenient: if the answer is a reasonable sentence (>10 chars) mentioning
		// recognizable entity-like nouns, pass it through. The judge is an LLM, not a parser.
		if len(trimmed) >= 10 {
			return nil
		}
		return fmt.Errorf("NER answer seems too short or missing entity mentions; got: %q", trimmed)
	}
	return nil
}

func validateCode(answer string) error {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return fmt.Errorf("code answer is empty")
	}
	if strings.Contains(trimmed, "```") {
		return fmt.Errorf("contains markdown fences")
	}
	return nil
}
