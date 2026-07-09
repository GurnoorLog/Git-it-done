package task

// Task is the input unit read from /input/tasks.json.
type Task struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
}

// Result is the output unit written to /output/results.json.
type Result struct {
	TaskID         string `json:"task_id"`
	Answer         string `json:"answer"`
	ResolutionPath string `json:"-"`
	ModelUsed      string `json:"-"`
	TotalTokens    int    `json:"-"`
}
