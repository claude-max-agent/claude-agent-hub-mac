import { createContext, useContext, useState, useCallback, useEffect } from 'react';
import type { Agent, BuildInfo, GitHubAPISummary } from '../types';
import { api } from '../api/client';

export interface TeamInfo {
  mode: string;
  max_teammates: number;
}

export interface AppDataContextType {
  agents: Agent[];
  teamInfo: TeamInfo | null;
  uiVersion: BuildInfo | null;
  apiVersion: { version: string; git_commit: string; build_time: string } | null;
  githubSummary: GitHubAPISummary | null;
  loading: boolean;
  error: string | null;
  refreshData: () => void;
  refreshGitHub: () => Promise<void>;
  githubRefreshing: boolean;
}

export const AppDataContext = createContext<AppDataContextType>(null!);

export const useAppData = () => useContext(AppDataContext);

export const useAppDataProvider = (): AppDataContextType => {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [teamInfo, setTeamInfo] = useState<TeamInfo | null>(null);
  const [uiVersion, setUiVersion] = useState<BuildInfo | null>(null);
  const [apiVersion, setApiVersion] = useState<{ version: string; git_commit: string; build_time: string } | null>(null);
  const [githubSummary, setGithubSummary] = useState<GitHubAPISummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [githubRefreshing, setGithubRefreshing] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const [agentsRes, githubRes] = await Promise.all([
        api.getAgents().catch(() => ({ agents: [] })),
        api.getGitHubSummary().catch(() => ({ repos: [] })),
      ]);
      setAgents(agentsRes.agents || []);
      setTeamInfo(agentsRes.team_info || null);
      setGithubSummary(githubRes);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const fetchVersions = async () => {
      const [uiRes, apiRes] = await Promise.all([
        fetch(`/version.json?ts=${Date.now()}`, { cache: 'no-store' })
          .then(async r => (r.ok ? r.json() : null))
          .catch(() => null),
        api.getVersion().catch(() => null),
      ]);

      if (uiRes && typeof uiRes === 'object') {
        setUiVersion(uiRes as BuildInfo);
      }
      if (apiRes) {
        setApiVersion(apiRes);
      }
    };

    fetchVersions().catch(() => null);
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => {
      fetchData();
      fetch(`/version.json?ts=${Date.now()}`, { cache: 'no-store' })
        .then(async r => (r.ok ? r.json() : null))
        .then(uiRes => {
          if (uiRes && typeof uiRes === 'object') {
            setUiVersion(uiRes as BuildInfo);
          }
        })
        .catch(() => null);
      api.getVersion()
        .then(setApiVersion)
        .catch(() => null);
    }, 3000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const refreshGitHub = useCallback(async () => {
    setGithubRefreshing(true);
    try {
      const freshData = await api.refreshGitHubSummary();
      setGithubSummary(freshData);
    } catch (err) {
      console.error('Failed to refresh GitHub data:', err);
    } finally {
      setGithubRefreshing(false);
    }
  }, []);

  return {
    agents,
    teamInfo,
    uiVersion,
    apiVersion,
    githubSummary,
    loading,
    error,
    refreshData: fetchData,
    refreshGitHub,
    githubRefreshing,
  };
};
