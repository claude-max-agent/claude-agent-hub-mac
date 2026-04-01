-- Migration: 004_add_pending_questions_postgres
-- Description: No-op for PostgreSQL (already included in 001_initial_schema_postgres)
-- The PostgreSQL initial schema already includes the pending_questions table.

INSERT INTO schema_migrations (version) VALUES ('004_add_pending_questions_postgres')
ON CONFLICT (version) DO NOTHING;
