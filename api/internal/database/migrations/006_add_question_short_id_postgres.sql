-- Migration: 006_add_question_short_id_postgres
-- Description: No-op for PostgreSQL (already included in 001_initial_schema_postgres)
-- The PostgreSQL initial schema already includes the short_id column on pending_questions.

INSERT INTO schema_migrations (version) VALUES ('006_add_question_short_id_postgres')
ON CONFLICT (version) DO NOTHING;
