import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeContext, useThemeProvider } from './theme';
import { AppDataContext, useAppDataProvider } from './hooks/useAppData';
import { ModalProvider } from './contexts/ModalContext';
import { MainLayout } from './layouts/MainLayout';
import { Dashboard } from './pages/Dashboard';
import { Apps } from './pages/Apps';
import { AgentEconomy } from './pages/AgentEconomy';
import { ActiveSessions } from './pages/ActiveSessions';
import { Strategies } from './pages/Strategies';
import { Usage } from './pages/Usage';
import { Triggers } from './pages/Triggers';
import { Revenue } from './pages/Revenue';

export const App: React.FC = () => {
  const themeValue = useThemeProvider();
  const dataValue = useAppDataProvider();

  return (
    <ThemeContext.Provider value={themeValue}>
      <AppDataContext.Provider value={dataValue}>
        <BrowserRouter>
          <ModalProvider>
            <Routes>
              <Route element={<MainLayout />}>
                <Route path="/" element={<Dashboard />} />
                <Route path="/apps" element={<Apps />} />
                <Route path="/sessions" element={<ActiveSessions />} />
                <Route path="/agent-economy" element={<AgentEconomy />} />
                <Route path="/strategies" element={<Strategies />} />
                <Route path="/usage" element={<Usage />} />
                <Route path="/triggers" element={<Triggers />} />
                <Route path="/revenue" element={<Revenue />} />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Route>
            </Routes>
          </ModalProvider>
        </BrowserRouter>
      </AppDataContext.Provider>
    </ThemeContext.Provider>
  );
};
