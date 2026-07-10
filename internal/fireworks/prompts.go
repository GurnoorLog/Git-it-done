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
			MaxTokens: 300,
		}

	case "sentiment":
		return CategoryPrompt{
			System: "Output '[Label]. [One-sentence justification].' Label: positive, negative, neutral. Be concise.",
			MaxTokens: 80,
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				System:    "Extract entities as JSON: {\"persons\":[],\"organizations\":[],\"locations\":[]}. Exact spans. Omit empty. No other text.",
				Prefill:   `{"persons":[`,
				MaxTokens: 200,
			}
		}
		return CategoryPrompt{
			System: "List entities by type: persons, organizations, locations. Format 'Type: Name1, Name2'. Omit empty. Concise.",
			MaxTokens: 250,
		}

	case "summarization":
		return CategoryPrompt{
			System: "Summarize concisely obeying any stated length constraint. Only facts from source. No preamble, no markdown.",
			MaxTokens: 300,
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Write Python code. Handle all edge cases. Output ONLY code — no markdown fences, no explanation outside # comments. First line is code or #.",
			MaxTokens: 800,
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Fix the bug. Output ONLY valid Python code. Top line: '# BUG: ...'. Then corrected function. No markdown fences.",
			MaxTokens: 800,
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve logically. Under 100 words. End with 'Answer: X'. No markdown.",
			MaxTokens: 350,
		}

	case "factual":
		return CategoryPrompt{
			System: "Answer directly and concisely. No hedging. No preamble. No markdown.",
			MaxTokens: 300,
		}

	default:
		return CategoryPrompt{
			System:    "Answer directly and concisely. No preamble, no markdown.",
			MaxTokens: 300,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
