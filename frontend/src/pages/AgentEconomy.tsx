import React, { useEffect, useState, useCallback } from 'react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';

const AGENT_ECONOMY_API = '/agent-economy-api';

interface ServiceInfo {
  name: string;
  price: string;
  description: string;
  estimated_time?: string;
}

interface AgentEconomyData {
  services?: ServiceInfo[];
  wallet_address?: string;
  network?: string;
  [key: string]: unknown;
}

interface HealthData {
  status: string;
  [key: string]: unknown;
}

export const AgentEconomy: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const [data, setData] = useState<AgentEconomyData | null>(null);
  const [health, setHealth] = useState<HealthData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [infoRes, healthRes] = await Promise.allSettled([
        fetch(`${AGENT_ECONOMY_API}/`),
        fetch(`${AGENT_ECONOMY_API}/health`),
      ]);

      if (infoRes.status === 'fulfilled' && infoRes.value.ok) {
        setData(await infoRes.value.json());
      } else {
        setError('Failed to fetch service info');
      }

      if (healthRes.status === 'fulfilled' && healthRes.value.ok) {
        setHealth(await healthRes.value.json());
      }
    } catch {
      setError('Cannot connect to Agent Economy API');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const cardStyle: React.CSSProperties = {
    backgroundColor: colors.cardBg,
    borderRadius: '8px',
    padding: '16px',
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
  };

  const sectionTitle: React.CSSProperties = {
    fontSize: '14px',
    fontWeight: '600',
    color: colors.textMuted,
    marginBottom: '12px',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  };

  const isHealthy = health?.status === 'healthy' || health?.status === 'ok';

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
        <h2 style={{ fontSize: '18px', fontWeight: '600', color: colors.text, margin: 0 }}>Agent Economy</h2>
        <div style={{ display: 'flex', gap: '6px' }}>
          <a
            href="/agent-economy-api/docs"
            target="_blank"
            rel="noopener noreferrer"
            style={{
              background: 'none',
              border: `1px solid ${colors.border}`,
              borderRadius: '4px',
              padding: '4px 8px',
              fontSize: '11px',
              color: colors.textMuted,
              textDecoration: 'none',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
            }}
          >
            &#x1F4D6; API Docs
          </a>
          <button
            onClick={fetchData}
            disabled={loading}
            style={{
              background: 'none',
              border: `1px solid ${colors.border}`,
              borderRadius: '4px',
              padding: '4px 8px',
              fontSize: '11px',
              color: colors.textMuted,
              cursor: loading ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              opacity: loading ? 0.6 : 1,
            }}
          >
            <span style={{ display: 'inline-block', animation: loading ? 'spin 1s linear infinite' : 'none' }}>&#x21bb;</span>
            {loading ? 'Loading...' : 'Refresh'}
          </button>
        </div>
      </div>

      {/* Health Status */}
      <div style={{ marginBottom: '24px' }}>
        <h3 style={sectionTitle}>Status</h3>
        <div style={cardStyle}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{
              width: '10px',
              height: '10px',
              borderRadius: '50%',
              backgroundColor: health ? (isHealthy ? '#22c55e' : '#ef4444') : '#6b7280',
              display: 'inline-block',
            }} />
            <span style={{ fontSize: '14px', color: colors.text, fontWeight: '500' }}>
              {health ? (isHealthy ? 'Operational' : `Unhealthy (${health.status})`) : (error ? 'Unreachable' : 'Checking...')}
            </span>
          </div>
        </div>
      </div>

      {error && !data && (
        <div style={{ ...cardStyle, borderLeft: '3px solid #ef4444', marginBottom: '24px' }}>
          <p style={{ margin: 0, fontSize: '13px', color: colors.textMuted }}>{error}</p>
        </div>
      )}

      {/* Wallet Info */}
      {data?.wallet_address && (
        <div style={{ marginBottom: '24px' }}>
          <h3 style={sectionTitle}>Wallet</h3>
          <div style={cardStyle}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '12px', color: colors.textMuted }}>Address:</span>
                <span style={{ fontSize: '12px', fontFamily: 'monospace', color: colors.text, wordBreak: 'break-all' }}>
                  {data.wallet_address}
                </span>
              </div>
              {data.network && (
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <span style={{ fontSize: '12px', color: colors.textMuted }}>Network:</span>
                  <span style={{ fontSize: '12px', color: colors.text }}>{data.network}</span>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Services */}
      {data?.services && data.services.length > 0 && (
        <div style={{ marginBottom: '24px' }}>
          <h3 style={sectionTitle}>Services</h3>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
            {data.services.map((svc, i) => (
              <div key={i} style={{ ...cardStyle, borderLeft: '3px solid #8b5cf6' }}>
                <div style={{ marginBottom: '8px' }}>
                  <span style={{ fontSize: '14px', fontWeight: '600', color: colors.text }}>{svc.name}</span>
                </div>
                <p style={{ margin: '0 0 12px', fontSize: '13px', color: colors.textMuted, lineHeight: '1.4' }}>
                  {svc.description}
                </p>
                <div style={{ display: 'flex', gap: '16px', fontSize: '12px' }}>
                  <div>
                    <span style={{ color: colors.textMuted }}>Price: </span>
                    <span style={{ color: '#10b981', fontWeight: '600' }}>{svc.price}</span>
                  </div>
                  {svc.estimated_time && (
                    <div>
                      <span style={{ color: colors.textMuted }}>Est. Time: </span>
                      <span style={{ color: colors.text }}>{svc.estimated_time}</span>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Raw API Response (if no structured services) */}
      {data && !data.services && (
        <div style={{ marginBottom: '24px' }}>
          <h3 style={sectionTitle}>API Response</h3>
          <div style={cardStyle}>
            <pre style={{ margin: 0, fontSize: '12px', color: colors.text, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {JSON.stringify(data, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
