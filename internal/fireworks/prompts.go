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

// GetPrompts returns the system prompt and token budget for a category.
// Prompts are tuned for LLM-judge evaluation: clear, complete, and semantically rich.
func GetPrompts(category, taskPrompt string) CategoryPrompt {
	wantsJSON := reJSON.MatchString(taskPrompt)

	switch category {
	case "math":
		return CategoryPrompt{
			// LLM judge rewards showing work — bare numbers score lower when context is missing.
			System: "You are a precise math solver. " +
				"Show a brief step-by-step calculation, then state the final answer clearly. " +
				"Format: calculation steps followed by 'Final answer: X'. " +
				"No markdown formatting.",
			MaxTokens: 1000,
		}

	case "sentiment":
		return CategoryPrompt{
			// The grader is an LLM that looks for the correct label AND justification.
			System: "You are a sentiment analyst. " +
				"First state the sentiment label (positive, negative, or neutral). " +
				"Then provide a one-sentence justification referencing specific words or phrases from the text. " +
				"Format: '[Label]. [Justification sentence].' " +
				"Example: 'Negative. The reviewer describes the food as terrible and the service as rude.' " +
				"No preamble.",
			MaxTokens: 150,
		}

	case "ner":
		if wantsJSON {
			return CategoryPrompt{
				// Task explicitly asks for JSON — output strict JSON.
				System: "Extract ALL named entities. Output ONLY a valid JSON object:\n" +
					`{"persons":["Name1"],"organizations":["Org1"],"locations":["Loc1"]}` + "\n" +
					"Use exact spans from text. Omit empty categories. No text before or after JSON.",
				Prefill:   `{"persons":[`,
				MaxTokens: 300,
			}
		}
		return CategoryPrompt{
			// Natural language NER for tasks that don't ask for JSON.
			System: "Extract all named entities from the text. " +
				"List them grouped by type: persons, organizations, and locations. " +
				"Format each group as 'Type: Name1, Name2'. " +
				"Example: 'Persons: Alice, Bob\nOrganizations: Acme Corp\nLocations: New York'. " +
				"If a category has no entities, omit it. No preamble.",
			MaxTokens: 400,
		}

	case "summarization":
		return CategoryPrompt{
			// Strictly obey any length constraint in the prompt.
			System: "You are a precise summarizer. " +
				"Summarize the given text while strictly obeying any length constraint stated (e.g. '2 sentences', 'one sentence', 'under 30 words', '3 bullet points'). " +
				"Use only information from the source text. " +
				"No preamble, no markdown formatting.",
			MaxTokens: 500,
		}

	case "code_generation":
		return CategoryPrompt{
			// Code specialist prompt: handle all edge cases, output clean code only.
			System: "Write the requested Python function or program. " +
				"Handle ALL edge cases explicitly (empty input, None, zero, negative numbers, duplicates, etc.). " +
				"Output ONLY valid, runnable Python code — no markdown code fences (```), no explanation outside of code comments. " +
				"The first line of output must be code or a # comment.",
			MaxTokens: 1500,
		}

	case "code_debugging":
		return CategoryPrompt{
			// Debugging: identify then fix. Start with a comment explaining the bug.
			System: "Find and fix the bug in the given code. " +
				"Output ONLY valid, runnable Python code. " +
				"Add a single comment at the top: '# BUG: [brief description of the bug]'. " +
				"Then output the complete corrected function or program. " +
				"No markdown code fences (```). No explanation outside of code comments.",
			MaxTokens: 1500,
		}

	case "logical":
		return CategoryPrompt{
			// Request concise reasoning to prevent infinite loops.
			System: "Solve the logic problem. " +
				"Work through the constraints concisely in under 150 words. " +
				"Do not repeat yourself. " +
				"End your response with 'Answer: [final answer]' on its own line. " +
				"No markdown.",
			MaxTokens: 800,
		}

	case "factual":
		return CategoryPrompt{
			// Factual: complete, accurate, direct. No hedging.
			System: "Answer the question directly and accurately. " +
				"Be comprehensive but concise. " +
				"If the question asks to explain a concept, provide a clear explanation. " +
				"If the question asks for a specific fact, state it directly. " +
				"No preamble, no hedging, no markdown formatting.",
			MaxTokens: 600,
		}

	default:
		return CategoryPrompt{
			System:    "Answer the question directly and accurately. No preamble, no markdown.",
			MaxTokens: 600,
		}
	}
}

func BuildPrompt(basePrompt, extraContext string) string {
	if extraContext == "" {
		return basePrompt
	}
	return strings.TrimSpace(basePrompt) + "\n\n" + extraContext
}
