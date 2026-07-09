package solvers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// VerifyCode syntax-checks the provided Python code in a subprocess.
// Returns true if the code passes py_compile. False + error message otherwise.
// We use syntax-check only (not full execution) to avoid false positives from
// code snippets that have no main entry point, use external deps, or are generators.
func VerifyCode(ctx context.Context, code string) (bool, string) {
	// Clean up markdown formatting if any snuck through.
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "```") {
		lines := strings.Split(code, "\n")
		if len(lines) > 2 {
			code = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	// Heuristic: if the content has no Python-like tokens, it is prose not code.
	// Prevents misclassified tasks (e.g., sentiment answer sent through code path) from
	// burning a retry slot on a spurious syntax failure.
	lowerCode := strings.ToLower(code)
	hasPythonToken := strings.Contains(lowerCode, "def ") ||
		strings.Contains(lowerCode, "import ") ||
		strings.Contains(lowerCode, "class ") ||
		strings.Contains(lowerCode, "return ") ||
		strings.Contains(lowerCode, "print(") ||
		strings.Contains(code, "# ")
	if !hasPythonToken {
		return false, "Response does not appear to be code (no Python keywords found)"
	}

	// Create a temporary file.
	tmpFile, err := os.CreateTemp("", "code_verify_*.py")
	if err != nil {
		return false, fmt.Sprintf("system error creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write code to file.
	if _, err := tmpFile.WriteString(code); err != nil {
		return false, fmt.Sprintf("system error writing temp file: %v", err)
	}
	tmpFile.Close()

	// Determine the python command to use.
	pyCmd := "python3"
	if _, err := exec.LookPath(pyCmd); err != nil {
		pyCmd = "python" // fallback for Windows/local environments
	}

	// Syntax check only — fast and avoids false positives from execution.
	cmd := exec.CommandContext(ctx, pyCmd, "-m", "py_compile", tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("Syntax error: %s", string(out))
	}

	return true, ""
}
