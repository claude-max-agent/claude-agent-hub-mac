-- Migration: 005_add_parent_task_id
-- Created: 2026-01-31
-- Description: Add parent_task_id column to support task hierarchy (subtasks)

-- Add parent_task_id column to tasks table
ALTER TABLE tasks ADD COLUMN parent_task_id TEXT REFERENCES tasks(id) ON DELETE CASCADE;

-- Create index for efficient subtask queries
CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks(parent_task_id);

-- Record migration
INSERT OR IGNORE INTO schema_migrations (version) VALUES ('005_add_parent_task_id');
