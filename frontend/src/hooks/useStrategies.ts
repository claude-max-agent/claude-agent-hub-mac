import { useState, useEffect, useCallback } from 'react';
import { api } from '../api/client';
import type { StrategyListItem, StrategyParamValue, StrategyStatus } from '../types';

export interface UseStrategiesResult {
  strategies: StrategyListItem[];
  loading: boolean;
  error: string | null;
  actionLoading: string | null;
  params: Record<string, StrategyParamValue[]>;
  paramsLoading: string | null;
  toggleStrategy: (id: string) => Promise<void>;
  setStatus: (id: string, status: StrategyStatus) => Promise<void>;
  fetchParams: (id: string) => Promise<void>;
  updateParams: (id: string, params: Record<string, string>) => Promise<{ success: boolean; errors?: string[] }>;
  refreshStrategies: () => void;
}

export const useStrategies = (): UseStrategiesResult => {
  const [strategies, setStrategies] = useState<StrategyListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [params, setParams] = useState<Record<string, StrategyParamValue[]>>({});
  const [paramsLoading, setParamsLoading] = useState<string | null>(null);

  const fetchStrategies = useCallback(async () => {
    try {
      const res = await api.getStrategies();
      setStrategies(res.strategies || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch strategies');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStrategies();
    const interval = setInterval(fetchStrategies, 10000);
    return () => clearInterval(interval);
  }, [fetchStrategies]);

  const toggleStrategy = useCallback(async (id: string) => {
    setActionLoading(id);
    try {
      await api.toggleStrategy(id);
      await fetchStrategies();
    } catch (err) {
      console.error('Failed to toggle strategy:', err);
    } finally {
      setActionLoading(null);
    }
  }, [fetchStrategies]);

  const setStatus = useCallback(async (id: string, status: StrategyStatus) => {
    setActionLoading(id);
    try {
      await api.setStrategyStatus(id, status);
      await fetchStrategies();
    } catch (err) {
      console.error('Failed to set strategy status:', err);
    } finally {
      setActionLoading(null);
    }
  }, [fetchStrategies]);

  const fetchParams = useCallback(async (id: string) => {
    setParamsLoading(id);
    try {
      const res = await api.getStrategyParams(id);
      setParams(prev => ({ ...prev, [id]: res.params || [] }));
    } catch (err) {
      console.error('Failed to fetch params:', err);
    } finally {
      setParamsLoading(null);
    }
  }, []);

  const updateParams = useCallback(async (id: string, paramValues: Record<string, string>) => {
    setActionLoading(id);
    try {
      const res = await api.updateStrategyParams(id, paramValues);
      if (res.success) {
        await fetchParams(id);
      }
      return res;
    } catch (err) {
      console.error('Failed to update params:', err);
      return { success: false, errors: [err instanceof Error ? err.message : 'Unknown error'] };
    } finally {
      setActionLoading(null);
    }
  }, [fetchParams]);

  return {
    strategies,
    loading,
    error,
    actionLoading,
    params,
    paramsLoading,
    toggleStrategy,
    setStatus,
    fetchParams,
    updateParams,
    refreshStrategies: fetchStrategies,
  };
};
