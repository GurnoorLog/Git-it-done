package solvers

import (
	"strings"
	"testing"
)

var lintTests = []struct {
	name     string
	prompt   string
	wantLang string
	wantAny  []string // at least one of these must appear in findings
}{
	{
		name: "Python zero division",
		prompt: "Fix the bug:\n```python\ndef divide(a, b):\n    return a / b\n\nresult = divide(10, 0)\n```",
		wantLang: "python",
		wantAny:  []string{"ZeroDivisionError", "division"},
	},
	{
		name: "Python missing return",
		prompt: "Why does this return None?\n```python\ndef greet(name):\n    message = 'Hello ' + name\n```",
		wantLang: "python",
		wantAny:  []string{"return"},
	},
	{
		name: "Python infinite recursion",
		prompt: "This crashes:\n```python\ndef factorial(n):\n    if n == 1:\n        return 1\n    return n * factorial(n)\n```",
		wantLang: "python",
		wantAny:  []string{"recursion", "infinite"},
	},
	{
		name: "JS missing return",
		prompt: "Why does this return undefined?\n```javascript\nfunction greet(name) {\n    let message = 'Hello, ' + name;\n}\n```",
		wantLang: "javascript",
		wantAny:  []string{"return", "undefined"},
	},
	{
		name: "Go off-by-one",
		prompt: "Fix the Go code:\n```go\nfunc sum(nums []int) int {\n    total := 0\n    for i := 0; i < len(nums); i++ {\n        total += nums[i+1]\n    }\n    return total\n}\n```",
		wantLang: "go",
		wantAny:  []string{"off-by-one", "out of bounds", "vet", "index"},
	},
}

func TestLintStaticAnalysis(t *testing.T) {
	for _, tc := range lintTests {
		t.Run(tc.name, func(t *testing.T) {
			got := StaticAnalyze(tc.prompt)

			if got.Language != tc.wantLang {
				t.Errorf("language: expected %q, got %q", tc.wantLang, got.Language)
			}

			combined := strings.Join(got.Findings, " ") + " " + got.ExtraContext
			combined = strings.ToLower(combined)
			found := false
			for _, want := range tc.wantAny {
				if strings.Contains(combined, strings.ToLower(want)) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("findings don't contain any of %v\n  got: %v", tc.wantAny, got.Findings)
			}
		})
	}
}

func TestLangDetect(t *testing.T) {
	cases := []struct{ prompt, want string }{
		{"```python\nprint('hi')\n```", "python"},
		{"```go\nfunc main() {}\n```", "go"},
		{"```javascript\nconsole.log('hi')\n```", "javascript"},
		{"no code here at all", ""},
	}
	for _, tc := range cases {
		got := LangDetect(tc.prompt)
		if got != tc.want {
			t.Errorf("LangDetect(%q) = %q, want %q", tc.prompt, got, tc.want)
		}
	}
}
