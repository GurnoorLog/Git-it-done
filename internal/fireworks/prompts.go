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
			System: "You are a precise math solver. Compute step by step, then output exactly:\n" +
				"[numeric answer]: [brief explanation]\n" +
				"Example: '90.00: 25% of 120 is 30, so 120 - 30 = 90.'\n" +
				"No preamble. No markdown. English only.",
			MaxTokens: 300,
		}

	case "sentiment":
		return CategoryPrompt{
			System: "Analyze the sentiment. First word MUST be exactly: positive, negative, or neutral. Then '.' and a one-sentence reason citing words from the text.\n" +
				"Examples:\n" +
				"positive. The reviewer praises the fast delivery and quality.\n" +
				"negative. The author criticizes the slow service and poor quality.\n" +
				"neutral. The statement is factual without emotional language.\n" +
				"No other format. No preamble. English only.",
			MaxTokens: 80,
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
			System: "List EVERY named entity with its type in ONE sentence. Format EXACTLY:\n" +
				"'The text mentions [Name] (person), [Name] (organization), and [Name] (location).'\n" +
				"Do not miss any entity. Omit empty categories. No preamble. English only.",
			MaxTokens: 250,
		}

	case "summarization":
		return CategoryPrompt{
			System: "Summarize precisely. Follow the length constraint EXACTLY:\n" +
				"- 'two sentences' = exactly 2 sentences\n" +
				"- 'under 15 words' = fewer than 15 words\n" +
				"- 'three bullet points' = exactly 3 bullets\n" +
				"Use ONLY facts from the source text. No opinions. No interpretations.\n" +
				"Count your output before finalizing. No preamble. English only.",
			MaxTokens: 200,
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Write the requested code. Handle all edge cases: empty input, single element, ties, punctuation, negative numbers, None.\n" +
				"Output ONLY valid, runnable Python code.\n" +
				"No markdown fences. No explanation before or after.\n" +
				"First line must be code or a code comment.",
			MaxTokens: 700,
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Find and fix the bug precisely. Output format:\n" +
				"First line: comment naming bug in one short phrase, e.g. '# BUG: off-by-one error in loop condition'\n" +
				"All remaining lines: complete fixed code.\n" +
				"No markdown fences. No explanation before or after the code block.",
			MaxTokens: 700,
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve the logic puzzle step by step. Write short reasoning lines.\n" +
				"The very last line MUST be exactly: 'Final answer: ...'\n" +
				"Include all relevant details (name, order, assignment) in the final answer.\n" +
				"No markdown. English only.",
			MaxTokens: 800,
		}

	case "factual":
		return CategoryPrompt{
			System: "Answer accurately and completely.\n" +
				"If asked for a list, number, or specific fact: give the exact answer first, then brief explanation if needed.\n" +
				"No hedging ('I think', 'probably', 'I believe'). Precise facts only.\n" +
				"No preamble. No markdown. English only.",
			MaxTokens: 250,
		}

	default:
		return CategoryPrompt{
			System: "Answer the question directly. Give the exact answer first, then a brief explanation if needed.\n" +
				"No hedging. No preamble. No markdown. English only.",
			MaxTokens: 250,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
