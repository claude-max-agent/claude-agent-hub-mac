-- Migration: 005_add_parent_task_id_postgres
-- Description: No-op for PostgreSQL (already included in 001_initial_schema_postgres)
-- The PostgreSQL initial schema already includes the parent_task_id column.

INSERT INTO schema_migrations (version) VALUES ('005_add_parent_task_id_postgres')
ON CONFLICT (version) DO NOTHING;
