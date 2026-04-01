-- Migration: 001_initial_schema_sqlite
-- Created: 2026-01-30
-- Description: Initial database schema for Raspy Agent (SQLite)

-- Enable foreign keys
PRAGMA foreign_keys = ON;

-- =============================================================================
-- 1. agents (Agent Master)
-- =============================================================================
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    role TEXT NOT NULL,
    nickname TEXT,
    pane_index INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Initial data (max 8 workers - actual count determined dynamically by system resources)
INSERT OR IGNORE INTO agents (id, role, pane_index) VALUES
    ('coordinator', 'coordinator', 0),
    ('worker1', 'worker', 1),
    ('worker2', 'worker', 2),
    ('worker3', 'worker', 3),
    ('worker4', 'worker', 4),
    ('worker5', 'worker', 5),
    ('worker6', 'worker', 6),
    ('worker7', 'worker', 7),
    ('worker8', 'worker', 8);

-- =============================================================================
-- 2. tasks
-- =============================================================================
CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL DEFAULT 'development' CHECK (type IN ('development', 'research', 'notification')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled')),
    assigned_to TEXT REFERENCES agents(id),
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    assigned_at TEXT,
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- =============================================================================
-- 3. agent_statuses (Status History)
-- =============================================================================
CREATE TABLE IF NOT EXISTS agent_statuses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL REFERENCES agents(id),
    status TEXT NOT NULL CHECK (status IN ('available', 'busy', 'stopped')),
    current_task TEXT,
    task_description TEXT,
    reported_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_agent_statuses_agent_id ON agent_statuses(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_statuses_reported_at ON agent_statuses(reported_at DESC);

-- =============================================================================
-- 4. reports (Task Reports)
-- =============================================================================
CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES agents(id),
    status TEXT NOT NULL CHECK (status IN ('completed', 'blocked', 'in_progress')),
    result TEXT,
    artifacts TEXT,  -- JSON array as text
    reported_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_reports_task_id ON reports(task_id);
CREATE INDEX IF NOT EXISTS idx_reports_agent_id ON reports(agent_id);

-- =============================================================================
-- 5. requests (Feature Requests)
-- =============================================================================
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('high', 'medium', 'low')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);

-- =============================================================================
-- 6. messages (Communication Log)
-- =============================================================================
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_agent TEXT NOT NULL,
    to_agent TEXT NOT NULL,
    message_type TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_agents ON messages(from_agent, to_agent);

-- =============================================================================
-- Migration tracking
-- =============================================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES ('001_initial_schema_sqlite');
