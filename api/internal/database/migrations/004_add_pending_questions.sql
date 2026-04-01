-- Migration: 004_add_pending_questions
-- Stores questions from Claude agents waiting for user response via Discord

CREATE TABLE IF NOT EXISTS pending_questions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    session_id TEXT,
    question_type TEXT NOT NULL DEFAULT 'question', -- 'question' or 'permission'
    question_text TEXT NOT NULL,
    options TEXT, -- JSON array of options, if any
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'answered', 'expired'
    answer TEXT,
    answered_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT -- optional expiration time
);

CREATE INDEX IF NOT EXISTS idx_pending_questions_agent ON pending_questions(agent_id);
CREATE INDEX IF NOT EXISTS idx_pending_questions_status ON pending_questions(status);
CREATE INDEX IF NOT EXISTS idx_pending_questions_created ON pending_questions(created_at DESC);

-- Record migration
INSERT INTO schema_migrations (version) VALUES ('004_add_pending_questions');
