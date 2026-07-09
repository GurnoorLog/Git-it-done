package solvers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// NEREntities is the canonical structured NER output.
type NEREntities struct {
	Persons       []string `json:"persons"`
	Organizations []string `json:"organizations"`
	Locations     []string `json:"locations"`
}

// ParseNERJSON parses a model's JSON output into NEREntities, tolerating
// surrounding text and common alternative key names.
func ParseNERJSON(s string) (NEREntities, bool) {
	s = strings.TrimSpace(s)
	// Trim to outermost JSON object if extra text surrounds it.
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return NEREntities{}, false
	}
	var out NEREntities
	get := func(keys ...string) []string {
		for _, k := range keys {
			for rk, rv := range raw {
				if strings.EqualFold(rk, k) {
					var arr []string
					if err := json.Unmarshal(rv, &arr); err == nil {
						return arr
					}
				}
			}
		}
		return nil
	}
	out.Persons = get("persons", "people", "person")
	out.Organizations = get("organizations", "companies", "orgs", "organisation", "organisations")
	out.Locations = get("locations", "places", "location")
	if len(out.Persons)+len(out.Organizations)+len(out.Locations) == 0 {
		return out, false
	}
	return out, true
}

// capitalised-token stopwords: common sentence-position words that are never
// entities on their own.
var nerStopCaps = map[string]bool{
	"The": true, "A": true, "An": true, "This": true, "That": true, "These": true,
	"Those": true, "It": true, "He": true, "She": true, "They": true, "We": true,
	"I": true, "You": true, "If": true, "In": true, "On": true, "At": true,
	"By": true, "For": true, "With": true, "And": true, "But": true, "Or": true,
	"When": true, "While": true, "After": true, "Before": true, "As": true,
	"Is": true, "Are": true, "Was": true, "Were": true, "What": true, "Who": true,
	"Which": true, "How": true, "Why": true, "Where": true, "Its": true,
	"His": true, "Her": true, "Their": true, "Our": true, "My": true, "To": true,
	"Of": true, "From": true, "Not": true, "So": true, "Then": true, "There": true,
}

var reCapWord = regexp.MustCompile(`^[A-Z][A-Za-z'&.-]*$`)

// VerifyNER deterministically validates extracted entities against the source:
//  1. every extracted entity must appear verbatim in the source text;
//  2. every capitalised token in the source (excluding stopwords and likely
//     non-entity sentence starters) must be accounted for by some entity.
// Any failure means the extraction cannot be trusted → escalate.
func VerifyNER(source string, e NEREntities) bool {
	all := append(append(append([]string{}, e.Persons...), e.Organizations...), e.Locations...)
	if len(all) == 0 {
		return false
	}
	for _, ent := range all {
		ent = strings.TrimSpace(ent)
		if ent == "" || !strings.Contains(source, ent) {
			return false
		}
	}

	inAnyEntity := func(tok string) bool {
		for _, ent := range all {
			if strings.Contains(ent, tok) {
				return true
			}
		}
		return false
	}

	tokens := strings.Fields(source)
	sentenceStart := true
	for i, rawTok := range tokens {
		tok := strings.Trim(rawTok, `"'.,:;!?()[]{}`)
		wasStart := sentenceStart
		sentenceStart = strings.ContainsAny(rawTok, ".!?")
		if tok == "" || !reCapWord.MatchString(tok) || nerStopCaps[tok] {
			continue
		}
		if inAnyEntity(tok) {
			continue
		}
		// Sentence-initial capitalised word NOT followed by another capitalised
		// word is most likely just a sentence starter — allow it.
		if wasStart {
			nextCap := false
			if i+1 < len(tokens) {
				next := strings.Trim(tokens[i+1], `"'.,:;!?()[]{}`)
				nextCap = next != "" && reCapWord.MatchString(next) && !nerStopCaps[next]
			}
			if !nextCap {
				continue
			}
		}
		return false // uncovered capitalised candidate — extraction likely missed it
	}
	return true
}

// ExtractNERSource returns the text to run extraction on: the longest quoted
// segment, else everything after the first colon, else the full prompt.
func ExtractNERSource(prompt string) string {
	return ExtractQuotedOrTail(prompt)
}

// FormatNERJSON renders entities as compact JSON.
func FormatNERJSON(e NEREntities) string {
	for _, p := range []*[]string{&e.Persons, &e.Organizations, &e.Locations} {
		if *p == nil {
			*p = []string{}
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(b)
}

// FormatNERSentence renders entities as a single natural-language sentence.
func FormatNERSentence(e NEREntities) string {
	var parts []string
	add := func(items []string, label string) {
		for _, it := range items {
			parts = append(parts, fmt.Sprintf("%s (%s)", it, label))
		}
	}
	add(e.Persons, "person")
	add(e.Organizations, "organization")
	add(e.Locations, "location")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return "The text mentions " + parts[0] + "."
	}
	return "The text mentions " + strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1] + "."
}

// WantsJSON reports whether the task explicitly asks for JSON output.
func WantsJSON(prompt string) bool {
	lower := strings.ToLower(prompt)
	return strings.Contains(lower, "json") || strings.Contains(lower, "as a dictionary") || strings.Contains(lower, "key-value")
}

// MentionsDates reports whether the task asks for date entities (our fixed
// three-field local pipeline cannot serve those — escalate).
func MentionsDates(prompt string) bool {
	lower := strings.ToLower(prompt)
	return strings.Contains(lower, "date")
}
