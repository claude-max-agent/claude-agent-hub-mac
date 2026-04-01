package database

import (
	"time"
)

// Message represents a communication log entry
type Message struct {
	ID          int64
	FromAgent   string
	ToAgent     string
	MessageType string
	Content     string
	CreatedAt   time.Time
}

// MessageRepository handles message database operations
type MessageRepository struct {
	db *DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create inserts a new message
func (r *MessageRepository) Create(msg *Message) error {
	if r.db.DBType() == DBTypePostgreSQL {
		// PostgreSQL: use RETURNING to get the inserted ID
		err := r.db.QueryRow(`
			INSERT INTO messages (from_agent, to_agent, message_type, content, created_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, msg.FromAgent, msg.ToAgent, msg.MessageType, msg.Content, msg.CreatedAt.Format(time.RFC3339)).Scan(&msg.ID)
		return err
	}
	// SQLite: use LastInsertId
	result, err := r.db.Exec(`
		INSERT INTO messages (from_agent, to_agent, message_type, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, msg.FromAgent, msg.ToAgent, msg.MessageType, msg.Content, msg.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}

// List retrieves recent messages
func (r *MessageRepository) List(limit int) ([]Message, error) {
	rows, err := r.db.Query(`
		SELECT id, from_agent, to_agent, message_type, content, created_at
		FROM messages
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var createdAt string

		err := rows.Scan(&msg.ID, &msg.FromAgent, &msg.ToAgent, &msg.MessageType, &msg.Content, &createdAt)
		if err != nil {
			return nil, err
		}

		msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		messages = append(messages, msg)
	}

	return messages, nil
}

// ListByTask retrieves messages related to a task (by searching content)
func (r *MessageRepository) ListByTask(taskID string, limit int) ([]Message, error) {
	rows, err := r.db.Query(`
		SELECT id, from_agent, to_agent, message_type, content, created_at
		FROM messages
		WHERE content LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`, "%"+taskID+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var createdAt string

		err := rows.Scan(&msg.ID, &msg.FromAgent, &msg.ToAgent, &msg.MessageType, &msg.Content, &createdAt)
		if err != nil {
			return nil, err
		}

		msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		messages = append(messages, msg)
	}

	return messages, nil
}

// Cleanup removes messages older than the specified duration
func (r *MessageRepository) Cleanup(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := r.db.Exec("DELETE FROM messages WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
