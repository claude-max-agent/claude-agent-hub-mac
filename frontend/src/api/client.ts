import type { Task, GitHubAPISummary, StrategyListItem, StrategyStatus, TmuxSession, AgentInfo, CliTypeInfo, JobStatus, LLMProvider, AgentProviderConfig, PoolStatus, TradingSchedule } from '../types';

export const api = {
  getVersion: () => fetch('/api/v1/version').then(r => r.json()),
  getAgents: () => fetch('/api/v1/agents').then(r => r.json()),
  getTasks: (page = 1, perPage = 20, showArchived = false) => fetch(`/api/v1/tasks?page=${page}&per_page=${perPage}${showArchived ? '&show_archived=true' : ''}`).then(r => r.json()),
  createTask: (task: Partial<Task>) =>
    fetch('/api/v1/tasks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(task),
    }).then(r => r.json()),
  cancelTask: (taskId: string) =>
    fetch(`/api/v1/tasks/${taskId}`, { method: 'DELETE' }).then(r => r.json()),
  completeTask: (taskId: string) =>
    fetch(`/api/v1/tasks/${taskId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status: 'completed' }),
    }).then(r => r.json()),
  archiveTask: (taskId: string) =>
    fetch(`/api/v1/tasks/${taskId}/archive`, { method: 'PUT' }).then(r => r.json()),
  unarchiveTask: (taskId: string) =>
    fetch(`/api/v1/tasks/${taskId}/unarchive`, { method: 'PUT' }).then(r => r.json()),
  bulkArchiveTasks: (params: { older_than_hours?: number; status?: string }) =>
    fetch('/api/v1/tasks/bulk-archive', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    }).then(r => r.json()),
  deleteTask: (taskId: string) =>
    fetch(`/api/v1/tasks/${taskId}?purge=true`, { method: 'DELETE' }).then(r => r.json()),
  cancelAllTasks: () =>
    fetch('/api/v1/tasks', { method: 'DELETE' }).then(r => r.json()),
  sendNotification: (title: string, message: string, isAlert: boolean) =>
    fetch('/api/v1/notify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, message, is_alert: isAlert }),
    }).then(r => r.json()),
  testNotification: () => fetch('/api/v1/notify/test', { method: 'POST' }).then(r => r.json()),
  getRequests: (page = 1, perPage = 20) => fetch(`/api/v1/requests?page=${page}&per_page=${perPage}`).then(r => r.json()),
  createRequest: (title: string, description: string, priority: string) =>
    fetch('/api/v1/requests', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, description, priority }),
    }).then(r => r.json()),
  deleteRequest: (id: string) =>
    fetch(`/api/v1/requests/${id}`, { method: 'DELETE' }).then(r => r.json()),
  executeRequest: (id: string) =>
    fetch(`/api/v1/requests/${id}/execute`, { method: 'POST' }).then(r => r.json()),

  // Team Lead Management
  getTeamLeads: () => fetch('/api/v1/teams').then(r => r.json()),
  getTeamLeadStatus: (name: string) => fetch(`/api/v1/teams/${name}/status`).then(r => r.json()),
  startTeamLead: (name: string, teammateCount?: number) =>
    fetch(`/api/v1/teams/${name}/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ teammate_count: teammateCount || 2 }),
    }).then(r => r.json()),
  stopTeamLead: (name: string) =>
    fetch(`/api/v1/teams/${name}/stop`, { method: 'POST' }).then(r => r.json()),

  // App Management
  getApps: () => fetch('/api/v1/apps').then(r => r.json()),
  buildApp: (name: string) =>
    fetch(`/api/v1/apps/${name}/build`, { method: 'POST' }).then(r => r.json()),
  getAppStatus: (name: string) => fetch(`/api/v1/apps/${name}/status`).then(r => r.json()),
  startApp: (name: string) =>
    fetch(`/api/v1/apps/${name}/start`, { method: 'POST' }).then(r => r.json()),
  stopApp: (name: string) =>
    fetch(`/api/v1/apps/${name}/stop`, { method: 'POST' }).then(r => r.json()),
  restartApp: (name: string) =>
    fetch(`/api/v1/apps/${name}/restart`, { method: 'POST' }).then(r => r.json()),
  getAppLogs: (name: string, lines = 100) =>
    fetch(`/api/v1/apps/${name}/logs?lines=${lines}`).then(r => r.json()),

  // GitHub Integration
  getGitHubSummary: async (): Promise<GitHubAPISummary> => {
    const response = await fetch('/api/v1/github/summary');
    if (!response.ok) {
      throw new Error(`Failed to fetch GitHub summary: ${response.statusText}`);
    }
    return response.json();
  },
  refreshGitHubSummary: async (): Promise<GitHubAPISummary> => {
    const response = await fetch('/api/v1/github/refresh', { method: 'POST' });
    if (!response.ok) {
      throw new Error(`Failed to refresh GitHub summary: ${response.statusText}`);
    }
    return response.json();
  },

  // Usage (ccusage)
  getUsage: (type: string = 'daily', since?: string) => {
    const params = new URLSearchParams({ type });
    if (since) params.set('since', since);
    return fetch(`/api/v1/usage?${params}`).then(r => {
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r.json();
    });
  },

  // Queue / Activity
  getQueue: (limit = 50) =>
    fetch(`/api/v1/queue?limit=${limit}`).then(r => {
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return r.json();
    }),

  // Strategies
  getStrategies: (): Promise<{ strategies: StrategyListItem[]; count: number }> =>
    fetch('/api/v1/strategies').then(r => r.json()),
  toggleStrategy: (id: string) =>
    fetch(`/api/v1/strategies/${id}/toggle`, { method: 'POST' }).then(r => r.json()),
  setStrategyStatus: (id: string, status: StrategyStatus) =>
    fetch(`/api/v1/strategies/${id}/status`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    }).then(r => r.json()),
  getStrategyParams: (id: string) =>
    fetch(`/api/v1/strategies/${id}/params`).then(r => r.json()),
  updateStrategyParams: (id: string, params: Record<string, string>) =>
    fetch(`/api/v1/strategies/${id}/params`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    }).then(r => r.json()),

  // Sessions
  getSessions: (): Promise<TmuxSession[]> =>
    fetch('/api/v1/sessions').then(r => r.json()),
  getSessionAgents: (): Promise<AgentInfo[]> =>
    fetch('/api/v1/sessions/agents').then(r => r.json()),
  getCliTypes: (): Promise<CliTypeInfo[]> =>
    fetch('/api/v1/sessions/cli-types').then(r => r.json()),
  getPoolStatus: (): Promise<PoolStatus> =>
    fetch('/api/v1/sessions/pool-status').then(r => r.json()),
  createSession: (agent: string, cliType: string = 'claude', model?: string, reasoningEffort?: string, description?: string, issueNumber?: string) =>
    fetch('/api/v1/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent, cli_type: cliType, model: model || '', reasoning_effort: reasoningEffort || '', description: description || '', issue_number: issueNumber || '' }),
    }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),
  deleteSession: (sessionName: string) =>
    fetch(`/api/v1/sessions/${sessionName}`, { method: 'DELETE' }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),
  restartSession: (sessionName: string) =>
    fetch(`/api/v1/sessions/${sessionName}/restart`, { method: 'POST' }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),

  updateSessionDescription: (sessionName: string, description: string) =>
    fetch(`/api/v1/sessions/${sessionName}/description`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ description }),
    }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),

  // Trading Schedules
  getTradingSchedules: (): Promise<{ schedules: TradingSchedule[] }> =>
    fetch('/api/v1/trading/schedules').then(r => r.json()),
  createTradingSchedule: (data: Partial<TradingSchedule>) =>
    fetch('/api/v1/trading/schedules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async r => {
      if (!r.ok) {
        const text = await r.text();
        throw new Error(text || `HTTP ${r.status}`);
      }
      return r.json();
    }),
  updateTradingSchedule: (id: string, data: Partial<TradingSchedule>) =>
    fetch(`/api/v1/trading/schedules/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async r => {
      if (!r.ok) {
        const text = await r.text();
        throw new Error(text || `HTTP ${r.status}`);
      }
      return r.json();
    }),
  deleteTradingSchedule: (id: string) =>
    fetch(`/api/v1/trading/schedules/${id}`, { method: 'DELETE' }).then(r => r.json()),

  // Triggers
  getTriggerStatus: (): Promise<Record<string, JobStatus>> =>
    fetch('/api/v1/triggers/status').then(r => r.json()),
  triggerJob: (jobName: string) =>
    fetch(`/api/v1/triggers/cron/${jobName}`, { method: 'POST' }).then(r => r.json()),
  triggerUpdateResources: () =>
    fetch('/api/v1/triggers/update-resources', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirm: true }),
    }).then(r => r.json()),
  triggerDeploy: () =>
    fetch('/api/v1/deploy/trigger', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirm: true }),
    }).then(r => r.json()),

  // Providers
  getProviders: (): Promise<LLMProvider[]> =>
    fetch('/api/v1/providers').then(r => r.json()),
  createProvider: (data: { id: string; name: string; api_key_ref?: string; default_model?: string }) =>
    fetch('/api/v1/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),
  getAgentConfigs: (): Promise<AgentProviderConfig[]> =>
    fetch('/api/v1/providers/agent-configs').then(r => r.json()),
  updateAgentConfig: (agentId: string, data: Partial<AgentProviderConfig>) =>
    fetch(`/api/v1/providers/agents/${agentId}/config`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(async r => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: `HTTP ${r.status}` }));
        throw new Error(body.error || `HTTP ${r.status}`);
      }
      return r.json();
    }),

  // Revenue
  getRevenue: (period: string = 'monthly') =>
    fetch(`/api/v1/revenue?period=${period}`).then(r => r.json()),
  getDailyRevenue: () =>
    fetch('/api/v1/revenue?period=daily').then(r => r.json()),
  createRevenue: (data: { date: string; source: string; amount: number; currency?: string; note?: string }) =>
    fetch('/api/v1/revenue', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(r => r.json()),

  // KPI
  getKpiLatest: () =>
    fetch('/api/v1/kpi/latest').then(r => r.json()),
  createKpi: (data: { metric: string; value: number; date?: string }) =>
    fetch('/api/v1/kpi', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }).then(r => r.json()),

  // Activity
  getActivity: (limit: number = 20) =>
    fetch(`/api/v1/activity?limit=${limit}`).then(r => r.json()),

  // Targets
  getTargets: (month?: string) =>
    fetch(`/api/v1/targets${month ? `?month=${month}` : ''}`).then(r => r.json()),
};
