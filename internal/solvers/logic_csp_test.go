package solvers

import (
	"strings"
	"testing"
)

type logicTest struct {
	prompt  string
	wantSolved bool
	checkFn func(answer string) bool // nil = don't check answer content
}

var logicTests = []logicTest{
	{
		// These puzzles have multiple valid arrangements given the parsed constraints;
		// the CSP correctly detects non-uniqueness and escalates.
		prompt: "Alice, Bob, Carol, Dave, and Eve are seated in a row. Alice is not next to Bob. Carol is immediately to the left of Dave. Eve is either first or last. Bob is not in position 3. Who is in position 2?",
		wantSolved: false,
	},
	{
		prompt: "Five people — Ann, Ben, Cara, Dan, Ed — sit in a row. Ben is immediately to the left of Cara. Ann is not next to Dan. Ed is either first or last. Dan is not in position 1. Who is in position 3?",
		wantSolved: false,
	},
	{
		prompt: "Three people sit in a row: Xena, Yara, Zack. Xena is not next to Zack. Who is in position 2?",
		wantSolved: true,
		checkFn: func(a string) bool { return strings.HasPrefix(a, "Yara") },
	},
	{
		prompt: "Three friends, Sam, Jo, and Lee, each own a different pet: cat, dog, bird. Sam does not own the bird. Jo owns the dog. Who owns the cat?",
		wantSolved: true,
		checkFn: func(a string) bool { return strings.HasPrefix(a, "Sam") },
	},
	// Complex logic that the parser can't confidently handle → must escalate
	{
		prompt: "If all glibbers are floobers and some floobers are not wumbles, can we conclude that no glibbers are wumbles?",
		wantSolved: false,
	},
	{
		prompt: "A philosopher says: This statement is false. What is the truth value of the statement?",
		wantSolved: false,
	},
}

func TestLogicSolver(t *testing.T) {
	for _, tc := range logicTests {
		got := SolveLogic(tc.prompt)
		if got.Solved != tc.wantSolved {
			t.Errorf("FAIL Solved mismatch for %q\n  expected Solved=%v got Solved=%v (answer=%q, reasoning=%q)",
				truncate(tc.prompt, 80), tc.wantSolved, got.Solved, got.Answer, got.Reasoning)
			continue
		}
		if tc.wantSolved && tc.checkFn != nil && !tc.checkFn(got.Answer) {
			t.Errorf("FAIL answer check for %q\n  answer=%q did not pass checkFn", truncate(tc.prompt, 80), got.Answer)
		}
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
