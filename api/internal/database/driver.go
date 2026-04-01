package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// DBType represents the type of database
type DBType string

const (
	DBTypeSQLite     DBType = "sqlite"
	DBTypePostgreSQL DBType = "postgresql"
)

// Config holds database configuration
type Config struct {
	Type DBType
	// SQLite
	SQLitePath string
	// PostgreSQL
	PostgresURL string
}

// ConfigFromEnv creates a Config from environment variables
// Priority: DATABASE_URL > DB_TYPE + individual settings
func ConfigFromEnv() *Config {
	cfg := &Config{}

	// Check for DATABASE_URL first (PostgreSQL connection string)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		cfg.Type = DBTypePostgreSQL
		cfg.PostgresURL = dbURL
		return cfg
	}

	// Check DB_TYPE environment variable
	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	switch dbType {
	case "postgresql", "postgres", "pg":
		cfg.Type = DBTypePostgreSQL
		// Build connection string from individual env vars
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "claude_hub")
		password := getEnvOrDefault("DB_PASSWORD", "claude_hub_password")
		database := getEnvOrDefault("DB_NAME", "claude_hub_agent")
		sslmode := getEnvOrDefault("DB_SSLMODE", "disable")

		cfg.PostgresURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, database, sslmode,
		)
	default:
		// Default to SQLite
		cfg.Type = DBTypeSQLite
		cfg.SQLitePath = getEnvOrDefault("DATA_DIR", "./data") + "/claude-hub.db"
	}

	return cfg
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// NewFromConfig creates a new database connection based on config
func NewFromConfig(cfg *Config) (*DB, error) {
	switch cfg.Type {
	case DBTypePostgreSQL:
		return NewPostgreSQL(cfg.PostgresURL)
	case DBTypeSQLite:
		return New(cfg.SQLitePath)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

// GetPlaceholder returns the appropriate placeholder for the database type
// SQLite uses ?, PostgreSQL uses $1, $2, etc.
func (db *DB) GetPlaceholder(index int) string {
	if db.dbType == DBTypePostgreSQL {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// ReplacePlaceholders replaces ? placeholders with $N for PostgreSQL
func (db *DB) ReplacePlaceholders(query string) string {
	if db.dbType != DBTypePostgreSQL {
		return query
	}

	var result strings.Builder
	paramIndex := 1
	for _, char := range query {
		if char == '?' {
			result.WriteString(fmt.Sprintf("$%d", paramIndex))
			paramIndex++
		} else {
			result.WriteRune(char)
		}
	}
	return result.String()
}

// DBType returns the type of database
func (db *DB) DBType() DBType {
	return db.dbType
}

// QueryWithPlaceholders wraps sql.DB.Query with placeholder conversion.
// Deprecated: Use db.Query() directly — placeholder conversion is now automatic.
func (db *DB) QueryWithPlaceholders(query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.Query(db.ReplacePlaceholders(query), args...)
}

// QueryRowWithPlaceholders wraps sql.DB.QueryRow with placeholder conversion.
// Deprecated: Use db.QueryRow() directly — placeholder conversion is now automatic.
func (db *DB) QueryRowWithPlaceholders(query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRow(db.ReplacePlaceholders(query), args...)
}

// ExecWithPlaceholders wraps sql.DB.Exec with placeholder conversion.
// Deprecated: Use db.Exec() directly — placeholder conversion is now automatic.
func (db *DB) ExecWithPlaceholders(query string, args ...interface{}) (sql.Result, error) {
	return db.DB.Exec(db.ReplacePlaceholders(query), args...)
}
