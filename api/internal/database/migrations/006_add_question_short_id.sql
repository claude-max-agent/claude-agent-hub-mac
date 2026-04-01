-- Migration: 006_add_question_short_id
-- Add short_id column for easier question identification in Discord

ALTER TABLE pending_questions ADD COLUMN short_id TEXT;

-- Create index for short_id lookups
CREATE INDEX IF NOT EXISTS idx_pending_questions_short_id ON pending_questions(short_id);

-- Record migration
INSERT INTO schema_migrations (version) VALUES ('006_add_question_short_id');
