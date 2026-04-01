package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zono819/claude-agent-hub/api/internal/service"
)

// CodexHandler handles Codex executor API requests
type CodexHandler struct {
	executor *service.CodexExecutor
}

// NewCodexHandler creates a new Codex handler
func NewCodexHandler(executor *service.CodexExecutor) *CodexHandler {
	return &CodexHandler{executor: executor}
}

// CodexSubmitRequest represents a task submission request
type CodexSubmitRequest struct {
	Prompt  string `json:"prompt"`
	WorkDir string `json:"work_dir,omitempty"`
	Model   string `json:"model,omitempty"`
}

// HandleSubmit handles POST /api/v1/codex/tasks
func (h *CodexHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	var req CodexSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, `{"error":"prompt is required"}`, http.StatusBadRequest)
		return
	}

	// Generate task ID
	taskID := fmt.Sprintf("codex-%d", time.Now().UnixMilli())

	task := h.executor.Submit(taskID, req.Prompt, req.WorkDir, req.Model)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(task)
}

// HandleGetTask handles GET /api/v1/codex/tasks/{id}
func (h *CodexHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	task := h.executor.GetTask(taskID)
	if task == nil {
		http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// HandleListTasks handles GET /api/v1/codex/tasks
func (h *CodexHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := h.executor.ListTasks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// HandleStats handles GET /api/v1/codex/stats
func (h *CodexHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.executor.Stats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
