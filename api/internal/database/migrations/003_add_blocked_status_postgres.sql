-- Migration: 003_add_blocked_status_postgres
-- Description: No-op for PostgreSQL (already included in 001_initial_schema_postgres)
-- The PostgreSQL initial schema already includes 'blocked' in the tasks_valid_status constraint.

INSERT INTO schema_migrations (version) VALUES ('003_add_blocked_status_postgres')
ON CONFLICT (version) DO NOTHING;
