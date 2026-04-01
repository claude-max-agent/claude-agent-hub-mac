package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zono819/claude-agent-hub/api/internal/database"
)

// CronHandler handles cron job CRUD operations
type CronHandler struct {
	repo *database.CronJobRepository
}

// NewCronHandler creates a new cron handler
func NewCronHandler(repo *database.CronJobRepository) *CronHandler {
	return &CronHandler{repo: repo}
}

// HandleList returns all cron jobs. Supports ?enabled=true query param.
func (h *CronHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	enabledOnly := r.URL.Query().Get("enabled") == "true"

	jobs, err := h.repo.List(enabledOnly)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs": jobs,
	})
}

// HandleGet returns a single cron job by ID.
func (h *CronHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	job, err := h.repo.GetByID(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "cron job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// cronCreateRequest represents the request body for creating a cron job
type cronCreateRequest struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CronExpression string `json:"cron_expression"`
	Prompt         string `json:"prompt"`
	RequiresAgent  bool   `json:"requires_agent"`
	Enabled        bool   `json:"enabled"`
}

// HandleCreate creates a new cron job.
func (h *CronHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req cronCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ID == "" || req.Name == "" || req.CronExpression == "" || req.Prompt == "" {
		http.Error(w, "id, name, cron_expression, and prompt are required", http.StatusBadRequest)
		return
	}

	job := &database.CronJob{
		ID:             req.ID,
		Name:           req.Name,
		CronExpression: req.CronExpression,
		Prompt:         req.Prompt,
		RequiresAgent:  req.RequiresAgent,
		Enabled:        req.Enabled,
	}

	if err := h.repo.Create(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// cronUpdateRequest represents the request body for updating a cron job.
// Pointer fields allow partial updates (nil = no change).
type cronUpdateRequest struct {
	Name           *string `json:"name"`
	CronExpression *string `json:"cron_expression"`
	Prompt         *string `json:"prompt"`
	RequiresAgent  *bool   `json:"requires_agent"`
	Enabled        *bool   `json:"enabled"`
}

// HandleUpdate updates an existing cron job with partial update support.
func (h *CronHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	job, err := h.repo.GetByID(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "cron job not found", http.StatusNotFound)
		return
	}

	var req cronUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Apply partial updates
	if req.Name != nil {
		job.Name = *req.Name
	}
	if req.CronExpression != nil {
		job.CronExpression = *req.CronExpression
	}
	if req.Prompt != nil {
		job.Prompt = *req.Prompt
	}
	if req.RequiresAgent != nil {
		job.RequiresAgent = *req.RequiresAgent
	}
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	}

	if err := h.repo.Update(job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// HandleDelete deletes a cron job by ID.
func (h *CronHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	if err := h.repo.Delete(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"id":     jobID,
	})
}
