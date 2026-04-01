-- Migration: 007_add_archived_status
-- Created: 2026-02-07
-- Description: Add 'archived' status to tasks for archive functionality

-- SQLite doesn't support modifying CHECK constraints directly
-- We need to recreate the table

-- Create new tasks table with archived status
CREATE TABLE IF NOT EXISTS tasks_new (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'development' CHECK (type IN ('development', 'research', 'notification', 'feature', 'bugfix')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled', 'blocked', 'archived')),
    assigned_to TEXT REFERENCES agents(id),
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    assigned_at TEXT,
    completed_at TEXT,
    parent_task_id TEXT REFERENCES tasks(id) ON DELETE CASCADE
);

-- Copy data from old table (explicitly specify columns to handle both schemas)
INSERT OR IGNORE INTO tasks_new (id, type, priority, description, status, assigned_to, source, created_at, assigned_at, completed_at, parent_task_id)
SELECT id, type, priority, description, status, assigned_to, source, created_at, assigned_at, completed_at, parent_task_id FROM tasks;

-- Drop old table
DROP TABLE IF EXISTS tasks;

-- Rename new table
ALTER TABLE tasks_new RENAME TO tasks;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_parent_task_id ON tasks(parent_task_id);

-- Migration tracking
INSERT OR IGNORE INTO schema_migrations (version) VALUES ('007_add_archived_status');
