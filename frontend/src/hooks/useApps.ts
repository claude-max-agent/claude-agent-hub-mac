import { useState, useCallback, useEffect } from 'react';
import type { AppStatus } from '../types';
import { api } from '../api/client';

export interface UseAppsResult {
  apps: AppStatus[];
  loading: boolean;
  error: string | null;
  actionLoading: string | null;
  logs: Record<string, string>;
  logsOpen: string | null;
  setLogsOpen: (name: string | null) => void;
  startApp: (id: string) => Promise<void>;
  stopApp: (id: string) => Promise<void>;
  restartApp: (id: string) => Promise<void>;
  fetchLogs: (id: string) => Promise<void>;
  refreshApps: () => void;
}

export const useApps = (): UseAppsResult => {
  const [apps, setApps] = useState<AppStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [logs, setLogs] = useState<Record<string, string>>({});
  const [logsOpen, setLogsOpen] = useState<string | null>(null);

  const fetchApps = useCallback(async () => {
    try {
      const res = await api.getApps().catch(() => ({ apps: [], count: 0 }));
      setApps(res.apps || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch apps');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchApps();
    const interval = setInterval(fetchApps, 10000);
    return () => clearInterval(interval);
  }, [fetchApps]);

  const startApp = useCallback(async (name: string) => {
    setActionLoading(name);
    try {
      await api.startApp(name);
      await fetchApps();
    } catch (err) {
      console.error('Failed to start app:', err);
    } finally {
      setActionLoading(null);
    }
  }, [fetchApps]);

  const stopApp = useCallback(async (name: string) => {
    setActionLoading(name);
    try {
      await api.stopApp(name);
      await fetchApps();
    } catch (err) {
      console.error('Failed to stop app:', err);
    } finally {
      setActionLoading(null);
    }
  }, [fetchApps]);

  const restartApp = useCallback(async (name: string) => {
    setActionLoading(name);
    try {
      await api.restartApp(name);
      await fetchApps();
    } catch (err) {
      console.error('Failed to restart app:', err);
    } finally {
      setActionLoading(null);
    }
  }, [fetchApps]);

  const fetchLogs = useCallback(async (name: string) => {
    try {
      const res = await api.getAppLogs(name);
      setLogs(prev => ({ ...prev, [name]: res.logs || 'No logs available' }));
    } catch {
      setLogs(prev => ({ ...prev, [name]: 'Failed to fetch logs' }));
    }
  }, []);

  return {
    apps,
    loading,
    error,
    actionLoading,
    logs,
    logsOpen,
    setLogsOpen,
    startApp,
    stopApp,
    restartApp,
    fetchLogs,
    refreshApps: fetchApps,
  };
};
