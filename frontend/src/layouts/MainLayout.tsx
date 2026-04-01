import React, { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { useTheme } from '../theme';
import { useAppData } from '../hooks/useAppData';
import { Header } from '../components/Header';
import { Sidebar } from '../components/Sidebar';

export const MainLayout: React.FC = () => {
  const { colors } = useTheme();
  const { loading, error } = useAppData();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  if (loading) {
    return <div style={{ padding: '20px', fontFamily: 'system-ui', backgroundColor: colors.bg, color: colors.text, minHeight: '100vh' }}>Loading...</div>;
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', fontFamily: 'system-ui, -apple-system, sans-serif', backgroundColor: colors.bg, overflow: 'hidden' }}>
      <Header onToggleSidebar={() => setSidebarOpen(prev => !prev)} />
      {error && (
        <div style={{ padding: '12px 20px', backgroundColor: colors.bg === '#0f172a' ? '#7f1d1d' : '#fef2f2', color: colors.bg === '#0f172a' ? '#fca5a5' : '#dc2626', fontSize: '14px' }}>
          Error: {error}
        </div>
      )}
      <div style={{ display: 'flex', flex: 1, overflow: 'hidden', minHeight: 0 }}>
        <Sidebar
          open={sidebarOpen}
          onClose={() => setSidebarOpen(false)}
        />
        <main style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden', maxWidth: '100%' }}>
          <Outlet />
        </main>
      </div>
    </div>
  );
};
