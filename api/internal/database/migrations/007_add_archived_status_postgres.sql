-- Migration: 007_add_archived_status_postgres
-- Created: 2026-03-09
-- Description: Add 'archived' status to tasks (PostgreSQL)

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_valid_status;
ALTER TABLE tasks ADD CONSTRAINT tasks_valid_status
    CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled', 'blocked', 'archived'));

INSERT INTO schema_migrations (version) VALUES ('007_add_archived_status_postgres')
ON CONFLICT (version) DO NOTHING;
