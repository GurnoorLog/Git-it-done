package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"track1-agent/internal/task"
)

// Write marshals results to path and validates the output before returning.
// Any marshal or write failure returns an error so main can exit non-zero.
func Write(path string, results []task.Result) error {
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("output writer: failed to marshal results: %w", err)
	}

	// Validate the marshalled bytes round-trip correctly.
	if !json.Valid(data) {
		return fmt.Errorf("output writer: marshalled output is not valid JSON (this should never happen)")
	}

	// Ensure the output directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("output writer: cannot create output directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("output writer: cannot write %s: %w", path, err)
	}

	// Final sanity-check: re-read and re-validate the file on disk.
	written, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("output writer: cannot re-read %s for validation: %w", path, err)
	}
	if !json.Valid(written) {
		return fmt.Errorf("output writer: file on disk is not valid JSON after write")
	}

	return nil
}
