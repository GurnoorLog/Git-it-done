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
			System: "You are a precise math solver. Think step by step, then output exactly one line: the final numeric answer, a colon, and a brief explanation. Example: '90.00: 25% of 120 is 30, so 120 - 30 = 90.' " + terse,
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
			System: "You are a precise summarizer. " + terse + " Read the length/format constraint in the task and follow it EXACTLY: 'two sentences' means exactly 2 sentences; 'under 15 words' means fewer than 15 words; 'three bullet points' means exactly 3 bullets. Use only facts from the source text. Do not add opinions or interpretations.",
		}

	case "code_generation":
		return CategoryPrompt{
			System: "Write the requested code. Handle edge cases (empty input, single element, ties, punctuation, negative numbers) per the spec. " +
				"Output ONLY valid, runnable code. No markdown code fences. No explanation before or after. The first line must be code (or a code comment).",
		}

	case "code_debugging":
		return CategoryPrompt{
			System: "Find and fix the bug in the given code. First identify the bug precisely, then output the complete fixed code. " +
				"First line must be a comment naming the bug in one short phrase (e.g. '# BUG: off-by-one error in loop condition'). " +
				"All remaining lines are the complete fixed code. " +
				"Output ONLY valid code. No markdown code fences. No explanation before or after the code block.",
		}

	case "logical":
		return CategoryPrompt{
			System: "Solve the logic puzzle carefully. Think step by step, writing short reasoning lines. " +
				"The very last line MUST be exactly: 'Final answer: ...' with the complete answer to the question asked. " +
				"If the question asks for a name, order, or assignment, include all relevant details in the final answer. " +
				"No markdown. All output in English.",
		}

	case "factual":
		fallthrough
	default:
		return CategoryPrompt{
			System: "Answer the question accurately and completely. " +
				"If the question asks for a list, number, or specific fact, give the exact answer first, then a brief explanation if helpful. " +
				"No hedging ('I think', 'probably', 'I believe'). Respond with precise, factual information. " + terse,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
