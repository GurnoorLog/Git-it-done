package task

import (
	"encoding/json"
	"fmt"
	"os"
)

// Load reads and parses /input/tasks.json into a slice of Task.
// On any error it logs a descriptive message to stderr and returns the error
// so the caller (main) can exit non-zero.
func Load(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("task loader: cannot read %s: %w", path, err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("task loader: %s contains invalid JSON", path)
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("task loader: failed to unmarshal tasks from %s: %w", path, err)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("task loader: %s contains zero tasks", path)
	}

	for i, t := range tasks {
		if t.TaskID == "" {
			return nil, fmt.Errorf("task loader: task at index %d has empty task_id", i)
		}
	}

	return tasks, nil
}
