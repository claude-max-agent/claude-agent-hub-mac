-- Migration: 002_add_task_types_postgres
-- Description: No-op for PostgreSQL (already included in 001_initial_schema_postgres)
-- The PostgreSQL initial schema already includes feature/bugfix task types and executed request status.

INSERT INTO schema_migrations (version) VALUES ('002_add_task_types_postgres')
ON CONFLICT (version) DO NOTHING;
