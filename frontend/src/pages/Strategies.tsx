import React, { useState, useEffect, useCallback, type CSSProperties } from 'react';
import { ChevronUp, ChevronDown, Info } from 'lucide-react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useStrategies } from '../hooks/useStrategies';
import type { StrategyParamValue, StrategyStatus, ThemeColors } from '../types';

const STATUS_TABS: { value: StrategyStatus | 'all'; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
  { value: 'archived', label: 'Archived' },
];

const STATUS_CONFIG: Record<StrategyStatus, { color: string; bgLight: string; bgDark: string; label: string }> = {
  active: { color: '#22c55e', bgLight: 'rgba(34,197,94,0.15)', bgDark: 'rgba(34,197,94,0.15)', label: 'Active' },
  inactive: { color: '#f59e0b', bgLight: 'rgba(245,158,11,0.1)', bgDark: 'rgba(245,158,11,0.15)', label: 'Inactive' },
  archived: { color: '#6b7280', bgLight: 'rgba(107,114,128,0.1)', bgDark: 'rgba(107,114,128,0.2)', label: 'Archived' },
};

export const Strategies: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const {
    strategies,
    loading,
    error,
    actionLoading,
    params,
    paramsLoading,
    setStatus,
    fetchParams,
    updateParams,
    refreshStrategies,
  } = useStrategies();

  const [statusFilter, setStatusFilter] = useState<StrategyStatus | 'all'>('all');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [editValues, setEditValues] = useState<Record<string, string>>({});
  const [saveStatus, setSaveStatus] = useState<{ id: string; message: string; isError: boolean } | null>(null);

  const handleExpand = (id: string) => {
    if (expandedId === id) {
      setExpandedId(null);
      return;
    }
    setExpandedId(id);
    if (!params[id]) {
      fetchParams(id);
    }
  };

  useEffect(() => {
    if (expandedId && params[expandedId]) {
      const values: Record<string, string> = {};
      for (const p of params[expandedId]) {
        values[p.key] = p.value;
      }
      setEditValues(values);
    }
  }, [expandedId, params]);

  const handleSave = async (id: string) => {
    const result = await updateParams(id, editValues);
    if (result.success) {
      setSaveStatus({ id, message: 'Saved', isError: false });
    } else {
      setSaveStatus({ id, message: result.errors?.join(', ') || 'Failed', isError: true });
    }
    setTimeout(() => setSaveStatus(null), 3000);
  };

  const cardStyle: CSSProperties = {
    backgroundColor: colors.cardBg,
    borderRadius: '12px',
    border: `1px solid ${colors.border}`,
    padding: isMobile ? '12px' : '14px',
    marginBottom: '10px',
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
  };

  if (loading && strategies.length === 0) {
    return (
      <div style={{ padding: isMobile ? '12px' : '16px' }}>
        <div style={cardStyle}>
          <div style={{ color: colors.textMuted, textAlign: 'center', padding: '20px' }}>Loading strategies...</div>
        </div>
      </div>
    );
  }

  const filteredStrategies = statusFilter === 'all'
    ? strategies
    : strategies.filter(s => s.status === statusFilter);

  const statusCounts = {
    all: strategies.length,
    active: strategies.filter(s => s.status === 'active').length,
    inactive: strategies.filter(s => s.status === 'inactive').length,
    archived: strategies.filter(s => s.status === 'archived').length,
  };

  return (
    <div style={{ padding: isMobile ? '12px' : '16px' }}>
      {/* Header */}
      <div style={cardStyle}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '8px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: isMobile ? '18px' : '20px', color: colors.text }}>Trading Strategies</h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: colors.textSecondary }}>
              {strategies.length} strategies / {statusCounts.active} active
            </p>
          </div>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            {error && <span style={{ fontSize: '12px', color: '#ef4444' }}>{error}</span>}
            <button
              onClick={refreshStrategies}
              style={{
                padding: '6px 12px', borderRadius: '4px', border: `1px solid ${colors.border}`,
                backgroundColor: colors.cardBg, color: colors.textSecondary, cursor: 'pointer', fontSize: '13px',
              }}
            >
              Refresh
            </button>
          </div>
        </div>

        {/* Status Tabs */}
        <div style={{ display: 'flex', gap: '4px', marginTop: '12px', flexWrap: 'wrap' }}>
          {STATUS_TABS.map(tab => {
            const isActive = statusFilter === tab.value;
            const count = statusCounts[tab.value];
            return (
              <button
                key={tab.value}
                onClick={() => setStatusFilter(tab.value)}
                style={{
                  padding: '6px 14px',
                  borderRadius: '6px',
                  border: `1px solid ${isActive ? '#3b82f6' : colors.border}`,
                  backgroundColor: isActive ? (isDark ? 'rgba(59,130,246,0.15)' : '#eff6ff') : 'transparent',
                  color: isActive ? '#3b82f6' : colors.textSecondary,
                  cursor: 'pointer',
                  fontSize: '13px',
                  fontWeight: isActive ? 600 : 400,
                }}
              >
                {tab.label} ({count})
              </button>
            );
          })}
        </div>
      </div>

      {/* Strategy Cards */}
      {filteredStrategies.length === 0 ? (
        <div style={cardStyle}>
          <div style={{ textAlign: 'center', padding: '40px 20px', color: colors.textMuted }}>
            <div style={{ fontSize: '32px', marginBottom: '8px' }}>
              {strategies.length === 0 ? 'No strategies configured' : `No ${statusFilter} strategies`}
            </div>
            <div style={{ fontSize: '14px' }}>
              {strategies.length === 0 ? 'Add strategies in config/strategies.yaml' : 'Try a different filter'}
            </div>
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {filteredStrategies.map(strategy => (
            <StrategyCard
              key={strategy.id}
              strategy={strategy}
              expanded={expandedId === strategy.id}
              params={params[strategy.id]}
              paramsLoading={paramsLoading === strategy.id}
              actionLoading={actionLoading === strategy.id}
              editValues={editValues}
              saveStatus={saveStatus?.id === strategy.id ? saveStatus : null}
              colors={colors}
              isDark={isDark}
              isMobile={isMobile}
              onExpand={() => handleExpand(strategy.id)}
              onSetStatus={(status) => setStatus(strategy.id, status)}
              onEditChange={(key, value) => setEditValues(prev => ({ ...prev, [key]: value }))}
              onSave={() => handleSave(strategy.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
};

interface StrategyCardProps {
  strategy: { id: string; name: string; description: string; app: string; status: StrategyStatus; active: boolean; exchange?: string; pairs?: string[]; param_count: number };
  expanded: boolean;
  params?: StrategyParamValue[];
  paramsLoading: boolean;
  actionLoading: boolean;
  editValues: Record<string, string>;
  saveStatus: { message: string; isError: boolean } | null;
  colors: ThemeColors;
  isDark: boolean;
  isMobile: boolean;
  onExpand: () => void;
  onSetStatus: (status: StrategyStatus) => void;
  onEditChange: (key: string, value: string) => void;
  onSave: () => void;
}

const StrategyCard: React.FC<StrategyCardProps> = ({
  strategy, expanded, params, paramsLoading, actionLoading,
  editValues, saveStatus, colors, isDark, isMobile,
  onExpand, onSetStatus, onEditChange, onSave,
}) => {
  const sc = STATUS_CONFIG[strategy.status];
  const borderColor = sc.color;
  const [hovered, setHovered] = useState(false);
  const handleMouseEnter = useCallback(() => setHovered(true), []);
  const handleMouseLeave = useCallback(() => setHovered(false), []);

  return (
    <div
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      style={{
        backgroundColor: colors.cardBg,
        borderRadius: '12px',
        border: `1px solid ${colors.border}`,
        borderLeft: `4px solid ${borderColor}`,
        opacity: actionLoading ? 0.6 : (strategy.status === 'inactive' || strategy.status === 'archived' ? 0.75 : 1),
        transition: 'transform 0.2s ease, box-shadow 0.2s ease, opacity 0.2s',
        transform: hovered ? 'translateY(-2px)' : 'translateY(0)',
        boxShadow: hovered
          ? (isDark ? '0 4px 12px rgba(0,0,0,0.4)' : '0 4px 12px rgba(0,0,0,0.15)')
          : (isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)'),
      }}
    >
      {/* Header */}
      <div
        onClick={onExpand}
        style={{
          padding: isMobile ? '10px' : '12px 14px',
          cursor: 'pointer',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <span style={{ fontWeight: 600, fontSize: isMobile ? '14px' : '15px', color: colors.accent }}>
                {strategy.name}
              </span>
              <span style={{
                fontSize: '11px', padding: '2px 8px', borderRadius: '10px',
                backgroundColor: isDark ? sc.bgDark : sc.bgLight,
                color: sc.color,
                fontWeight: 500,
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}>
                <span style={{
                  width: '6px', height: '6px', borderRadius: '50%',
                  backgroundColor: sc.color, display: 'inline-block',
                }} />
                {sc.label}
              </span>
              <span style={{
                fontSize: '11px', padding: '2px 6px', borderRadius: '4px',
                backgroundColor: isDark ? 'rgba(59,130,246,0.15)' : 'rgba(59,130,246,0.1)',
                color: '#3b82f6',
              }}>
                {strategy.app}
              </span>
            </div>
            {(strategy.exchange || (strategy.pairs && strategy.pairs.length > 0)) && (
              <div style={{ display: 'flex', gap: '4px', marginTop: '4px', flexWrap: 'wrap' }}>
                {strategy.exchange && (
                  <span style={{
                    fontSize: '10px', padding: '1px 6px', borderRadius: '4px',
                    backgroundColor: isDark ? 'rgba(168,85,247,0.15)' : 'rgba(168,85,247,0.1)',
                    color: '#a855f7', fontWeight: 500,
                  }}>
                    {strategy.exchange}
                  </span>
                )}
                {strategy.pairs?.map(pair => (
                  <span key={pair} style={{
                    fontSize: '10px', padding: '1px 6px', borderRadius: '4px',
                    backgroundColor: isDark ? 'rgba(20,184,166,0.15)' : 'rgba(20,184,166,0.1)',
                    color: '#14b8a6', fontWeight: 500,
                  }}>
                    {pair}
                  </span>
                ))}
              </div>
            )}
            <div style={{ fontSize: '12px', color: colors.textSecondary, marginTop: '2px' }}>
              {strategy.description}
            </div>
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontSize: '12px', color: colors.textMuted }}>
            {strategy.param_count} params
          </span>
          <span style={{ display: 'inline-flex', color: colors.textMuted }}>
            {expanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </span>
        </div>
      </div>

      {/* Toggle + Params (expanded) */}
      {expanded && (
        <div style={{ padding: `0 ${isMobile ? '10px' : '14px'} ${isMobile ? '10px' : '14px'}`, borderTop: `1px solid ${colors.border}` }}>
          {/* Status controls */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 0', borderBottom: `1px solid ${colors.border}`, flexWrap: 'wrap' }}>
            {strategy.status !== 'active' && (
              <button
                onClick={(e) => { e.stopPropagation(); onSetStatus('active'); }}
                disabled={actionLoading}
                style={{
                  padding: '6px 16px', borderRadius: '8px', border: 'none', cursor: actionLoading ? 'default' : 'pointer',
                  fontSize: '13px', fontWeight: 500, backgroundColor: '#22c55e', color: '#fff',
                  transition: 'opacity 0.2s ease',
                }}
              >
                Activate
              </button>
            )}
            {strategy.status === 'active' && (
              <button
                onClick={(e) => { e.stopPropagation(); onSetStatus('inactive'); }}
                disabled={actionLoading}
                style={{
                  padding: '6px 16px', borderRadius: '8px', border: 'none', cursor: actionLoading ? 'default' : 'pointer',
                  fontSize: '13px', fontWeight: 500, backgroundColor: '#f59e0b', color: '#fff',
                  transition: 'opacity 0.2s ease',
                }}
              >
                Deactivate
              </button>
            )}
            {strategy.status !== 'archived' && (
              <button
                onClick={(e) => { e.stopPropagation(); onSetStatus('archived'); }}
                disabled={actionLoading}
                style={{
                  padding: '6px 16px', borderRadius: '8px', border: `1px solid ${colors.border}`, cursor: actionLoading ? 'default' : 'pointer',
                  fontSize: '13px', fontWeight: 500, backgroundColor: 'transparent', color: '#6b7280',
                  transition: 'opacity 0.2s ease',
                }}
              >
                Archive
              </button>
            )}
          </div>

          {/* Params */}
          <div style={{ paddingTop: '10px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: colors.text, marginBottom: '6px' }}>
              Parameters (1Password)
            </div>
            {paramsLoading ? (
              <div style={{ color: colors.textMuted, fontSize: '13px', padding: '8px 0' }}>Loading parameters...</div>
            ) : params ? (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr', gap: '6px' }}>
                  {params.map(p => (
                    <div key={p.key} style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                      <label style={{ fontSize: '12px', color: colors.textSecondary, fontWeight: 500, display: 'flex', alignItems: 'center', gap: '4px' }}>
                        {p.label}
                        {p.description && (
                          <span
                            title={p.description}
                            style={{
                              display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
                              cursor: 'help', flexShrink: 0, color: colors.textMuted,
                            }}
                          >
                            <Info size={14} />
                          </span>
                        )}
                      </label>
                      <input
                        type={p.type === 'number' ? 'number' : 'text'}
                        value={editValues[p.key] ?? p.value}
                        placeholder={p.placeholder || (p.default ? `デフォルト: ${p.default}` : '')}
                        onChange={e => onEditChange(p.key, e.target.value)}
                        min={p.min}
                        max={p.max}
                        step={p.step}
                        style={{
                          padding: '6px 10px', borderRadius: '6px',
                          border: `1px solid ${colors.inputBorder}`,
                          backgroundColor: colors.inputBg,
                          color: colors.text,
                          fontSize: '13px',
                          outline: 'none',
                          transition: 'border-color 0.2s ease, box-shadow 0.2s ease',
                        }}
                        onFocus={e => {
                          e.currentTarget.style.borderColor = colors.accent;
                          e.currentTarget.style.boxShadow = `0 0 0 3px ${colors.accent}33`;
                        }}
                        onBlur={e => {
                          e.currentTarget.style.borderColor = colors.inputBorder;
                          e.currentTarget.style.boxShadow = 'none';
                        }}
                      />
                    </div>
                  ))}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginTop: '10px' }}>
                  <button
                    onClick={onSave}
                    disabled={actionLoading}
                    style={{
                      padding: '8px 24px', borderRadius: '8px', border: 'none',
                      backgroundColor: colors.accent, color: '#fff', cursor: actionLoading ? 'default' : 'pointer',
                      fontSize: '13px', fontWeight: 600,
                      transition: 'background-color 0.2s ease, transform 0.1s ease',
                    }}
                    onMouseEnter={e => {
                      e.currentTarget.style.backgroundColor = colors.accentHover;
                    }}
                    onMouseLeave={e => {
                      e.currentTarget.style.backgroundColor = colors.accent;
                    }}
                  >
                    Save to 1Password
                  </button>
                  {saveStatus && (
                    <span style={{ fontSize: '12px', color: saveStatus.isError ? '#ef4444' : '#22c55e' }}>
                      {saveStatus.message}
                    </span>
                  )}
                </div>
              </>
            ) : (
              <div style={{ color: colors.textMuted, fontSize: '13px' }}>No parameters available</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
