import React, { useState, useEffect, useCallback } from 'react';
import { RefreshCw, MessageSquare, ArrowRight, AlertTriangle, CheckCircle2, Clock } from 'lucide-react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { api } from '../api/client';

interface QueueMessage {
  id?: string;
  timestamp: string;
  type: string;
  from?: string;
  to?: string;
  message: string;
  channel?: string;
  status?: string;
}

const TYPE_CONFIG: Record<string, { color: string; icon: React.ReactNode }> = {
  task_assigned: { color: '#3b82f6', icon: <ArrowRight size={14} /> },
  task_completed: { color: '#22c55e', icon: <CheckCircle2 size={14} /> },
  notification: { color: '#8b5cf6', icon: <MessageSquare size={14} /> },
  question: { color: '#f59e0b', icon: <AlertTriangle size={14} /> },
  blocked: { color: '#ef4444', icon: <AlertTriangle size={14} /> },
  pr_created: { color: '#10b981', icon: <CheckCircle2 size={14} /> },
  message: { color: '#6366f1', icon: <MessageSquare size={14} /> },
  discord_reply: { color: '#5865f2', icon: <MessageSquare size={14} /> },
};

const DEFAULT_TYPE = { color: '#6b7280', icon: <Clock size={14} /> };

export const Activity: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();

  const [messages, setMessages] = useState<QueueMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const fetchQueue = useCallback(async () => {
    try {
      const data = await api.getQueue(100);
      const msgs: QueueMessage[] = data.messages || data.items || data || [];
      if (Array.isArray(msgs)) {
        setMessages(msgs);
      }
      setError(null);
    } catch (e) {
      setError('Failed to fetch activity log');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchQueue();
    if (!autoRefresh) return;
    const interval = setInterval(fetchQueue, 5000);
    return () => clearInterval(interval);
  }, [fetchQueue, autoRefresh]);

  const formatTime = (ts: string) => {
    try {
      const d = new Date(ts);
      const now = new Date();
      const diffMs = now.getTime() - d.getTime();
      const diffMin = Math.floor(diffMs / 60000);
      if (diffMin < 1) return 'just now';
      if (diffMin < 60) return `${diffMin}m ago`;
      const diffHr = Math.floor(diffMin / 60);
      if (diffHr < 24) return `${diffHr}h ago`;
      return d.toLocaleDateString('ja-JP', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    } catch {
      return ts;
    }
  };

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      {/* Header */}
      <div style={{
        backgroundColor: colors.cardBg,
        borderRadius: '12px',
        border: `1px solid ${colors.border}`,
        padding: isMobile ? '12px' : '14px',
        marginBottom: '12px',
        boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '8px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: isMobile ? '16px' : '18px', color: colors.text }}>
              Activity Feed
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: colors.textMuted }}>
              Agent communication log
              {autoRefresh && (
                <span style={{
                  marginLeft: '8px', fontSize: '11px', padding: '1px 8px',
                  borderRadius: '10px',
                  backgroundColor: isDark ? 'rgba(34,197,94,0.15)' : 'rgba(34,197,94,0.1)',
                  color: '#22c55e',
                }}>
                  Live
                </span>
              )}
            </p>
          </div>
          <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
            {error && <span style={{ fontSize: '12px', color: '#ef4444' }}>{error}</span>}
            <label style={{
              display: 'flex', alignItems: 'center', gap: '4px', fontSize: '12px',
              color: colors.textSecondary, cursor: 'pointer',
            }}>
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                style={{ accentColor: colors.accent }}
              />
              Auto-refresh
            </label>
            <button
              onClick={fetchQueue}
              disabled={loading}
              style={{
                padding: '6px 10px', border: `1px solid ${colors.border}`, borderRadius: '6px',
                backgroundColor: colors.inputBg, color: colors.text, cursor: 'pointer', fontSize: '12px',
                display: 'flex', alignItems: 'center', gap: '4px',
              }}
            >
              <RefreshCw size={12} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
              Refresh
            </button>
          </div>
        </div>
      </div>

      {/* Activity Timeline */}
      {loading && messages.length === 0 ? (
        <div style={{
          backgroundColor: colors.cardBg, borderRadius: '12px', border: `1px solid ${colors.border}`,
          padding: '32px', textAlign: 'center',
          boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
        }}>
          <div style={{ fontSize: '14px', color: colors.textMuted }}>Loading activity...</div>
        </div>
      ) : messages.length === 0 ? (
        <div style={{
          backgroundColor: colors.cardBg, borderRadius: '12px', border: `1px solid ${colors.border}`,
          padding: '40px 20px', textAlign: 'center',
          boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
        }}>
          <div style={{ fontSize: '14px', color: colors.textMuted }}>
            No activity yet. Messages between agents will appear here.
          </div>
        </div>
      ) : (
        <div style={{ position: 'relative' }}>
          {/* Timeline line */}
          <div style={{
            position: 'absolute',
            left: isMobile ? '15px' : '19px',
            top: '0',
            bottom: '0',
            width: '2px',
            backgroundColor: isDark ? 'rgba(59,130,246,0.2)' : 'rgba(59,130,246,0.15)',
          }} />

          {messages.map((msg, i) => {
            const tc = TYPE_CONFIG[msg.type] || DEFAULT_TYPE;
            return (
              <div
                key={msg.id || i}
                style={{
                  display: 'flex',
                  gap: isMobile ? '10px' : '14px',
                  marginBottom: '4px',
                  position: 'relative',
                }}
              >
                {/* Timeline dot */}
                <div style={{
                  width: isMobile ? '30px' : '38px',
                  minWidth: isMobile ? '30px' : '38px',
                  display: 'flex',
                  alignItems: 'flex-start',
                  justifyContent: 'center',
                  paddingTop: '14px',
                  position: 'relative',
                  zIndex: 1,
                }}>
                  <div style={{
                    width: '28px', height: '28px', borderRadius: '50%',
                    backgroundColor: isDark ? `${tc.color}22` : `${tc.color}15`,
                    border: `2px solid ${tc.color}`,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    color: tc.color,
                  }}>
                    {tc.icon}
                  </div>
                </div>

                {/* Content */}
                <div style={{
                  flex: 1,
                  backgroundColor: colors.cardBg,
                  borderRadius: '10px',
                  padding: '10px 14px',
                  border: `1px solid ${colors.border}`,
                  boxShadow: isDark ? '0 1px 2px rgba(0,0,0,0.2)' : '0 1px 2px rgba(0,0,0,0.06)',
                  minWidth: 0,
                }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px', flexWrap: 'wrap', gap: '4px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                      <span style={{
                        fontSize: '10px', padding: '1px 6px', borderRadius: '4px',
                        backgroundColor: isDark ? `${tc.color}22` : `${tc.color}12`,
                        color: tc.color, fontWeight: 600, textTransform: 'uppercase',
                      }}>
                        {msg.type.replace(/_/g, ' ')}
                      </span>
                      {msg.from && (
                        <span style={{ fontSize: '12px', fontWeight: 600, color: colors.accent }}>
                          {msg.from}
                        </span>
                      )}
                      {msg.from && msg.to && (
                        <ArrowRight size={12} style={{ color: colors.textMuted }} />
                      )}
                      {msg.to && (
                        <span style={{ fontSize: '12px', fontWeight: 500, color: colors.textSecondary }}>
                          {msg.to}
                        </span>
                      )}
                    </div>
                    <span style={{ fontSize: '11px', color: colors.textMuted, flexShrink: 0 }}>
                      {formatTime(msg.timestamp)}
                    </span>
                  </div>
                  <div style={{
                    fontSize: '13px', color: colors.text, lineHeight: '1.5',
                    wordBreak: 'break-word',
                  }}>
                    {msg.message}
                  </div>
                  {msg.channel && (
                    <div style={{ fontSize: '11px', color: colors.textMuted, marginTop: '4px' }}>
                      #{msg.channel}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
