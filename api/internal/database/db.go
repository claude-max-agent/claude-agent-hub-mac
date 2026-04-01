package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the sql.DB connection with database type info
type DB struct {
	*sql.DB
	dbType DBType
}

// New creates a new SQLite database connection
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings (SQLite works best with single connection)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	log.Printf("Connected to SQLite database: %s", dbPath)
	return &DB{DB: db, dbType: DBTypeSQLite}, nil
}

// NewPostgreSQL creates a new PostgreSQL database connection
func NewPostgreSQL(connStr string) (*DB, error) {
	// Open database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	// Set connection pool settings for PostgreSQL
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Log connection (without credentials)
	log.Printf("Connected to PostgreSQL database")
	return &DB{DB: db, dbType: DBTypePostgreSQL}, nil
}

// Exec wraps sql.DB.Exec with automatic placeholder conversion for PostgreSQL.
// This shadows the embedded sql.DB.Exec so all repository code works transparently.
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.DB.Exec(db.ReplacePlaceholders(query), args...)
}

// Query wraps sql.DB.Query with automatic placeholder conversion for PostgreSQL.
func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.DB.Query(db.ReplacePlaceholders(query), args...)
}

// QueryRow wraps sql.DB.QueryRow with automatic placeholder conversion for PostgreSQL.
func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.DB.QueryRow(db.ReplacePlaceholders(query), args...)
}

// Now returns the SQL expression for current timestamp appropriate for the database type.
func (db *DB) Now() string {
	if db.dbType == DBTypePostgreSQL {
		return "NOW()"
	}
	return "datetime('now')"
}

// OlderThanHoursCondition returns a SQL condition fragment checking if a column
// is older than the given number of hours. The hours value is passed as a bind parameter.
// For SQLite: "column < datetime('now', '-' || ? || ' hours')"
// For PostgreSQL: "column < NOW() - ($N * INTERVAL '1 hour')"
func (db *DB) OlderThanHoursCondition(column string) string {
	if db.dbType == DBTypePostgreSQL {
		return fmt.Sprintf("%s < NOW() - (? * INTERVAL '1 hour')", column)
	}
	return fmt.Sprintf("%s < datetime('now', '-' || ? || ' hours')", column)
}

// Migrate runs all pending migrations
func (db *DB) Migrate() error {
	// Create migrations table if not exists
	createMigrationTableSQL := db.getMigrationTableSQL()
	_, err := db.Exec(createMigrationTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of migration files
	migrations, err := db.getMigrationList()
	if err != nil {
		return fmt.Errorf("failed to get migration list: %w", err)
	}

	for _, migrationName := range migrations {
		// Check if already applied
		var count int
		query := db.ReplacePlaceholders("SELECT COUNT(*) FROM schema_migrations WHERE version = ?")
		err = db.QueryRow(query, migrationName).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", migrationName, err)
		}

		if count > 0 {
			log.Printf("Migration %s already applied", migrationName)
			continue
		}

		// Read migration file
		migrationSQL, err := migrationsFS.ReadFile("migrations/" + migrationName + ".sql")
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migrationName, err)
		}

		// Apply migration
		_, err = db.Exec(string(migrationSQL))
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migrationName, err)
		}

		log.Printf("Applied migration: %s", migrationName)
	}

	return nil
}

// getMigrationTableSQL returns the SQL to create the migrations table
func (db *DB) getMigrationTableSQL() string {
	if db.dbType == DBTypePostgreSQL {
		return `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version VARCHAR(64) PRIMARY KEY,
				applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)
		`
	}
	// SQLite
	return `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`
}

// getMigrationList returns the list of migrations to apply based on DB type
func (db *DB) getMigrationList() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []string
	suffix := "_sqlite.sql"
	if db.dbType == DBTypePostgreSQL {
		suffix = "_postgres.sql"
	}

	// Also include migrations without suffix (backwards compatibility)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		// Remove .sql extension
		baseName := strings.TrimSuffix(name, ".sql")

		// Check if this is a DB-specific migration
		if strings.HasSuffix(baseName, "_sqlite") && db.dbType == DBTypeSQLite {
			migrations = append(migrations, baseName)
		} else if strings.HasSuffix(baseName, "_postgres") && db.dbType == DBTypePostgreSQL {
			migrations = append(migrations, baseName)
		} else if !strings.HasSuffix(baseName, "_sqlite") && !strings.HasSuffix(baseName, "_postgres") {
			// Legacy migration without suffix - apply to both (but prefer DB-specific)
			// Check if a DB-specific version exists
			specificName := baseName + suffix[:len(suffix)-4] // remove .sql
			hasSpecific := false
			for _, e := range entries {
				if e.Name() == specificName+".sql" {
					hasSpecific = true
					break
				}
			}
			if !hasSpecific && db.dbType == DBTypeSQLite {
				// Only apply to SQLite for backwards compatibility
				migrations = append(migrations, baseName)
			}
		}
	}

	// Sort migrations by name (they should be numbered)
	sort.Strings(migrations)

	return migrations, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
