package database

import (
	"database/sql"
	"time"
)

// CronJob represents a scheduled cron job
type CronJob struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	Prompt         string     `json:"prompt"`
	RequiresAgent  bool       `json:"requires_agent"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	RunCount       int        `json:"run_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CronJobRepository handles cron job database operations
type CronJobRepository struct {
	db *DB
}

// NewCronJobRepository creates a new cron job repository
func NewCronJobRepository(db *DB) *CronJobRepository {
	return &CronJobRepository{db: db}
}

// List retrieves cron jobs, optionally filtering by enabled status
func (r *CronJobRepository) List(enabledOnly bool) ([]CronJob, error) {
	query := `
		SELECT id, name, cron_expression, prompt, requires_agent, enabled,
		       last_run_at, run_count, created_at, updated_at
		FROM cron_jobs
	`
	if enabledOnly {
		query += " WHERE enabled = true"
	}
	query += " ORDER BY name"

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []CronJob
	for rows.Next() {
		var job CronJob
		var lastRunAt sql.NullString
		var createdAt, updatedAt string

		err := rows.Scan(
			&job.ID, &job.Name, &job.CronExpression, &job.Prompt,
			&job.RequiresAgent, &job.Enabled, &lastRunAt, &job.RunCount,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if lastRunAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastRunAt.String)
			job.LastRunAt = &t
		}
		job.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		job.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetByID retrieves a cron job by ID. Returns (nil, nil) if not found.
func (r *CronJobRepository) GetByID(id string) (*CronJob, error) {
	var job CronJob
	var lastRunAt sql.NullString
	var createdAt, updatedAt string

	err := r.db.QueryRow(`
		SELECT id, name, cron_expression, prompt, requires_agent, enabled,
		       last_run_at, run_count, created_at, updated_at
		FROM cron_jobs WHERE id = ?
	`, id).Scan(
		&job.ID, &job.Name, &job.CronExpression, &job.Prompt,
		&job.RequiresAgent, &job.Enabled, &lastRunAt, &job.RunCount,
		&createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if lastRunAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastRunAt.String)
		job.LastRunAt = &t
	}
	job.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	job.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &job, nil
}

// Create inserts a new cron job
func (r *CronJobRepository) Create(job *CronJob) error {
	_, err := r.db.Exec(`
		INSERT INTO cron_jobs (id, name, cron_expression, prompt, requires_agent, enabled, run_count)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, job.ID, job.Name, job.CronExpression, job.Prompt, job.RequiresAgent, job.Enabled)
	return err
}

// Update updates an existing cron job (all fields except id and created_at)
func (r *CronJobRepository) Update(job *CronJob) error {
	_, err := r.db.Exec(`
		UPDATE cron_jobs
		SET name = ?, cron_expression = ?, prompt = ?, requires_agent = ?,
		    enabled = ?, last_run_at = ?, run_count = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, job.Name, job.CronExpression, job.Prompt, job.RequiresAgent, job.Enabled,
		job.LastRunAt, job.RunCount, job.ID)
	return err
}

// Delete removes a cron job by ID
func (r *CronJobRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM cron_jobs WHERE id = ?", id)
	return err
}

// UpdateLastRun updates last_run_at to now and increments run_count
func (r *CronJobRepository) UpdateLastRun(id string) error {
	_, err := r.db.Exec(`
		UPDATE cron_jobs
		SET last_run_at = CURRENT_TIMESTAMP, run_count = run_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	return err
}
