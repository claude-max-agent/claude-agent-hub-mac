import React, { useState, useCallback } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { api } from '../api/client';

interface ModelBreakdown {
  modelName: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  cost: number;
}

interface UsageEntry {
  date?: string;
  week?: string;
  month?: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  totalCost: number;
  modelsUsed: string[];
  modelBreakdowns: ModelBreakdown[];
}

type ReportType = 'daily' | 'weekly' | 'monthly';

export const Usage: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const [reportType, setReportType] = useState<ReportType>('daily');
  const [entries, setEntries] = useState<UsageEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasFetched, setHasFetched] = useState(false);

  const fetchUsage = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getUsage(reportType);
      const items: UsageEntry[] = data[reportType] || data.daily || [];
      setEntries(items);
      setHasFetched(true);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch usage data');
    } finally {
      setLoading(false);
    }
  }, [reportType]);

  const getLabel = (entry: UsageEntry) => {
    const raw = entry.date || entry.week || entry.month || '???';
    // daily: show MM/DD
    if (entry.date) {
      const parts = raw.split('-');
      return `${parts[1]}/${parts[2]}`;
    }
    return raw;
  };

  const chartData = entries.map(e => ({
    name: getLabel(e),
    cost: Math.round(e.totalCost * 100) / 100,
    tokens: Math.round(e.totalTokens / 1_000_000 * 10) / 10,
  }));

  const totalCost = entries.reduce((sum, e) => sum + e.totalCost, 0);
  const totalTokens = entries.reduce((sum, e) => sum + e.totalTokens, 0);

  const tabStyle = (active: boolean): React.CSSProperties => ({
    padding: '6px 16px',
    borderRadius: '6px',
    border: `1px solid ${active ? '#3b82f6' : colors.border}`,
    backgroundColor: active ? (isDark ? '#1e3a5f' : '#eff6ff') : 'transparent',
    color: active ? '#3b82f6' : colors.textMuted,
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: active ? '600' : '400',
  });

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', flexWrap: 'wrap', gap: '8px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: '600', color: colors.text, margin: 0 }}>Claude Usage</h2>
        <div style={{ display: 'flex', gap: '6px' }}>
          {(['daily', 'weekly', 'monthly'] as ReportType[]).map(t => (
            <button key={t} onClick={() => setReportType(t)} style={tabStyle(reportType === t)}>
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
          <button
            onClick={fetchUsage}
            disabled={loading}
            style={{
              background: 'none', border: `1px solid ${colors.border}`, borderRadius: '6px',
              padding: '6px 10px', fontSize: '13px', color: colors.textMuted, cursor: loading ? 'not-allowed' : 'pointer',
              opacity: loading ? 0.6 : 1,
            }}
          >
            &#x21bb;
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr 1fr' : 'repeat(3, 1fr)', gap: '12px', marginBottom: '24px' }}>
        <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '16px', boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)' }}>
          <div style={{ fontSize: '12px', color: colors.textMuted, marginBottom: '4px' }}>Total Cost</div>
          <div style={{ fontSize: '24px', fontWeight: '700', color: '#ef4444' }}>${totalCost.toFixed(2)}</div>
        </div>
        <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '16px', boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)' }}>
          <div style={{ fontSize: '12px', color: colors.textMuted, marginBottom: '4px' }}>Total Tokens</div>
          <div style={{ fontSize: '24px', fontWeight: '700', color: '#3b82f6' }}>{(totalTokens / 1_000_000).toFixed(1)}M</div>
        </div>
        <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '16px', boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)', gridColumn: isMobile ? 'span 2' : 'auto' }}>
          <div style={{ fontSize: '12px', color: colors.textMuted, marginBottom: '4px' }}>Avg Cost / {reportType === 'daily' ? 'Day' : reportType === 'weekly' ? 'Week' : 'Month'}</div>
          <div style={{ fontSize: '24px', fontWeight: '700', color: '#f59e0b' }}>${entries.length ? (totalCost / entries.length).toFixed(2) : '0.00'}</div>
        </div>
      </div>

      {/* Chart */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px', color: colors.textMuted }}>Loading...</div>
      ) : error ? (
        <div style={{ textAlign: 'center', padding: '40px', color: '#ef4444' }}>{error}</div>
      ) : !hasFetched ? (
        <div style={{ textAlign: 'center', padding: '60px 20px', color: colors.textMuted }}>
          <div style={{ fontSize: '32px', marginBottom: '12px' }}>&#x21bb;</div>
          <div style={{ fontSize: '14px' }}>リロードボタンを押してデータを取得してください</div>
        </div>
      ) : (
        <>
          <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '16px', marginBottom: '24px', boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)' }}>
            <h3 style={{ fontSize: '14px', fontWeight: '600', color: colors.textMuted, marginTop: 0, marginBottom: '16px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
              Cost Trend
            </h3>
            <ResponsiveContainer width="100%" height={isMobile ? 220 : 300}>
              <BarChart data={chartData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" stroke={isDark ? '#374151' : '#e5e7eb'} />
                <XAxis dataKey="name" tick={{ fill: colors.textMuted, fontSize: 11 }} />
                <YAxis tick={{ fill: colors.textMuted, fontSize: 11 }} tickFormatter={(v: number) => `$${v}`} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: colors.cardBg,
                    border: `1px solid ${colors.border}`,
                    borderRadius: '6px',
                    color: colors.text,
                    fontSize: '12px',
                  }}
                  formatter={(value) => [`$${Number(value).toFixed(2)}`, 'Cost']}
                />
                <Bar dataKey="cost" fill="#3b82f6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Table */}
          <div style={{ backgroundColor: colors.cardBg, borderRadius: '8px', padding: '16px', boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)', overflowX: 'auto' }}>
            <h3 style={{ fontSize: '14px', fontWeight: '600', color: colors.textMuted, marginTop: 0, marginBottom: '12px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
              Detail
            </h3>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px' }}>
              <thead>
                <tr style={{ borderBottom: `2px solid ${colors.border}` }}>
                  <th style={{ textAlign: 'left', padding: '8px', color: colors.textMuted, fontWeight: '600' }}>Date</th>
                  <th style={{ textAlign: 'right', padding: '8px', color: colors.textMuted, fontWeight: '600' }}>Cost</th>
                  <th style={{ textAlign: 'right', padding: '8px', color: colors.textMuted, fontWeight: '600' }}>Tokens</th>
                  {!isMobile && <th style={{ textAlign: 'left', padding: '8px', color: colors.textMuted, fontWeight: '600' }}>Models</th>}
                </tr>
              </thead>
              <tbody>
                {[...entries].reverse().map((entry, i) => {
                  const label = entry.date || entry.week || entry.month || '???';
                  return (
                    <tr key={i} style={{ borderBottom: `1px solid ${colors.border}` }}>
                      <td style={{ padding: '8px', color: colors.text }}>{label}</td>
                      <td style={{ padding: '8px', color: '#ef4444', textAlign: 'right', fontWeight: '600', fontFamily: 'monospace' }}>
                        ${entry.totalCost.toFixed(2)}
                      </td>
                      <td style={{ padding: '8px', color: '#3b82f6', textAlign: 'right', fontFamily: 'monospace' }}>
                        {(entry.totalTokens / 1_000_000).toFixed(1)}M
                      </td>
                      {!isMobile && (
                        <td style={{ padding: '8px', color: colors.textMuted, fontSize: '11px' }}>
                          {entry.modelsUsed?.map(m => m.replace('claude-', '').replace('-20251001', '')).join(', ')}
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
};
