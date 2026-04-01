import React, { useEffect, useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { LayoutDashboard, AppWindow, TrendingUp, BarChart3, Coins, Sun, Moon, Monitor, Bot, Terminal, Zap, DollarSign } from 'lucide-react';
import { useTheme } from '../theme';
import type { ThemeMode } from '../types';
import { useAppData } from '../hooks/useAppData';
import { useIsMobile } from '../hooks/useIsMobile';
import { BUILD_INFO } from '../utils/helpers';

export const NAV_ITEMS = [
  { path: '/', label: 'Dashboard', icon: LayoutDashboard },
  { path: '/sessions', label: 'Sessions', icon: Terminal },
  { path: '/apps', label: 'Apps', icon: AppWindow },
  { path: '/strategies', label: 'Strategies', icon: TrendingUp },
  { path: '/usage', label: 'Usage', icon: BarChart3 },
  { path: '/agent-economy', label: 'Agent Economy', icon: Coins },
  { path: '/triggers', label: 'Triggers', icon: Zap },
  { path: '/revenue', label: 'Revenue', icon: DollarSign },
];

export const SIDEBAR_WIDTH = 260;

interface SidebarProps {
  open: boolean;
  onClose: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({ open, onClose }) => {
  const { themeMode, setThemeMode, isDark, colors } = useTheme();
  const { uiVersion, apiVersion } = useAppData();
  const navigate = useNavigate();
  const location = useLocation();
  const isMobile = useIsMobile();
  const [hoveredPath, setHoveredPath] = useState<string | null>(null);

  // Close on route change (mobile only)
  useEffect(() => {
    if (isMobile) {
      onClose();
    }
  }, [location.pathname]);

  const cycleTheme = () => {
    const modes: ThemeMode[] = ['light', 'dark', 'system'];
    const currentIndex = modes.indexOf(themeMode);
    setThemeMode(modes[(currentIndex + 1) % modes.length]);
  };

  const ThemeIcon = themeMode === 'light' ? Sun : themeMode === 'dark' ? Moon : Monitor;
  const themeLabel = themeMode === 'light' ? 'Light' : themeMode === 'dark' ? 'Dark' : 'System';

  return (
    <>
      {/* Overlay (mobile only) */}
      {isMobile && (
        <div
          onClick={onClose}
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0,0,0,0.5)',
            backdropFilter: 'blur(2px)',
            zIndex: 200,
            opacity: open ? 1 : 0,
            pointerEvents: open ? 'auto' : 'none',
            transition: 'opacity 200ms ease',
          }}
        />
      )}
      {/* Sidebar panel */}
      <div
        style={{
          position: isMobile ? 'fixed' : 'relative',
          top: 0,
          left: 0,
          bottom: 0,
          width: `${SIDEBAR_WIDTH}px`,
          minWidth: `${SIDEBAR_WIDTH}px`,
          backgroundColor: isDark ? '#0d1117' : '#ffffff',
          zIndex: isMobile ? 201 : 'auto',
          transform: isMobile ? (open ? 'translateX(0)' : 'translateX(-100%)') : 'none',
          transition: isMobile ? 'transform 200ms ease' : 'none',
          display: 'flex',
          flexDirection: 'column',
          boxShadow: isMobile && open
            ? '4px 0 24px rgba(0,0,0,0.3)'
            : isDark
              ? '1px 0 0 rgba(30,58,95,0.5)'
              : '1px 0 0 rgba(226,232,240,1)',
          borderRight: 'none',
        }}
      >
        {/* Logo / Brand */}
        <div style={{
          padding: '20px 20px 16px',
          borderBottom: `1px solid ${isDark ? 'rgba(30,58,95,0.4)' : colors.border}`,
        }}>
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
          }}>
            <div style={{
              width: '32px',
              height: '32px',
              borderRadius: '8px',
              background: isDark
                ? 'linear-gradient(135deg, #3b82f6, #8b5cf6)'
                : 'linear-gradient(135deg, #3b82f6, #6366f1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
              color: '#ffffff',
            }}>
              <Bot size={18} />
            </div>
            <div>
              <div style={{
                fontSize: '15px',
                fontWeight: 700,
                color: colors.text,
                letterSpacing: '-0.01em',
              }}>
                Claude Hub
              </div>
              <div style={{
                fontSize: '11px',
                color: colors.textMuted,
                marginTop: '1px',
              }}>
                Admin Console
              </div>
            </div>
          </div>
        </div>

        {/* Nav links */}
        <nav style={{ flex: 1, overflowY: 'auto', padding: '12px 10px' }}>
          {NAV_ITEMS.map((item, index) => {
            const isActive = item.path === '/' ? location.pathname === '/' : location.pathname.startsWith(item.path);
            const isHovered = hoveredPath === item.path;
            return (
              <button
                key={item.path}
                onClick={() => navigate(item.path)}
                onMouseEnter={() => setHoveredPath(item.path)}
                onMouseLeave={() => setHoveredPath(null)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '12px',
                  width: '100%',
                  padding: '10px 14px',
                  border: 'none',
                  borderRadius: '8px',
                  marginBottom: '2px',
                  backgroundColor: isActive
                    ? (isDark ? 'rgba(59,130,246,0.15)' : 'rgba(59,130,246,0.08)')
                    : isHovered
                      ? (isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.04)')
                      : 'transparent',
                  color: isActive ? colors.accent : isHovered ? colors.text : colors.textSecondary,
                  cursor: 'pointer',
                  fontSize: '14px',
                  fontWeight: isActive ? 600 : 500,
                  textAlign: 'left',
                  transition: 'all 0.15s ease',
                  position: 'relative',
                  animation: `slideIn 0.2s ease ${index * 0.03}s both`,
                }}
              >
                {isActive && (
                  <div style={{
                    position: 'absolute',
                    left: '0',
                    top: '50%',
                    transform: 'translateY(-50%)',
                    width: '3px',
                    height: '20px',
                    borderRadius: '0 3px 3px 0',
                    backgroundColor: colors.accent,
                  }} />
                )}
                <span style={{
                  width: '28px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexShrink: 0,
                }}><item.icon size={20} /></span>
                {item.label}
              </button>
            );
          })}
        </nav>

        {/* Bottom section */}
        <div style={{
          borderTop: `1px solid ${isDark ? 'rgba(30,58,95,0.4)' : colors.border}`,
          padding: '12px 10px',
        }}>
          {/* Theme toggle */}
          <button
            onClick={cycleTheme}
            onMouseEnter={() => setHoveredPath('__theme__')}
            onMouseLeave={() => setHoveredPath(null)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              width: '100%',
              padding: '10px 14px',
              border: 'none',
              borderRadius: '8px',
              backgroundColor: hoveredPath === '__theme__'
                ? (isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.04)')
                : 'transparent',
              color: colors.textSecondary,
              cursor: 'pointer',
              fontSize: '13px',
              textAlign: 'left',
              transition: 'all 0.15s ease',
            }}
          >
            <span style={{ width: '28px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><ThemeIcon size={18} /></span>
            <span style={{ flex: 1 }}>Theme</span>
            <span style={{
              padding: '2px 10px',
              borderRadius: '12px',
              fontSize: '11px',
              fontWeight: 500,
              backgroundColor: isDark ? 'rgba(96,165,250,0.15)' : 'rgba(59,130,246,0.1)',
              color: colors.accent,
            }}>
              {themeLabel}
            </span>
          </button>

          {/* Version info */}
          <div style={{
            fontSize: '11px',
            color: colors.textMuted,
            padding: '8px 14px 4px',
            opacity: 0.7,
          }}>
            <div>
              UI: {(uiVersion?.gitCommit || BUILD_INFO.gitCommit)} ({(uiVersion?.buildTime || BUILD_INFO.buildTime).slice(0, 10)})
            </div>
            <div>API: {apiVersion?.git_commit || '?'} ({apiVersion?.build_time?.slice(0, 10) || '?'})</div>
          </div>
        </div>
      </div>
    </>
  );
};
