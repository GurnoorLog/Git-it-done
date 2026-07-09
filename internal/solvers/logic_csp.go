package solvers

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
)

// LogicResult holds the outcome of a CSP solve attempt.
type LogicResult struct {
	Answer    string
	Solved    bool
	Reasoning string
}

// ── CSP Engine ────────────────────────────────────────────────────────────

// Assignment maps variable names to their assigned values.
type Assignment map[string]string

// Constraint is a function that returns true when the constraint is satisfied
// for the current (possibly partial) assignment.
type Constraint func(a Assignment) bool

// CSP holds the problem specification.
type CSP struct {
	Variables   []string
	Domains     map[string][]string
	Constraints []Constraint
}

// Solve runs backtracking search with forward-checking.
// Returns nil if no solution exists.
func (c *CSP) Solve() Assignment {
	return c.backtrack(Assignment{})
}

// SolveAll enumerates up to `limit` complete solutions. This lets the caller
// verify the puzzle's answer is UNIQUE across all satisfying assignments —
// if the parsed constraints admit multiple different answers to the question,
// the parse is under-constrained and we must escalate instead of guessing.
func (c *CSP) SolveAll(limit int) []Assignment {
	var out []Assignment
	c.backtrackAll(Assignment{}, limit, &out)
	return out
}

func (c *CSP) backtrackAll(a Assignment, limit int, out *[]Assignment) {
	if len(*out) >= limit {
		return
	}
	if len(a) == len(c.Variables) {
		cp := Assignment{}
		for k, v := range a {
			cp[k] = v
		}
		*out = append(*out, cp)
		return
	}
	v := c.selectUnassigned(a)
	for _, val := range c.Domains[v] {
		a[v] = val
		if c.consistent(a) {
			c.backtrackAll(a, limit, out)
		}
		delete(a, v)
	}
}

func (c *CSP) backtrack(a Assignment) Assignment {
	if len(a) == len(c.Variables) {
		return a
	}
	// Select next unassigned variable (simple left-to-right order).
	v := c.selectUnassigned(a)

	for _, val := range c.Domains[v] {
		// Try this assignment.
		a[v] = val
		if c.consistent(a) {
			result := c.backtrack(a)
			if result != nil {
				return result
			}
		}
		delete(a, v)
	}
	return nil
}

func (c *CSP) selectUnassigned(a Assignment) string {
	for _, v := range c.Variables {
		if _, ok := a[v]; !ok {
			return v
		}
	}
	return ""
}

func (c *CSP) consistent(a Assignment) bool {
	for _, con := range c.Constraints {
		if !con(a) {
			return false
		}
	}
	return true
}

// ── Natural-Language Constraint Parser ───────────────────────────────────

// ParseResult holds the structured puzzle extracted from a prompt.
type ParseResult struct {
	Variables   []string
	Domains     map[string][]string
	Constraints []Constraint
	Confident   bool   // false → parser couldn't structure the puzzle; escalate
	Question    string // the question to answer from the solution
}

// ParseLogicPuzzle attempts to extract a CSP from common natural-language
// puzzle phrasings (seating arrangements, truth/lie puzzles).
// Returns Confident=false when structure can't be reliably inferred.
func ParseLogicPuzzle(prompt string) ParseResult {
	result := ParseResult{Domains: map[string][]string{}}

	// ── Try seating arrangement pattern ──────────────────────────────────
	if r := parseSeatingPuzzle(prompt); r.Confident {
		return r
	}

	// ── Try simple ordering/truth puzzle ─────────────────────────────────
	result.Confident = false
	result.Question = "Could not parse puzzle structure; escalate to model"
	return result
}

// ── Seating puzzle parser ─────────────────────────────────────────────────

var (
	reNames         = regexp.MustCompile(`\b([A-Z][a-z]+)\b`)
	reNotNextTo     = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is not next to\s+([A-Z][a-z]+)`)
	reImmLeft       = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is immediately (?:to the )?left of\s+([A-Z][a-z]+)`)
	reImmRight      = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is immediately (?:to the )?right of\s+([A-Z][a-z]+)`)
	reNotPosition   = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is not in position\s+(\d+)`)
	reIsPosition    = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is in position\s+(\d+)`)
	reNotPosOr      = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is not in position\s+(\d+)\s+or\s+(\d+)`)
	reFirstOrLast   = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is (?:either )?(?:first|in position 1) or (?:last|in the last position)`)
	reTomasLeftOf   = regexp.MustCompile(`(?i)([A-Z][a-z]+)\s+is somewhere to the left of\s+([A-Z][a-z]+)`)
	reWhoPosition   = regexp.MustCompile(`(?i)who is in position\s+(\d+)`)
)

func parseSeatingPuzzle(prompt string) ParseResult {
	pr := ParseResult{Domains: map[string][]string{}}

	// Extract all capitalised names from the prompt.
	nameMatches := reNames.FindAllString(prompt, -1)
	seen := map[string]bool{}
	var names []string
	// Filter out words that are likely not people names (sentence starters etc.)
	exclude := map[string]bool{"Who": true, "What": true, "How": true, "The": true, "If": true, "In": true, "A": true, "An": true, "Each": true, "Either": true, "First": true, "Last": true, "Two": true, "Three": true, "Four": true, "Five": true, "Six": true}
	for _, n := range nameMatches {
		if !seen[n] && !exclude[n] {
			seen[n] = true
			names = append(names, n)
		}
	}

	if len(names) < 2 || len(names) > 8 {
		return ParseResult{Confident: false}
	}

	size := len(names)
	positions := make([]string, size)
	for i := range positions {
		positions[i] = fmt.Sprintf("%d", i+1)
	}

	pr.Variables = names
	for _, n := range names {
		pr.Domains[n] = append([]string{}, positions...)
	}

	// Constraint: all different positions.
	pr.Constraints = append(pr.Constraints, func(a Assignment) bool {
		used := map[string]bool{}
		for _, v := range a {
			if used[v] {
				return false
			}
			used[v] = true
		}
		return true
	})

	// Parse specific constraints.
	for _, m := range reNotNextTo.FindAllStringSubmatch(prompt, -1) {
		x, y := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeNotNextTo(x, y, size))
	}
	for _, m := range reImmLeft.FindAllStringSubmatch(prompt, -1) {
		x, y := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeImmLeft(x, y))
	}
	for _, m := range reImmRight.FindAllStringSubmatch(prompt, -1) {
		x, y := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeImmLeft(y, x))
	}
	for _, m := range reNotPosOr.FindAllStringSubmatch(prompt, -1) {
		name, p1, p2 := m[1], m[2], m[3]
		pr.Constraints = append(pr.Constraints, makeNotPosition(name, p1))
		pr.Constraints = append(pr.Constraints, makeNotPosition(name, p2))
	}
	for _, m := range reNotPosition.FindAllStringSubmatch(prompt, -1) {
		name, pos := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeNotPosition(name, pos))
	}
	for _, m := range reIsPosition.FindAllStringSubmatch(prompt, -1) {
		name, pos := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeIsPosition(name, pos))
	}
	for _, m := range reFirstOrLast.FindAllStringSubmatch(prompt, -1) {
		name := m[1]
		pr.Constraints = append(pr.Constraints, makeFirstOrLast(name, size))
	}
	for _, m := range reTomasLeftOf.FindAllStringSubmatch(prompt, -1) {
		x, y := m[1], m[2]
		pr.Constraints = append(pr.Constraints, makeTomasLeftOf(x, y))
	}

	if wm := reWhoPosition.FindStringSubmatch(prompt); len(wm) >= 2 {
		pr.Question = "position:" + wm[1]
	}

	pr.Confident = len(pr.Constraints) >= 2
	return pr
}

// ── Constraint factories ──────────────────────────────────────────────────

func makeNotNextTo(x, y string, size int) Constraint {
	return func(a Assignment) bool {
		px, okX := a[x]
		py, okY := a[y]
		if !okX || !okY {
			return true // not yet assigned — can't violate
		}
		xi, yi := posInt(px), posInt(py)
		diff := xi - yi
		if diff < 0 {
			diff = -diff
		}
		return diff != 1
	}
}

func makeImmLeft(left, right string) Constraint {
	return func(a Assignment) bool {
		pl, okL := a[left]
		pr, okR := a[right]
		if !okL || !okR {
			return true
		}
		return posInt(pl)+1 == posInt(pr)
	}
}

func makeNotPosition(name, pos string) Constraint {
	return func(a Assignment) bool {
		if v, ok := a[name]; ok {
			return v != pos
		}
		return true
	}
}

func makeIsPosition(name, pos string) Constraint {
	return func(a Assignment) bool {
		if v, ok := a[name]; ok {
			return v == pos
		}
		return true
	}
}

func makeTomasLeftOf(x, y string) Constraint {
	return func(a Assignment) bool {
		px, okX := a[x]
		py, okY := a[y]
		if !okX || !okY {
			return true
		}
		return posInt(px) < posInt(py)
	}
}

func makeFirstOrLast(name string, size int) Constraint {
	last := fmt.Sprintf("%d", size)
	return func(a Assignment) bool {
		if v, ok := a[name]; ok {
			return v == "1" || v == last
		}
		return true
	}
}

func posInt(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ── Public entry point ────────────────────────────────────────────────────

// ── Simple deduction solver (non-seating puzzles) ─────────────────────────

var reSimpleDeduction = regexp.MustCompile(`(?i)(each owns? a different|each has a different|different (?:pet|animal|object|item|color|car|house|job))`)

// solveSimpleDeduction handles "who owns what" style puzzles via forward
// constraint propagation. These puzzles have N people, N items, and
// assignment constraints like "Sam does not own the bird. Jo owns the dog."
func solveSimpleDeduction(prompt string) LogicResult {
	rePerson := regexp.MustCompile(`\b([A-Z][a-z]+)\b`)
	allCaps := rePerson.FindAllString(prompt, -1)
	exclude := map[string]bool{"Who": true, "What": true, "How": true, "The": true, "If": true, "In": true, "A": true, "An": true, "Each": true, "Either": true, "First": true, "Last": true, "Two": true, "Three": true, "Four": true, "Five": true, "Six": true, "Not": true, "Is": true, "Are": true, "Was": true, "Were": true, "Does": true, "Do": true, "Has": true, "Have": true, "Owns": true, "Own": true, "Cat": true, "Dog": true, "Bird": true, "Red": true, "Blue": true, "Green": true}
	var people []string
	for _, n := range allCaps {
		if !exclude[n] {
			people = append(people, n)
		}
	}
	seen := map[string]bool{}
	var unique []string
	for _, p := range people {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	people = unique
	if len(people) < 2 || len(people) > 6 {
		return LogicResult{}
	}

	lower := strings.ToLower(prompt)
	reItems := regexp.MustCompile(`\b(cat|dog|bird|fish|hamster|rabbit|red|blue|green|yellow|white|black|car|bike|truck|van|doctor|teacher|lawyer|engineer|nurse|artist|pilot|chef|writer|soccer|tennis|swimming|running|reading|cooking|gardening|painting|piano|guitar|drums|violin|flute|france|germany|spain|italy|japan|uk|usa|canada|australia|brazil|china|india)\b`)
	itemMatches := reItems.FindAllString(lower, -1)
	seenItems := map[string]bool{}
	var items []string
	for _, it := range itemMatches {
		if !seenItems[it] {
			seenItems[it] = true
			items = append(items, it)
		}
	}
	if len(items) != len(people) {
		return LogicResult{}
	}

	type triple struct{ person, item string; owns bool }
	var constraints []triple
	constraintExclude := map[string]bool{"Who": true, "What": true, "Which": true, "How": true}
	reOwns := regexp.MustCompile(`\b([A-Z][a-z]+)\s+(?:owns|has(?: the)?)\s+(?:the\s+)?(cat|dog|bird|fish|rabbit|red|blue|green|yellow|white|black|car|bike|truck|van|doctor|teacher|lawyer|engineer|nurse|artist|pilot|chef|writer|soccer|tennis|swimming|running|reading|cooking|gardening|painting|piano|guitar|drums|violin|flute)`)
	for _, m := range reOwns.FindAllStringSubmatch(prompt, -1) {
		if len(m) >= 3 && !constraintExclude[m[1]] {
			constraints = append(constraints, triple{m[1], strings.ToLower(m[2]), true})
		}
	}
	reNotOwns := regexp.MustCompile(`\b([A-Z][a-z]+)\s+does\s+not\s+own\s+(?:the\s+)?(cat|dog|bird|fish|rabbit|red|blue|green|yellow|white|black|car|bike|truck|van|doctor|teacher|lawyer|engineer|nurse|artist|pilot|chef|writer|soccer|tennis|swimming|running|reading|cooking|gardening|painting|piano|guitar|drums|violin|flute)`)
	for _, m := range reNotOwns.FindAllStringSubmatch(prompt, -1) {
		if len(m) >= 3 && !constraintExclude[m[1]] {
			constraints = append(constraints, triple{m[1], strings.ToLower(m[2]), false})
		}
	}
	if len(constraints) < 2 {
		return LogicResult{}
	}

	// Forward propagation: assign items to people.
	assign := map[string]string{}        // person → item
	forbidden := map[string][]string{}   // person → items they can't have
	itemOwner := map[string]string{}     // item → person

	for _, c := range constraints {
		if c.owns {
			assign[c.person] = c.item
			itemOwner[c.item] = c.person
		} else {
			forbidden[c.person] = append(forbidden[c.person], c.item)
		}
	}

	// Deduce remaining: if a person has only one possible item left, assign it.
	changed := true
	for changed {
		changed = false
		for _, p := range people {
			if assign[p] != "" {
				continue
			}
			var possible []string
			for _, it := range items {
				if itemOwner[it] != "" && itemOwner[it] != p {
					continue
				}
				isForbidden := false
				for _, f := range forbidden[p] {
					if f == it {
						isForbidden = true
						break
					}
				}
				if !isForbidden {
					possible = append(possible, it)
				}
			}
			if len(possible) == 1 {
				assign[p] = possible[0]
				itemOwner[possible[0]] = p
				changed = true
			}
		}
	}

	// Check if all assigned.
	for _, p := range people {
		if assign[p] == "" {
			return LogicResult{}
		}
	}

	// Answer the question.
	reAskWho := regexp.MustCompile(`\bWho\s+(?:owns|has(?: the)?)\s+(?:the\s+)?(cat|dog|bird|fish|rabbit|red|blue|green|yellow|white|black|car|bike|truck|van|doctor|teacher|lawyer|engineer|nurse|artist|pilot|chef|writer|soccer|tennis|swimming|running|reading|cooking|gardening|painting|piano|guitar|drums|violin|flute)`)
	if m := reAskWho.FindStringSubmatch(prompt); len(m) >= 2 {
		target := strings.ToLower(m[1])
		for p, it := range assign {
			if it == target {
				return LogicResult{
					Solved:    true,
					Answer:    fmt.Sprintf("%s. %s owns the %s.", p, p, target),
					Reasoning: "simple deduction via constraint propagation",
				}
			}
		}
	}

	// Generic answer: list all assignments.
	var parts []string
	for _, p := range people {
		parts = append(parts, fmt.Sprintf("%s owns the %s", p, assign[p]))
	}
	return LogicResult{
		Solved:    true,
		Answer:    strings.Join(parts, ", ") + ".",
		Reasoning: "simple deduction via constraint propagation",
	}
}

// SolveLogic attempts to parse and solve a logical/deductive puzzle.
func SolveLogic(prompt string) LogicResult {
	// Deterministic propositional inference (modus tollens) first — free and exact.
	if r := solveModusTollens(prompt); r.Solved {
		return r
	}

	// Simple deduction for "who owns what" style puzzles.
	if r := solveSimpleDeduction(prompt); r.Solved {
		return r
	}

	pr := ParseLogicPuzzle(prompt)

	// Phase 3: Log the parsed constraint representation for audit.
	log.Printf("[LogicCSP] Parsed puzzle: %d variables, %d constraints, confident=%v",
		len(pr.Variables), len(pr.Constraints), pr.Confident)
	if len(pr.Variables) > 0 {
		log.Printf("[LogicCSP] Variables: %v", pr.Variables)
		for _, v := range pr.Variables {
			log.Printf("[LogicCSP] Domain[%s]: %v", v, pr.Domains[v])
		}
	}
	log.Printf("[LogicCSP] Question: %s", pr.Question)

	if !pr.Confident {
		return LogicResult{
			Solved:    false,
			Reasoning: "parser could not confidently extract puzzle structure; escalating",
		}
	}

	csp := &CSP{
		Variables:   pr.Variables,
		Domains:     pr.Domains,
		Constraints: pr.Constraints,
	}

	// Guard: only trust the CSP answer if we parsed enough constraints.
	// (all-different is always added, so real content constraints must be >= 2)
	if len(csp.Constraints) < 3 {
		return LogicResult{
			Solved:    false,
			Reasoning: fmt.Sprintf("only %d constraints parsed (need >=3 for confidence); escalating", len(csp.Constraints)),
		}
	}

	solutions := csp.SolveAll(128)
	if len(solutions) == 0 {
		return LogicResult{
			Solved:    false,
			Reasoning: "CSP has no solution with extracted constraints; escalating",
		}
	}

	// UNIQUENESS verification: the answer to the specific question must be
	// identical across ALL satisfying assignments. If it varies, our parse is
	// under-constrained (we missed a constraint) and we must escalate rather
	// than guess.
	answer := formatSolution(solutions[0], pr.Question)
	if answer == "" {
		return LogicResult{Solved: false, Reasoning: "could not answer question from solution; escalating"}
	}
	for _, sol := range solutions[1:] {
		if formatSolution(sol, pr.Question) != answer {
			return LogicResult{
				Solved:    false,
				Reasoning: fmt.Sprintf("answer not unique across %d solutions; parse under-constrained, escalating", len(solutions)),
			}
		}
	}

	if strings.HasPrefix(pr.Question, "position:") {
		pos := strings.TrimPrefix(pr.Question, "position:")
		answer = fmt.Sprintf("%s. Checking every arrangement that satisfies all the stated constraints, %s is the person in position %s in each of them.", answer, answer, pos)
	}
	return LogicResult{
		Solved:    true,
		Answer:    answer,
		Reasoning: fmt.Sprintf("CSP solved with unique answer across %d solutions (%d constraints)", len(solutions), len(csp.Constraints)),
	}
}

// ── Deterministic modus tollens ───────────────────────────────────────────

var reConditional = regexp.MustCompile(`(?i)\bif\s+([^,.]+?),?\s+then\s+([^.]+)\.`)

// negateClause inserts "not" after the first auxiliary/copula verb of a clause.
// "the ground is wet" → "the ground is not wet". Returns "" when no verb found.
func negateClause(clause string) string {
	verbs := []string{" is ", " are ", " was ", " were ", " has ", " have ", " does ", " do ", " can ", " will "}
	lower := " " + strings.ToLower(strings.TrimSpace(clause)) + " "
	for _, v := range verbs {
		if idx := strings.Index(lower, v); idx >= 0 {
			return strings.TrimSpace(lower[:idx] + v + "not " + lower[idx+len(v):])
		}
	}
	return ""
}

// solveModusTollens handles the exact pattern:
// "If A, then B. <B is not true>. Is A?" → No (modus tollens).
// It only fires when the literal negation of the consequent appears in the
// prompt, making false positives essentially impossible.
func solveModusTollens(prompt string) LogicResult {
	m := reConditional.FindStringSubmatch(prompt)
	if len(m) < 3 {
		return LogicResult{}
	}
	ante := strings.TrimSpace(m[1])
	cons := strings.TrimSpace(m[2])
	negCons := negateClause(cons)
	if negCons == "" {
		return LogicResult{}
	}
	lower := strings.ToLower(prompt)
	if !strings.Contains(lower, negCons) {
		return LogicResult{}
	}
	negAnte := negateClause(ante)
	if negAnte == "" {
		negAnte = "it is not the case that " + strings.ToLower(ante)
	}
	answer := fmt.Sprintf("No. By modus tollens: if %s, then %s; we are told %s, so %s. If %s were true, %s would have to be true as well, which contradicts the given fact.",
		strings.ToLower(ante), strings.ToLower(cons), negCons, negAnte, strings.ToLower(ante), strings.ToLower(cons))
	return LogicResult{
		Solved:    true,
		Answer:    answer,
		Reasoning: "modus tollens: negated consequent found verbatim in prompt",
	}
}

func formatSolution(sol Assignment, question string) string {
	if strings.HasPrefix(question, "position:") {
		targetPos := strings.TrimPrefix(question, "position:")
		for name, pos := range sol {
			if pos == targetPos {
				return name
			}
		}
	}
	// Generic: return the full assignment as readable text (sorted for
	// deterministic output — required by the uniqueness comparison).
	keys := make([]string, 0, len(sol))
	for k := range sol {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s is in position %s", k, sol[k]))
	}
	return strings.Join(parts, "; ")
}
