-- Migration: 010_add_discord_channels
-- Created: 2026-03-26
-- Description: Add discord_channels table for DB-driven channel metadata management (PostgreSQL)

CREATE TABLE IF NOT EXISTS discord_channels (
    name TEXT PRIMARY KEY,
    repo TEXT NOT NULL DEFAULT '',
    template TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    read_only BOOLEAN NOT NULL DEFAULT FALSE,
    monitor_exclude BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 初期データ（channelRegistryから移行）
-- Repository channels
INSERT INTO discord_channels (name, repo, template) VALUES
('core', 'claude-max-agent/claude-agent-hub', 'dev'),
('trading', 'claude-max-agent/crypto-bot', 'trading'),
('crypto-news', 'claude-max-agent/crypto-daily-news', 'dev'),
('cmc', 'claude-max-agent/cmc-listing-bot', 'dev'),
('suumo', 'claude-max-agent/property-notifier', 'dev'),
('zenn', 'claude-max-agent/zenn-content', 'dev')
ON CONFLICT (name) DO NOTHING;

-- Domain-specific channels
INSERT INTO discord_channels (name, template, type) VALUES
('coconala', 'freelance', 'coconala'),
('strategy', 'trading', 'strategy')
ON CONFLICT (name) DO NOTHING;

-- Common channels
INSERT INTO discord_channels (name, type) VALUES
('alerts', 'alerts'),
('general', 'general')
ON CONFLICT (name) DO NOTHING;

INSERT INTO discord_channels (name, type, read_only) VALUES
('status', 'status', TRUE)
ON CONFLICT (name) DO NOTHING;

-- System channels
INSERT INTO discord_channels (name, type) VALUES
('system-log', 'system_log'),
('autonomous', 'autonomous'),
('session-monitor', 'system_log')
ON CONFLICT (name) DO NOTHING;

-- Ollama chat
INSERT INTO discord_channels (name, type) VALUES
('ollama-chat', 'ollama')
ON CONFLICT (name) DO NOTHING;

INSERT INTO schema_migrations (version) VALUES ('010_add_discord_channels') ON CONFLICT DO NOTHING;
