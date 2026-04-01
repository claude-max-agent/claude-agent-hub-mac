-- Migration: 009_add_archived_column
-- Created: 2026-03-25
-- Description: Add 'archived' boolean column to tasks table (SQLite)
-- This column allows tasks to be archived independently of their status.

-- Add archived column (INTEGER 0/1 for SQLite boolean compatibility)
ALTER TABLE tasks ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

-- Add session_id column to pending_questions if not exists
-- (some installations may already have this from manual migration)
-- SQLite does not support IF NOT EXISTS for ALTER TABLE, so we ignore errors

-- Add expires_at column to pending_questions if not exists

-- Record migration
INSERT OR IGNORE INTO schema_migrations (version) VALUES ('009_add_archived_column');
