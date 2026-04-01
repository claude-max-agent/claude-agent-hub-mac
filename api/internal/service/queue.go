package service

import (
	"fmt"
	"time"

	"github.com/zono819/claude-agent-hub/api/internal/database"
)

// QueueMessage represents a message in the communication queue
type QueueMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Task represents a task in the system
type Task struct {
	TaskID      string     `json:"task_id"`
	Type        string     `json:"type"`
	Priority    string     `json:"priority"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	AssignedTo  *string    `json:"assigned_to"`
	Source      string     `json:"source,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	AssignedAt  *time.Time `json:"assigned_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// QueueService manages tasks and communication using SQLite
type QueueService struct {
	taskRepo    *database.TaskRepository
	messageRepo *database.MessageRepository
}

// NewQueueService creates a new queue service with database repositories
func NewQueueService(taskRepo *database.TaskRepository, messageRepo *database.MessageRepository) *QueueService {
	return &QueueService{
		taskRepo:    taskRepo,
		messageRepo: messageRepo,
	}
}

// GetTasks returns all tasks from database
func (s *QueueService) GetTasks() []Task {
	if s.taskRepo == nil {
		return []Task{}
	}

	dbTasks, err := s.taskRepo.List("", 100)
	if err != nil {
		fmt.Printf("Error loading tasks from database: %v\n", err)
		return []Task{}
	}

	tasks := make([]Task, len(dbTasks))
	for i, t := range dbTasks {
		tasks[i] = Task{
			TaskID:      t.ID,
			Type:        t.Type,
			Priority:    t.Priority,
			Description: t.Description,
			Status:      t.Status,
			AssignedTo:  t.AssignedTo,
			Source:      t.Source,
			CreatedAt:   t.CreatedAt,
			AssignedAt:  t.AssignedAt,
			CompletedAt: t.CompletedAt,
		}
	}
	return tasks
}

// CreateTask creates a new task
func (s *QueueService) CreateTask(task Task) (*Task, error) {
	return s.CreateTaskWithSource(task, "api")
}

// CreateTaskWithSource creates a new task with a specific source
func (s *QueueService) CreateTaskWithSource(task Task, source string) (*Task, error) {
	if s.taskRepo == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if task.TaskID == "" {
		task.TaskID = fmt.Sprintf("TASK-%d", time.Now().UnixNano())
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.Source = source

	dbTask := &database.Task{
		ID:          task.TaskID,
		Type:        task.Type,
		Priority:    task.Priority,
		Description: task.Description,
		Status:      task.Status,
		AssignedTo:  task.AssignedTo,
		Source:      source,
		CreatedAt:   task.CreatedAt,
	}

	if err := s.taskRepo.Create(dbTask); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Log the creation
	// Note: In Agent Teams mode, "manager" role maps to "team_lead"
	s.addMessage(source, "manager", "task_created",
		fmt.Sprintf("Task created: %s - %s", task.TaskID, task.Description))

	// Note: In Agent Teams mode, "manager" role maps to "team_lead"
	s.addMessage("system", "manager", "assignment_pending",
		fmt.Sprintf("Task %s awaiting Manager assignment", task.TaskID))

	return &task, nil
}

// GetTask returns a task by ID
func (s *QueueService) GetTask(taskID string) *Task {
	if s.taskRepo == nil {
		return nil
	}

	dbTask, err := s.taskRepo.GetByID(taskID)
	if err != nil || dbTask == nil {
		return nil
	}

	return &Task{
		TaskID:      dbTask.ID,
		Type:        dbTask.Type,
		Priority:    dbTask.Priority,
		Description: dbTask.Description,
		Status:      dbTask.Status,
		AssignedTo:  dbTask.AssignedTo,
		Source:      dbTask.Source,
		CreatedAt:   dbTask.CreatedAt,
		AssignedAt:  dbTask.AssignedAt,
		CompletedAt: dbTask.CompletedAt,
	}
}

// UpdateTaskStatus updates a task's status
func (s *QueueService) UpdateTaskStatus(taskID, status string) error {
	if s.taskRepo == nil {
		return fmt.Errorf("database not initialized")
	}

	if err := s.taskRepo.UpdateStatus(taskID, status); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	s.addMessage("system", "manager", "status_update",
		fmt.Sprintf("Task %s status changed to %s", taskID, status))

	return nil
}

// AssignTask assigns a task to an agent
func (s *QueueService) AssignTask(taskID, agentID string) error {
	if s.taskRepo == nil {
		return fmt.Errorf("database not initialized")
	}

	if err := s.taskRepo.Assign(taskID, agentID); err != nil {
		return fmt.Errorf("failed to assign task: %w", err)
	}

	s.addMessage("manager", agentID, "task_assigned",
		fmt.Sprintf("Assigned task %s", taskID))

	return nil
}

// CancelTask cancels a specific task
func (s *QueueService) CancelTask(taskID string) error {
	if s.taskRepo == nil {
		return fmt.Errorf("database not initialized")
	}

	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == "completed" || task.Status == "cancelled" {
		return fmt.Errorf("task already %s", task.Status)
	}

	if err := s.taskRepo.Delete(taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	s.addMessage("api", "manager", "task_cancelled",
		fmt.Sprintf("Task %s cancelled", taskID))

	return nil
}

// CancelAllTasks cancels all pending and assigned tasks
func (s *QueueService) CancelAllTasks() (int, error) {
	if s.taskRepo == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	count, err := s.taskRepo.DeleteByStatus("pending", "assigned", "in_progress")
	if err != nil {
		return 0, fmt.Errorf("failed to cancel tasks: %w", err)
	}

	if count > 0 {
		s.addMessage("api", "manager", "bulk_cancel",
			fmt.Sprintf("Cancelled %d tasks", count))
	}

	return int(count), nil
}

// GetMessages returns all messages from database
func (s *QueueService) GetMessages() []QueueMessage {
	if s.messageRepo == nil {
		return []QueueMessage{}
	}

	dbMessages, err := s.messageRepo.List(100)
	if err != nil {
		fmt.Printf("Error loading messages from database: %v\n", err)
		return []QueueMessage{}
	}

	messages := make([]QueueMessage, len(dbMessages))
	for i, m := range dbMessages {
		messages[i] = QueueMessage{
			ID:        fmt.Sprintf("MSG-%d", m.ID),
			From:      m.FromAgent,
			To:        m.ToAgent,
			Type:      m.MessageType,
			Content:   m.Content,
			Timestamp: m.CreatedAt,
		}
	}
	return messages
}

// addMessage adds a message to the communication log
func (s *QueueService) addMessage(from, to, msgType, content string) {
	if s.messageRepo == nil {
		return
	}

	msg := &database.Message{
		FromAgent:   from,
		ToAgent:     to,
		MessageType: msgType,
		Content:     content,
		CreatedAt:   time.Now(),
	}

	if err := s.messageRepo.Create(msg); err != nil {
		fmt.Printf("Warning: failed to log message: %v\n", err)
	}
}

// CountPendingTasks returns the number of pending tasks
func (s *QueueService) CountPendingTasks() int {
	if s.taskRepo == nil {
		return 0
	}

	count, err := s.taskRepo.CountPending()
	if err != nil {
		return 0
	}
	return count
}
