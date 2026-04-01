import React, { useState, useMemo } from 'react';
import { ArrowDownAZ, BarChart3, Package } from 'lucide-react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useApps } from '../hooks/useApps';
import { AppCard } from '../components/AppCard';
import type { AppStatus } from '../types';

export const Apps: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const {
    apps,
    loading,
    error,
    actionLoading,
    logs,
    logsOpen,
    setLogsOpen,
    startApp,
    stopApp,
    restartApp,
    fetchLogs,
    refreshApps,
  } = useApps();

  const [sortByStatus, setSortByStatus] = useState(false);

  // Sort apps: by status (Active > Stopped > Not configured) or by name
  const sortedApps = useMemo(() => {
    const getStatusOrder = (status: string): number => {
      switch (status) {
        case 'running':
          return 0; // Active
        case 'stopped':
        case 'stopping':
          return 1; // Stopped
        default:
          return 2; // Not configured / error / partial / starting / etc.
      }
    };

    return [...apps].sort((a: AppStatus, b: AppStatus) => {
      if (sortByStatus) {
        const statusDiff = getStatusOrder(a.status) - getStatusOrder(b.status);
        if (statusDiff !== 0) return statusDiff;
      }
      return a.name.localeCompare(b.name);
    });
  }, [apps, sortByStatus]);

  const runningCount = apps.filter(a => a.status === 'running').length;

  if (loading && apps.length === 0) {
    return (
      <div style={{ padding: isMobile ? '12px' : '16px' }}>
        <div style={{
          backgroundColor: colors.cardBg,
          borderRadius: '8px',
          padding: '32px',
          boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
          textAlign: 'center',
        }}>
          <div style={{ fontSize: '14px', color: colors.textMuted }}>Loading applications...</div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      {/* Header */}
      <div style={{
        backgroundColor: colors.cardBg,
        borderRadius: '8px',
        padding: '16px',
        marginBottom: '16px',
        boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
      }}>
        <div style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexWrap: 'wrap',
          gap: '8px',
        }}>
          <div>
            <h2 style={{ fontSize: '16px', fontWeight: '600', margin: 0, color: colors.text }}>
              Application Manager
            </h2>
            <p style={{ fontSize: '12px', color: colors.textMuted, margin: '4px 0 0' }}>
              {apps.length} apps registered
              {runningCount > 0 && (
                <span style={{ color: '#22c55e', fontWeight: '500' }}> / {runningCount} running</span>
              )}
            </p>
          </div>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            {error && (
              <span style={{
                fontSize: '11px',
                padding: '4px 10px',
                borderRadius: '6px',
                backgroundColor: isDark ? '#7f1d1d' : '#fef2f2',
                color: isDark ? '#fca5a5' : '#ef4444',
              }}>
                {error}
              </span>
            )}
            <button
              onClick={() => setSortByStatus(!sortByStatus)}
              style={{
                padding: '6px 12px',
                borderRadius: '6px',
                border: `1px solid ${sortByStatus ? (isDark ? '#3b82f6' : '#2563eb') : colors.inputBorder}`,
                backgroundColor: sortByStatus ? (isDark ? '#1e3a5f' : '#dbeafe') : colors.inputBg,
                color: sortByStatus ? (isDark ? '#93c5fd' : '#2563eb') : colors.textMuted,
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: '500',
                transition: 'all 0.2s ease',
              }}
              title={sortByStatus ? 'Sort by name' : 'Sort by status'}
            >
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                {sortByStatus ? <><BarChart3 size={14} /> Status</> : <><ArrowDownAZ size={14} /> Name</>}
              </span>
            </button>
            <button
              onClick={refreshApps}
              style={{
                padding: '6px 12px',
                borderRadius: '6px',
                border: `1px solid ${colors.inputBorder}`,
                backgroundColor: colors.inputBg,
                color: colors.textMuted,
                cursor: 'pointer',
                fontSize: '12px',
                fontWeight: '500',
              }}
            >
              Refresh
            </button>
          </div>
        </div>
      </div>

      {/* App Cards */}
      {apps.length === 0 ? (
        <div style={{
          backgroundColor: colors.cardBg,
          borderRadius: '8px',
          padding: '32px',
          boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
          textAlign: 'center',
        }}>
          <div style={{ marginBottom: '16px', display: 'flex', justifyContent: 'center' }}><Package size={48} color={colors.textMuted} /></div>
          <p style={{ color: colors.textMuted, fontSize: '14px' }}>
            No applications configured. Add apps to config/apps.yaml.
          </p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {sortedApps.map((app) => (
            <div
              key={app.app_id}
              style={{
                transition: 'transform 0.3s ease, opacity 0.3s ease',
              }}
            >
              <AppCard
                app={app}
                actionLoading={actionLoading}
                logs={logs[app.app_id]}
                logsOpen={logsOpen === app.app_id}
                onStart={startApp}
                onStop={stopApp}
                onRestart={restartApp}
                onToggleLogs={setLogsOpen}
                onFetchLogs={fetchLogs}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
