package solvers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// LintResult holds the outcome of a static analysis pass.
type LintResult struct {
	Language     string
	Findings     []string // human-readable issue descriptions
	AutoFixed    bool     // true if the fix is confident enough to use directly
	FixedCode    string   // non-empty when AutoFixed=true
	ExtraContext string   // appended to model prompt when AutoFixed=false
}

// LangDetect returns the programming language detected in a prompt.
// Returns "" if no code block can be identified.
func LangDetect(prompt string) string {
	// Check explicit fenced language tag: ```python, ```go, etc.
	reFenced := regexp.MustCompile("(?s)```([a-zA-Z]+)\\n")
	if m := reFenced.FindStringSubmatch(prompt); len(m) >= 2 {
		return strings.ToLower(m[1])
	}

	// Heuristic: detect by characteristic tokens.
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "def ") && strings.Contains(lower, "import "):
		return "python"
	case strings.Contains(lower, "func ") && strings.Contains(lower, "package "):
		return "go"
	case strings.Contains(lower, "function") && (strings.Contains(lower, "const ") || strings.Contains(lower, "let ") || strings.Contains(lower, "var ")):
		return "javascript"
	case strings.Contains(lower, "public static") || strings.Contains(lower, "public class"):
		return "java"
	case strings.Contains(lower, "#include"):
		return "c"
	}
	return ""
}

// ExtractCodeBlock pulls the first fenced code block from a prompt.
func ExtractCodeBlock(prompt string) string {
	reFenced := regexp.MustCompile("(?s)```[a-zA-Z]*\\n(.*?)```")
	if m := reFenced.FindStringSubmatch(prompt); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// StaticAnalyze runs a static analysis pass on the code block embedded in
// a prompt. Returns findings that can be appended as context to a model call,
// or an auto-fixed answer when confidence is high enough to skip escalation.
func StaticAnalyze(prompt string) LintResult {
	lang := LangDetect(prompt)
	code := ExtractCodeBlock(prompt)

	result := LintResult{Language: lang}
	if code == "" || lang == "" {
		result.Findings = []string{"Could not extract code block or detect language"}
		return result
	}

	switch lang {
	case "go":
		return analyzeGo(code, result)
	case "python":
		return analyzePython(code, result)
	case "javascript", "js":
		return analyzeJS(code, result)
	default:
		return analyzeGeneric(code, lang, result)
	}
}

// ── Go static analysis: shell out to go vet ──────────────────────────────

func analyzeGo(code string, result LintResult) LintResult {
	// Write snippet to temp file.
	dir, err := os.MkdirTemp("", "lint-go-*")
	if err != nil {
		result.Findings = []string{"could not create temp dir for go vet"}
		return result
	}
	defer os.RemoveAll(dir)

	// Wrap snippet in a minimal package.
	src := fmt.Sprintf("package main\n\n%s", code)
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		result.Findings = []string{"could not write go snippet for analysis"}
		return result
	}

	// Try go vet.
	cmd := exec.Command("go", "vet", srcPath)
	out, _ := cmd.CombinedOutput()
	if len(out) > 0 {
		result.Findings = append(result.Findings, "go vet: "+strings.TrimSpace(string(out)))
	}

	// Pattern checks even if vet passes.
	result.Findings = append(result.Findings, goPatternChecks(code)...)

	result.ExtraContext = buildContext(result.Findings)
	return result
}

func goPatternChecks(code string) []string {
	var issues []string
	if regexp.MustCompile(`\bfor\s+\w+\s*:=\s*0;[^;]+;\s*\w+\+\+`).MatchString(code) &&
		regexp.MustCompile(`\[\w+\+1\]`).MatchString(code) {
		issues = append(issues, "Possible off-by-one: loop accesses index+1 which may go out of bounds")
	}
	if regexp.MustCompile(`var\s+\w+\s*\[\]\w+\b`).MatchString(code) &&
		regexp.MustCompile(`\w+\[0\]`).MatchString(code) {
		issues = append(issues, "Possible nil slice dereference: indexing a nil/empty slice")
	}
	return issues
}

// ── Python pattern analysis ───────────────────────────────────────────────

func analyzePython(code string, result LintResult) LintResult {
	var findings []string

	// Division by zero risk.
	if regexp.MustCompile(`/\s*\w+`).MatchString(code) && !strings.Contains(code, "!= 0") && !strings.Contains(code, "if b") {
		if regexp.MustCompile(`def \w+\([^)]*\):\s*\n\s+return \w+ / \w+`).MatchString(code) {
			findings = append(findings, "Possible ZeroDivisionError: divisor is not checked before division")
		}
	}

	// Off-by-one in range.
	reRange := regexp.MustCompile(`range\(len\(\w+\)\)`)
	reIndexPlus := regexp.MustCompile(`\[\w+\+1\]`)
	if reRange.MatchString(code) && reIndexPlus.MatchString(code) {
		findings = append(findings, "Possible off-by-one: range(len(x)) but accessing index+1 is out of bounds on last iteration")
	}

	// Infinite recursion: recursive call with same args.
	reFuncDef := regexp.MustCompile(`def (\w+)\((\w+)\):`)
	m := reFuncDef.FindStringSubmatch(code)
	if len(m) >= 3 {
		funcName, paramName := m[1], m[2]
		reRecurse := regexp.MustCompile(funcName + `\(` + paramName + `\)`)
		if reRecurse.MatchString(code) {
			findings = append(findings, fmt.Sprintf("Possible infinite recursion: %s() calls itself with the same argument %s", funcName, paramName))
		}
	}

	// Missing return value.
	if regexp.MustCompile(`def \w+\([^)]*\):`).MatchString(code) &&
		!strings.Contains(code, "return ") {
		findings = append(findings, "Function body has no return statement — caller will receive None")
	}

	// Attribute access on None.
	if strings.Contains(code, "= None") &&
		regexp.MustCompile(`\w+\.\w+\(`).MatchString(code) {
		findings = append(findings, "Possible AttributeError: variable may be None when method is called")
	}

	result.Findings = findings
	result.ExtraContext = buildContext(findings)
	return result
}

// ── JavaScript pattern analysis ───────────────────────────────────────────

func analyzeJS(code string, result LintResult) LintResult {
	var findings []string

	if !strings.Contains(code, "return ") &&
		regexp.MustCompile(`function \w+\(`).MatchString(code) {
		findings = append(findings, "Function has no return statement — will return undefined")
	}

	if strings.Contains(code, "== null") || strings.Contains(code, "== undefined") {
		findings = append(findings, "Use === for strict equality; == null matches both null and undefined which may be unintended")
	}

	if regexp.MustCompile(`\w+\.\w+`).MatchString(code) && strings.Contains(code, "null") {
		findings = append(findings, "Possible null/undefined dereference — check object is defined before accessing properties")
	}

	result.Findings = findings
	result.ExtraContext = buildContext(findings)
	return result
}

// ── Generic pattern analysis ──────────────────────────────────────────────

func analyzeGeneric(code, lang string, result LintResult) LintResult {
	var findings []string
	findings = append(findings, fmt.Sprintf("Language %q detected; pattern-based checks only", lang))

	// Common: unclosed string literal.
	singleCount := strings.Count(code, "'")
	doubleCount := strings.Count(code, "\"")
	if singleCount%2 != 0 {
		findings = append(findings, "Odd number of single-quotes — possible unclosed string literal")
	}
	if doubleCount%2 != 0 {
		findings = append(findings, "Odd number of double-quotes — possible unclosed string literal")
	}

	result.Findings = findings
	result.ExtraContext = buildContext(findings)
	return result
}

func buildContext(findings []string) string {
	if len(findings) == 0 {
		return ""
	}
	return "Static analysis findings (use as debugging context):\n- " + strings.Join(findings, "\n- ")
}
