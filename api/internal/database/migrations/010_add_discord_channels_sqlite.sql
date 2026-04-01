-- Migration: 010_add_discord_channels
-- Created: 2026-03-26
-- Description: Add discord_channels table for DB-driven channel metadata management

CREATE TABLE IF NOT EXISTS discord_channels (
    name TEXT PRIMARY KEY,
    repo TEXT NOT NULL DEFAULT '',
    template TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    read_only INTEGER NOT NULL DEFAULT 0,
    monitor_exclude INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 初期データ（channelRegistryから移行）
-- Repository channels
INSERT OR IGNORE INTO discord_channels (name, repo, template) VALUES
('core', 'claude-max-agent/claude-agent-hub', 'dev'),
('trading', 'claude-max-agent/crypto-bot', 'trading'),
('crypto-news', 'claude-max-agent/crypto-daily-news', 'dev'),
('cmc', 'claude-max-agent/cmc-listing-bot', 'dev'),
('suumo', 'claude-max-agent/property-notifier', 'dev'),
('zenn', 'claude-max-agent/zenn-content', 'dev');

-- Domain-specific channels
INSERT OR IGNORE INTO discord_channels (name, template, type) VALUES
('coconala', 'freelance', 'coconala'),
('strategy', 'trading', 'strategy');

-- Common channels
INSERT OR IGNORE INTO discord_channels (name, type) VALUES
('alerts', 'alerts'),
('general', 'general');

INSERT OR IGNORE INTO discord_channels (name, type, read_only) VALUES
('status', 'status', 1);

-- System channels
INSERT OR IGNORE INTO discord_channels (name, type) VALUES
('system-log', 'system_log'),
('autonomous', 'autonomous'),
('session-monitor', 'system_log');

-- Ollama chat
INSERT OR IGNORE INTO discord_channels (name, type) VALUES
('ollama-chat', 'ollama');

INSERT OR IGNORE INTO schema_migrations (version) VALUES ('010_add_discord_channels');
