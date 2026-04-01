-- Migration: 009_add_revenue_tables (PostgreSQL)
-- Created: 2026-03-25
-- Description: Add revenue, kpi_snapshots, activity_log, and targets tables (migrated from my-copy)

-- Revenue tracking table
CREATE TABLE IF NOT EXISTS revenue (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('apify', 'kdp', 'affiliate', 'trade', 'coconala', 'other')),
    amount NUMERIC NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'JPY',
    note TEXT
);

CREATE INDEX IF NOT EXISTS idx_revenue_date ON revenue(date);
CREATE INDEX IF NOT EXISTS idx_revenue_source ON revenue(source);

-- KPI snapshots table
CREATE TABLE IF NOT EXISTS kpi_snapshots (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    metric TEXT NOT NULL,
    value NUMERIC NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_kpi_snapshots_date ON kpi_snapshots(date);

-- Activity log table
CREATE TABLE IF NOT EXISTS activity_log (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    agent TEXT NOT NULL,
    action TEXT NOT NULL,
    detail TEXT
);

CREATE INDEX IF NOT EXISTS idx_activity_log_timestamp ON activity_log(timestamp);

-- Revenue targets table
CREATE TABLE IF NOT EXISTS targets (
    id SERIAL PRIMARY KEY,
    month TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('apify', 'kdp', 'affiliate', 'trade', 'coconala', 'other')),
    target_amount NUMERIC NOT NULL DEFAULT 0,
    UNIQUE(month, source)
);

-- Migration tracking
INSERT INTO schema_migrations (version) VALUES ('009_add_revenue_tables') ON CONFLICT DO NOTHING;
