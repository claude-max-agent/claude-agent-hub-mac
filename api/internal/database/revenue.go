package database

import (
	"database/sql"
	"fmt"
)

// RevenueEntry represents a single revenue record
type RevenueEntry struct {
	ID       int64   `json:"id"`
	Date     string  `json:"date"`
	Source   string  `json:"source"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Note     *string `json:"note"`
}

// MonthlyRevenue represents aggregated monthly revenue
type MonthlyRevenue struct {
	Month   string             `json:"month"`
	Total   float64            `json:"total"`
	Sources map[string]float64 `json:"sources"`
}

// KpiSnapshot represents a KPI metric snapshot
type KpiSnapshot struct {
	ID     int64   `json:"id"`
	Date   string  `json:"date"`
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
}

// ActivityEntry represents an activity log entry
type ActivityEntry struct {
	ID        int64   `json:"id"`
	Timestamp string  `json:"timestamp"`
	Agent     string  `json:"agent"`
	Action    string  `json:"action"`
	Detail    *string `json:"detail"`
}

// Target represents a revenue target
type Target struct {
	ID           int64   `json:"id"`
	Month        string  `json:"month"`
	Source       string  `json:"source"`
	TargetAmount float64 `json:"target_amount"`
}

// RevenueRepository handles revenue-related database operations
type RevenueRepository struct {
	db *DB
}

// NewRevenueRepository creates a new RevenueRepository
func NewRevenueRepository(db *DB) *RevenueRepository {
	return &RevenueRepository{db: db}
}

// --- Revenue ---

// ListRevenue returns revenue entries filtered by period
func (r *RevenueRepository) ListRevenue(period string) ([]RevenueEntry, error) {
	query := `SELECT id, date, source, amount, currency, note FROM revenue ORDER BY date DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query revenue: %w", err)
	}
	defer rows.Close()

	var entries []RevenueEntry
	for rows.Next() {
		var e RevenueEntry
		if err := rows.Scan(&e.ID, &e.Date, &e.Source, &e.Amount, &e.Currency, &e.Note); err != nil {
			return nil, fmt.Errorf("scan revenue: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetMonthlyRevenue returns revenue aggregated by month
func (r *RevenueRepository) GetMonthlyRevenue() ([]MonthlyRevenue, error) {
	query := `SELECT substr(date, 1, 7) as month, source, SUM(amount) as total
		FROM revenue GROUP BY month, source ORDER BY month DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query monthly revenue: %w", err)
	}
	defer rows.Close()

	monthMap := make(map[string]*MonthlyRevenue)
	var months []string
	for rows.Next() {
		var month, source string
		var total float64
		if err := rows.Scan(&month, &source, &total); err != nil {
			return nil, fmt.Errorf("scan monthly revenue: %w", err)
		}
		if _, ok := monthMap[month]; !ok {
			monthMap[month] = &MonthlyRevenue{
				Month:   month,
				Sources: make(map[string]float64),
			}
			months = append(months, month)
		}
		monthMap[month].Sources[source] = total
		monthMap[month].Total += total
	}

	result := make([]MonthlyRevenue, 0, len(months))
	for _, m := range months {
		result = append(result, *monthMap[m])
	}
	return result, nil
}

// CreateRevenue inserts a new revenue entry
func (r *RevenueRepository) CreateRevenue(e *RevenueEntry) error {
	query := `INSERT INTO revenue (date, source, amount, currency, note) VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.Exec(query, e.Date, e.Source, e.Amount, e.Currency, e.Note)
	if err != nil {
		return fmt.Errorf("insert revenue: %w", err)
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return nil
}

// --- KPI ---

// GetLatestKpi returns the latest snapshot for each metric
func (r *RevenueRepository) GetLatestKpi() ([]KpiSnapshot, error) {
	query := `SELECT k.id, k.date, k.metric, k.value FROM kpi_snapshots k
		INNER JOIN (SELECT metric, MAX(date) as max_date FROM kpi_snapshots GROUP BY metric) latest
		ON k.metric = latest.metric AND k.date = latest.max_date`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query kpi: %w", err)
	}
	defer rows.Close()

	var snapshots []KpiSnapshot
	for rows.Next() {
		var s KpiSnapshot
		if err := rows.Scan(&s.ID, &s.Date, &s.Metric, &s.Value); err != nil {
			return nil, fmt.Errorf("scan kpi: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// CreateKpi inserts a new KPI snapshot
func (r *RevenueRepository) CreateKpi(s *KpiSnapshot) error {
	query := `INSERT INTO kpi_snapshots (date, metric, value) VALUES (?, ?, ?)`
	date := s.Date
	if date == "" {
		date = "date('now')"
		query = `INSERT INTO kpi_snapshots (date, metric, value) VALUES (date('now'), ?, ?)`
		res, err := r.db.Exec(query, s.Metric, s.Value)
		if err != nil {
			return fmt.Errorf("insert kpi: %w", err)
		}
		id, _ := res.LastInsertId()
		s.ID = id
		return nil
	}
	res, err := r.db.Exec(query, date, s.Metric, s.Value)
	if err != nil {
		return fmt.Errorf("insert kpi: %w", err)
	}
	id, _ := res.LastInsertId()
	s.ID = id
	return nil
}

// --- Activity Log ---

// ListActivity returns recent activity log entries
func (r *RevenueRepository) ListActivity(limit int) ([]ActivityEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	query := `SELECT id, timestamp, agent, action, detail FROM activity_log ORDER BY timestamp DESC LIMIT ?`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}
	defer rows.Close()

	var entries []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Agent, &e.Action, &e.Detail); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// CreateActivity inserts a new activity log entry
func (r *RevenueRepository) CreateActivity(e *ActivityEntry) error {
	query := `INSERT INTO activity_log (agent, action, detail) VALUES (?, ?, ?)`
	res, err := r.db.Exec(query, e.Agent, e.Action, e.Detail)
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return nil
}

// --- Targets ---

// ListTargets returns all targets, optionally filtered by month
func (r *RevenueRepository) ListTargets(month string) ([]Target, error) {
	var query string
	var rows *sql.Rows
	var err error

	if month != "" {
		query = `SELECT id, month, source, target_amount FROM targets WHERE month = ? ORDER BY source`
		rows, err = r.db.Query(query, month)
	} else {
		query = `SELECT id, month, source, target_amount FROM targets ORDER BY month DESC, source`
		rows, err = r.db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("query targets: %w", err)
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Month, &t.Source, &t.TargetAmount); err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// UpsertTarget creates or updates a target
func (r *RevenueRepository) UpsertTarget(t *Target) error {
	query := `INSERT INTO targets (month, source, target_amount) VALUES (?, ?, ?)
		ON CONFLICT(month, source) DO UPDATE SET target_amount = excluded.target_amount`
	res, err := r.db.Exec(query, t.Month, t.Source, t.TargetAmount)
	if err != nil {
		return fmt.Errorf("upsert target: %w", err)
	}
	id, _ := res.LastInsertId()
	t.ID = id
	return nil
}
