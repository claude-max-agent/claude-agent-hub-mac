import React, { useState, useEffect, useCallback, type CSSProperties } from 'react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useModal } from '../contexts/ModalContext';
import { api } from '../api/client';
import type { Task, TmuxSession, AgentInfo, TeamLeadInfo, TeamLeadStatus, PoolStatus } from '../types';

interface TeamWithStatus extends TeamLeadInfo {
  status?: TeamLeadStatus;
}

interface UnifiedSession extends TmuxSession {
  team?: TeamWithStatus;
}

const SessionCard: React.FC<{
  children: React.ReactNode;
  style: CSSProperties;
  isDark: boolean;
}> = ({ children, style, isDark }) => {
  const [hovered, setHovered] = useState(false);
  return (
    <div
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        ...style,
        transition: 'transform 0.15s ease, box-shadow 0.15s ease',
        transform: hovered ? 'translateY(-1px)' : 'translateY(0)',
        boxShadow: hovered
          ? (isDark ? '0 4px 12px rgba(0,0,0,0.4)' : '0 4px 12px rgba(0,0,0,0.12)')
          : (isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.08)'),
      }}
    >
      {children}
    </div>
  );
};

export const ActiveSessions: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const { showConfirm } = useModal();

  // Sessions state
  const [sessions, setSessions] = useState<TmuxSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(true);
  const [copiedName, setCopiedName] = useState<string | null>(null);

  // Teams state
  const [teams, setTeams] = useState<TeamWithStatus[]>([]);
  const [teamsLoading, setTeamsLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({});

  // Tasks state (for showing assigned task summary on pool sessions)
  const [tasksByAssignee, setTasksByAssignee] = useState<Record<string, Task>>({});

  // Provider-Model mapping
  const PROVIDER_MODELS: Record<string, { label: string; models: { id: string; name: string }[] }> = {
    anthropic: {
      label: 'Anthropic',
      models: [
        { id: 'claude-opus-4-6', name: 'Opus 4.6 · Most capable for complex work' },
        { id: 'claude-sonnet-4-6', name: 'Sonnet 4.6 · Best for everyday tasks' },
        { id: 'claude-haiku-4-5-20251001', name: 'Haiku 4.5 · Fastest for quick answers' },
      ],
    },
    openai: {
      label: 'OpenAI',
      models: [
        { id: 'gpt-5.4', name: 'GPT-5.4 · Latest frontier agentic coding model' },
        { id: 'gpt-5.4-mini', name: 'GPT-5.4 Mini · Smaller frontier agentic coding model' },
        { id: 'gpt-5.3-codex', name: 'GPT-5.3 Codex · Frontier Codex-optimized' },
        { id: 'gpt-5.2-codex', name: 'GPT-5.2 Codex · Frontier agentic coding model' },
        { id: 'gpt-5.2', name: 'GPT-5.2 · Optimized for professional work' },
        { id: 'gpt-5.1-codex-max', name: 'GPT-5.1 Codex Max · Deep and fast reasoning' },
        { id: 'gpt-5.1-codex-mini', name: 'GPT-5.1 Codex Mini · Cheaper, faster' },
      ],
    },
  };

  const REASONING_EFFORTS = [
    { id: '', name: 'Default' },
    { id: 'low', name: 'Low · Quick answers' },
    { id: 'medium', name: 'Medium · Balanced' },
    { id: 'high', name: 'High · Deep thinking' },
  ];

  // Provider → CLI type (auto-derived)
  const PROVIDER_CLI: Record<string, string> = { anthropic: 'claude', openai: 'codex' };

  // Create form state
  const [showForm, setShowForm] = useState(false);
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [agentsLoaded, setAgentsLoaded] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState('anthropic');
  const [selectedModel, setSelectedModel] = useState('claude-opus-4-6');
  const [selectedEffort, setSelectedEffort] = useState('');
  const [selectedAgent, setSelectedAgent] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [poolStatus, setPoolStatus] = useState<PoolStatus | null>(null);
  const [description, setDescription] = useState('');

  // Fetch sessions
  const fetchSessions = useCallback(async () => {
    try {
      const data = await api.getSessions();
      setSessions(data);
    } catch {
      setSessions([]);
    } finally {
      setSessionsLoading(false);
    }
  }, []);

  // Fetch teams
  const fetchTeams = useCallback(async () => {
    try {
      const data = await api.getTeamLeads();
      const leads: TeamLeadInfo[] = data.team_leads || [];
      const teamsWithStatus = await Promise.all(
        leads.map(async (lead) => {
          try {
            const status = await api.getTeamLeadStatus(lead.name);
            return { ...lead, status };
          } catch {
            return { ...lead, status: undefined };
          }
        })
      );
      teamsWithStatus.sort((a, b) => a.name.localeCompare(b.name));
      setTeams(teamsWithStatus);
    } catch {
      setTeams([]);
    } finally {
      setTeamsLoading(false);
    }
  }, []);

  // Fetch active tasks to show assigned task summary on session cards
  const fetchTasks = useCallback(async () => {
    try {
      const data = await api.getTasks(1, 100);
      const tasks: Task[] = data?.tasks || [];
      const map: Record<string, Task> = {};
      for (const t of tasks) {
        if (t.assigned_to && (t.status === 'assigned' || t.status === 'in_progress')) {
          map[t.assigned_to] = t;
        }
      }
      setTasksByAssignee(map);
    } catch {
      setTasksByAssignee({});
    }
  }, []);

  // Fetch pool status
  const fetchPoolStatus = useCallback(async () => {
    try {
      const data = await api.getPoolStatus();
      setPoolStatus(data);
    } catch {
      setPoolStatus(null);
    }
  }, []);

  useEffect(() => {
    fetchSessions();
    fetchTeams();
    fetchTasks();
    fetchPoolStatus();
    const interval = setInterval(() => {
      fetchSessions();
      fetchTeams();
      fetchTasks();
      fetchPoolStatus();
    }, 10_000);
    return () => clearInterval(interval);
  }, [fetchSessions, fetchTeams, fetchTasks, fetchPoolStatus]);

  // Build unified session list: tmux sessions as base, team info overlaid
  const buildUnifiedSessions = useCallback((): UnifiedSession[] => {
    // Map team tmux_session names
    const teamBySession = new Map<string, TeamWithStatus>();
    for (const t of teams) {
      if (t.tmux_session) {
        teamBySession.set(t.tmux_session, t);
      }
    }

    // Overlay team info on running tmux sessions
    const unified: UnifiedSession[] = sessions.map(s => ({
      ...s,
      team: teamBySession.get(s.name),
    }));

    // Add offline teams that don't have a running tmux session
    const runningNames = new Set(sessions.map(s => s.name));
    for (const t of teams) {
      if (t.tmux_session && !runningNames.has(t.tmux_session)) {
        unified.push({
          name: t.tmux_session,
          created: 0,
          attached: false,
          status: 'offline',
          agent: '',
          cli_type: 'claude',
          team: t,
        });
      }
    }

    // Sort: running first, then by name
    unified.sort((a, b) => {
      const aRunning = a.status !== 'stopped' && a.status !== 'offline' ? 0 : 1;
      const bRunning = b.status !== 'stopped' && b.status !== 'offline' ? 0 : 1;
      if (aRunning !== bRunning) return aRunning - bRunning;
      return a.name.localeCompare(b.name);
    });

    return unified;
  }, [sessions, teams]);

  const unifiedSessions = buildUnifiedSessions();
  const runningSessions = unifiedSessions.filter(s => s.status !== 'stopped' && s.status !== 'offline');

  // Copy tmux attach command
  const handleCopy = async (name: string) => {
    try {
      await navigator.clipboard.writeText(`tmux attach -t ${name}`);
      setCopiedName(name);
      setTimeout(() => setCopiedName(null), 1500);
    } catch {}
  };

  // Open create form
  const handleOpenForm = async () => {
    setError(null);
    setDescription('');
    setAgentsLoaded(false);
    setShowForm(true);
    setSelectedProvider('anthropic');
    const models = PROVIDER_MODELS['anthropic']?.models || [];
    if (models.length > 0) setSelectedModel(models[0].id);
    setSelectedEffort('');
    await fetchPoolStatus();
    try {
      const agentList = await api.getSessionAgents();
      setAgents(agentList);
      setAgentsLoaded(true);
      if (agentList.length > 0 && !selectedAgent) {
        setSelectedAgent(agentList[0].name);
      }
    } catch {
      setAgentsLoaded(true);
      setError('Failed to load agents');
    }
  };

  // Create session (pool-aware, name auto-assigned by backend)
  const handleCreate = async () => {
    const cliType = PROVIDER_CLI[selectedProvider] || 'claude';
    if (cliType === 'claude' && !selectedAgent) return;
    if (poolStatus && poolStatus.pooled === 0) {
      setError('利用可能なスロットがありません（プール満タン）');
      return;
    }
    setCreating(true);
    setError(null);
    try {
      const agent = cliType === 'codex' ? '' : selectedAgent;
      const effort = selectedProvider === 'anthropic' ? selectedEffort : '';
      await api.createSession(agent, cliType, selectedModel, effort, description);
      setDescription('');
      setShowForm(false);
      await Promise.all([fetchSessions(), fetchPoolStatus()]);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create session');
    } finally {
      setCreating(false);
    }
  };

  // Start team
  const handleStartTeam = (name: string) => {
    showConfirm({
      message: `Team "${name}" を起動しますか？`,
      confirmText: 'Start',
      onConfirm: async () => {
        setActionLoading(prev => ({ ...prev, [name]: true }));
        try {
          await api.startTeamLead(name);
          await Promise.all([fetchTeams(), fetchSessions()]);
        } catch (e) {
          setError(e instanceof Error ? e.message : 'Failed to start team');
        } finally {
          setActionLoading(prev => ({ ...prev, [name]: false }));
        }
      },
    });
  };

  const handleRestartSession = (name: string) => {
    showConfirm({
      message: `${name} を再起動しますか？`,
      confirmText: 'Restart',
      onConfirm: async () => {
        setActionLoading(prev => ({ ...prev, [name]: true }));
        try {
          await api.restartSession(name);
          await Promise.all([fetchSessions(), fetchTeams(), fetchPoolStatus()]);
        } catch (e) {
          setError(e instanceof Error ? e.message : 'Failed to restart session');
        } finally {
          setActionLoading(prev => ({ ...prev, [name]: false }));
        }
      },
    });
  };

  const loading = sessionsLoading || teamsLoading;

  // Styles
  const cardStyle: CSSProperties = {
    borderRadius: '12px',
    border: `1px solid ${colors.border}`,
    backgroundColor: colors.cardBg,
    padding: isMobile ? '16px' : '20px',
  };

  const badgeStyle = (color: string, bgLight: string, bgDark: string): CSSProperties => ({
    display: 'inline-flex',
    alignItems: 'center',
    padding: '2px 8px',
    borderRadius: '6px',
    fontSize: '11px',
    fontWeight: 600,
    color,
    backgroundColor: isDark ? bgDark : bgLight,
  });

  const inputStyle: CSSProperties = {
    width: '100%',
    padding: '8px 12px',
    borderRadius: '8px',
    border: `1px solid ${colors.inputBorder}`,
    backgroundColor: colors.inputBg,
    color: colors.text,
    fontSize: '13px',
    outline: 'none',
  };

  const btnPrimary: CSSProperties = {
    padding: '8px 16px',
    borderRadius: '8px',
    border: 'none',
    backgroundColor: colors.accent,
    color: '#fff',
    fontSize: '13px',
    fontWeight: 600,
    cursor: 'pointer',
  };

  const btnSecondary: CSSProperties = {
    padding: '8px 16px',
    borderRadius: '8px',
    border: `1px solid ${colors.border}`,
    backgroundColor: 'transparent',
    color: colors.textSecondary,
    fontSize: '13px',
    fontWeight: 500,
    cursor: 'pointer',
  };

  const copyBtnStyle = (name: string): CSSProperties => ({
    padding: '2px 8px',
    borderRadius: '4px',
    border: 'none',
    fontSize: '11px',
    fontWeight: 500,
    cursor: 'pointer',
    backgroundColor: copiedName === name
      ? (isDark ? 'rgba(34,197,94,0.2)' : 'rgba(34,197,94,0.1)')
      : (isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.06)'),
    color: copiedName === name
      ? (isDark ? '#86efac' : '#16a34a')
      : colors.textSecondary,
  });

  const isRunning = (s: UnifiedSession) => s.status !== 'stopped' && s.status !== 'offline';

  const getStatusColor = (s: UnifiedSession): string => {
    if (s.status === 'offline' || s.status === 'stopped') return isDark ? '#ef4444' : '#dc2626';
    if (s.pool_status === 'pooled') return isDark ? '#60a5fa' : '#3b82f6';
    if (isRunning(s)) return isDark ? '#4ade80' : '#22c55e';
    return isDark ? '#475569' : '#cbd5e1';
  };

  const getStatusLabel = (s: UnifiedSession): string => {
    if (s.status === 'offline') return 'offline';
    if (s.status === 'stopped') return 'stopped';
    return 'running';
  };

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '24px', flexWrap: 'wrap', gap: '12px' }}>
        <div>
          <h1 style={{ fontSize: '24px', fontWeight: 700, color: colors.text, margin: 0 }}>
            Tmux Sessions
          </h1>
          <p style={{ fontSize: '14px', color: colors.textMuted, marginTop: '4px' }}>
            {runningSessions.length} sessions running
          </p>
          {poolStatus && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
              <div style={{
                display: 'flex', height: '6px', borderRadius: '3px', overflow: 'hidden',
                width: '120px', backgroundColor: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.08)',
              }}>
                {poolStatus.running > 0 && (
                  <div style={{ width: `${(poolStatus.running / poolStatus.total) * 100}%`, backgroundColor: isDark ? '#60a5fa' : '#3b82f6' }} />
                )}
                {poolStatus.pooled > 0 && (
                  <div style={{ width: `${(poolStatus.pooled / poolStatus.total) * 100}%`, backgroundColor: isDark ? '#4ade80' : '#22c55e' }} />
                )}
                {poolStatus.stopping > 0 && (
                  <div style={{ width: `${(poolStatus.stopping / poolStatus.total) * 100}%`, backgroundColor: isDark ? '#fbbf24' : '#f59e0b' }} />
                )}
              </div>
              <span style={{ fontSize: '12px', color: colors.textMuted }}>
                {poolStatus.pooled} free / {poolStatus.running} active / {poolStatus.total} total
              </span>
            </div>
          )}
        </div>
        <div style={{ display: 'flex', gap: '8px' }}>
          {!showForm && (
            <button onClick={handleOpenForm} style={btnPrimary}>
              + New Session
            </button>
          )}
          <a
            href="https://status.anthropic.com"
            target="_blank"
            rel="noopener noreferrer"
            style={{ fontSize: '11px', color: colors.textMuted, textDecoration: 'none', padding: '4px 8px', borderRadius: '4px', border: `1px solid ${colors.border}` }}
          >
            Anthropic Status
          </a>
          <a
            href="https://status.openai.com"
            target="_blank"
            rel="noopener noreferrer"
            style={{ fontSize: '11px', color: colors.textMuted, textDecoration: 'none', padding: '4px 8px', borderRadius: '4px', border: `1px solid ${colors.border}` }}
          >
            OpenAI Status
          </a>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div style={{
          padding: '12px 16px',
          borderRadius: '8px',
          backgroundColor: isDark ? 'rgba(239,68,68,0.1)' : '#fef2f2',
          border: `1px solid ${isDark ? 'rgba(239,68,68,0.3)' : '#fecaca'}`,
          color: isDark ? '#fca5a5' : '#dc2626',
          fontSize: '13px',
          marginBottom: '16px',
        }}>
          {error}
        </div>
      )}

      {/* Create form */}
      {showForm && (
        <div style={{ ...cardStyle, marginBottom: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
            <h3 style={{ fontSize: '14px', fontWeight: 600, color: colors.text, margin: 0 }}>
              New Session
            </h3>
            {poolStatus && (
              <span style={badgeStyle(
                poolStatus.pooled > 0
                  ? (isDark ? '#86efac' : '#16a34a')
                  : (isDark ? '#fca5a5' : '#dc2626'),
                poolStatus.pooled > 0 ? 'rgba(22,163,74,0.1)' : 'rgba(239,68,68,0.1)',
                poolStatus.pooled > 0 ? 'rgba(22,163,74,0.15)' : 'rgba(239,68,68,0.15)',
              )}>
                {poolStatus.pooled} / {poolStatus.total} slots available
              </span>
            )}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxWidth: '480px' }}>
            {/* Description (task summary) */}
            <div>
              <label style={{ fontSize: '12px', fontWeight: 600, color: colors.textSecondary, marginBottom: '4px', display: 'block' }}>Description</label>
              <input
                type="text"
                value={description}
                onChange={e => setDescription(e.target.value)}
                placeholder="仕事の概要（例: Issue #123 の修正）"
                style={inputStyle}
              />
            </div>

            {/* Provider / Model row */}
            <div style={{ display: 'flex', gap: '8px' }}>
              <div style={{ flex: 1 }}>
                <label style={{ fontSize: '12px', fontWeight: 600, color: colors.textSecondary, marginBottom: '4px', display: 'block' }}>Provider</label>
                <select
                  value={selectedProvider}
                  onChange={e => {
                    const p = e.target.value;
                    setSelectedProvider(p);
                    const models = PROVIDER_MODELS[p]?.models || [];
                    if (models.length > 0) setSelectedModel(models[0].id);
                    setSelectedEffort('');
                  }}
                  style={inputStyle}
                >
                  {Object.entries(PROVIDER_MODELS).map(([id, { label }]) => (
                    <option key={id} value={id}>{label}</option>
                  ))}
                </select>
              </div>
              <div style={{ flex: 2 }}>
                <label style={{ fontSize: '12px', fontWeight: 600, color: colors.textSecondary, marginBottom: '4px', display: 'block' }}>Model</label>
                <select
                  value={selectedModel}
                  onChange={e => setSelectedModel(e.target.value)}
                  style={inputStyle}
                >
                  {(PROVIDER_MODELS[selectedProvider]?.models || []).map(m => (
                    <option key={m.id} value={m.id}>{m.name}</option>
                  ))}
                </select>
              </div>
            </div>

            {/* Reasoning Effort (Anthropic only) */}
            {selectedProvider === 'anthropic' && (
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: colors.textSecondary, marginBottom: '4px', display: 'block' }}>Reasoning Effort</label>
                <select
                  value={selectedEffort}
                  onChange={e => setSelectedEffort(e.target.value)}
                  style={inputStyle}
                >
                  {REASONING_EFFORTS.map(e => (
                    <option key={e.id} value={e.id}>{e.name}</option>
                  ))}
                </select>
              </div>
            )}

            {/* Agent (Anthropic/Claude only) */}
            {selectedProvider === 'anthropic' && (
              <div>
                <label style={{ fontSize: '12px', fontWeight: 600, color: colors.textSecondary, marginBottom: '4px', display: 'block' }}>Agent</label>
                <select
                  value={selectedAgent}
                  onChange={e => setSelectedAgent(e.target.value)}
                  style={inputStyle}
                >
                  {!agentsLoaded && <option value="">Loading...</option>}
                  {agentsLoaded && agents.length === 0 && <option value="">No agents found (~/.claude/agents/)</option>}
                  {agents.map(a => (
                    <option key={a.name} value={a.name}>{a.name}</option>
                  ))}
                </select>
              </div>
            )}

            {/* Actions */}
            <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
              <button
                onClick={handleCreate}
                disabled={creating || (selectedProvider === 'anthropic' && !selectedAgent) || (poolStatus !== null && poolStatus.pooled === 0)}
                style={{ ...btnPrimary, opacity: creating || (poolStatus !== null && poolStatus.pooled === 0) ? 0.5 : 1 }}
              >
                {creating ? 'Creating...' : poolStatus && poolStatus.pooled === 0 ? 'No Slots' : 'Create'}
              </button>
              <button onClick={() => { setShowForm(false); setDescription(''); setError(null); }} style={btnSecondary}>
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div style={{ textAlign: 'center', padding: '40px', color: colors.textMuted }}>
          Loading...
        </div>
      )}

      {/* Unified Tmux Sessions */}
      {!loading && (
        <div>
          {unifiedSessions.length === 0 ? (
            <div style={{ ...cardStyle, textAlign: 'center', padding: '32px', color: colors.textMuted }}>
              No tmux sessions
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(auto-fill, minmax(380px, 1fr))', gap: '10px' }}>
              {unifiedSessions.map(s => {
                const running = isRunning(s);
                const team = s.team;
                const teamName = team?.name;
                const isTeamLoading = teamName ? actionLoading[teamName] : false;
                const assignedTask = tasksByAssignee[s.name];

                return (
                  <SessionCard key={s.name} isDark={isDark} style={{
                    ...cardStyle,
                    padding: isMobile ? '14px' : '16px 20px',
                    borderColor: (s.status === 'offline' || s.status === 'stopped')
                      ? (isDark ? 'rgba(239,68,68,0.35)' : 'rgba(239,68,68,0.4)')
                      : s.pool_status === 'pooled'
                        ? (isDark ? 'rgba(96,165,250,0.3)' : 'rgba(59,130,246,0.3)')
                        : running
                          ? (isDark ? 'rgba(74,222,128,0.3)' : 'rgba(34,197,94,0.4)')
                          : colors.border,
                    opacity: (s.status === 'offline' || s.status === 'stopped') ? 0.7 : 1,
                  }}>
                    {/* Row 1: Name + status + actions */}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                        {/* Status dot */}
                        <span style={{
                          width: '8px', height: '8px', borderRadius: '50%', flexShrink: 0,
                          backgroundColor: getStatusColor(s),
                        }} />
                        {/* Session name + description alias */}
                        <span style={{ fontSize: '15px', fontWeight: 600, color: colors.text, fontFamily: 'monospace' }}>
                          {s.name}
                        </span>
                        {/* Status label */}
                        <span style={badgeStyle(
                          (s.status === 'offline' || s.status === 'stopped')
                            ? (isDark ? '#fca5a5' : '#dc2626')
                            : s.pool_status === 'pooled'
                              ? (isDark ? '#93c5fd' : '#2563eb')
                              : running
                                ? (isDark ? '#86efac' : '#16a34a')
                                : (isDark ? '#94a3b8' : '#64748b'),
                          (s.status === 'offline' || s.status === 'stopped')
                            ? 'rgba(239,68,68,0.1)'
                            : s.pool_status === 'pooled'
                              ? 'rgba(37,99,235,0.1)'
                              : running
                                ? 'rgba(22,163,74,0.1)'
                                : 'rgba(100,116,139,0.1)',
                          (s.status === 'offline' || s.status === 'stopped')
                            ? 'rgba(239,68,68,0.15)'
                            : s.pool_status === 'pooled'
                              ? 'rgba(37,99,235,0.15)'
                              : running
                                ? 'rgba(22,163,74,0.15)'
                                : 'rgba(100,116,139,0.15)',
                        )}>
                          {getStatusLabel(s)}
                        </span>
                      </div>

                      {/* Actions */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
                        {running && (
                          <button onClick={() => handleCopy(s.name)} style={copyBtnStyle(s.name)}>
                            {copiedName === s.name ? 'Copied!' : 'Copy attach'}
                          </button>
                        )}
                        {running && s.pool_status !== 'pooled' && (
                          <button
                            onClick={() => handleRestartSession(s.name)}
                            disabled={Boolean(actionLoading[s.name])}
                            style={{
                              ...btnSecondary,
                              padding: '6px 12px',
                              fontSize: '12px',
                              backgroundColor: isDark ? 'rgba(245,158,11,0.14)' : 'rgba(245,158,11,0.1)',
                              borderColor: isDark ? 'rgba(245,158,11,0.35)' : 'rgba(245,158,11,0.35)',
                              color: isDark ? '#fcd34d' : '#b45309',
                              opacity: actionLoading[s.name] ? 0.6 : 1,
                            }}
                          >
                            {actionLoading[s.name] ? '...' : 'Restart'}
                          </button>
                        )}
                        {team && !running ? (
                          <button
                            onClick={() => handleStartTeam(teamName!)}
                            disabled={isTeamLoading}
                            style={{
                              ...btnPrimary,
                              padding: '6px 12px', fontSize: '12px',
                              opacity: isTeamLoading ? 0.5 : 1,
                            }}
                          >
                            {isTeamLoading ? '...' : 'Start'}
                          </button>
                        ) : null}
                      </div>
                    </div>

                    {/* Description (prominent) */}
                    {(s.description || (s.team && s.team.description)) && (
                      <div style={{
                        marginTop: '6px', padding: '8px 12px',
                        borderRadius: '8px',
                        backgroundColor: isDark ? 'rgba(99,102,241,0.12)' : 'rgba(99,102,241,0.06)',
                        border: `1px solid ${isDark ? 'rgba(99,102,241,0.25)' : 'rgba(99,102,241,0.15)'}`,
                      }}>
                        <p style={{
                          fontSize: '13px', fontWeight: 500,
                          color: isDark ? '#c7d2fe' : '#4338ca',
                          margin: 0,
                          lineHeight: '1.4',
                          overflow: 'hidden', textOverflow: 'ellipsis',
                          display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical',
                        }}>
                          {s.description || s.team?.description}
                        </p>
                        {s.issue_number && (
                          <span style={{
                            fontSize: '11px', color: isDark ? '#a5b4fc' : '#6366f1',
                            fontFamily: 'monospace', marginTop: '4px', display: 'inline-block',
                          }}>
                            #{s.issue_number}
                          </span>
                        )}
                      </div>
                    )}

                    {/* Row 2: Badges */}
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap', paddingLeft: '16px' }}>
                      {/* Pool status badge */}
                      {s.pool_status && s.pool_status !== 'running' && (
                        <span style={badgeStyle(
                          s.pool_status === 'pooled'
                            ? (isDark ? '#86efac' : '#16a34a')
                            : s.pool_status === 'persistent'
                              ? (isDark ? '#c4b5fd' : '#7c3aed')
                              : (isDark ? '#fbbf24' : '#d97706'),
                          s.pool_status === 'pooled'
                            ? 'rgba(22,163,74,0.1)'
                            : s.pool_status === 'persistent'
                              ? 'rgba(139,92,246,0.1)'
                              : 'rgba(217,119,6,0.1)',
                          s.pool_status === 'pooled'
                            ? 'rgba(22,163,74,0.15)'
                            : s.pool_status === 'persistent'
                              ? 'rgba(139,92,246,0.15)'
                              : 'rgba(217,119,6,0.15)',
                        )}>
                          {s.pool_status}
                        </span>
                      )}
                      {/* CLI type badge */}
                      {s.cli_type && (
                        <span style={badgeStyle(
                          s.cli_type === 'codex'
                            ? (isDark ? '#fcd34d' : '#d97706')
                            : (isDark ? '#93c5fd' : '#2563eb'),
                          s.cli_type === 'codex' ? 'rgba(217,119,6,0.1)' : 'rgba(37,99,235,0.1)',
                          s.cli_type === 'codex' ? 'rgba(217,119,6,0.15)' : 'rgba(37,99,235,0.15)',
                        )}>
                          {s.cli_type}
                        </span>
                      )}
                      {/* Team badge (hide for manager — obvious from session name) */}
                      {team && teamName !== 'manager' && (
                        <span style={badgeStyle(
                          isDark ? '#c4b5fd' : '#7c3aed',
                          'rgba(139,92,246,0.1)',
                          'rgba(139,92,246,0.15)',
                        )}>
                          team: {teamName}
                        </span>
                      )}
                      {/* Model badge */}
                      {team?.status?.model && (
                        <span style={badgeStyle(
                          isDark ? '#fbbf24' : '#b45309',
                          'rgba(180,83,9,0.1)',
                          'rgba(251,191,36,0.15)',
                        )}>
                          {team.status.model}
                        </span>
                      )}
                      {/* Panes badge (hide for manager — single-pane permanent session) */}
                      {running && team?.status?.panes && s.name !== 'manager' && (
                        <span style={badgeStyle(
                          isDark ? '#93c5fd' : '#2563eb',
                          'rgba(37,99,235,0.1)',
                          'rgba(37,99,235,0.15)',
                        )}>
                          {team.status.panes} panes
                        </span>
                      )}
                      {/* Agent name */}
                      {s.agent && (
                        <span style={{ fontSize: '12px', color: isDark ? '#a78bfa' : '#7c3aed' }}>
                          {s.agent}
                        </span>
                      )}
                    </div>

                    {/* Assigned task summary */}
                    {assignedTask && (
                      <div style={{
                        marginTop: '4px', paddingLeft: '16px',
                        padding: '8px 12px 8px 16px',
                        borderRadius: '8px',
                        backgroundColor: isDark ? 'rgba(59,130,246,0.08)' : 'rgba(59,130,246,0.05)',
                        border: `1px solid ${isDark ? 'rgba(59,130,246,0.15)' : 'rgba(59,130,246,0.12)'}`,
                      }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '2px' }}>
                          <span style={badgeStyle(
                            isDark ? '#93c5fd' : '#2563eb',
                            'rgba(37,99,235,0.1)',
                            'rgba(37,99,235,0.15)',
                          )}>
                            {assignedTask.status === 'in_progress' ? 'working' : 'assigned'}
                          </span>
                          {assignedTask.priority && (
                            <span style={{ fontSize: '11px', color: colors.textMuted }}>
                              {assignedTask.priority}
                            </span>
                          )}
                        </div>
                        <p style={{
                          fontSize: '12px', color: colors.text, margin: '4px 0 0 0',
                          overflow: 'hidden', textOverflow: 'ellipsis',
                          display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical',
                        }}>
                          {assignedTask.description}
                        </p>
                      </div>
                    )}


                  </SessionCard>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
