package fireworks

import (
	"regexp"
	"strings"
)

type CategoryPrompt struct {
	System    string
	Prefill   string
	MaxTokens int
}

var reJSON = regexp.MustCompile(`(?i)\bjson\b`)
var reSimpleEntities = regexp.MustCompile(`(?i)\b(people|persons|organizations?|locations?|places?|companies|compan)\b`)

func GetPrompts(category, taskPrompt string) CategoryPrompt {
	wantsJSON := reJSON.MatchString(taskPrompt)

	switch category {
	case "math":
		return CategoryPrompt{
			System: "Solve concisely. Show steps then 'Final answer: X'. No markdown.",
			MaxTokens: 240,
		}

	case "sentiment":
		return CategoryPrompt{
			System: "Output '[Label]. [One-sentence justification].' Label: positive, negative, neutral, or mixed. Be concise.",
			MaxTokens: 60,
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				System:    "Extract entities as JSON: {\"persons\":[],\"organizations\":[],\"locations\":[],\"dates\":[]}. Exact spans. Omit empty. No other text.",
				Prefill:   `{"persons":[`,
				MaxTokens: 160,
			}
		}
		return CategoryPrompt{
			System: "List entities by type: persons, organizations, locations, dates. Format 'Type: Name1, Name2'. Omit empty. Concise.",
			MaxTokens: 200,
		}

	case "summarization":
		return CategoryPrompt{
			System: "Summarize concisely obeying any stated length constraint. Only facts from source. No preamble, no markdown.",
			MaxTokens: 240,
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Write Python code. Handle all edge cases. Output ONLY code — no markdown fences, no explanation outside # comments. First line is code or #.",
			MaxTokens: 600,
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Fix the bug. Output ONLY valid Python code. Top line: '# BUG: ...'. Then corrected function. No markdown fences.",
			MaxTokens: 600,
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve logically. Under 100 words. End with 'Answer: X'. No markdown.",
			MaxTokens: 270,
		}

	case "factual":
		return CategoryPrompt{
			System: "Answer directly and concisely. No hedging. No preamble. No markdown.",
			MaxTokens: 220,
		}

	default:
		return CategoryPrompt{
			System:    "Answer directly and concisely. No preamble, no markdown.",
			MaxTokens: 220,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
