package database

import (
	"database/sql"
	"fmt"
	"time"
)

// Task represents a task in the database
type Task struct {
	ID           string
	Type         string
	Priority     string
	Description  string
	Status       string
	AssignedTo   *string
	Source       string
	ParentTaskID *string
	Archived     bool
	CreatedAt    time.Time
	AssignedAt   *time.Time
	CompletedAt  *time.Time
}

// TaskRepository handles task database operations
type TaskRepository struct {
	db *DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create inserts a new task
func (r *TaskRepository) Create(task *Task) error {
	_, err := r.db.Exec(`
		INSERT INTO tasks (id, type, priority, description, status, assigned_to, source, parent_task_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.Type, task.Priority, task.Description, task.Status, task.AssignedTo, task.Source, task.ParentTaskID, task.CreatedAt.Format(time.RFC3339))
	return err
}

// GetByID retrieves a task by ID
func (r *TaskRepository) GetByID(id string) (*Task, error) {
	task := &Task{}
	var createdAt, assignedAt, completedAt sql.NullString
	var assignedTo, parentTaskID sql.NullString
	var archived sql.NullInt64

	err := r.db.QueryRow(`
		SELECT id, type, priority, description, status, assigned_to, source, parent_task_id, created_at, assigned_at, completed_at, COALESCE(archived, 0)
		FROM tasks WHERE id = ?
	`, id).Scan(&task.ID, &task.Type, &task.Priority, &task.Description, &task.Status, &assignedTo, &task.Source, &parentTaskID, &createdAt, &assignedAt, &completedAt, &archived)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		task.AssignedTo = &assignedTo.String
	}
	if parentTaskID.Valid {
		task.ParentTaskID = &parentTaskID.String
	}
	if archived.Valid {
		task.Archived = archived.Int64 != 0
	}
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		task.CreatedAt = t
	}
	if assignedAt.Valid {
		t, _ := time.Parse(time.RFC3339, assignedAt.String)
		task.AssignedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		task.CompletedAt = &t
	}

	return task, nil
}

// List retrieves all tasks, optionally filtered by status
func (r *TaskRepository) List(status string, limit int) ([]Task, error) {
	return r.ListWithPagination(status, limit, 0)
}

// ListWithPagination retrieves tasks with pagination support
func (r *TaskRepository) ListWithPagination(status string, limit, offset int) ([]Task, error) {
	var rows *sql.Rows
	var err error

	query := `
		SELECT id, type, priority, description, status, assigned_to, source, parent_task_id, created_at, assigned_at, completed_at
		FROM tasks
	`
	if status != "" {
		query += " WHERE status = ?"
		query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
		rows, err = r.db.Query(query, status, limit, offset)
	} else {
		query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
		rows, err = r.db.Query(query, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		var createdAt, assignedAt, completedAt sql.NullString
		var assignedTo, parentTaskID sql.NullString

		err := rows.Scan(&task.ID, &task.Type, &task.Priority, &task.Description, &task.Status, &assignedTo, &task.Source, &parentTaskID, &createdAt, &assignedAt, &completedAt)
		if err != nil {
			return nil, err
		}

		if assignedTo.Valid {
			task.AssignedTo = &assignedTo.String
		}
		if parentTaskID.Valid {
			task.ParentTaskID = &parentTaskID.String
		}
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			task.CreatedAt = t
		}
		if assignedAt.Valid {
			t, _ := time.Parse(time.RFC3339, assignedAt.String)
			task.AssignedAt = &t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			task.CompletedAt = &t
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ListWithPaginationFiltered retrieves tasks with pagination and optional hide_completed/show_archived filters
func (r *TaskRepository) ListWithPaginationFiltered(status string, limit, offset int, hideCompleted bool, showArchived bool) ([]Task, error) {
	var rows *sql.Rows
	var err error

	query := `
		SELECT id, type, priority, description, status, assigned_to, source, parent_task_id, created_at, assigned_at, completed_at, COALESCE(archived, 0)
		FROM tasks
	`

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if hideCompleted {
		conditions = append(conditions, "status NOT IN ('completed', 'cancelled', 'archived')")
	}
	if !showArchived {
		conditions = append(conditions, "COALESCE(archived, 0) = 0")
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err = r.db.Query(query, args...)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		var createdAt, assignedAt, completedAt sql.NullString
		var assignedTo, parentTaskID sql.NullString
		var archived sql.NullInt64

		err := rows.Scan(&task.ID, &task.Type, &task.Priority, &task.Description, &task.Status, &assignedTo, &task.Source, &parentTaskID, &createdAt, &assignedAt, &completedAt, &archived)
		if err != nil {
			return nil, err
		}

		if assignedTo.Valid {
			task.AssignedTo = &assignedTo.String
		}
		if parentTaskID.Valid {
			task.ParentTaskID = &parentTaskID.String
		}
		if archived.Valid {
			task.Archived = archived.Int64 != 0
		}
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			task.CreatedAt = t
		}
		if assignedAt.Valid {
			t, _ := time.Parse(time.RFC3339, assignedAt.String)
			task.AssignedAt = &t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			task.CompletedAt = &t
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Count returns the total number of tasks, optionally filtered by status
func (r *TaskRepository) Count(status string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM tasks"
	if status != "" {
		query += " WHERE status = ?"
		err := r.db.QueryRow(query, status).Scan(&count)
		return count, err
	}
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// CountFiltered returns the total number of tasks with optional hide_completed/show_archived filters
func (r *TaskRepository) CountFiltered(status string, hideCompleted bool, showArchived bool) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM tasks"

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if hideCompleted {
		conditions = append(conditions, "status NOT IN ('completed', 'cancelled', 'archived')")
	}
	if !showArchived {
		conditions = append(conditions, "COALESCE(archived, 0) = 0")
	}

	if len(conditions) > 0 {
		query += " WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// UpdateStatus updates a task's status
func (r *TaskRepository) UpdateStatus(id, status string) error {
	var completedAt *string
	if status == "completed" || status == "cancelled" || status == "archived" {
		now := time.Now().Format(time.RFC3339)
		completedAt = &now
	}

	_, err := r.db.Exec(`
		UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?
	`, status, completedAt, id)
	return err
}

// Assign assigns a task to an agent
func (r *TaskRepository) Assign(id, agentID string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Exec(`
		UPDATE tasks SET assigned_to = ?, status = 'assigned', assigned_at = ? WHERE id = ?
	`, agentID, now, id)
	return err
}

// Delete removes a task and its subtasks (cascade delete)
func (r *TaskRepository) Delete(id string) error {
	// First, delete all subtasks (tasks where parent_task_id = id)
	_, err := r.db.Exec("DELETE FROM tasks WHERE parent_task_id = ?", id)
	if err != nil {
		return err
	}
	// Then delete the parent task
	_, err = r.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

// DeleteWithSubtaskCount removes a task and returns the count of deleted subtasks
func (r *TaskRepository) DeleteWithSubtaskCount(id string) (int64, error) {
	// Count subtasks first
	var subtaskCount int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE parent_task_id = ?", id).Scan(&subtaskCount)
	if err != nil {
		return 0, err
	}

	// Delete all subtasks
	_, err = r.db.Exec("DELETE FROM tasks WHERE parent_task_id = ?", id)
	if err != nil {
		return 0, err
	}

	// Delete the parent task
	_, err = r.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return 0, err
	}

	return subtaskCount, nil
}

// DeleteByStatus removes all tasks with given status
func (r *TaskRepository) DeleteByStatus(statuses ...string) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}

	query := "DELETE FROM tasks WHERE status IN (?" + fmt.Sprintf("%s", repeatString(",?", len(statuses)-1)) + ")"
	args := make([]any, len(statuses))
	for i, s := range statuses {
		args[i] = s
	}

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountPending returns the number of pending tasks (excluding archived)
func (r *TaskRepository) CountPending() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'pending' AND COALESCE(archived, 0) = 0").Scan(&count)
	return count, err
}

// CountActive returns the number of active tasks (not completed/cancelled/expired),
// excluding test commands (notification/test types, ping commands) and archived tasks
func (r *TaskRepository) CountActive() (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM tasks
		WHERE status NOT IN ('completed', 'cancelled', 'expired')
		AND COALESCE(archived, 0) = 0
		AND type NOT IN ('notification', 'test')
		AND LOWER(description) != 'ping'
		AND LOWER(description) NOT LIKE 'ping %'
	`).Scan(&count)
	return count, err
}

// CloseOldPendingTasks marks pending tasks older than the given hours as 'expired'
func (r *TaskRepository) CloseOldPendingTasks(olderThanHours int) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE tasks SET status = 'expired', completed_at = %s
		WHERE status = 'pending'
		AND %s
	`, r.db.Now(), r.db.OlderThanHoursCondition("created_at"))
	result, err := r.db.Exec(query, olderThanHours)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ArchiveTask sets the archived flag on a single task
func (r *TaskRepository) ArchiveTask(id string) error {
	_, err := r.db.Exec("UPDATE tasks SET archived = 1 WHERE id = ?", id)
	return err
}

// UnarchiveTask clears the archived flag on a single task
func (r *TaskRepository) UnarchiveTask(id string) error {
	_, err := r.db.Exec("UPDATE tasks SET archived = 0 WHERE id = ?", id)
	return err
}

// ArchiveOldTasks bulk-archives completed/cancelled tasks older than the given hours
func (r *TaskRepository) ArchiveOldTasks(olderThanHours int) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE tasks SET archived = 1
		WHERE status IN ('completed', 'cancelled')
		AND COALESCE(archived, 0) = 0
		AND %s
	`, r.db.OlderThanHoursCondition("created_at"))
	result, err := r.db.Exec(query, olderThanHours)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ArchiveByStatus bulk-archives tasks with the given status
func (r *TaskRepository) ArchiveByStatus(status string) (int64, error) {
	result, err := r.db.Exec(`
		UPDATE tasks SET archived = 1
		WHERE status = ? AND COALESCE(archived, 0) = 0
	`, status)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
