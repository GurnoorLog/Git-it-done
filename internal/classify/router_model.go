package classify

import (
	"embed"
	"encoding/json"
	"math"
	"strings"
	"unicode"
)

//go:embed router_weights.json
var weightsFS embed.FS

type routerModel struct {
	Vocab     map[string]int       `json:"vocab"`
	IDF       map[string]float64   `json:"idf"`
	Coef      [][]float64          `json:"coef"`
	Intercept []float64            `json:"intercept"`
	Classes   []string             `json:"classes"`
}

var learnedRouter *routerModel

func init() {
	data, err := weightsFS.ReadFile("router_weights.json")
	if err != nil {
		return
	}
	var m routerModel
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	learnedRouter = &m
}

func (m *routerModel) predict(prompt string) string {
	tokens := tokenizeLearned(prompt)
	scores := make([]float64, len(m.Intercept))
	copy(scores, m.Intercept)
	seen := make(map[int]bool)
	for _, w := range tokens {
		idx, ok := m.Vocab[w]
		if !ok || seen[idx] {
			continue
		}
		seen[idx] = true
		idfVal := m.IDF[w]
		for c := range m.Classes {
			if idx < len(m.Coef[c]) {
				scores[c] += m.Coef[c][idx] * idfVal
			}
		}
	}
	best := 0
	for i := 1; i < len(scores); i++ {
		if scores[i] > scores[best] {
			best = i
		}
	}
	return m.Classes[best]
}

func tokenizeLearned(s string) []string {
	var tokens []string
	var buf strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(unicode.ToLower(r))
		} else {
			if buf.Len() > 0 {
				w := buf.String()
				if len(w) > 1 && !isNumeric(w) {
					tokens = append(tokens, w)
				}
				buf.Reset()
			}
		}
	}
	if buf.Len() > 0 {
		w := buf.String()
		if len(w) > 1 && !isNumeric(w) {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func softmax(scores []float64) []float64 {
	max := scores[0]
	for _, s := range scores[1:] {
		if s > max {
			max = s
		}
	}
	var sum float64
	probs := make([]float64, len(scores))
	for i, s := range scores {
		probs[i] = math.Exp(s - max)
		sum += probs[i]
	}
	for i := range probs {
		probs[i] /= sum
	}
	return probs
}
