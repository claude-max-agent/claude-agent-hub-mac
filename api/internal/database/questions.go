package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// generateShortID generates a short 5-character alphanumeric ID for easier reference
func generateShortID() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Exclude confusing chars: I, O, 0, 1
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// PendingQuestion represents a question from Claude waiting for user response
type PendingQuestion struct {
	ID           string     `json:"id"`
	ShortID      string     `json:"short_id"`
	AgentID      string     `json:"agent_id"`
	SessionID    *string    `json:"session_id,omitempty"`
	QuestionType string     `json:"question_type"` // "question" or "permission"
	QuestionText string     `json:"question_text"`
	Options      []string   `json:"options,omitempty"`
	Status       string     `json:"status"` // "pending", "answered", "expired"
	Answer       *string    `json:"answer,omitempty"`
	AnsweredAt   *time.Time `json:"answered_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

// QuestionRepository handles pending questions database operations
type QuestionRepository struct {
	db *DB
}

// NewQuestionRepository creates a new question repository
func NewQuestionRepository(db *DB) *QuestionRepository {
	return &QuestionRepository{db: db}
}

// Create creates a new pending question
func (r *QuestionRepository) Create(q *PendingQuestion) error {
	var optionsJSON *string
	if len(q.Options) > 0 {
		data, err := json.Marshal(q.Options)
		if err != nil {
			return fmt.Errorf("failed to marshal options: %w", err)
		}
		s := string(data)
		optionsJSON = &s
	}

	var expiresAt *string
	if q.ExpiresAt != nil {
		s := q.ExpiresAt.Format(time.RFC3339)
		expiresAt = &s
	}

	// Generate short_id if not set
	if q.ShortID == "" {
		q.ShortID = generateShortID()
	}

	_, err := r.db.Exec(`
		INSERT INTO pending_questions (id, short_id, agent_id, session_id, question_type, question_text, options, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, q.ID, q.ShortID, q.AgentID, q.SessionID, q.QuestionType, q.QuestionText, optionsJSON, q.Status, q.CreatedAt.Format(time.RFC3339), expiresAt)

	return err
}

// GetByID retrieves a question by ID (supports both full ID and short_id)
func (r *QuestionRepository) GetByID(id string) (*PendingQuestion, error) {
	// If ID is short (5 chars), search by short_id first
	if len(id) == 5 {
		row := r.db.QueryRow(`
			SELECT id, short_id, agent_id, session_id, question_type, question_text, options, status, answer, answered_at, created_at, expires_at
			FROM pending_questions
			WHERE short_id = ?
		`, strings.ToUpper(id))
		q, err := r.scanQuestion(row)
		if err == nil && q != nil {
			return q, nil
		}
	}

	// Search by full ID
	row := r.db.QueryRow(`
		SELECT id, short_id, agent_id, session_id, question_type, question_text, options, status, answer, answered_at, created_at, expires_at
		FROM pending_questions
		WHERE id = ?
	`, id)

	return r.scanQuestion(row)
}

// GetByShortID retrieves a question by short_id
func (r *QuestionRepository) GetByShortID(shortID string) (*PendingQuestion, error) {
	row := r.db.QueryRow(`
		SELECT id, short_id, agent_id, session_id, question_type, question_text, options, status, answer, answered_at, created_at, expires_at
		FROM pending_questions
		WHERE short_id = ?
	`, strings.ToUpper(shortID))

	return r.scanQuestion(row)
}

// GetPendingByAgent retrieves pending questions for a specific agent
func (r *QuestionRepository) GetPendingByAgent(agentID string) ([]*PendingQuestion, error) {
	rows, err := r.db.Query(`
		SELECT id, short_id, agent_id, session_id, question_type, question_text, options, status, answer, answered_at, created_at, expires_at
		FROM pending_questions
		WHERE agent_id = ? AND status = 'pending'
		ORDER BY created_at DESC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanQuestions(rows)
}

// ListPending retrieves all pending questions
func (r *QuestionRepository) ListPending(limit int) ([]*PendingQuestion, error) {
	rows, err := r.db.Query(`
		SELECT id, short_id, agent_id, session_id, question_type, question_text, options, status, answer, answered_at, created_at, expires_at
		FROM pending_questions
		WHERE status = 'pending'
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanQuestions(rows)
}

// Answer marks a question as answered
func (r *QuestionRepository) Answer(id string, answer string) error {
	now := time.Now().Format(time.RFC3339)
	result, err := r.db.Exec(`
		UPDATE pending_questions
		SET status = 'answered', answer = ?, answered_at = ?
		WHERE id = ? AND status = 'pending'
	`, answer, now, id)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("question not found or already answered")
	}
	return nil
}

// Expire marks expired questions
func (r *QuestionRepository) ExpireOld(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := r.db.Exec(`
		UPDATE pending_questions
		SET status = 'expired'
		WHERE status = 'pending' AND created_at < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

// Delete deletes a question
func (r *QuestionRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM pending_questions WHERE id = ?", id)
	return err
}

// Helper functions

func (r *QuestionRepository) scanQuestion(row *sql.Row) (*PendingQuestion, error) {
	var q PendingQuestion
	var shortID, sessionID, optionsJSON, answer, answeredAt, expiresAt sql.NullString
	var createdAt string

	err := row.Scan(&q.ID, &shortID, &q.AgentID, &sessionID, &q.QuestionType, &q.QuestionText,
		&optionsJSON, &q.Status, &answer, &answeredAt, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if shortID.Valid {
		q.ShortID = shortID.String
	}
	if sessionID.Valid {
		q.SessionID = &sessionID.String
	}
	if optionsJSON.Valid {
		json.Unmarshal([]byte(optionsJSON.String), &q.Options)
	}
	if answer.Valid {
		q.Answer = &answer.String
	}
	if answeredAt.Valid {
		t, _ := time.Parse(time.RFC3339, answeredAt.String)
		q.AnsweredAt = &t
	}
	q.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if expiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		q.ExpiresAt = &t
	}

	return &q, nil
}

func (r *QuestionRepository) scanQuestions(rows *sql.Rows) ([]*PendingQuestion, error) {
	var questions []*PendingQuestion

	for rows.Next() {
		var q PendingQuestion
		var shortID, sessionID, optionsJSON, answer, answeredAt, expiresAt sql.NullString
		var createdAt string

		err := rows.Scan(&q.ID, &shortID, &q.AgentID, &sessionID, &q.QuestionType, &q.QuestionText,
			&optionsJSON, &q.Status, &answer, &answeredAt, &createdAt, &expiresAt)
		if err != nil {
			return nil, err
		}

		if shortID.Valid {
			q.ShortID = shortID.String
		}
		if sessionID.Valid {
			q.SessionID = &sessionID.String
		}
		if optionsJSON.Valid {
			json.Unmarshal([]byte(optionsJSON.String), &q.Options)
		}
		if answer.Valid {
			q.Answer = &answer.String
		}
		if answeredAt.Valid {
			t, _ := time.Parse(time.RFC3339, answeredAt.String)
			q.AnsweredAt = &t
		}
		q.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			q.ExpiresAt = &t
		}

		questions = append(questions, &q)
	}

	return questions, rows.Err()
}
