import React, { useState, useEffect, useCallback } from 'react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useModal } from '../contexts/ModalContext';
import { api } from '../api/client';
import type { JobStatus, TradingSchedule } from '../types';

const STATUS_COLORS: Record<string, { bg: string; darkBg: string; text: string; darkText: string }> = {
  idle: { bg: 'rgba(100,116,139,0.1)', darkBg: 'rgba(100,116,139,0.15)', text: '#64748b', darkText: '#94a3b8' },
  running: { bg: 'rgba(59,130,246,0.1)', darkBg: 'rgba(59,130,246,0.15)', text: '#2563eb', darkText: '#93c5fd' },
  done: { bg: 'rgba(16,185,129,0.1)', darkBg: 'rgba(16,185,129,0.15)', text: '#059669', darkText: '#6ee7b7' },
  error: { bg: 'rgba(239,68,68,0.1)', darkBg: 'rgba(239,68,68,0.15)', text: '#dc2626', darkText: '#fca5a5' },
};

const STATUS_LABELS: Record<string, string> = {
  idle: '待機中',
  running: '実行中',
  done: '完了',
  error: 'エラー',
};

function formatTime(iso: string | null): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return d.toLocaleString('ja-JP', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// ─── Trading Schedule Form ───────────────────────────────────────────────────

interface ScheduleFormData {
  coin: string;
  side: 'buy' | 'sell';
  order_type: 'market' | 'limit';
  price: string;
  amount: string;
  leverage: string;
  cron: string;
  description: string;
}

const INITIAL_FORM: ScheduleFormData = {
  coin: 'BTC',
  side: 'buy',
  order_type: 'market',
  price: '',
  amount: '',
  leverage: '1',
  cron: '0 9 * * *',
  description: '',
};

const POPULAR_COINS = ['BTC', 'ETH', 'SOL', 'HYPE', 'XRP', 'DOGE', 'AVAX', 'LINK'];

// ─── Main Component ──────────────────────────────────────────────────────────

export const Triggers: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const { showConfirm } = useModal();

  // Trigger jobs state
  const [jobs, setJobs] = useState<Record<string, JobStatus>>({});
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState<Set<string>>(new Set());
  const [updatingResources, setUpdatingResources] = useState(false);

  // Trading schedules state
  const [schedules, setSchedules] = useState<TradingSchedule[]>([]);
  const [schedulesLoading, setSchedulesLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<ScheduleFormData>(INITIAL_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  // ─── Fetch ──────────────────────────────────────────

  const fetchStatus = useCallback(async () => {
    try {
      const data = await api.getTriggerStatus();
      setJobs(data);
    } catch {}
    finally { setLoading(false); }
  }, []);

  const fetchSchedules = useCallback(async () => {
    try {
      const data = await api.getTradingSchedules();
      setSchedules(data.schedules || []);
    } catch {}
    finally { setSchedulesLoading(false); }
  }, []);

  useEffect(() => {
    fetchStatus();
    fetchSchedules();
    const interval = setInterval(fetchStatus, 5000);
    return () => clearInterval(interval);
  }, [fetchStatus, fetchSchedules]);

  // ─── Trigger handlers ──────────────────────────────

  const triggerJob = async (jobName: string) => {
    setTriggering(prev => new Set(prev).add(jobName));
    try {
      await api.triggerJob(jobName);
      await fetchStatus();
    } catch {}
    finally {
      setTriggering(prev => {
        const next = new Set(prev);
        next.delete(jobName);
        return next;
      });
    }
  };

  // ─── Schedule handlers ─────────────────────────────

  const handleCreateSchedule = async () => {
    if (!form.coin || !form.amount || !form.cron) return;
    setSubmitting(true);
    try {
      await api.createTradingSchedule({
        coin: form.coin.toUpperCase(),
        side: form.side,
        order_type: form.order_type,
        price: form.order_type === 'limit' && form.price ? parseFloat(form.price) : undefined,
        amount: parseFloat(form.amount),
        leverage: parseInt(form.leverage) || 1,
        cron: form.cron,
        enabled: true,
        description: form.description,
      });
      setForm(INITIAL_FORM);
      setShowForm(false);
      await fetchSchedules();
    } catch {}
    finally { setSubmitting(false); }
  };

  const handleToggle = async (schedule: TradingSchedule) => {
    setTogglingId(schedule.id);
    try {
      await api.updateTradingSchedule(schedule.id, { enabled: !schedule.enabled });
      await fetchSchedules();
    } catch {}
    finally { setTogglingId(null); }
  };

  const handleDelete = (schedule: TradingSchedule) => {
    showConfirm({
      message: `スケジュール「${schedule.coin} ${schedule.side.toUpperCase()}」を削除しますか？`,
      confirmText: '削除',
      isDanger: true,
      onConfirm: async () => {
        await api.deleteTradingSchedule(schedule.id);
        await fetchSchedules();
      },
    });
  };

  // ─── Shared styles ─────────────────────────────────

  const cardStyle: React.CSSProperties = {
    backgroundColor: colors.cardBg,
    borderRadius: '12px',
    border: `1px solid ${colors.border}`,
    padding: '16px',
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
  };

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '8px 12px',
    borderRadius: '8px',
    border: `1px solid ${colors.inputBorder}`,
    backgroundColor: colors.inputBg,
    color: colors.text,
    fontSize: '13px',
    outline: 'none',
    boxSizing: 'border-box',
  };

  const labelStyle: React.CSSProperties = {
    fontSize: '12px',
    fontWeight: 600,
    color: colors.textMuted,
    marginBottom: '4px',
    display: 'block',
  };

  const smallBtnStyle = (active: boolean, color: string = colors.accent): React.CSSProperties => ({
    padding: '4px 12px',
    borderRadius: '6px',
    border: active ? `1.5px solid ${color}` : `1px solid ${colors.border}`,
    backgroundColor: active ? (isDark ? `${color}22` : `${color}15`) : 'transparent',
    color: active ? color : colors.textMuted,
    fontSize: '12px',
    fontWeight: 600,
    cursor: 'pointer',
  });

  // ─── Loading state ─────────────────────────────────

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '60px', color: colors.textMuted, fontSize: '14px' }}>
        Loading...
      </div>
    );
  }

  const jobEntries = Object.entries(jobs);

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      <h1 style={{ fontSize: '20px', fontWeight: 700, color: colors.text, margin: '0 0 4px' }}>
        Triggers
      </h1>
      <p style={{ fontSize: '13px', color: colors.textMuted, margin: '0 0 20px' }}>
        cronジョブを手動で実行できます。実行中のジョブは重複実行されません。
      </p>

      {/* ─── Cron Jobs Grid ─── */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: isMobile ? '1fr' : 'repeat(auto-fill, minmax(280px, 1fr))',
        gap: '12px',
        marginBottom: '24px',
      }}>
        {jobEntries.map(([name, job]) => {
          const isRunning = job.status === 'running';
          const isTriggering = triggering.has(name);
          const sc = STATUS_COLORS[job.status] || STATUS_COLORS.idle;

          return (
            <div key={name} style={cardStyle}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span style={{ fontWeight: 600, fontSize: '14px', color: colors.text }}>
                  {job.label}
                </span>
                <span style={{
                  padding: '2px 10px',
                  borderRadius: '12px',
                  fontSize: '11px',
                  fontWeight: 600,
                  backgroundColor: isDark ? sc.darkBg : sc.bg,
                  color: isDark ? sc.darkText : sc.text,
                }}>
                  {STATUS_LABELS[job.status] || job.status}
                </span>
              </div>

              <div style={{ fontSize: '12px', color: colors.textMuted }}>
                {job.started_at && <div>開始: {formatTime(job.started_at)}</div>}
                {job.finished_at && <div>完了: {formatTime(job.finished_at)}</div>}
              </div>

              <button
                onClick={() => triggerJob(name)}
                disabled={isRunning || isTriggering}
                style={{
                  marginTop: 'auto',
                  width: '100%',
                  padding: '10px',
                  borderRadius: '8px',
                  border: 'none',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: (isRunning || isTriggering) ? 'not-allowed' : 'pointer',
                  backgroundColor: (isRunning || isTriggering)
                    ? (isDark ? 'rgba(100,116,139,0.15)' : 'rgba(100,116,139,0.1)')
                    : colors.accent,
                  color: (isRunning || isTriggering)
                    ? colors.textMuted
                    : '#fff',
                }}
              >
                {isRunning ? '実行中...' : isTriggering ? '送信中...' : '実行'}
              </button>
            </div>
          );
        })}
      </div>

      {jobEntries.length === 0 && (
        <div style={{ textAlign: 'center', padding: '40px', color: colors.textMuted }}>
          トリガー可能なジョブがありません
        </div>
      )}

      {/* ─── Trading Schedules Section ─── */}
      <div style={{ borderTop: `1px solid ${colors.border}`, margin: '24px 0' }} />

      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
        <div>
          <h2 style={{ fontSize: '16px', fontWeight: 700, color: colors.text, margin: '0 0 4px' }}>
            Trading Schedules
          </h2>
          <p style={{ fontSize: '12px', color: colors.textMuted, margin: 0 }}>
            定期トレードのスケジュール管理。cron式で実行タイミングを設定できます。
          </p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          style={{
            padding: '8px 16px',
            borderRadius: '8px',
            border: 'none',
            fontSize: '13px',
            fontWeight: 600,
            cursor: 'pointer',
            backgroundColor: showForm ? (isDark ? 'rgba(100,116,139,0.15)' : 'rgba(100,116,139,0.1)') : colors.accent,
            color: showForm ? colors.textMuted : '#fff',
            flexShrink: 0,
          }}
        >
          {showForm ? 'キャンセル' : '+ 新規追加'}
        </button>
      </div>

      {/* ─── Add Form ─── */}
      {showForm && (
        <div style={{
          ...cardStyle,
          marginBottom: '16px',
          background: isDark
            ? 'linear-gradient(135deg, rgba(59,130,246,0.06) 0%, rgba(139,92,246,0.06) 100%)'
            : 'linear-gradient(135deg, rgba(59,130,246,0.04) 0%, rgba(139,92,246,0.04) 100%)',
          borderColor: isDark ? 'rgba(59,130,246,0.2)' : 'rgba(59,130,246,0.15)',
        }}>
          <div style={{ fontSize: '14px', fontWeight: 600, color: colors.text }}>
            新規スケジュール
          </div>

          {/* Row 1: Coin + Side + Order Type */}
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr 1fr', gap: '12px' }}>
            <div>
              <span style={labelStyle}>Coin</span>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginBottom: '6px' }}>
                {POPULAR_COINS.map(c => (
                  <button key={c} onClick={() => setForm(f => ({ ...f, coin: c }))}
                    style={smallBtnStyle(form.coin === c)}>
                    {c}
                  </button>
                ))}
              </div>
              <input
                style={inputStyle}
                value={form.coin}
                onChange={e => setForm(f => ({ ...f, coin: e.target.value.toUpperCase() }))}
                placeholder="BTC"
              />
            </div>
            <div>
              <span style={labelStyle}>Side</span>
              <div style={{ display: 'flex', gap: '6px' }}>
                {(['buy', 'sell'] as const).map(s => (
                  <button key={s} onClick={() => setForm(f => ({ ...f, side: s }))}
                    style={{
                      ...smallBtnStyle(form.side === s, s === 'buy' ? '#10b981' : '#ef4444'),
                      flex: 1,
                      padding: '8px',
                      fontSize: '13px',
                    }}>
                    {s === 'buy' ? 'BUY' : 'SELL'}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <span style={labelStyle}>Order Type</span>
              <div style={{ display: 'flex', gap: '6px' }}>
                {(['market', 'limit'] as const).map(t => (
                  <button key={t} onClick={() => setForm(f => ({ ...f, order_type: t }))}
                    style={{ ...smallBtnStyle(form.order_type === t), flex: 1, padding: '8px', fontSize: '13px' }}>
                    {t.charAt(0).toUpperCase() + t.slice(1)}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Row 2: Amount + Leverage + Price (conditional) */}
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : (form.order_type === 'limit' ? '1fr 1fr 1fr' : '1fr 1fr'), gap: '12px' }}>
            <div>
              <span style={labelStyle}>Amount (USD)</span>
              <input
                style={inputStyle}
                type="number"
                value={form.amount}
                onChange={e => setForm(f => ({ ...f, amount: e.target.value }))}
                placeholder="100"
                min="0"
                step="0.01"
              />
            </div>
            <div>
              <span style={labelStyle}>Leverage</span>
              <input
                style={inputStyle}
                type="number"
                value={form.leverage}
                onChange={e => setForm(f => ({ ...f, leverage: e.target.value }))}
                placeholder="1"
                min="1"
                max="100"
              />
            </div>
            {form.order_type === 'limit' && (
              <div>
                <span style={labelStyle}>Price</span>
                <input
                  style={inputStyle}
                  type="number"
                  value={form.price}
                  onChange={e => setForm(f => ({ ...f, price: e.target.value }))}
                  placeholder="65000"
                  min="0"
                  step="0.01"
                />
              </div>
            )}
          </div>

          {/* Row 3: Cron + Description */}
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 2fr', gap: '12px' }}>
            <div>
              <span style={labelStyle}>Cron Expression</span>
              <input
                style={inputStyle}
                value={form.cron}
                onChange={e => setForm(f => ({ ...f, cron: e.target.value }))}
                placeholder="0 9 * * *"
              />
              <span style={{ fontSize: '11px', color: colors.textMuted, marginTop: '2px', display: 'block' }}>
                min hour day month weekday
              </span>
            </div>
            <div>
              <span style={labelStyle}>Description</span>
              <input
                style={inputStyle}
                value={form.description}
                onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
                placeholder="毎朝9時のBTC積立"
              />
            </div>
          </div>

          {/* Submit */}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px' }}>
            <button
              onClick={() => { setShowForm(false); setForm(INITIAL_FORM); }}
              style={{
                padding: '8px 16px', borderRadius: '8px', fontSize: '13px', fontWeight: 600,
                border: `1px solid ${colors.border}`, backgroundColor: 'transparent',
                color: colors.textMuted, cursor: 'pointer',
              }}
            >
              キャンセル
            </button>
            <button
              onClick={handleCreateSchedule}
              disabled={submitting || !form.coin || !form.amount || !form.cron}
              style={{
                padding: '8px 20px', borderRadius: '8px', fontSize: '13px', fontWeight: 600,
                border: 'none', cursor: submitting ? 'not-allowed' : 'pointer',
                backgroundColor: submitting ? colors.textMuted : colors.accent,
                color: '#fff',
              }}
            >
              {submitting ? '作成中...' : 'スケジュール作成'}
            </button>
          </div>
        </div>
      )}

      {/* ─── Schedule List ─── */}
      {schedulesLoading ? (
        <div style={{ textAlign: 'center', padding: '30px', color: colors.textMuted, fontSize: '13px' }}>
          Loading schedules...
        </div>
      ) : schedules.length === 0 ? (
        <div style={{
          textAlign: 'center', padding: '40px', color: colors.textMuted,
          border: `1px dashed ${colors.border}`, borderRadius: '12px',
        }}>
          <div style={{ fontSize: '14px', marginBottom: '4px' }}>スケジュールがありません</div>
          <div style={{ fontSize: '12px' }}>「+ 新規追加」からスケジュールを作成してください</div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {schedules.map(schedule => {
            const isBuy = schedule.side === 'buy';
            const sideColor = isBuy ? '#10b981' : '#ef4444';
            const isToggling = togglingId === schedule.id;

            return (
              <div key={schedule.id} style={{
                ...cardStyle,
                flexDirection: 'row',
                alignItems: 'center',
                padding: '12px 16px',
                gap: '16px',
                opacity: schedule.enabled ? 1 : 0.55,
                transition: 'opacity 0.2s',
              }}>
                {/* Toggle */}
                <button
                  onClick={() => handleToggle(schedule)}
                  disabled={isToggling}
                  style={{
                    width: '44px', height: '24px', borderRadius: '12px', border: 'none',
                    cursor: isToggling ? 'not-allowed' : 'pointer', position: 'relative',
                    backgroundColor: schedule.enabled
                      ? (isDark ? '#10b981' : '#059669')
                      : (isDark ? 'rgba(100,116,139,0.3)' : 'rgba(100,116,139,0.2)'),
                    transition: 'background-color 0.2s',
                    flexShrink: 0,
                  }}
                >
                  <div style={{
                    width: '18px', height: '18px', borderRadius: '50%',
                    backgroundColor: '#fff',
                    position: 'absolute', top: '3px',
                    left: schedule.enabled ? '23px' : '3px',
                    transition: 'left 0.2s',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
                  }} />
                </button>

                {/* Coin + Side badge */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: '100px' }}>
                  <span style={{ fontWeight: 700, fontSize: '15px', color: colors.text }}>
                    {schedule.coin}
                  </span>
                  <span style={{
                    padding: '2px 8px', borderRadius: '4px', fontSize: '11px', fontWeight: 700,
                    backgroundColor: isDark ? `${sideColor}22` : `${sideColor}15`,
                    color: sideColor,
                  }}>
                    {schedule.side.toUpperCase()}
                  </span>
                </div>

                {/* Details */}
                <div style={{ flex: 1, display: 'flex', flexWrap: 'wrap', gap: isMobile ? '4px' : '16px', alignItems: 'center' }}>
                  <div style={{ fontSize: '12px', color: colors.textMuted }}>
                    <span style={{ color: colors.textSecondary, fontWeight: 600 }}>${schedule.amount}</span>
                    {schedule.leverage > 1 && (
                      <span style={{ marginLeft: '4px', color: '#f59e0b', fontWeight: 600 }}>
                        {schedule.leverage}x
                      </span>
                    )}
                  </div>
                  <div style={{
                    fontSize: '11px', fontFamily: 'monospace',
                    padding: '2px 8px', borderRadius: '4px',
                    backgroundColor: isDark ? 'rgba(139,92,246,0.1)' : 'rgba(139,92,246,0.06)',
                    color: isDark ? '#c4b5fd' : '#7c3aed',
                  }}>
                    {schedule.cron}
                  </div>
                  {schedule.description && (
                    <span style={{ fontSize: '12px', color: colors.textMuted }}>
                      {schedule.description}
                    </span>
                  )}
                  {schedule.run_count > 0 && (
                    <span style={{ fontSize: '11px', color: colors.textMuted }}>
                      {schedule.run_count}回実行
                    </span>
                  )}
                </div>

                {/* Order type badge */}
                <span style={{
                  fontSize: '11px', padding: '2px 8px', borderRadius: '4px', fontWeight: 600,
                  backgroundColor: isDark ? 'rgba(100,116,139,0.15)' : 'rgba(100,116,139,0.08)',
                  color: colors.textMuted,
                }}>
                  {schedule.order_type}
                </span>

                {/* Delete */}
                <button
                  onClick={() => handleDelete(schedule)}
                  style={{
                    padding: '6px 10px', borderRadius: '6px', border: 'none',
                    backgroundColor: 'transparent', color: colors.textMuted,
                    cursor: 'pointer', fontSize: '14px', flexShrink: 0,
                  }}
                  onMouseEnter={e => (e.currentTarget.style.color = '#ef4444')}
                  onMouseLeave={e => (e.currentTarget.style.color = colors.textMuted)}
                >
                  &#x2715;
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* ─── Dangerous Operations ─── */}
      <div style={{ borderTop: `1px solid ${colors.border}`, margin: '24px 0' }} />

      <h2 style={{ fontSize: '16px', fontWeight: 600, color: colors.text, margin: '0 0 12px' }}>
        デプロイ
      </h2>
      <button
        onClick={() => {
          showConfirm({
            message: 'GitHub Actions deploy workflow を実行します。よろしいですか？',
            confirmText: '実行',
            onConfirm: async () => {
              setUpdatingResources(true);
              try {
                await api.triggerDeploy();
              } catch {}
              finally { setUpdatingResources(false); }
            },
          });
        }}
        disabled={updatingResources}
        style={{
          padding: '10px 20px',
          borderRadius: '8px',
          border: 'none',
          fontSize: '13px',
          fontWeight: 600,
          cursor: updatingResources ? 'not-allowed' : 'pointer',
          backgroundColor: updatingResources
            ? (isDark ? 'rgba(59,130,246,0.15)' : 'rgba(59,130,246,0.1)')
            : colors.accent,
          color: updatingResources ? colors.textMuted : '#fff',
        }}
      >
        {updatingResources ? 'トリガー中...' : 'デプロイ実行 (GitHub Actions)'}
      </button>
    </div>
  );
};
