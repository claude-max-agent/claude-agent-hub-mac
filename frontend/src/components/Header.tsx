import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Menu } from 'lucide-react';
import { useTheme } from '../theme';
import { useAppData } from '../hooks/useAppData';
import { useIsMobile } from '../hooks/useIsMobile';

interface HeaderProps {
  onToggleSidebar: () => void;
}

export const Header: React.FC<HeaderProps> = ({ onToggleSidebar }) => {
  const isMobile = useIsMobile();
  const { isDark } = useTheme();
  const { agents, teamInfo } = useAppData();
  const navigate = useNavigate();

  const agentStatus = (() => {
    if (agents.length === 0) return null;
    const hasTeams = !!teamInfo;
    const activeCount = agents.filter(a => a.status !== 'stopped').length;
    const isActive = activeCount > 0;
    const teammates = agents.filter(a => a.agent_type === 'teammate');
    const activeTeammates = teammates.filter(a => a.status !== 'stopped').length;
    const label = isMobile
      ? (hasTeams ? `1L+${activeTeammates}/${teammates.length}T` : `${activeCount}/${agents.length}`)
      : (hasTeams ? `Team: 1 lead + ${activeTeammates}/${teammates.length} teammates` : `Agents: ${activeCount}/${agents.length}`);
    return (
      <span style={{
        padding: '4px 12px',
        borderRadius: '20px',
        fontSize: isMobile ? '11px' : '12px',
        fontWeight: 500,
        backgroundColor: isActive ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
        color: isActive ? '#4ade80' : '#f87171',
        border: `1px solid ${isActive ? 'rgba(34,197,94,0.25)' : 'rgba(239,68,68,0.25)'}`,
        letterSpacing: '0.01em',
      }}>
        <span style={{
          display: 'inline-block',
          width: '6px',
          height: '6px',
          borderRadius: '50%',
          backgroundColor: isActive ? '#22c55e' : '#ef4444',
          marginRight: '6px',
          verticalAlign: 'middle',
          boxShadow: isActive ? '0 0 6px rgba(34,197,94,0.5)' : 'none',
        }} />
        {label}
      </span>
    );
  })();

  return (
    <header style={{
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      padding: isMobile ? '12px 16px' : '14px 24px',
      backgroundColor: isDark ? '#070b14' : '#0f172a',
      color: '#fff',
      borderBottom: isDark ? '1px solid rgba(30,58,95,0.4)' : '1px solid rgba(15,23,42,0.1)',
      backdropFilter: 'blur(12px)',
      position: 'relative',
      zIndex: 10,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: isMobile ? '10px' : '16px' }}>
        <h1
          style={{
            margin: 0,
            fontSize: isMobile ? '16px' : '18px',
            fontWeight: 700,
            cursor: 'pointer',
            letterSpacing: '-0.02em',
            background: 'linear-gradient(135deg, #ffffff 0%, #94a3b8 100%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
          }}
          onClick={() => navigate('/')}
        >
          Claude Hub Admin
        </h1>
        {agentStatus}
      </div>
      {isMobile && (
        <button
          onClick={onToggleSidebar}
          style={{
            padding: '8px 12px',
            borderRadius: '8px',
            border: '1px solid rgba(255,255,255,0.1)',
            backgroundColor: 'rgba(255,255,255,0.08)',
            color: '#fff',
            cursor: 'pointer',
            fontSize: '16px',
            backdropFilter: 'blur(4px)',
            transition: 'all 0.15s ease',
          }}
        >
          <Menu size={18} />
        </button>
      )}
    </header>
  );
};
