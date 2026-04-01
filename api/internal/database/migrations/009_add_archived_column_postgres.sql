-- Migration: 009_add_archived_column_postgres
-- Created: 2026-03-25
-- Description: Add 'archived' boolean column to tasks table (PostgreSQL)
-- This column allows tasks to be archived independently of their status.

-- Add archived column (INTEGER for cross-DB query compatibility with COALESCE(archived, 0))
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'archived'
    ) THEN
        ALTER TABLE tasks ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
    END IF;
END $$;

-- Add session_id column to pending_questions if not exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'pending_questions' AND column_name = 'session_id'
    ) THEN
        ALTER TABLE pending_questions ADD COLUMN session_id VARCHAR(64);
    END IF;
END $$;

-- Add expires_at column to pending_questions if not exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'pending_questions' AND column_name = 'expires_at'
    ) THEN
        ALTER TABLE pending_questions ADD COLUMN expires_at TIMESTAMPTZ;
    END IF;
END $$;

-- Record migration
INSERT INTO schema_migrations (version) VALUES ('009_add_archived_column_postgres')
ON CONFLICT (version) DO NOTHING;
