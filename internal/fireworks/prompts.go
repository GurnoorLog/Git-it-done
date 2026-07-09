package fireworks

import "strings"

type CategoryPrompt struct {
	System  string
	Prefill string
}

// GetPrompts returns the category system prompt. taskPrompt is inspected so
// that explicit format requests in the task (e.g. "return as JSON") always win
// over category defaults.
func GetPrompts(category, taskPrompt string) CategoryPrompt {
	const terse = "Answer only, no preamble, no markdown formatting. All output must be in English."
	wantsJSON := strings.Contains(strings.ToLower(taskPrompt), "json")

	switch category {
	case "math":
		return CategoryPrompt{
			System: "You are a precise math solver. " + terse + " Compute carefully step by step in your head, then output exactly one line: the final numeric answer, a colon, and a one-line explanation of the calculation. Example: '90.00: 25% of 120 is 30, so 120 - 30 = 90.'",
		}

	case "sentiment":
		return CategoryPrompt{
			System: "You are a sentiment analyst. " + terse + " Your response MUST begin with exactly one word: positive, negative, or neutral. Then a period and a one-sentence reason citing words from the text. Example: 'negative. The author criticizes the slow service and poor food quality.'",
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				System: "You are an expert named-entity extractor. Output ONLY a valid JSON object in exactly the shape the task requests (e.g. {\"persons\":[],\"organizations\":[],\"locations\":[]}). Use exact spans from the text. No text before or after the JSON.",
			}
		}
		return CategoryPrompt{
			System: "You are an expert named-entity extractor. List every named entity in the text with its type in ONE sentence. Format: 'The text mentions [Name] (person), [Name] (organization), and [Name] (location).' Omit empty categories. Do not miss any entity. " + terse,
		}

	case "summarization":
		return CategoryPrompt{
			System: "You are a precise summarizer. " + terse + " Read the length/format constraint in the task and follow it EXACTLY: 'two sentences' means exactly 2 sentences; 'under 15 words' means fewer than 15 words; 'three bullet points' means exactly 3 bullets. Use only facts from the source text.",
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Write the requested code. Handle edge cases (empty input, single element, ties, punctuation) per the spec. " +
				"Output ONLY valid, runnable code. No markdown code fences. No explanation before or after. The first line must be code (or a code comment).",
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Find and fix the bug in the given code. " +
				"First line must be a comment (e.g. '# BUG: ...') naming the bug in one short phrase. " +
				"All remaining lines are the complete fixed code. " +
				"Output ONLY valid code. No markdown code fences. No explanation before or after.",
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve the logic problem. Reason concisely step by step (short lines, no filler). " +
				"The very last line MUST be exactly: 'Final answer: ...' with the complete answer to the question asked. " +
				"No markdown. All output in English.",
		}

	case "factual":
		fallthrough
	default:
		return CategoryPrompt{
			System: "Answer the question accurately and directly in at most 4 short sentences. " +
				"No hedging ('I think', 'probably'). If asked to explain simply, use plain language. " + terse,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
