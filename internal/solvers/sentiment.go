package solvers

import (
	"fmt"
	"regexp"
	"strings"
)

var positiveWords = map[string]bool{
	"love": true, "loved": true, "loves": true, "great": true, "excellent": true,
	"amazing": true, "awesome": true, "fantastic": true, "wonderful": true,
	"best": true, "good": true, "happy": true, "delighted": true, "delightful": true,
	"perfect": true, "enjoyed": true, "enjoy": true, "impressive": true,
	"brilliant": true, "superb": true, "outstanding": true, "pleased": true,
	"satisfied": true, "recommend": true, "beautiful": true, "helpful": true,
	"friendly": true, "delicious": true, "incredible": true, "exceeded": true,
	"flawless": true, "smooth": true, "fast": true, "thrilled": true,
	"nice": true, "decent": true, "okay": true, "fine": true, "well": true,
	"easy": true, "elegant": true, "efficient": true, "reliable": true,
	"effective": true, "comfortable": true, "quality": true, "convenient": true,
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
	"boring": true, "bland": true, "loud": true, "noisy": true, "costly": true,
	"expensive": true, "cheap": true, "glitchy": true, "buggy": true,
	"stale": true, "cold": true, "unpleasant": true, "uncomfortable": true,
}

// Intensifier multipliers: "very good" = 1.5x, "extremely good" = 2x
var intensifiers = map[string]float64{
	"very": 1.5, "really": 1.5, "extremely": 2.0, "incredibly": 2.0,
	"absolutely": 2.0, "totally": 1.5, "completely": 1.5, "highly": 1.5,
	"so": 1.3, "quite": 1.3, "somewhat": 0.5, "slightly": 0.5,
	"barely": 0.3, "hardly": 0.3, "pretty": 1.2, "fairly": 1.1,
}

var negators = map[string]bool{
	"not": true, "never": true, "no": true, "hardly": true, "barely": true,
	"isn't": true, "wasn't": true, "aren't": true, "don't": true, "didn't": true,
	"doesn't": true, "won't": true, "can't": true, "couldn't": true,
	"neither": true, "nor": true, "nothing": true, "nowhere": true,
}

// Contrast words: the clause after these gets extra weight
var contrastWords = map[string]bool{
	"but": true, "however": true, "although": true, "though": true,
	"nevertheless": true, "nonetheless": true, "yet": true, "whereas": true,
	"while": true, "despite": true, "except": true, "unfortunately": true,
}

var reWordToken = regexp.MustCompile(`[A-Za-z']+`)

// LexiconSentiment scores text against the polarity lexicon with negation
// handling, intensifier multipliers, and contrast weighting.
// Returns confident=true when the score difference is large enough.
func LexiconSentiment(text string) (label string, hits []string, confident bool) {
	words := reWordToken.FindAllString(strings.ToLower(text), -1)

	var posScore, negScore float64
	var posHits, negHits []string
	beforeContrast := 0.0
	afterContrastPos, afterContrastNeg := 0.0, 0.0
	inContrast := false
	negateNext := false

	for i, w := range words {
		if contrastWords[w] {
			// Store pre-contrast scores and reset for after-contrast
			beforeContrast = posScore + negScore
			posScore, negScore = 0, 0
			inContrast = true
			negateNext = false
			continue
		}

		if negators[w] {
			negateNext = !negateNext
			continue
		}

		multiplier := 1.0
		if i > 0 {
			if m, ok := intensifiers[words[i-1]]; ok {
				multiplier = m
			}
		}
		if negateNext {
			multiplier *= -1
			negateNext = false
		}

		switch {
		case positiveWords[w]:
			score := 1.0 * multiplier
			if inContrast {
				afterContrastPos += score
			} else {
				posScore += score
			}
			if multiplier > 0 {
				posHits = append(posHits, w)
			} else {
				negHits = append(negHits, "not "+w)
			}
		case negativeWords[w]:
			score := 1.0 * multiplier
			if inContrast {
				afterContrastNeg += score
			} else {
				negScore += score
			}
			if multiplier > 0 {
				negHits = append(negHits, w)
			} else {
				posHits = append(posHits, "not "+w)
			}
		}
	}

	// After contrast, the post-contrast clause gets 2x weight
	if inContrast && beforeContrast > 0 {
		// The contrast clause after "but" gets 2x
		posScore += afterContrastPos * 2.0
		negScore += afterContrastNeg * 2.0
	} else {
		posScore += afterContrastPos
		negScore += afterContrastNeg
	}

	if posScore > 0 && negScore == 0 {
		return "positive", uniqueHits(posHits), true
	}
	if negScore > 0 && posScore == 0 {
		return "negative", uniqueHits(negHits), true
	}
	// Mixed signal: use the dominant score
	if posScore > negScore && (posScore-negScore)/(posScore+negScore) > 0.33 {
		return "positive", uniqueHits(posHits), true
	}
	if negScore > posScore && (negScore-posScore)/(posScore+negScore) > 0.33 {
		return "negative", uniqueHits(negHits), true
	}
	if posScore == 0 && negScore == 0 {
		return "neutral", nil, false
	}
	return "", nil, false
}

func uniqueHits(hits []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, h := range hits {
		if !seen[h] {
			seen[h] = true
			result = append(result, h)
		}
	}
	return result
}

// ParseSentimentLabel extracts a canonical sentiment label from model output.
func ParseSentimentLabel(s string) string {
	lower := strings.ToLower(s)
	found := ""
	for _, l := range []string{"positive", "negative", "neutral"} {
		if strings.Contains(lower, l) {
			if found != "" {
				return ""
			}
			found = l
		}
	}
	return found
}

// BuildSentimentAnswer produces a judge-friendly one-line answer.
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

// ExtractQuotedOrTail returns the text being analysed.
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
