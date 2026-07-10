package fireworks

import (
	"regexp"
	"strings"
)

type CategoryPrompt struct {
	System   string
	Prefill  string
	MaxTokens int
}

var reJSON = regexp.MustCompile(`(?i)\bjson\b`)

// GetPrompts returns the category system prompt and token budget.
func GetPrompts(category, taskPrompt string) CategoryPrompt {
	wantsJSON := reJSON.MatchString(taskPrompt)

	switch category {
	case "math":
		return CategoryPrompt{
			System: "You are a precise math solver. Output ONLY the final numeric answer. " +
				"No explanation, no steps, no units unless explicitly asked.\n" +
				"Example: '90' or '50%' or '160.00'.\n" +
				"No preamble. No markdown.",
			MaxTokens: 300,
		}

	case "sentiment":
		return CategoryPrompt{
			System: "Analyze the sentiment. Output exactly one word: positive, negative, or neutral.\n" +
				"No punctuation. No explanation. No preamble.\n" +
				"Examples: 'positive' / 'negative' / 'neutral'.",
			MaxTokens: 40,
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				System: "Extract ALL named entities. Output ONLY a valid JSON object in exactly:\n" +
					"{\"persons\":[\"Name1\",\"Name2\"],\"organizations\":[\"Org1\"],\"locations\":[\"Loc1\"]}\n" +
					"Use exact spans from text. Omit empty categories. No text before or after JSON.",
				MaxTokens: 250,
			}
		}
		return CategoryPrompt{
			System: "List entities grouped by type (persons, organizations, locations, dates).\n" +
				"Format: 'persons: Name1, Name2\norganizations: Org1\nlocations: Loc1'\n" +
				"No commentary. No preamble.",
			MaxTokens: 200,
		}

	case "summarization":
		return CategoryPrompt{
			System: "Summarize obeying the exact length constraint (sentence count, word count, or bullet count).\n" +
				"Use ONLY facts from the source text. No opinions. No preamble.",
			MaxTokens: 200,
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Return only the code. Handle all edge cases.\n" +
				"No markdown fences. No explanation. No preamble.\n" +
				"First line must be code or a code comment.",
			MaxTokens: 700,
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Return only the corrected code with a single-line bug comment at the top.\n" +
				"Format: '# BUG: short description' then the fixed code.\n" +
				"No markdown fences. No explanation. No preamble.",
			MaxTokens: 700,
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve the logic puzzle. Give only the final answer or arrangement.\n" +
				"No preamble. No markdown.",
			MaxTokens: 400,
		}

	case "factual":
		return CategoryPrompt{
			System: "Answer in one short, accurate sentence or a single fact.\n" +
				"No hedging. No preamble. No markdown.",
			MaxTokens: 200,
		}

	default:
		return CategoryPrompt{
			System: "Answer the question directly. Give the exact answer first.\n" +
				"No hedging. No preamble. No markdown.",
			MaxTokens: 200,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
