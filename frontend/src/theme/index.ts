import { createContext, useContext, useEffect, useState } from 'react';
import type { ThemeMode, ThemeColors } from '../types';

export const lightTheme: ThemeColors = {
  bg: '#f8fafc',
  bgSecondary: '#ffffff',
  bgTertiary: '#f1f5f9',
  text: '#1e293b',
  textSecondary: '#374151',
  textMuted: '#6b7280',
  border: '#e2e8f0',
  header: '#0f172a',
  cardBg: '#ffffff',
  inputBg: '#ffffff',
  inputBorder: '#cbd5e1',
  accent: '#3b82f6',
  accentHover: '#2563eb',
};

export const darkTheme: ThemeColors = {
  bg: '#0a0e1a',
  bgSecondary: '#111827',
  bgTertiary: '#1e293b',
  text: '#f1f5f9',
  textSecondary: '#cbd5e1',
  textMuted: '#94a3b8',
  border: '#1e3a5f',
  header: '#070b14',
  cardBg: '#1a2332',
  inputBg: '#1e293b',
  inputBorder: '#334155',
  accent: '#60a5fa',
  accentHover: '#3b82f6',
};

const useSystemTheme = () => {
  const [systemTheme, setSystemTheme] = useState<'light' | 'dark'>(() => {
    if (typeof window !== 'undefined' && window.matchMedia) {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    return 'light';
  });

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (e: MediaQueryListEvent) => setSystemTheme(e.matches ? 'dark' : 'light');
    mediaQuery.addEventListener('change', handler);
    return () => mediaQuery.removeEventListener('change', handler);
  }, []);

  return systemTheme;
};

export interface ThemeContextType {
  themeMode: ThemeMode;
  setThemeMode: (mode: ThemeMode) => void;
  resolvedTheme: 'light' | 'dark';
  colors: ThemeColors;
  isDark: boolean;
}

export const ThemeContext = createContext<ThemeContextType>(null!);

export const useTheme = () => useContext(ThemeContext);

export const useThemeProvider = (): ThemeContextType => {
  const systemTheme = useSystemTheme();
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => {
    const saved = localStorage.getItem('themeMode');
    return (saved as ThemeMode) || 'system';
  });

  useEffect(() => {
    localStorage.setItem('themeMode', themeMode);
  }, [themeMode]);

  const resolvedTheme = themeMode === 'system' ? systemTheme : themeMode;
  const colors = resolvedTheme === 'dark' ? darkTheme : lightTheme;
  const isDark = resolvedTheme === 'dark';

  return { themeMode, setThemeMode, resolvedTheme, colors, isDark };
};
