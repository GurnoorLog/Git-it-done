package solvers

import (
	"regexp"
	"strconv"
	"strings"
)

// SummaryConstraint captures the explicit format constraint in a summarization task.
type SummaryConstraint struct {
	Sentences int // exact sentence count required (0 = unconstrained)
	MaxWords  int // word cap (0 = unconstrained)
	Bullets   int // exact bullet count required (0 = unconstrained)
}

var wordNums = map[string]int{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6}

var (
	reSentCount   = regexp.MustCompile(`(?i)\b(?:in|to)\s+(one|two|three|four|five|\d+)\s+sentences?\b`)
	reWordCap     = regexp.MustCompile(`(?i)\b(?:under|fewer than|less than|no more than|at most|maximum(?: of)?|within)\s+(\d+)\s+words?\b`)
	reBulletCount = regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|\d+)\s+bullet\s*points?\b`)
)

func parseNumWord(s string) int {
	s = strings.ToLower(s)
	if n, ok := wordNums[s]; ok {
		return n
	}
	n, _ := strconv.Atoi(s)
	return n
}

// ParseSummaryConstraint extracts explicit constraints from the task prompt.
func ParseSummaryConstraint(prompt string) SummaryConstraint {
	var c SummaryConstraint
	if m := reSentCount.FindStringSubmatch(prompt); len(m) >= 2 {
		c.Sentences = parseNumWord(m[1])
	}
	if m := reWordCap.FindStringSubmatch(prompt); len(m) >= 2 {
		c.MaxWords, _ = strconv.Atoi(m[1])
	}
	if m := reBulletCount.FindStringSubmatch(prompt); len(m) >= 2 {
		c.Bullets = parseNumWord(m[1])
	}
	return c
}

var reSentenceSplit = regexp.MustCompile(`[.!?]+(?:\s|$)`)

// CountSentences counts sentences in text (terminal punctuation groups).
func CountSentences(text string) int {
	t := strings.TrimSpace(text)
	if t == "" {
		return 0
	}
	n := len(reSentenceSplit.FindAllString(t, -1))
	if n == 0 {
		n = 1 // no terminal punctuation — treat as one sentence
	}
	return n
}

func countBullets(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "-") || strings.HasPrefix(l, "*") || strings.HasPrefix(l, "\u2022") ||
			regexp.MustCompile(`^\d+[.)]\s`).MatchString(l) {
			n++
		}
	}
	return n
}

var reContentWord = regexp.MustCompile(`[A-Za-z]{5,}`)

// VerifySummary deterministically validates a candidate summary against the
// source text and the parsed constraint. Returns false if anything is off —
// caller must escalate rather than ship an unverified summary.
func VerifySummary(source, summary string, c SummaryConstraint) bool {
	summary = strings.TrimSpace(summary)
	if summary == "" || len(summary) < 20 {
		return false
	}
	// A summary must actually be shorter than the source.
	if len(summary) >= len(source) {
		return false
	}
	if c.Sentences > 0 && CountSentences(summary) != c.Sentences {
		return false
	}
	if c.MaxWords > 0 && len(strings.Fields(summary)) >= c.MaxWords+1 {
		return false
	}
	if c.Bullets > 0 && countBullets(summary) != c.Bullets {
		return false
	}

	srcLower := strings.ToLower(source)

	// Hallucination guard: capitalised words in the summary (excluding sentence
	// starts) must exist in the source.
	tokens := strings.Fields(summary)
	sentenceStart := true
	for _, rawTok := range tokens {
		tok := strings.Trim(rawTok, `"'.,:;!?()[]{}`)
		wasStart := sentenceStart
		sentenceStart = strings.ContainsAny(rawTok, ".!?")
		if tok == "" || !reCapWord.MatchString(tok) || nerStopCaps[tok] || wasStart {
			continue
		}
		if !strings.Contains(srcLower, strings.ToLower(tok)) {
			return false
		}
	}

	// Content-overlap: at least 60% of the summary's content words must come
	// from the source (allowing simple suffix variation via 5-char prefix match).
	words := reContentWord.FindAllString(summary, -1)
	if len(words) < 4 {
		return false
	}
	matched := 0
	for _, w := range words {
		lw := strings.ToLower(w)
		if strings.Contains(srcLower, lw) || strings.Contains(srcLower, lw[:5]) {
			matched++
		}
	}
	return float64(matched)/float64(len(words)) >= 0.6
}

// ExtractSummarySource returns the passage to be summarised: text after the
// first colon if it is long enough, otherwise the full prompt.
func ExtractSummarySource(prompt string) string {
	if idx := strings.Index(prompt, ":"); idx >= 0 && len(prompt)-idx > 100 {
		return strings.TrimSpace(prompt[idx+1:])
	}
	return prompt
}
