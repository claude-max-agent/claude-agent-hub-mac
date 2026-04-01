package service

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CodexTask represents a task to be executed by Codex CLI
type CodexTask struct {
	ID         string     `json:"id"`
	Prompt     string     `json:"prompt"`
	WorkDir    string     `json:"work_dir"`
	Model      string     `json:"model,omitempty"`
	Status     string     `json:"status"` // pending, running, completed, failed
	Output     string     `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	TokensUsed int        `json:"tokens_used,omitempty"`
}

// TaskCompleteCallback is called when a task finishes
type TaskCompleteCallback func(task *CodexTask)

// CodexExecutor manages Codex CLI task execution
type CodexExecutor struct {
	mu         sync.RWMutex
	tasks      map[string]*CodexTask
	maxConc    int
	running    int
	taskChan   chan *CodexTask
	onComplete TaskCompleteCallback
}

// NewCodexExecutor creates a new Codex executor
func NewCodexExecutor(maxConcurrent int) *CodexExecutor {
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	e := &CodexExecutor{
		tasks:    make(map[string]*CodexTask),
		maxConc:  maxConcurrent,
		taskChan: make(chan *CodexTask, 20),
	}
	// Start worker goroutines
	for i := 0; i < maxConcurrent; i++ {
		go e.worker(i)
	}
	return e
}

// SetOnComplete sets the callback for task completion
func (e *CodexExecutor) SetOnComplete(cb TaskCompleteCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onComplete = cb
}

// Submit submits a task for Codex execution
func (e *CodexExecutor) Submit(id, prompt, workDir, model string) *CodexTask {
	task := &CodexTask{
		ID:      id,
		Prompt:  prompt,
		WorkDir: workDir,
		Model:   model,
		Status:  "pending",
	}

	e.mu.Lock()
	e.tasks[id] = task
	e.mu.Unlock()

	e.taskChan <- task
	log.Printf("[CodexExecutor] Task submitted: %s (workdir: %s)", id, workDir)
	return task
}

// GetTask returns a task by ID
func (e *CodexExecutor) GetTask(id string) *CodexTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tasks[id]
}

// ListTasks returns all tasks
func (e *CodexExecutor) ListTasks() []*CodexTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	tasks := make([]*CodexTask, 0, len(e.tasks))
	for _, t := range e.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// worker processes tasks from the channel
func (e *CodexExecutor) worker(workerID int) {
	for task := range e.taskChan {
		log.Printf("[CodexExecutor] Worker %d starting task: %s", workerID, task.ID)
		e.executeTask(task)
	}
}

// executeTask runs a single Codex CLI task
func (e *CodexExecutor) executeTask(task *CodexTask) {
	now := time.Now()
	e.mu.Lock()
	task.Status = "running"
	task.StartedAt = &now
	e.running++
	e.mu.Unlock()

	// Build codex exec command
	args := []string{"exec", "--full-auto", "--skip-git-repo-check"}
	if task.Model != "" {
		args = append(args, "-m", task.Model)
	}
	if task.WorkDir != "" {
		args = append(args, "-C", task.WorkDir)
	}
	args = append(args, task.Prompt)

	// Execute with 10 minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "codex", args...)
	output, err := cmd.CombinedOutput()

	endTime := time.Now()
	e.mu.Lock()
	task.EndedAt = &endTime
	task.Output = string(output)
	e.running--

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		log.Printf("[CodexExecutor] Task %s failed: %v", task.ID, err)
	} else {
		task.Status = "completed"
		task.TokensUsed = extractTokensUsed(string(output))
		log.Printf("[CodexExecutor] Task %s completed (tokens: %d)", task.ID, task.TokensUsed)
	}

	cb := e.onComplete
	e.mu.Unlock()

	// Fire callback outside lock
	if cb != nil {
		cb(task)
	}
}

// Stats returns executor statistics
func (e *CodexExecutor) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var pending, running, completed, failed int
	for _, t := range e.tasks {
		switch t.Status {
		case "pending":
			pending++
		case "running":
			running++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}

	return map[string]interface{}{
		"total":          len(e.tasks),
		"pending":        pending,
		"running":        running,
		"completed":      completed,
		"failed":         failed,
		"max_concurrent": e.maxConc,
	}
}

// extractTokensUsed parses token count from Codex output
func extractTokensUsed(output string) int {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "tokens used") && i+1 < len(lines) {
			tokenStr := strings.TrimSpace(lines[i+1])
			tokenStr = strings.ReplaceAll(tokenStr, ",", "")
			var tokens int
			fmt.Sscanf(tokenStr, "%d", &tokens)
			return tokens
		}
	}
	return 0
}

// extractLastMessage extracts the final assistant message from Codex output
func ExtractLastMessage(output string) string {
	lines := strings.Split(output, "\n")
	// Find the last "codex" section before "tokens used"
	lastCodexIdx := -1
	tokensIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "codex" {
			lastCodexIdx = i
		}
		if strings.Contains(trimmed, "tokens used") {
			tokensIdx = i
			break
		}
	}
	if lastCodexIdx >= 0 && tokensIdx > lastCodexIdx {
		msgLines := lines[lastCodexIdx+1 : tokensIdx]
		msg := strings.TrimSpace(strings.Join(msgLines, "\n"))
		if msg != "" {
			return msg
		}
	}
	// Fallback: return last non-empty line before "tokens used"
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.Contains(trimmed, "tokens used") && !strings.Contains(trimmed, "EXIT:") {
			return trimmed
		}
	}
	return ""
}
