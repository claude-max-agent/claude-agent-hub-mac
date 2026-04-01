-- Migration: 009_add_cron_jobs
-- Created: 2026-03-25
-- Description: Add cron_jobs table for DB-driven cron management (PostgreSQL)

CREATE TABLE IF NOT EXISTS cron_jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    prompt TEXT NOT NULL,
    requires_agent BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    run_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_enabled ON cron_jobs(enabled);

INSERT INTO cron_jobs (id, name, cron_expression, prompt, requires_agent, enabled) VALUES
('heartbeat', 'Heartbeat Patrol', '*/5 * * * *', 'Heartbeat巡回を開始。GitHub readyラベルのIssueを検出し、空きプールに割り当てる。', TRUE, TRUE),
('admin-boyaki', 'Admin Boyaki Scan', '*/5 * * * *', 'Discord hub-generalチャンネルをスキャンし、Adminのぼやき・要望を検出する。', FALSE, TRUE),
('chrome-cleanup', 'Chrome Tab Cleanup', '23 */2 * * *', '不要なChromeタブを閉じる。ChatGPT, Claude Web, coconala.comは保持。', FALSE, TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO schema_migrations (version) VALUES ('009_add_cron_jobs') ON CONFLICT DO NOTHING;
