import type { Task } from '../types';

export const BUILD_INFO = {
  version: __APP_VERSION__,
  gitCommit: __GIT_COMMIT__,
  buildTime: __BUILD_TIME__,
};

export const statusColor = (status: string): string => {
  switch (status) {
    case 'available': return '#22c55e';
    case 'busy': return '#3b82f6';
    case 'running': return '#f59e0b';
    case 'stopped': return '#ef4444';
    case 'completed': return '#22c55e';
    case 'assigned': case 'in_progress': return '#3b82f6';
    case 'pending': return '#f59e0b';
    case 'archived': return '#6b7280';
    case 'active': return '#22c55e';
    case 'idle': return '#f59e0b';
    case 'error': return '#ef4444';
    default: return '#6b7280';
  }
};

export const isTestCommand = (task: Task): boolean => {
  const desc = task.description.toLowerCase();
  return (
    task.type === 'notification' ||
    task.type === 'test' ||
    desc === 'ping' ||
    desc.startsWith('ping ') ||
    (desc.includes('テスト') && desc.length < 20)
  );
};

export const MOBILE_BREAKPOINT = 768;
