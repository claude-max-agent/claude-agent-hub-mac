-- Migration: 003_add_blocked_status
-- Created: 2026-01-30
-- Description: Add 'blocked' status to tasks

-- SQLite doesn't support modifying CHECK constraints directly
-- We need to recreate the table

-- Create new tasks table with blocked status
CREATE TABLE IF NOT EXISTS tasks_new (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'development' CHECK (type IN ('development', 'research', 'notification', 'feature', 'bugfix')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled', 'blocked')),
    assigned_to TEXT REFERENCES agents(id),
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    assigned_at TEXT,
    completed_at TEXT
);

-- Copy data from old table
INSERT OR IGNORE INTO tasks_new SELECT * FROM tasks;

-- Drop old table
DROP TABLE IF EXISTS tasks;

-- Rename new table
ALTER TABLE tasks_new RENAME TO tasks;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- Migration tracking
INSERT OR IGNORE INTO schema_migrations (version) VALUES ('003_add_blocked_status');
