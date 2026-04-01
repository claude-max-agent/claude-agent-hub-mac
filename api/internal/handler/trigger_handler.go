package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type TriggerJob struct {
	Label   string   `json:"label"`
	Command []string `json:"-"`
}

type JobState struct {
	Label      string  `json:"label"`
	Status     string  `json:"status"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	ExitCode   *int    `json:"exit_code"`
}

type TriggerHandler struct {
	mu       sync.RWMutex
	jobs     map[string]TriggerJob
	state    map[string]*JobState
	repoRoot string
}

func NewTriggerHandler(repoRoot string) *TriggerHandler {
	h := &TriggerHandler{
		repoRoot: repoRoot,
		jobs:     make(map[string]TriggerJob),
		state:    make(map[string]*JobState),
	}

	// Define trigger jobs for claude-agent-hub
	// These spawn tmp-agents for content creation tasks
	triggerScript := filepath.Join(repoRoot, "scripts", "trigger-agent.sh")

	h.jobs = map[string]TriggerJob{
		"blog-post": {
			Label:   "ブログ記事作成",
			Command: []string{"bash", triggerScript, "blog", "ブログ記事を作成して投稿してください"},
		},
		"ai-news": {
			Label:   "AIニュース収集・投稿",
			Command: []string{"bash", triggerScript, "ai-news", "最新のAIニュースを収集して記事を作成してください"},
		},
		"changelog-check": {
			Label:   "Claude Code CHANGELOG巡回",
			Command: []string{"bash", filepath.Join(repoRoot, "scripts", "cron-runner.sh"), "changelog-check", "bash", filepath.Join(repoRoot, "scripts", "changelog-check.sh")},
		},
	}

	// Initialize state for all jobs
	for name, job := range h.jobs {
		h.state[name] = &JobState{
			Label:  job.Label,
			Status: "idle",
		}
	}

	return h
}

func (h *TriggerHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.state)
}

func (h *TriggerHandler) HandleTrigger(w http.ResponseWriter, r *http.Request) {
	jobName := chi.URLParam(r, "jobName")

	h.mu.RLock()
	job, exists := h.jobs[jobName]
	state := h.state[jobName]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"unknown job: %s"}`, jobName), http.StatusNotFound)
		return
	}

	if state != nil && state.Status == "running" {
		http.Error(w, `{"error":"job already running"}`, http.StatusConflict)
		return
	}

	// Run job in background
	go h.runJob(jobName, job.Command)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"job":    jobName,
		"status": "started",
	})
}

func (h *TriggerHandler) HandleUpdateResources(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !req.Confirm {
		http.Error(w, `{"error":"confirmation required"}`, http.StatusBadRequest)
		return
	}

	go func() {
		log.Println("Updating resources: git pull")
		cmd := exec.Command("git", "pull", "origin", "main")
		cmd.Dir = h.repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("git pull failed: %v\n%s", err, out)
		} else {
			log.Printf("git pull succeeded: %s", out)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":     true,
		"status": "updating",
	})
}

func (h *TriggerHandler) runJob(jobName string, command []string) {
	now := time.Now().Format(time.RFC3339)

	h.mu.Lock()
	h.state[jobName] = &JobState{
		Label:     h.jobs[jobName].Label,
		Status:    "running",
		StartedAt: &now,
	}
	h.mu.Unlock()

	log.Printf("Trigger job started: %s", jobName)

	var exitCode int
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = h.repoRoot
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		log.Printf("Trigger job failed: %s (exit=%d, err=%v)", jobName, exitCode, err)
	}

	finished := time.Now().Format(time.RFC3339)
	status := "done"
	if exitCode != 0 {
		status = "error"
	}

	h.mu.Lock()
	h.state[jobName] = &JobState{
		Label:      h.jobs[jobName].Label,
		Status:     status,
		StartedAt:  &now,
		FinishedAt: &finished,
		ExitCode:   &exitCode,
	}
	h.mu.Unlock()

	log.Printf("Trigger job finished: %s (status=%s, exit=%d)", jobName, status, exitCode)
}
