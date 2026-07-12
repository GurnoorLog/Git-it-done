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
	"os/exec"
	"time"
)

const serverURL = "http://127.0.0.1:8080/v1/chat/completions"

type Client struct {
	cmd    *exec.Cmd
	client *http.Client
	ready  bool
}

func New() *Client {
	return &Client{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Start(ctx context.Context) error {
	modelPath := os.Getenv("LOCAL_MODEL_PATH")
	if modelPath == "" {
		modelPath = "/app/model.gguf"
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Printf("[Local] Model not found at %s, local mode disabled", modelPath)
		return nil
	}

	c.cmd = exec.CommandContext(ctx, "llama-server",
		"-m", modelPath,
		"--host", "127.0.0.1",
		"--port", "8080",
		"-c", "2048",
		"--parallel", "1",
		"-ngl", "0",
	)
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}

	for i := 0; i < 60; i++ {
		resp, err := http.Get("http://127.0.0.1:8080/health")
		if err == nil {
			resp.Body.Close()
			c.ready = true
			log.Printf("[Local] llama-server ready on pid=%d", c.cmd.Process.Pid)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("llama-server did not become ready within 30s")
}

func (c *Client) Stop() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}

func (c *Client) IsReady() bool { return c.ready }

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) chat(ctx context.Context, msgs []chatMsg, maxTokens int) (string, error) {
	body := chatReq{
		Model:       "gemma-2-2b-it",
		Messages:    msgs,
		Temperature: 0.0,
		MaxTokens:   maxTokens,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, &buf)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llama request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}

func (c *Client) CompactPrompt(ctx context.Context, prompt string) (string, error) {
	msgs := []chatMsg{
		{Role: "system", Content: "Minimize the following prompt to its essential query. Remove conversational framing, pleasantries, and meta-commentary. Preserve all requirements, constraints, and formatting instructions. Output only the compressed task."},
		{Role: "user", Content: prompt},
	}
	return c.chat(ctx, msgs, 200)
}

func (c *Client) GenerateAnswer(ctx context.Context, system, prompt string, maxTokens int) (string, error) {
	msgs := []chatMsg{
		{Role: "system", Content: system},
		{Role: "user", Content: prompt},
	}
	return c.chat(ctx, msgs, maxTokens)
}
