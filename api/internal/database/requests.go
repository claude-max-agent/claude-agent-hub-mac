package database

import (
	"database/sql"
	"time"
)

// Request represents a feature request in the database
type Request struct {
	ID          string
	Title       string
	Description *string
	Priority    string
	Status      string
	TaskID      *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RequestRepository handles request database operations
type RequestRepository struct {
	db *DB
}

// NewRequestRepository creates a new request repository
func NewRequestRepository(db *DB) *RequestRepository {
	return &RequestRepository{db: db}
}

// Create inserts a new request
func (r *RequestRepository) Create(req *Request) error {
	_, err := r.db.Exec(`
		INSERT INTO requests (id, title, description, priority, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, req.ID, req.Title, req.Description, req.Priority, req.Status,
		req.CreatedAt.Format(time.RFC3339), req.UpdatedAt.Format(time.RFC3339))
	return err
}

// GetByID retrieves a request by ID
func (r *RequestRepository) GetByID(id string) (*Request, error) {
	var req Request
	var description, taskID sql.NullString
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, title, description, priority, status, task_id, created_at, updated_at
		FROM requests WHERE id = ?
	`, id).Scan(&req.ID, &req.Title, &description, &req.Priority, &req.Status, &taskID, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if description.Valid {
		req.Description = &description.String
	}
	if taskID.Valid {
		req.TaskID = &taskID.String
	}
	req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	req.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &req, nil
}

// List retrieves all requests
func (r *RequestRepository) List(limit int) ([]Request, error) {
	return r.ListWithPagination(limit, 0)
}

// ListWithPagination retrieves requests with pagination support
func (r *RequestRepository) ListWithPagination(limit, offset int) ([]Request, error) {
	rows, err := r.db.Query(`
		SELECT id, title, description, priority, status, task_id, created_at, updated_at
		FROM requests
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []Request
	for rows.Next() {
		var req Request
		var description, taskID sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(&req.ID, &req.Title, &description, &req.Priority, &req.Status, &taskID, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		if description.Valid {
			req.Description = &description.String
		}
		if taskID.Valid {
			req.TaskID = &taskID.String
		}
		req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		req.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		requests = append(requests, req)
	}

	return requests, nil
}

// Count returns the total number of requests
func (r *RequestRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&count)
	return count, err
}

// UpdateStatus updates a request's status
func (r *RequestRepository) UpdateStatus(id, status string, taskID *string) error {
	_, err := r.db.Exec(`
		UPDATE requests SET status = ?, task_id = ?, updated_at = ? WHERE id = ?
	`, status, taskID, time.Now().Format(time.RFC3339), id)
	return err
}

// Update updates a request's title, description, and priority
func (r *RequestRepository) Update(id string, title string, description *string, priority string) error {
	_, err := r.db.Exec(`
		UPDATE requests SET title = ?, description = ?, priority = ?, updated_at = ? WHERE id = ?
	`, title, description, priority, time.Now().Format(time.RFC3339), id)
	return err
}

// Delete removes a request
func (r *RequestRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM requests WHERE id = ?", id)
	return err
}
