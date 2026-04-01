-- Migration: 009_add_cron_jobs
-- Created: 2026-03-25
-- Description: Add cron_jobs table for DB-driven cron management

CREATE TABLE IF NOT EXISTS cron_jobs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    prompt TEXT NOT NULL,
    requires_agent INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    last_run_at TEXT,
    run_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_enabled ON cron_jobs(enabled);

-- 初期データ（cron-manifest.jsonから移行）
INSERT OR IGNORE INTO cron_jobs (id, name, cron_expression, prompt, requires_agent, enabled) VALUES
('heartbeat', 'Heartbeat Patrol', '*/5 * * * *', 'Heartbeat巡回を開始。GitHub readyラベルのIssueを検出し、空きプールに割り当てる。', 1, 1),
('admin-boyaki', 'Admin Boyaki Scan', '*/5 * * * *', 'Discord hub-generalチャンネルをスキャンし、Adminのぼやき・要望を検出する。', 0, 1),
('chrome-cleanup', 'Chrome Tab Cleanup', '23 */2 * * *', '不要なChromeタブを閉じる。ChatGPT, Claude Web, coconala.comは保持。', 0, 1);

INSERT OR IGNORE INTO schema_migrations (version) VALUES ('009_add_cron_jobs');
