package solvers

import (
	"fmt"
	"regexp"
	"strings"
)

// Deterministic sentiment lexicon used to cross-check the local model's label.
// A local answer is only accepted when the lexicon signal is unambiguous AND
// matches the local model's label. Anything ambiguous escalates to Fireworks.

var positiveWords = map[string]bool{
	"love": true, "loved": true, "loves": true, "great": true, "excellent": true,
	"amazing": true, "awesome": true, "fantastic": true, "wonderful": true,
	"best": true, "good": true, "happy": true, "delighted": true, "delightful": true,
	"perfect": true, "enjoyed": true, "enjoy": true, "impressive": true,
	"brilliant": true, "superb": true, "outstanding": true, "pleased": true,
	"satisfied": true, "recommend": true, "beautiful": true, "helpful": true,
	"friendly": true, "delicious": true, "incredible": true, "exceeded": true,
	"flawless": true, "smooth": true, "fast": true, "thrilled": true,
}

var negativeWords = map[string]bool{
	"terrible": true, "awful": true, "horrible": true, "bad": true, "worst": true,
	"hate": true, "hated": true, "hates": true, "rude": true, "poor": true,
	"disappointing": true, "disappointed": true, "disappointment": true,
	"broken": true, "useless": true, "waste": true, "wasted": true,
	"annoying": true, "frustrating": true, "frustrated": true, "unacceptable": true,
	"dirty": true, "slow": true, "mediocre": true, "refund": true, "unusable": true,
	"crashed": true, "crashes": true, "failure": true, "failed": true,
	"pathetic": true, "disgusting": true, "overpriced": true, "regret": true,
}

var negators = map[string]bool{
	"not": true, "never": true, "no": true, "hardly": true, "barely": true,
	"isn't": true, "wasn't": true, "aren't": true, "don't": true, "didn't": true,
	"doesn't": true, "won't": true, "can't": true, "couldn't": true,
}

var reWordToken = regexp.MustCompile(`[A-Za-z']+`)

// LexiconSentiment scores text against the polarity lexicon.
// confident is true only when one polarity has >=1 hit and the other has zero —
// a clear, unambiguous signal. hits contains the matched words (for the
// templated justification).
func LexiconSentiment(text string) (label string, hits []string, confident bool) {
	words := reWordToken.FindAllString(strings.ToLower(text), -1)
	var posHits, negHits []string
	for i, w := range words {
		negated := i > 0 && negators[words[i-1]]
		switch {
		case positiveWords[w]:
			if negated {
				negHits = append(negHits, "not "+w)
			} else {
				posHits = append(posHits, w)
			}
		case negativeWords[w]:
			if negated {
				posHits = append(posHits, "not "+w)
			} else {
				negHits = append(negHits, w)
			}
		}
	}
	switch {
	case len(posHits) >= 1 && len(negHits) == 0:
		return "positive", posHits, true
	case len(negHits) >= 1 && len(posHits) == 0:
		return "negative", negHits, true
	case len(posHits) == 0 && len(negHits) == 0:
		return "neutral", nil, false // no signal — do not trust
	default:
		return "", nil, false // mixed signal — escalate
	}
}

// ParseSentimentLabel extracts a canonical sentiment label from model output.
// Returns "" if no unambiguous label is found.
func ParseSentimentLabel(s string) string {
	lower := strings.ToLower(s)
	found := ""
	for _, l := range []string{"positive", "negative", "neutral"} {
		if strings.Contains(lower, l) {
			if found != "" {
				return "" // multiple labels mentioned — ambiguous
			}
			found = l
		}
	}
	return found
}

// BuildSentimentAnswer produces a judge-friendly one-line answer:
// "<label>. <one-sentence justification citing the strongest cue words>"
func BuildSentimentAnswer(label string, hits []string) string {
	if len(hits) == 0 {
		return fmt.Sprintf("%s. The text expresses a clearly %s tone overall.", label, label)
	}
	max := len(hits)
	if max > 3 {
		max = 3
	}
	quoted := make([]string, 0, max)
	for _, h := range hits[:max] {
		quoted = append(quoted, "\""+h+"\"")
	}
	return fmt.Sprintf("%s. The sentiment is %s, conveyed by strongly %s language such as %s with no opposing cues.",
		label, label, label, strings.Join(quoted, " and "))
}

// ExtractQuotedOrTail returns the text being analysed: the longest quoted
// segment if present, otherwise everything after the last colon, otherwise
// the full prompt.
func ExtractQuotedOrTail(prompt string) string {
	best := ""
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`'([^']{10,})'`),
		regexp.MustCompile(`"([^"]{10,})"`),
		regexp.MustCompile("\u2018([^\u2019]{10,})\u2019"),
		regexp.MustCompile("\u201c([^\u201d]{10,})\u201d"),
	} {
		for _, m := range re.FindAllStringSubmatch(prompt, -1) {
			if len(m[1]) > len(best) {
				best = m[1]
			}
		}
	}
	if best != "" {
		return best
	}
	if idx := strings.Index(prompt, ":"); idx >= 0 && idx < len(prompt)-10 {
		return strings.TrimSpace(prompt[idx+1:])
	}
	return prompt
}
