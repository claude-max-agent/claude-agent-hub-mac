import React, { useState, useCallback } from 'react';
import { Circle, Package, ChevronUp, ChevronDown, Heart, HeartCrack, HelpCircle } from 'lucide-react';
import { useTheme } from '../theme';
import type { AppStatus } from '../types';

interface AppCardProps {
  app: AppStatus;
  actionLoading: string | null;
  logs?: string;
  logsOpen: boolean;
  onStart: (name: string) => void;
  onStop: (name: string) => void;
  onRestart: (name: string) => void;
  onToggleLogs: (name: string | null) => void;
  onFetchLogs: (name: string) => void;
}

const langIcon = (lang?: string): React.ReactNode => {
  switch (lang) {
    case 'go': return <Circle size={16} fill="#3b82f6" color="#3b82f6" />;
    case 'python': return <Circle size={16} fill="#eab308" color="#eab308" />;
    case 'nodejs': return <Circle size={16} fill="#22c55e" color="#22c55e" />;
    default: return <Package size={16} />;
  }
};

const appStatusColor = (status?: string): string => {
  switch (status) {
    case 'running': return '#22c55e';
    case 'stopped': return '#6b7280';
    case 'error': return '#ef4444';
    case 'partial': return '#f59e0b';
    case 'starting': return '#3b82f6';
    case 'stopping': return '#f59e0b';
    default: return '#9ca3af';
  }
};

const healthIcon = (health?: string): React.ReactNode => {
  switch (health) {
    case 'healthy': return <Heart size={14} fill="#22c55e" color="#22c55e" />;
    case 'unhealthy': return <HeartCrack size={14} color="#ef4444" />;
    default: return <HelpCircle size={14} />;
  }
};

export const AppCard: React.FC<AppCardProps> = ({
  app,
  actionLoading,
  logs,
  logsOpen,
  onStart,
  onStop,
  onRestart,
  onToggleLogs,
  onFetchLogs,
}) => {
  const { colors, isDark } = useTheme();
  const [expanded, setExpanded] = useState(false);
  const [hovered, setHovered] = useState(false);

  const isLoading = actionLoading === app.app_id;
  const isRunning = app.status === 'running';

  const handleMouseEnter = useCallback(() => setHovered(true), []);
  const handleMouseLeave = useCallback(() => setHovered(false), []);

  const cardStyle: React.CSSProperties = {
    backgroundColor: colors.cardBg,
    borderRadius: '12px',
    border: `1px solid ${colors.border}`,
    borderLeft: `4px solid ${appStatusColor(app.status)}`,
    boxShadow: hovered
      ? (isDark ? '0 4px 12px rgba(0,0,0,0.4)' : '0 4px 12px rgba(0,0,0,0.15)')
      : (isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)'),
    overflow: 'hidden',
    transition: 'transform 0.2s ease, box-shadow 0.2s ease',
    transform: hovered ? 'translateY(-2px)' : 'translateY(0)',
  };

  const headerStyle: React.CSSProperties = {
    padding: '10px 14px',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    cursor: 'pointer',
  };

  const btnBase: React.CSSProperties = {
    padding: '5px 10px',
    borderRadius: '5px',
    border: 'none',
    cursor: isLoading ? 'not-allowed' : 'pointer',
    fontSize: '11px',
    fontWeight: '500',
    opacity: isLoading ? 0.6 : 1,
  };

  return (
    <div style={cardStyle} onMouseEnter={handleMouseEnter} onMouseLeave={handleMouseLeave}>
      {/* Header */}
      <div style={headerStyle} onClick={() => setExpanded(!expanded)}>
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '2px', flexWrap: 'wrap' }}>
            <span style={{ display: 'inline-flex', alignItems: 'center' }}>{langIcon(app.language)}</span>
            <span style={{ fontSize: '14px', fontWeight: '600', color: colors.accent }}>{app.name}</span>
            <span style={{
              fontSize: '10px',
              padding: '2px 8px',
              borderRadius: '10px',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px',
              backgroundColor: isDark
                ? `${appStatusColor(app.status)}22`
                : `${appStatusColor(app.status)}18`,
              color: appStatusColor(app.status),
              fontWeight: '600',
            }}>
              <span style={{
                width: '6px',
                height: '6px',
                borderRadius: '50%',
                backgroundColor: appStatusColor(app.status),
                display: 'inline-block',
              }} />
              {app.status}
            </span>
            {app.type && (
              <span style={{
                fontSize: '10px',
                padding: '2px 6px',
                borderRadius: '4px',
                backgroundColor: isDark ? '#1e3a5f' : '#dbeafe',
                color: isDark ? '#93c5fd' : '#2563eb',
              }}>
                {app.type}
              </span>
            )}
            {app.uses_claude && (
              <span style={{
                fontSize: '10px',
                padding: '2px 6px',
                borderRadius: '4px',
                backgroundColor: isDark ? '#4a2c0a' : '#fff7ed',
                color: isDark ? '#fdba74' : '#c2410c',
                fontWeight: '600',
              }}>
                Claude
              </span>
            )}
            {app.uses_claude && app.model && (
              <span style={{
                fontSize: '10px',
                padding: '2px 6px',
                borderRadius: '4px',
                backgroundColor: isDark ? '#1e1b4b' : '#eef2ff',
                color: isDark ? '#a5b4fc' : '#4f46e5',
                fontWeight: '600',
              }}>
                {app.model}
              </span>
            )}
          </div>
          <div style={{ fontSize: '12px', color: colors.textMuted, lineHeight: '1.4' }}>
            {app.description}
          </div>
        </div>
        <span style={{ display: 'inline-flex', color: colors.textMuted, marginLeft: '8px', flexShrink: 0 }}>
          {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </span>
      </div>

      {/* Info row (always visible) */}
      <div style={{
        padding: '0 14px 8px',
        display: 'flex',
        gap: '10px',
        flexWrap: 'wrap',
        alignItems: 'center',
      }}>
        {app.port > 0 && (
          <span style={{ fontSize: '11px', color: colors.textMuted }}>
            Port: <span style={{ color: colors.text, fontWeight: '500' }}>{app.port}</span>
          </span>
        )}
        {app.health && app.health !== 'unknown' && (
          <span style={{
            fontSize: '11px',
            display: 'inline-flex',
            alignItems: 'center',
            gap: '4px',
            padding: '2px 8px',
            borderRadius: '10px',
            backgroundColor: app.health === 'healthy'
              ? (isDark ? 'rgba(34,197,94,0.15)' : 'rgba(34,197,94,0.1)')
              : (isDark ? 'rgba(239,68,68,0.15)' : 'rgba(239,68,68,0.1)'),
            color: app.health === 'healthy' ? '#22c55e' : '#ef4444',
            fontWeight: '500',
          }}>
            {healthIcon(app.health)} {app.health}
          </span>
        )}
        {app.repo && (
          <span style={{ fontSize: '11px', color: colors.textMuted, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '150px' }}>
            {app.repo}
          </span>
        )}
        {app.services && app.services.length > 0 && (
          <span style={{ fontSize: '11px', color: colors.textMuted }}>
            {app.services.filter(s => s.status === 'running').length}/{app.services.length} services
          </span>
        )}

        {/* Action buttons */}
        <div style={{ marginLeft: 'auto', display: 'flex', gap: '6px' }}>
          {!isRunning ? (
            <button
              style={{ ...btnBase, backgroundColor: '#22c55e', color: '#fff' }}
              onClick={(e) => { e.stopPropagation(); onStart(app.app_id); }}
              disabled={isLoading}
            >
              {isLoading ? '...' : 'Start'}
            </button>
          ) : (
            <>
              <button
                style={{ ...btnBase, backgroundColor: '#f59e0b', color: '#fff' }}
                onClick={(e) => { e.stopPropagation(); onRestart(app.app_id); }}
                disabled={isLoading}
              >
                {isLoading ? '...' : 'Restart'}
              </button>
              <button
                style={{ ...btnBase, backgroundColor: '#ef4444', color: '#fff' }}
                onClick={(e) => { e.stopPropagation(); onStop(app.app_id); }}
                disabled={isLoading}
              >
                {isLoading ? '...' : 'Stop'}
              </button>
            </>
          )}
          <button
            style={{
              ...btnBase,
              backgroundColor: logsOpen ? (isDark ? '#1e3a5f' : '#dbeafe') : colors.inputBg,
              color: logsOpen ? (isDark ? '#93c5fd' : '#2563eb') : colors.textMuted,
              border: `1px solid ${colors.inputBorder}`,
            }}
            onClick={(e) => {
              e.stopPropagation();
              if (logsOpen) {
                onToggleLogs(null);
              } else {
                onToggleLogs(app.app_id);
                onFetchLogs(app.app_id);
              }
            }}
          >
            Logs
          </button>
        </div>
      </div>

      {/* Expanded details */}
      {expanded && (
        <div style={{
          padding: '10px 14px',
          borderTop: `1px solid ${colors.border}`,
          backgroundColor: colors.bgTertiary,
          fontSize: '12px',
        }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr)', gap: '4px 12px', overflow: 'hidden' }}>
            <span style={{ color: colors.textMuted }}>ID:</span>
            <span style={{ color: colors.text }}>{app.app_id}</span>
            {app.language && (
              <>
                <span style={{ color: colors.textMuted }}>Language:</span>
                <span style={{ color: colors.text }}>{app.language}</span>
              </>
            )}
            <span style={{ color: colors.textMuted }}>Type:</span>
            <span style={{ color: colors.text }}>{app.type}</span>
            {app.repo && (
              <>
                <span style={{ color: colors.textMuted }}>Repository:</span>
                <span style={{ color: colors.text }}>{app.repo}</span>
              </>
            )}
            {app.path && (
              <>
                <span style={{ color: colors.textMuted }}>Path:</span>
                <code style={{ color: colors.text, fontSize: '11px', backgroundColor: isDark ? '#0f172a' : '#f1f5f9', padding: '1px 4px', borderRadius: '3px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}>{app.path}</code>
              </>
            )}
            {app.error && (
              <>
                <span style={{ color: '#ef4444' }}>Error:</span>
                <span style={{ color: '#ef4444' }}>{app.error}</span>
              </>
            )}
            {app.services && app.services.length > 0 && (
              <>
                <span style={{ color: colors.textMuted }}>Services:</span>
                <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                  {app.services.map(svc => (
                    <span key={svc.name} style={{
                      padding: '1px 6px',
                      borderRadius: '4px',
                      fontSize: '11px',
                      backgroundColor: svc.status === 'running' ? (isDark ? '#14532d' : '#dcfce7') : (isDark ? '#7f1d1d' : '#fef2f2'),
                      color: svc.status === 'running' ? (isDark ? '#86efac' : '#16a34a') : (isDark ? '#fca5a5' : '#dc2626'),
                    }}>
                      {svc.name}: {svc.status}
                    </span>
                  ))}
                </div>
              </>
            )}
            <span style={{ color: colors.textMuted }}>Updated:</span>
            <span style={{ color: colors.text }}>{app.updated_at ? new Date(app.updated_at).toLocaleString() : '-'}</span>
          </div>
        </div>
      )}

      {/* Logs panel */}
      {logsOpen && (
        <div style={{
          padding: '10px 14px',
          borderTop: `1px solid ${colors.border}`,
          backgroundColor: isDark ? '#0f172a' : '#f8fafc',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <span style={{ fontSize: '12px', fontWeight: '600', color: colors.text }}>Logs</span>
            <button
              style={{
                padding: '3px 8px',
                fontSize: '10px',
                borderRadius: '4px',
                border: `1px solid ${colors.inputBorder}`,
                backgroundColor: colors.inputBg,
                color: colors.textMuted,
                cursor: 'pointer',
              }}
              onClick={() => onFetchLogs(app.app_id)}
            >
              Refresh
            </button>
          </div>
          <pre style={{
            margin: 0,
            padding: '10px',
            backgroundColor: isDark ? '#020617' : '#f1f5f9',
            borderRadius: '6px',
            fontSize: '11px',
            lineHeight: '1.5',
            color: isDark ? '#a5f3fc' : '#334155',
            maxHeight: '300px',
            overflowY: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            fontFamily: '"SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace',
          }}>
            {logs || 'Loading logs...'}
          </pre>
        </div>
      )}
    </div>
  );
};
