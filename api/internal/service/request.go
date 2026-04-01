package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/zono819/claude-agent-hub/api/internal/database"
)

// FeatureRequest represents a feature/improvement request
type FeatureRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"` // high, medium, low
	Status      string `json:"status"`   // pending, approved, rejected, completed
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	TaskID      string `json:"task_id,omitempty"` // If converted to task
}

// RequestService manages feature requests using SQLite
type RequestService struct {
	requestRepo *database.RequestRepository
	taskRepo    *database.TaskRepository
}

// NewRequestService creates a new request service with database repositories
func NewRequestService(requestRepo *database.RequestRepository, taskRepo *database.TaskRepository) *RequestService {
	return &RequestService{
		requestRepo: requestRepo,
		taskRepo:    taskRepo,
	}
}

// CreateRequest creates a new feature request
func (s *RequestService) CreateRequest(title, description, priority string) (*FeatureRequest, error) {
	if s.requestRepo == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if priority == "" {
		priority = "medium"
	}

	now := time.Now().UTC()
	var desc *string
	if description != "" {
		desc = &description
	}

	dbReq := &database.Request{
		ID:          fmt.Sprintf("REQ-%d", now.UnixNano()/1000000),
		Title:       title,
		Description: desc,
		Priority:    priority,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.requestRepo.Create(dbReq); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return &FeatureRequest{
		ID:          dbReq.ID,
		Title:       dbReq.Title,
		Description: description,
		Priority:    dbReq.Priority,
		Status:      dbReq.Status,
		CreatedAt:   dbReq.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dbReq.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetRequests returns all requests
func (s *RequestService) GetRequests() []*FeatureRequest {
	if s.requestRepo == nil {
		return []*FeatureRequest{}
	}

	dbRequests, err := s.requestRepo.List(100)
	if err != nil {
		fmt.Printf("Error loading requests from database: %v\n", err)
		return []*FeatureRequest{}
	}

	requests := make([]*FeatureRequest, len(dbRequests))
	for i, r := range dbRequests {
		desc := ""
		if r.Description != nil {
			desc = *r.Description
		}
		taskID := ""
		if r.TaskID != nil {
			taskID = *r.TaskID
		}
		requests[i] = &FeatureRequest{
			ID:          r.ID,
			Title:       r.Title,
			Description: desc,
			Priority:    r.Priority,
			Status:      r.Status,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
			TaskID:      taskID,
		}
	}
	return requests
}

// GetRequest returns a specific request
func (s *RequestService) GetRequest(id string) *FeatureRequest {
	if s.requestRepo == nil {
		return nil
	}

	r, err := s.requestRepo.GetByID(id)
	if err != nil || r == nil {
		return nil
	}

	desc := ""
	if r.Description != nil {
		desc = *r.Description
	}
	taskID := ""
	if r.TaskID != nil {
		taskID = *r.TaskID
	}

	return &FeatureRequest{
		ID:          r.ID,
		Title:       r.Title,
		Description: desc,
		Priority:    r.Priority,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		TaskID:      taskID,
	}
}

// UpdateRequestStatus updates the status of a request
func (s *RequestService) UpdateRequestStatus(id, status string) error {
	if s.requestRepo == nil {
		return fmt.Errorf("database not initialized")
	}

	return s.requestRepo.UpdateStatus(id, status, nil)
}

// DeleteRequest deletes a request
func (s *RequestService) DeleteRequest(id string) error {
	if s.requestRepo == nil {
		return fmt.Errorf("database not initialized")
	}

	return s.requestRepo.Delete(id)
}

// ConvertToTask converts a request to a task
func (s *RequestService) ConvertToTask(id string) (*Task, error) {
	if s.requestRepo == nil || s.taskRepo == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	request, err := s.requestRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get request: %w", err)
	}
	if request == nil {
		return nil, fmt.Errorf("request not found: %s", id)
	}

	if request.Status == "completed" || request.Status == "approved" {
		return nil, fmt.Errorf("request already %s", request.Status)
	}

	// Create task from request
	taskDesc := request.Title
	if request.Description != nil && *request.Description != "" {
		taskDesc = request.Title + "\n\n" + *request.Description
	}

	// Add context about being from a feature request
	taskDesc = fmt.Sprintf("[Feature Request: %s]\n\n%s", request.ID, taskDesc)

	now := time.Now().UTC()
	taskID := fmt.Sprintf("TASK-%d", now.UnixNano()/1000000)

	dbTask := &database.Task{
		ID:          taskID,
		Type:        "development",
		Priority:    request.Priority,
		Description: strings.TrimSpace(taskDesc),
		Status:      "pending",
		Source:      "request",
		CreatedAt:   now,
	}

	if err := s.taskRepo.Create(dbTask); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Update request status
	if err := s.requestRepo.UpdateStatus(id, "approved", &taskID); err != nil {
		return nil, fmt.Errorf("failed to update request status: %w", err)
	}

	return &Task{
		TaskID:      dbTask.ID,
		Type:        dbTask.Type,
		Priority:    dbTask.Priority,
		Description: dbTask.Description,
		Status:      dbTask.Status,
		Source:      dbTask.Source,
		CreatedAt:   dbTask.CreatedAt,
	}, nil
}
