-- Migration: 001_initial_schema
-- Created: 2026-01-30
-- Description: Initial database schema for Raspy Agent

-- =============================================================================
-- 1. agents (Agent Master)
-- =============================================================================
CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(32) PRIMARY KEY,
    role VARCHAR(32) NOT NULL,
    nickname VARCHAR(64),
    pane_index INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Initial data (max 8 workers - actual count determined dynamically by system resources)
INSERT INTO agents (id, role, pane_index) VALUES
    ('coordinator', 'coordinator', 0),
    ('worker1', 'worker', 1),
    ('worker2', 'worker', 2),
    ('worker3', 'worker', 3),
    ('worker4', 'worker', 4),
    ('worker5', 'worker', 5),
    ('worker6', 'worker', 6),
    ('worker7', 'worker', 7),
    ('worker8', 'worker', 8)
ON CONFLICT (id) DO NOTHING;

-- =============================================================================
-- 2. tasks
-- =============================================================================
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(64) PRIMARY KEY,
    type VARCHAR(32) NOT NULL DEFAULT 'development',
    priority VARCHAR(16) NOT NULL DEFAULT 'medium',
    description TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    assigned_to VARCHAR(32) REFERENCES agents(id),
    source VARCHAR(32) NOT NULL DEFAULT 'api',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    CONSTRAINT tasks_valid_type CHECK (type IN ('development', 'research', 'notification')),
    CONSTRAINT tasks_valid_priority CHECK (priority IN ('high', 'medium', 'low')),
    CONSTRAINT tasks_valid_status CHECK (status IN ('pending', 'assigned', 'in_progress', 'completed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- =============================================================================
-- 3. agent_statuses (Status History)
-- =============================================================================
CREATE TABLE IF NOT EXISTS agent_statuses (
    id BIGSERIAL PRIMARY KEY,
    agent_id VARCHAR(32) NOT NULL REFERENCES agents(id),
    status VARCHAR(16) NOT NULL,
    current_task VARCHAR(64),
    task_description TEXT,
    reported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT agent_statuses_valid_status CHECK (status IN ('available', 'busy', 'stopped'))
);

CREATE INDEX IF NOT EXISTS idx_agent_statuses_agent_id ON agent_statuses(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_statuses_reported_at ON agent_statuses(reported_at DESC);

-- View for latest status per agent
CREATE OR REPLACE VIEW agent_latest_status AS
SELECT DISTINCT ON (agent_id)
    agent_id,
    status,
    current_task,
    task_description,
    reported_at
FROM agent_statuses
ORDER BY agent_id, reported_at DESC;

-- =============================================================================
-- 4. reports (Task Reports)
-- =============================================================================
CREATE TABLE IF NOT EXISTS reports (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id VARCHAR(32) NOT NULL REFERENCES agents(id),
    status VARCHAR(32) NOT NULL,
    result TEXT,
    artifacts JSONB,
    reported_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT reports_valid_status CHECK (status IN ('completed', 'blocked', 'in_progress'))
);

CREATE INDEX IF NOT EXISTS idx_reports_task_id ON reports(task_id);
CREATE INDEX IF NOT EXISTS idx_reports_agent_id ON reports(agent_id);

-- =============================================================================
-- 5. requests (Feature Requests)
-- =============================================================================
CREATE TABLE IF NOT EXISTS requests (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(256) NOT NULL,
    description TEXT,
    priority VARCHAR(16) NOT NULL DEFAULT 'medium',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    task_id VARCHAR(64) REFERENCES tasks(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT requests_valid_priority CHECK (priority IN ('high', 'medium', 'low')),
    CONSTRAINT requests_valid_status CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);

-- =============================================================================
-- 6. messages (Communication Log)
-- =============================================================================
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    from_agent VARCHAR(32) NOT NULL,
    to_agent VARCHAR(32) NOT NULL,
    message_type VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_agents ON messages(from_agent, to_agent);

-- =============================================================================
-- Helper function: Update updated_at timestamp
-- =============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to tables with updated_at
DROP TRIGGER IF EXISTS update_agents_updated_at ON agents;
CREATE TRIGGER update_agents_updated_at
    BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_requests_updated_at ON requests;
CREATE TRIGGER update_requests_updated_at
    BEFORE UPDATE ON requests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Migration tracking table
-- =============================================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(64) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version) VALUES ('001_initial_schema')
ON CONFLICT (version) DO NOTHING;
