package solvers

import (
	"strings"
	"unicode"
)

var framingPatterns = []string{
	"read the following", "read the text", "read the passage",
	"based on the text", "based on the given", "based on the above",
	"based on this", "given the following", "consider the following",
	"look at the following", "the following is", "below is",
	"please read", "please analyze", "please answer", "please solve",
	"please classify", "please extract", "please summarize",
	"your task is", "you are given", "you will be given",
	"i need you to", "i want you to",
	"output only the", "provide only the", "return only the",
	"give only the", "output the answer", "provide the answer",
	"be concise", "be precise", "be accurate",
	"do not include", "do not provide", "do not add",
	"ensure your answer", "make sure to", "make sure your",
	"in your response", "in the output",
	"extract all named entities", "extract the entities",
	"classify the sentiment", "determine the sentiment",
	"analyze the sentiment", "identify the sentiment",
	"summarize the text", "summarize the following",
	"summarize the passage", "summarize the given",
	"translate the following", "translate this",
	"answer the question", "answer the following",
	"solve the problem", "solve the following",
	"find the bug", "debug the following", "debug this code",
	"write a function", "write code", "write a program",
	"implement a", "implement the",
	"complete the function", "complete the code",
	"fix the code", "fix the bug",
}

var noStripIndicators = []string{
	"func ", "def ", "class ", "import ", "package ",
	"const ", "var ", "type ", "return ", "if ", "for ", "while ",
	"func(", "func (", "def(", "def (",
}

func CompressPrompt(prompt string) string {
	original := strings.TrimSpace(prompt)
	if len(original) < 60 {
		return original
	}

	lines := strings.Split(original, "\n")
	var kept []string
	var stripped int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			kept = append(kept, "")
			continue
		}

		if isFraming(trimmed) {
			stripped++
			continue
		}

		kept = append(kept, trimmed)
	}

	// If we stripped nothing, return original
	if stripped == 0 {
		return original
	}

	// If we stripped more than 80% of lines, it's probably wrong — return original
	if len(kept) > 0 && stripped*5 > len(lines)*4 {
		return original
	}

	// Collapse multiple blank lines to one
	var result []string
	prevBlank := false
	for _, line := range kept {
		if line == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
		} else {
			prevBlank = false
		}
		result = append(result, line)
	}

	compressed := strings.TrimSpace(strings.Join(result, "\n"))

	// If compression saved < 10%, not worth it — return original
	if len(original)-len(compressed) < len(original)/10 {
		return original
	}

	return compressed
}

func isFraming(line string) bool {
	lower := strings.ToLower(line)

	// Never strip short lines — they might be the actual question
	if len([]rune(lower)) < 15 {
		return false
	}

	// Never strip lines that look like code or data
	for _, ind := range noStripIndicators {
		if strings.Contains(lower, ind) {
			return false
		}
	}

	// Never strip lines with operators (likely math or logic)
	if containsMathOperators(line) {
		return false
	}

	// Check framing patterns
	for _, pat := range framingPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}

	return false
}

func containsMathOperators(s string) bool {
	for _, r := range s {
		if r == '+' || r == '=' || r == '*' || r == '/' || r == '%' {
			return true
		}
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
