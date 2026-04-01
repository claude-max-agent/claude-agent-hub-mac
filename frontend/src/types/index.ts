// Agent type for Agent Teams feature
export type AgentType = 'team_lead' | 'teammate' | 'independent';

export interface Agent {
  id: string;
  role: string;
  nickname?: string;
  status: string;
  current_task: string | null;
  current_task_description: string | null;
  started_at: string | null;
  agent_type?: AgentType;  // Agent Teams: team_lead, teammate, or independent
  team_name?: string;      // Team name if part of Agent Teams
  teammates_count?: number; // Number of teammates under this Team Lead
}

export interface Task {
  task_id: string;
  type: string;
  priority: string;
  description: string;
  status: string;
  assigned_to: string | null;
  archived?: boolean;
  created_at: string;
  source?: string;
}

export interface FeatureRequest {
  id: string;
  title: string;
  description: string;
  priority: string;
  status: string;
  created_at: string;
  updated_at: string;
  task_id?: string;
}

export interface Pagination {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export type ThemeMode = 'light' | 'dark' | 'system';

export interface BuildInfo {
  version: string;
  gitCommit: string;
  buildTime: string;
}

export interface ThemeColors {
  bg: string;
  bgSecondary: string;
  bgTertiary: string;
  text: string;
  textSecondary: string;
  textMuted: string;
  border: string;
  header: string;
  cardBg: string;
  inputBg: string;
  inputBorder: string;
  accent: string;
  accentHover: string;
}

export interface ConfirmModalState {
  message: string;
  onConfirm: () => void;
  confirmText?: string;
  cancelText?: string;
  isDanger?: boolean;
}

export interface TeamLeadInfo {
  name: string;
  description: string;
  tmux_session: string;
  repos: string[];
  model?: string;
}

export interface TeamLeadStatus {
  team_lead: string;
  tmux_session: string;
  running: boolean;
  panes: number;
  description: string;
  repos: string[];
  model?: string;
}

export interface ServiceStatus {
  name: string;
  status: string;
  health?: string;
}

export interface AppStatus {
  app_id: string;
  name: string;
  description: string;
  repo: string;
  language?: string;
  type: string;
  port: number;
  status: string;
  health: string;
  pid?: number;
  services?: ServiceStatus[];
  path?: string;
  log_file?: string;
  error?: string;
  uses_claude?: boolean;
  model?: string;
  updated_at: string;
}

// GitHub API Summary
export interface GitHubRepoSummary {
  repo: string;
  recent_prs: GitHubPRSummary[];
  recent_commits: GitHubCommitSummary[];
  open_issues: number;
}

export interface GitHubPRSummary {
  number: number;
  title: string;
  state: string;
  user: string;
  html_url: string;
  created_at: string;
  updated_at: string;
  merged: boolean;
}

export interface GitHubCommitSummary {
  sha: string;
  message: string;
  author: string;
  date: string;
  html_url: string;
}

export interface GitHubAPISummary {
  repos: GitHubRepoSummary[];
}

// Strategy Management
export type StrategyStatus = 'active' | 'inactive' | 'archived';

export interface StrategyListItem {
  id: string;
  name: string;
  description: string;
  app: string;
  status: StrategyStatus;
  active: boolean;
  exchange?: string;
  pairs?: string[];
  param_count: number;
  params: StrategyParamDef[];
}

export interface StrategyParamDef {
  key: string;
  label: string;
  description?: string;
  type: string;
  min?: number;
  max?: number;
  step?: number;
  placeholder?: string;
  default?: string;
}

export interface StrategyParamValue extends StrategyParamDef {
  value: string;
}

// Session Management
export interface TmuxSession {
  name: string;
  created: number;
  attached: boolean;
  status: string;
  agent: string;
  cli_type: string;
  description?: string;
  template?: string;
  issue_number?: string;
  pool_status?: string;
}

export interface PoolStatus {
  pooled: number;
  running: number;
  stopping: number;
  total: number;
}

export interface AgentInfo {
  name: string;
  type: string;
}

export interface CliTypeInfo {
  id: string;
  name: string;
  default_args: string;
}

// Trigger Jobs
export interface JobStatus {
  label: string;
  status: 'idle' | 'running' | 'done' | 'error';
  started_at: string | null;
  finished_at: string | null;
  exit_code: number | null;
}

// Trading Schedules
export interface TradingSchedule {
  id: string;
  coin: string;
  side: 'buy' | 'sell';
  order_type: 'market' | 'limit';
  price?: number;
  amount: number;
  leverage: number;
  cron: string;
  enabled: boolean;
  last_run_at: string | null;
  next_run_at: string | null;
  run_count: number;
  description: string;
  created_at: string;
  updated_at: string;
}

// LLM Providers
export interface LLMProvider {
  id: string;
  name: string;
  api_key_ref: string;
  default_model: string;
  status: string;
}

export interface AgentProviderConfig {
  agent_id: string;
  provider_id: string;
  model: string;
  reasoning_effort: string;
  updated_at?: string;
}
