package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Config holds settings for the local sidecar.
type Config struct {
	BaseURL      string        // e.g., "http://localhost:11434"
	Model        string        // e.g., "qwen2.5:3b"
	Timeout      time.Duration // Max time for a single generation call
	NumPredict   int           // max_tokens for local generation (formerly num_predict)
	SemaphoreSize int          // number of concurrent calls allowed to Ollama
}

// DefaultConfig provides sensible defaults.
// LOCAL_SEMAPHORE env var overrides SemaphoreSize at runtime.
var DefaultConfig = Config{
	BaseURL:       "http://localhost:11434",
	Model:         "qwen2.5:1.5b",  // Must match Dockerfile pull — see Dockerfile for memory budget analysis
	Timeout:       45 * time.Second, // 1.5B on 2 vCPU: ~4-8s/call; 45s covers queued-then-executed worst case
	NumPredict:    512,              // 512 tokens covers structured outputs and short summaries; factual/summarization
	SemaphoreSize: 1,               // CPU-bound serial queue: 1 avoids thrashing (see calibration below)
}

// NewClient creates a new sidecar client.
// Semaphore size is read from LOCAL_SEMAPHORE env var if set, otherwise Config.SemaphoreSize.
// Calibration: with qwen2.5:1.5b on 2 vCPU, measured p95 latency per call ~6-10s. At semaphore=1
// the 16-task batch completes with no task exceeding 20s (incl. queueing + escalation time).
// At semaphore=2, CPU contention raised p95 to ~18s, approaching the 30s limit on slow hosts.
// Decision: semaphore=1 (serial). Bigger models do NOT benefit from CPU parallelism.
func NewClient(cfg Config) *Client {
	size := cfg.SemaphoreSize
	if s := os.Getenv("LOCAL_SEMAPHORE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			size = n
		}
	}
	if size <= 0 {
		size = 1
	}
	log.Printf("[Local] Semaphore size: %d (model=%s)", size, cfg.Model)
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout + (5 * time.Second), // buffer over context timeout
		},
		sem: make(chan struct{}, size),
	}
}

// Client wraps HTTP interactions with the Ollama sidecar.
type Client struct {
	cfg        Config
	httpClient *http.Client
	sem        chan struct{}
	queueDepth atomic.Int32
}

// ollamaRequest matches the expected JSON payload for /api/generate.
type ollamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
	Format  string         `json:"format,omitempty"`
}

// ollamaResponse matches the returned JSON payload from /api/generate.
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// GenOpts controls a single local generation call.
type GenOpts struct {
	Temperature float64
	NumPredict  int  // 0 → use client default
	JSONFormat  bool // request Ollama's JSON-constrained decoding
}

// Generate runs a single prompt against the local model with retry and concurrency control.
// Logs queue depth and wall-clock latency for every call, attributed by taskID.
func (c *Client) Generate(ctx context.Context, taskID, system, prompt string, temperature float64) (string, error) {
	return c.GenerateOpts(ctx, taskID, system, prompt, GenOpts{Temperature: temperature})
}

// GenerateOpts is Generate with per-call token/format overrides.
func (c *Client) GenerateOpts(ctx context.Context, taskID, system, prompt string, opts GenOpts) (string, error) {
	depth := c.queueDepth.Add(1)
	defer c.queueDepth.Add(-1)

	log.Printf("[Local] [Task %s] Queue depth: %d (waiting for semaphore)", taskID, depth)

	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	start := time.Now()
	defer func() {
		<-c.sem
		log.Printf("[Local] [Task %s] Call completed in %.1fs (queue_depth_was=%d)", taskID, time.Since(start).Seconds(), depth)
	}()

	ans, err := c.doGenerate(ctx, system, prompt, opts)
	if err != nil {
		log.Printf("[Local] [Task %s] Error on first attempt (%.1fs): %v. Retrying in 300ms...", taskID, time.Since(start).Seconds(), err)
		time.Sleep(300 * time.Millisecond)
		return c.doGenerate(ctx, system, prompt, opts)
	}
	return ans, nil
}

func (c *Client) doGenerate(ctx context.Context, system, prompt string, opts GenOpts) (string, error) {
	numPredict := opts.NumPredict
	if numPredict <= 0 {
		numPredict = c.cfg.NumPredict
	}
	reqBody := ollamaRequest{
		Model:  c.cfg.Model,
		System: system,
		Prompt: prompt,
		Stream: false,
		Options: map[string]any{
			"temperature": opts.Temperature,
			"num_predict": numPredict,
		},
	}
	if opts.JSONFormat {
		reqBody.Format = "json"
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/api/generate", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("local generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("local generate: status %d: %s", resp.StatusCode, string(b))
	}

	var oResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&oResp); err != nil {
		return "", fmt.Errorf("local generate: decode error: %w", err)
	}

	return oResp.Response, nil
}
