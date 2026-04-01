-- Discussion threads for agent collaboration
-- PostgreSQL version

-- Discussions table (threads)
CREATE TABLE IF NOT EXISTS discussions (
    id SERIAL PRIMARY KEY,
    discussion_id TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    created_by TEXT NOT NULL,
    participants TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active',
    related_task TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Discussion messages
CREATE TABLE IF NOT EXISTS discussion_messages (
    id SERIAL PRIMARY KEY,
    discussion_id TEXT NOT NULL REFERENCES discussions(discussion_id) ON DELETE CASCADE,
    from_agent TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Collaboration tasks (extends tasks table)
-- Add collaborators column if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'tasks' AND column_name = 'collaborators') THEN
        ALTER TABLE tasks ADD COLUMN collaborators TEXT[] DEFAULT '{}';
    END IF;
END $$;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_discussions_status ON discussions(status);
CREATE INDEX IF NOT EXISTS idx_discussions_created_by ON discussions(created_by);
CREATE INDEX IF NOT EXISTS idx_discussions_related_task ON discussions(related_task);
CREATE INDEX IF NOT EXISTS idx_discussion_messages_discussion_id ON discussion_messages(discussion_id);
CREATE INDEX IF NOT EXISTS idx_discussion_messages_created_at ON discussion_messages(created_at DESC);
