package solvers

import (
	"strings"
	"testing"
)

type mathTest struct {
	prompt  string
	wantAns string // empty string means we just verify Solved==true
	wantSolved bool
}

var mathTests = []mathTest{
	// Percentage of
	{"What is 25% of 200?", "50.00", true},
	{"Calculate 15 percent of 80.", "12.00", true},
	{"Find 10% of 500.", "50.00", true},

	// Discount
	{"A jacket costs $120 with a 25% discount. What is the final price?", "$90.00", true},
	{"An item originally priced at $200 is 30% off. What do you pay?", "$140.00", true},

	// Raise/increase
	{"John earns $55,000 per year and gets a 10% raise. What is his new salary?", "$60,500.00", true},
	{"A price of $80 increases by 20%. What is the new price?", "$96.00", true},

	// Work rate
	{"3 workers can complete a job in 12 days. How many days will 9 workers take?", "4 days", true},
	{"6 workers finish a task in 10 days. How long will 15 workers take?", "4 days", true},

	// Growth projection
	{"A city population is 2 million growing at 3% per year. What will it be in 5 years?", "", true},

	// Unsolvable — must NOT solve
	{"What is the derivative of sin(x)?", "", false},
	{"Solve the system: 3x + 2y = 12, x - y = 1.", "", false},
}

func TestMathSolver(t *testing.T) {
	solved := 0
	notSolved := 0
	for _, tc := range mathTests {
		got := SolveMath(tc.prompt)
		if got.Solved != tc.wantSolved {
			t.Errorf("FAIL Solved mismatch for %q\n  expected Solved=%v got Solved=%v (answer=%q)",
				tc.prompt, tc.wantSolved, got.Solved, got.Answer)
			continue
		}
		if tc.wantSolved {
			solved++
		if tc.wantAns != "" {
			wantPrefix := tc.wantAns + ":"
			if !strings.HasPrefix(got.Answer, wantPrefix) && got.Answer != tc.wantAns {
				t.Errorf("FAIL answer mismatch for %q\n  expected_prefix=%q got=%q",
					tc.prompt, wantPrefix, got.Answer)
			}
		}
		} else {
			notSolved++
		}
	}
	t.Logf("Math solver: %d solved correctly, %d correctly declined", solved, notSolved)
}
