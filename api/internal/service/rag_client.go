package service

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RAGClient handles RAG query execution via simple_rag.py
type RAGClient struct {
	scriptPath string
	timeout    time.Duration
}

// RAGResult represents the result of a RAG query
type RAGResult struct {
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
	Model   string   `json:"model"`
}

// NewRAGClient creates a new RAG client
func NewRAGClient(baseDir string) *RAGClient {
	return &RAGClient{
		scriptPath: filepath.Join(baseDir, "scripts", "simple_rag.py"),
		timeout:    10 * time.Minute,
	}
}

// Query executes a RAG query and returns the result
func (c *RAGClient) Query(question string) (*RAGResult, error) {
	cmd := exec.Command("python3", c.scriptPath, "query", question, "--json")

	// Capture stdout and stderr separately
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run with timeout using a channel
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			stderrStr := stderr.String()
			// Check if Ollama is not running
			if strings.Contains(stderrStr, "Connection refused") ||
				strings.Contains(stderrStr, "Failed to query Ollama") ||
				strings.Contains(stderrStr, "ConnectionError") {
				return nil, fmt.Errorf("ollama_unavailable: %s", stderrStr)
			}
			return nil, fmt.Errorf("script_error: %s (stderr: %s)", err, stderrStr)
		}
	case <-time.After(c.timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("script_error: query timed out after %v", c.timeout)
	}

	// Parse JSON output from stdout
	// Filter out non-JSON lines (print statements from the script)
	output := strings.TrimSpace(stdout.String())
	lines := strings.Split(output, "\n")

	// Find the last line that looks like JSON
	var jsonLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			jsonLine = line
			break
		}
	}

	if jsonLine == "" {
		return nil, fmt.Errorf("script_error: no JSON output found in: %s", output)
	}

	var result RAGResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		return nil, fmt.Errorf("script_error: failed to parse JSON output: %w (raw: %s)", err, jsonLine)
	}

	return &result, nil
}
