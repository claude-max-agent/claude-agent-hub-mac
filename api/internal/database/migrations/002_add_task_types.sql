-- Migration: 002_add_task_types
-- Created: 2026-01-30
-- Description: Add 'feature' and 'bugfix' task types, and 'executed' request status

-- SQLite doesn't support modifying CHECK constraints directly
-- We need to recreate the tables

-- =============================================================================
-- 1. Recreate tasks table with updated CHECK constraint
-- =============================================================================

-- Create new tasks table with expanded type constraint
CREATE TABLE IF NOT EXISTS tasks_new (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'development' CHECK (type IN ('development', 'research', 'notification', 'feature', 'bugfix')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled')),
    assigned_to TEXT REFERENCES agents(id),
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    assigned_at TEXT,
    completed_at TEXT
);

-- Copy data from old table (if exists)
INSERT OR IGNORE INTO tasks_new SELECT * FROM tasks;

-- Drop old table
DROP TABLE IF EXISTS tasks;

-- Rename new table
ALTER TABLE tasks_new RENAME TO tasks;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- =============================================================================
-- 2. Recreate requests table with updated CHECK constraint for status
-- =============================================================================

-- Create new requests table with expanded status constraint
CREATE TABLE IF NOT EXISTS requests_new (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'executed')),
    task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Copy data from old table (if exists)
INSERT OR IGNORE INTO requests_new SELECT * FROM requests;

-- Drop old table
DROP TABLE IF EXISTS requests;

-- Rename new table
ALTER TABLE requests_new RENAME TO requests;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);

-- =============================================================================
-- Migration tracking
-- =============================================================================
INSERT OR IGNORE INTO schema_migrations (version) VALUES ('002_add_task_types');
