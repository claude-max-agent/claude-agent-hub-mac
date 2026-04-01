import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { api } from '../api/client';

// Types
interface MonthlyRevenue {
  month: string;
  total: number;
  sources: Record<string, number>;
}

interface DailyRevenue {
  id: number;
  date: string;
  source: string;
  amount: number;
  currency: string;
  note: string | null;
}

interface KpiSnapshot {
  id: number;
  date: string;
  metric: string;
  value: number;
}

// Constants
const SOURCE_COLORS: Record<string, string> = {
  apify: '#8b5cf6',
  kdp: '#f59e0b',
  affiliate: '#3b82f6',
  trade: '#ef4444',
  coconala: '#10b981',
  other: '#6b7280',
};

const SOURCE_LABELS: Record<string, string> = {
  apify: 'Apify',
  kdp: 'KDP',
  affiliate: 'Affiliate',
  trade: 'Trade',
  coconala: 'Coconala',
  other: 'Other',
};

const ALL_SOURCES = ['apify', 'kdp', 'affiliate', 'trade', 'coconala', 'other'] as const;

const fmt = (n: number) =>
  n >= 10000 ? `\u00a5${(n / 10000).toFixed(1)}万` : `\u00a5${n.toLocaleString()}`;

function getDaysInMonth(year: number, month: number): number {
  return new Date(year, month, 0).getDate();
}

// --- Sub-components ---

const MonthSelector: React.FC<{
  year: number;
  month: number;
  onChange: (y: number, m: number) => void;
  isDark: boolean;
  colors: any;
}> = ({ year, month, onChange, isDark, colors }) => {
  const goPrev = () => month === 1 ? onChange(year - 1, 12) : onChange(year, month - 1);
  const goNext = () => month === 12 ? onChange(year + 1, 1) : onChange(year, month + 1);
  const btnStyle: React.CSSProperties = {
    width: 32, height: 32, display: 'flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: 8, border: 'none', cursor: 'pointer', fontSize: 14, fontWeight: 700,
    backgroundColor: isDark ? '#1e293b' : '#f1f5f9',
    color: isDark ? '#cbd5e1' : '#475569',
  };
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <button onClick={goPrev} style={btnStyle}>&lt;</button>
      <span style={{ fontSize: 14, fontWeight: 600, color: colors.text, minWidth: 100, textAlign: 'center' }}>
        {year}年{month}月
      </span>
      <button onClick={goNext} style={btnStyle}>&gt;</button>
    </div>
  );
};

const KpiCard: React.FC<{
  label: string; value: string; sub?: string; accentColor: string; isDark: boolean; colors: any;
}> = ({ label, value, sub, accentColor, isDark, colors }) => (
  <div style={{
    borderRadius: 12, padding: 20,
    backgroundColor: isDark ? '#1e293b' : '#ffffff',
    border: `1px solid ${isDark ? '#334155' : '#e2e8f0'}`,
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.08)',
  }}>
    <p style={{ fontSize: 11, fontWeight: 500, color: colors.textMuted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 4 }}>
      {label}
    </p>
    <p style={{ fontSize: 24, fontWeight: 700, color: accentColor, margin: 0 }}>{value}</p>
    {sub && <p style={{ fontSize: 11, color: colors.textMuted, marginTop: 4 }}>{sub}</p>}
  </div>
);

// --- Main Component ---

export const Revenue: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const now = new Date();

  const [selectedYear, setSelectedYear] = useState(now.getFullYear());
  const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);
  const [revenue, setRevenue] = useState<MonthlyRevenue[]>([]);
  const [dailyRevenue, setDailyRevenue] = useState<DailyRevenue[]>([]);
  const [kpi, setKpi] = useState<KpiSnapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const selectedMonthStr = `${selectedYear}-${String(selectedMonth).padStart(2, '0')}`;

  const fetchData = useCallback(async () => {
    try {
      const [monthlyRes, dailyRes, kpiRes] = await Promise.all([
        api.getRevenue('monthly').catch(() => []),
        api.getDailyRevenue().catch(() => []),
        api.getKpiLatest().catch(() => []),
      ]);
      setRevenue(Array.isArray(monthlyRes) ? monthlyRes : []);
      setDailyRevenue(Array.isArray(dailyRes) ? dailyRes : []);
      setKpi(Array.isArray(kpiRes) ? kpiRes : []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleMonthChange = (y: number, m: number) => {
    setSelectedYear(y);
    setSelectedMonth(m);
  };

  // Chart data
  const chartData = useMemo(() => {
    const daysInMonth = getDaysInMonth(selectedYear, selectedMonth);
    const monthStr = selectedMonthStr;

    const dailySourceMap: Record<string, Record<string, number>> = {};
    if (dailyRevenue && dailyRevenue.length > 0) {
      for (const entry of dailyRevenue) {
        if (entry.date.startsWith(monthStr)) {
          if (!dailySourceMap[entry.date]) dailySourceMap[entry.date] = {};
          const src = (ALL_SOURCES as readonly string[]).includes(entry.source) ? entry.source : 'other';
          dailySourceMap[entry.date][src] = (dailySourceMap[entry.date][src] || 0) + entry.amount;
        }
      }
    }

    if (!dailyRevenue || dailyRevenue.length === 0) {
      const currentMonthData = revenue.find(r => r.month === monthStr);
      if (currentMonthData && currentMonthData.total > 0) {
        const dayStr = `${monthStr}-01`;
        dailySourceMap[dayStr] = { ...currentMonthData.sources };
      }
    }

    const data: Record<string, any>[] = [];
    for (let d = 1; d <= daysInMonth; d++) {
      const dateStr = `${monthStr}-${String(d).padStart(2, '0')}`;
      const dayData: Record<string, any> = { name: `${d}` };
      for (const src of ALL_SOURCES) {
        dayData[src] = dailySourceMap[dateStr]?.[src] || 0;
      }
      data.push(dayData);
    }

    const activeSources = ALL_SOURCES.filter(src => data.some(d => d[src] > 0));
    return { data, activeSources };
  }, [revenue, dailyRevenue, selectedYear, selectedMonth, selectedMonthStr]);

  // KPI
  const currentMonthData = revenue.find(r => r.month === selectedMonthStr);
  const monthRevenue = currentMonthData?.total ?? 0;
  const totalRevenue = revenue.reduce((s, r) => s + r.total, 0);
  const kpiMap = Object.fromEntries(kpi.map(k => [k.metric, k.value]));

  // Revenue breakdown
  const sourceTotals: Record<string, number> = {};
  for (const src of ALL_SOURCES) sourceTotals[src] = 0;
  const filtered = revenue.filter(r => r.month === selectedMonthStr);
  for (const r of filtered) {
    for (const [src, amt] of Object.entries(r.sources)) {
      sourceTotals[src] = (sourceTotals[src] || 0) + amt;
    }
  }
  const sortedSources = Object.entries(sourceTotals).sort(([, a], [, b]) => b - a);
  const breakdownTotal = sortedSources.reduce((s, [, v]) => s + v, 0);

  // Daily activity entries
  const activityEntries = dailyRevenue
    .filter(e => e.amount > 0)
    .sort((a, b) => b.date.localeCompare(a.date));

  const axisColor = isDark ? '#94a3b8' : '#6b7280';
  const gridColor = isDark ? '#334155' : '#e5e7eb';

  const cardStyle: React.CSSProperties = {
    borderRadius: 12, padding: 20,
    backgroundColor: isDark ? '#1e293b' : '#ffffff',
    border: `1px solid ${isDark ? '#334155' : '#e2e8f0'}`,
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.08)',
  };

  if (loading) {
    return (
      <div style={{ padding: isMobile ? 12 : 16, maxWidth: '100%', overflow: 'hidden' }}>
        <h2 style={{ fontSize: 18, fontWeight: 600, color: colors.text, marginBottom: 16 }}>Revenue</h2>
        <div style={{ ...cardStyle, height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', color: colors.textMuted }}>
          読み込み中...
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: isMobile ? 12 : 16, maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 18, fontWeight: 600, color: colors.text, margin: 0 }}>Revenue</h2>
        <MonthSelector year={selectedYear} month={selectedMonth} onChange={handleMonthChange} isDark={isDark} colors={colors} />
      </div>

      {error && (
        <div style={{
          ...cardStyle, marginBottom: 16, padding: 12,
          backgroundColor: isDark ? 'rgba(245,158,11,0.1)' : '#fffbeb',
          borderColor: isDark ? '#92400e' : '#fde68a',
          color: isDark ? '#fcd34d' : '#92400e', fontSize: 13,
        }}>
          API接続エラー: {error}
        </div>
      )}

      {/* KPI Cards */}
      <div style={{ display: 'grid', gap: 12, gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, 1fr)', marginBottom: 16 }}>
        <KpiCard
          label="月間収益" isDark={isDark} colors={colors} accentColor="#10b981"
          value={revenue.length > 0 ? fmt(monthRevenue) : 'データなし'}
          sub={revenue.length > 0 ? selectedMonthStr : 'APIに収益データを登録してください'}
        />
        <KpiCard
          label="累計収益" isDark={isDark} colors={colors} accentColor="#3b82f6"
          value={revenue.length > 0 ? fmt(totalRevenue) : 'データなし'}
          sub={revenue.length > 0 ? `${revenue[revenue.length - 1]?.month} 〜 ${revenue[0]?.month}` : undefined}
        />
        {kpiMap['coconala_orders'] !== undefined && (
          <KpiCard label="ココナラ受注" isDark={isDark} colors={colors} accentColor="#8b5cf6"
            value={`${kpiMap['coconala_orders']}件`}
          />
        )}
        {kpiMap['active_agents'] !== undefined && (
          <KpiCard label="稼働エージェント" isDark={isDark} colors={colors} accentColor="#f59e0b"
            value={`${kpiMap['active_agents']}`}
          />
        )}
      </div>

      {/* Revenue Chart */}
      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ fontSize: 12, fontWeight: 600, color: colors.textMuted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 16 }}>
          {selectedYear}年{selectedMonth}月 日別収益
        </h3>
        <ResponsiveContainer width="100%" height={isMobile ? 220 : 300}>
          <BarChart data={chartData.data} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
            <XAxis dataKey="name" tick={{ fill: axisColor, fontSize: 10 }} interval={isMobile ? 4 : 1} />
            <YAxis tick={{ fill: axisColor, fontSize: 11 }}
              tickFormatter={(v: number) => v >= 10000 ? `${v / 10000}万` : `${v}`}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: isDark ? '#1e293b' : '#fff',
                border: `1px solid ${isDark ? '#334155' : '#e5e7eb'}`,
                borderRadius: 8, fontSize: 12, color: isDark ? '#f1f5f9' : '#1e293b',
              }}
              cursor={{ fill: isDark ? 'rgba(148,163,184,0.1)' : 'rgba(0,0,0,0.05)' }}
              formatter={(value: any) => [`\u00a5${(value ?? 0).toLocaleString()}`, undefined]}
              labelFormatter={(label) => `${selectedYear}年${selectedMonth}月${label}日`}
            />
            <Legend formatter={(value: string) => SOURCE_LABELS[value] || value} wrapperStyle={{ fontSize: 12 }} />
            {chartData.activeSources.map(src => (
              <Bar key={src} dataKey={src} stackId="revenue" fill={SOURCE_COLORS[src]} name={src} />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div style={{ display: 'grid', gap: 16, gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr' }}>
        {/* Revenue Breakdown */}
        <div style={cardStyle}>
          <h3 style={{ fontSize: 12, fontWeight: 600, color: colors.textMuted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 16 }}>
            収益源内訳
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {sortedSources.map(([src, amount]) => {
              const pct = breakdownTotal > 0 ? (amount / breakdownTotal) * 100 : 0;
              return (
                <div key={src}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, marginBottom: 4 }}>
                    <span style={{ color: colors.text, fontWeight: 500 }}>{SOURCE_LABELS[src] || src}</span>
                    <span style={{ color: colors.textMuted, fontFamily: 'monospace' }}>{`\u00a5${amount.toLocaleString()}`}</span>
                  </div>
                  <div style={{ width: '100%', backgroundColor: isDark ? '#334155' : '#e2e8f0', borderRadius: 9999, height: 8 }}>
                    <div style={{ height: 8, borderRadius: 9999, width: `${pct}%`, backgroundColor: SOURCE_COLORS[src] || '#6b7280', transition: 'width 0.3s' }} />
                  </div>
                </div>
              );
            })}
          </div>
          <div style={{ marginTop: 16, paddingTop: 12, borderTop: `1px solid ${isDark ? '#334155' : '#e2e8f0'}`, display: 'flex', justifyContent: 'space-between', fontSize: 14, fontWeight: 600 }}>
            <span style={{ color: colors.text }}>合計</span>
            <span style={{ color: '#10b981', fontFamily: 'monospace' }}>{`\u00a5${breakdownTotal.toLocaleString()}`}</span>
          </div>
        </div>

        {/* Activity Log */}
        <div style={cardStyle}>
          <h3 style={{ fontSize: 12, fontWeight: 600, color: colors.textMuted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 16 }}>
            案件リザルト
          </h3>
          {activityEntries.length === 0 ? (
            <div style={{ height: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', color: colors.textMuted, fontSize: 13 }}>
              まだ案件はありません
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ textAlign: 'left', color: colors.textMuted, borderBottom: `1px solid ${isDark ? '#334155' : '#e2e8f0'}` }}>
                    <th style={{ paddingBottom: 8, fontWeight: 500 }}>日付</th>
                    <th style={{ paddingBottom: 8, fontWeight: 500 }}>媒体</th>
                    <th style={{ paddingBottom: 8, fontWeight: 500 }}>案件名</th>
                    <th style={{ paddingBottom: 8, fontWeight: 500, textAlign: 'right' }}>収益額</th>
                  </tr>
                </thead>
                <tbody>
                  {activityEntries.slice(0, 20).map((entry, i) => (
                    <tr key={`${entry.date}-${entry.source}-${i}`} style={{ borderBottom: `1px solid ${isDark ? 'rgba(51,65,85,0.5)' : '#f1f5f9'}` }}>
                      <td style={{ padding: '8px 0', color: colors.textMuted, fontFamily: 'monospace', fontSize: 11, whiteSpace: 'nowrap' }}>{entry.date}</td>
                      <td style={{ padding: '8px 0', color: colors.text, fontWeight: 500 }}>{SOURCE_LABELS[entry.source] || entry.source}</td>
                      <td style={{ padding: '8px 0', color: colors.textSecondary }}>{entry.note || '-'}</td>
                      <td style={{ padding: '8px 0', textAlign: 'right', fontFamily: 'monospace', color: '#10b981' }}>{`\u00a5${(entry.amount ?? 0).toLocaleString()}`}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
