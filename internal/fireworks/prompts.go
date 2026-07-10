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
			System: "Steps, then 'Final: X'. Under 15 words. No md.",
			MaxTokens: 80,
		}

	case "sentiment":
		return CategoryPrompt{
			System: "Label + reason. Under 10 words. No md.",
			MaxTokens: 40,
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				System:    "JSON: {\"persons\":[],\"orgs\":[],\"locs\":[]}. Exact spans. Omit empty. Under 20 words total.",
				Prefill:   `{"persons":[`,
				MaxTokens: 100,
			}
		}
		return CategoryPrompt{
			System: "Entities by type. 'T: N1,N2'. Omit empty. Under 15 words.",
			MaxTokens: 120,
		}

	case "summarization":
		return CategoryPrompt{
			System: "Summarize. Obey length limit. Only facts. Under 30 words.",
			MaxTokens: 200,
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Python. Handle edge cases. Code only — no ```, no explanation. First line: code or #comment. Under 40 lines.",
			MaxTokens: 600,
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Fix bug. Code only. '# BUG: ...' at top. No ```. Under 40 lines.",
			MaxTokens: 600,
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve. Under 40 words. End 'Answer: X'. No md.",
			MaxTokens: 120,
		}

	case "factual":
		return CategoryPrompt{
			System: "Answer directly. Under 12 words. No md.",
			MaxTokens: 60,
		}

	default:
		return CategoryPrompt{
			System:    "Answer directly. Under 12 words. No md.",
			MaxTokens: 60,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
