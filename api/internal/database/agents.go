package database

import (
	"database/sql"
	"time"
)

// Agent represents an agent in the database
type Agent struct {
	ID        string
	Role      string
	Nickname  *string
	PaneIndex int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AgentStatus represents a status report from an agent
type AgentStatus struct {
	ID              int64
	AgentID         string
	Status          string
	CurrentTask     *string
	TaskDescription *string
	ReportedAt      time.Time
}

// AgentRepository handles agent database operations
type AgentRepository struct {
	db *DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// GetAll retrieves all agents
func (r *AgentRepository) GetAll() ([]Agent, error) {
	rows, err := r.db.Query(`
		SELECT id, role, nickname, pane_index, created_at, updated_at
		FROM agents ORDER BY pane_index
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var agent Agent
		var nickname sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(&agent.ID, &agent.Role, &nickname, &agent.PaneIndex, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		if nickname.Valid {
			agent.Nickname = &nickname.String
		}
		agent.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		agent.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		agents = append(agents, agent)
	}

	return agents, nil
}

// GetByID retrieves an agent by ID
func (r *AgentRepository) GetByID(id string) (*Agent, error) {
	var agent Agent
	var nickname sql.NullString
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, role, nickname, pane_index, created_at, updated_at
		FROM agents WHERE id = ?
	`, id).Scan(&agent.ID, &agent.Role, &nickname, &agent.PaneIndex, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if nickname.Valid {
		agent.Nickname = &nickname.String
	}
	agent.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	agent.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &agent, nil
}

// UpdateNickname updates an agent's nickname
func (r *AgentRepository) UpdateNickname(id string, nickname *string) error {
	_, err := r.db.Exec("UPDATE agents SET nickname = ? WHERE id = ?", nickname, id)
	return err
}

// ReportStatus records a new status for an agent
func (r *AgentRepository) ReportStatus(agentID, status string, currentTask, taskDescription *string) error {
	_, err := r.db.Exec(`
		INSERT INTO agent_statuses (agent_id, status, current_task, task_description)
		VALUES (?, ?, ?, ?)
	`, agentID, status, currentTask, taskDescription)
	return err
}

// GetLatestStatus retrieves the latest status for an agent
func (r *AgentRepository) GetLatestStatus(agentID string) (*AgentStatus, error) {
	var status AgentStatus
	var currentTask, taskDescription sql.NullString
	var reportedAt string

	err := r.db.QueryRow(`
		SELECT id, agent_id, status, current_task, task_description, reported_at
		FROM agent_statuses
		WHERE agent_id = ?
		ORDER BY reported_at DESC
		LIMIT 1
	`, agentID).Scan(&status.ID, &status.AgentID, &status.Status, &currentTask, &taskDescription, &reportedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if currentTask.Valid {
		status.CurrentTask = &currentTask.String
	}
	if taskDescription.Valid {
		status.TaskDescription = &taskDescription.String
	}
	status.ReportedAt, _ = time.Parse(time.RFC3339, reportedAt)

	return &status, nil
}

// GetAllLatestStatuses retrieves the latest status for all agents
func (r *AgentRepository) GetAllLatestStatuses() (map[string]*AgentStatus, error) {
	// SQLite doesn't have DISTINCT ON, so we use a subquery
	rows, err := r.db.Query(`
		SELECT s.id, s.agent_id, s.status, s.current_task, s.task_description, s.reported_at
		FROM agent_statuses s
		INNER JOIN (
			SELECT agent_id, MAX(reported_at) as max_reported_at
			FROM agent_statuses
			GROUP BY agent_id
		) latest ON s.agent_id = latest.agent_id AND s.reported_at = latest.max_reported_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make(map[string]*AgentStatus)
	for rows.Next() {
		var status AgentStatus
		var currentTask, taskDescription sql.NullString
		var reportedAt string

		err := rows.Scan(&status.ID, &status.AgentID, &status.Status, &currentTask, &taskDescription, &reportedAt)
		if err != nil {
			return nil, err
		}

		if currentTask.Valid {
			status.CurrentTask = &currentTask.String
		}
		if taskDescription.Valid {
			status.TaskDescription = &taskDescription.String
		}
		status.ReportedAt, _ = time.Parse(time.RFC3339, reportedAt)

		statuses[status.AgentID] = &status
	}

	return statuses, nil
}

// CleanupOldStatuses removes status entries older than the specified duration
func (r *AgentRepository) CleanupOldStatuses(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	result, err := r.db.Exec("DELETE FROM agent_statuses WHERE reported_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
