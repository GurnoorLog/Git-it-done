package solvers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// BuildSmokeTest derives a deterministic Python test snippet for common code
// specs. Returns ok=false when the spec is not recognised (caller should then
// rely on Fireworks instead of an unverifiable local answer).
func BuildSmokeTest(prompt, code string) (script string, ok bool) {
	reDef := regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`)
	m := reDef.FindStringSubmatch(code)
	if len(m) < 2 {
		return "", false
	}
	fn := m[1]
	lower := strings.ToLower(prompt)

	var tests string
	switch {
	case strings.Contains(lower, "palindrome"):
		tests = fmt.Sprintf(`assert bool(%[1]s("A man, a plan, a canal: Panama")) == True
assert bool(%[1]s("racecar")) == True
assert bool(%[1]s("hello world")) == False`, fn)
	case strings.Contains(lower, "second largest") || strings.Contains(lower, "second-largest"):
		tests = fmt.Sprintf(`assert %[1]s([1, 5, 3, 9, 7]) == 7
assert %[1]s([3, 1, 2]) == 2
assert %[1]s([10, 20]) == 10`, fn)
	case strings.Contains(lower, "reverse") && strings.Contains(lower, "string"):
		tests = fmt.Sprintf(`assert %[1]s("abc") == "cba"
assert %[1]s("a") == "a"`, fn)
	case strings.Contains(lower, "factorial"):
		tests = fmt.Sprintf(`assert %[1]s(5) == 120
assert %[1]s(1) == 1`, fn)
	case strings.Contains(lower, "prime"):
		tests = fmt.Sprintf(`assert bool(%[1]s(7)) == True
assert bool(%[1]s(8)) == False
assert bool(%[1]s(2)) == True`, fn)
	case strings.Contains(lower, "anagram"):
		tests = fmt.Sprintf(`assert bool(%[1]s("listen", "silent")) == True
assert bool(%[1]s("abc", "abd")) == False`, fn)
	case strings.Contains(lower, "vowel"):
		tests = fmt.Sprintf(`assert %[1]s("hello") == 2
assert %[1]s("xyz") == 0`, fn)
	case strings.Contains(lower, "duplicate"):
		tests = fmt.Sprintf(`assert bool(%[1]s([1, 2, 3, 2])) != bool(%[1]s([1, 2, 3]))`, fn)
	default:
		return "", false
	}

	return code + "\n\n" + tests + "\nprint('SMOKE_OK')\n", true
}

// RunPython executes a Python script with a hard timeout.
// Returns true only on exit 0 with the SMOKE_OK marker printed.
func RunPython(ctx context.Context, script string) (bool, string) {
	tmp, err := os.CreateTemp("", "smoke_*.py")
	if err != nil {
		return false, err.Error()
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return false, err.Error()
	}
	tmp.Close()

	pyCmd := "python3"
	if _, err := exec.LookPath(pyCmd); err != nil {
		pyCmd = "python"
	}

	runCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(runCtx, pyCmd, tmp.Name()).CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out)) + " " + err.Error()
	}
	if !strings.Contains(string(out), "SMOKE_OK") {
		return false, "smoke marker missing"
	}
	return true, ""
}
