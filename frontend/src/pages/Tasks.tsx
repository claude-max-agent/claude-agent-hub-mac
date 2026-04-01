import React, { useState, useEffect, useCallback, type CSSProperties } from 'react';
import { RefreshCw, Archive, Trash2, CheckCircle, XCircle, ChevronDown, ChevronUp } from 'lucide-react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useModal } from '../contexts/ModalContext';
import { api } from '../api/client';
import type { Task, Pagination } from '../types';

type StatusFilter = 'all' | 'pending' | 'in_progress' | 'completed' | 'cancelled';

const STATUS_CONFIG: Record<string, { color: string; bgLight: string; bgDark: string; label: string }> = {
  pending: { color: '#f59e0b', bgLight: 'rgba(245,158,11,0.1)', bgDark: 'rgba(245,158,11,0.15)', label: 'Pending' },
  in_progress: { color: '#3b82f6', bgLight: 'rgba(59,130,246,0.1)', bgDark: 'rgba(59,130,246,0.15)', label: 'In Progress' },
  completed: { color: '#22c55e', bgLight: 'rgba(34,197,94,0.1)', bgDark: 'rgba(34,197,94,0.15)', label: 'Completed' },
  cancelled: { color: '#6b7280', bgLight: 'rgba(107,114,128,0.1)', bgDark: 'rgba(107,114,128,0.2)', label: 'Cancelled' },
  assigned: { color: '#8b5cf6', bgLight: 'rgba(139,92,246,0.1)', bgDark: 'rgba(139,92,246,0.15)', label: 'Assigned' },
};

const PRIORITY_CONFIG: Record<string, { color: string }> = {
  critical: { color: '#ef4444' },
  high: { color: '#f97316' },
  medium: { color: '#eab308' },
  low: { color: '#6b7280' },
};

const FILTERS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'completed', label: 'Completed' },
  { value: 'cancelled', label: 'Cancelled' },
];

export const Tasks: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const { showConfirm } = useModal();

  const [tasks, setTasks] = useState<Task[]>([]);
  const [pagination, setPagination] = useState<Pagination | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [showArchived, setShowArchived] = useState(false);
  const [page, setPage] = useState(1);
  const [expandedTask, setExpandedTask] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({});

  const fetchTasks = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getTasks(page, 30, showArchived);
      setTasks(data.tasks || []);
      setPagination(data.pagination || null);
      setError(null);
    } catch {
      setError('Failed to fetch tasks');
    } finally {
      setLoading(false);
    }
  }, [page, showArchived]);

  useEffect(() => {
    fetchTasks();
    const interval = setInterval(fetchTasks, 15000);
    return () => clearInterval(interval);
  }, [fetchTasks]);

  const filteredTasks = statusFilter === 'all'
    ? tasks
    : tasks.filter(t => t.status === statusFilter);

  const statusCounts = {
    all: tasks.length,
    pending: tasks.filter(t => t.status === 'pending').length,
    in_progress: tasks.filter(t => t.status === 'in_progress' || t.status === 'assigned').length,
    completed: tasks.filter(t => t.status === 'completed').length,
    cancelled: tasks.filter(t => t.status === 'cancelled').length,
  };

  const handleComplete = (taskId: string) => {
    showConfirm({
      message: `Task "${taskId.substring(0, 8)}..." to complete?`,
      confirmText: 'Complete',
      onConfirm: async () => {
        setActionLoading(prev => ({ ...prev, [taskId]: true }));
        try {
          await api.completeTask(taskId);
          await fetchTasks();
        } finally {
          setActionLoading(prev => ({ ...prev, [taskId]: false }));
        }
      },
    });
  };

  const handleCancel = (taskId: string) => {
    showConfirm({
      message: `Task "${taskId.substring(0, 8)}..." to cancel?`,
      confirmText: 'Cancel Task',
      isDanger: true,
      onConfirm: async () => {
        setActionLoading(prev => ({ ...prev, [taskId]: true }));
        try {
          await api.cancelTask(taskId);
          await fetchTasks();
        } finally {
          setActionLoading(prev => ({ ...prev, [taskId]: false }));
        }
      },
    });
  };

  const handleArchive = (taskId: string) => {
    showConfirm({
      message: `Task "${taskId.substring(0, 8)}..." to archive?`,
      confirmText: 'Archive',
      onConfirm: async () => {
        setActionLoading(prev => ({ ...prev, [taskId]: true }));
        try {
          await api.archiveTask(taskId);
          await fetchTasks();
        } finally {
          setActionLoading(prev => ({ ...prev, [taskId]: false }));
        }
      },
    });
  };

  const handleBulkArchive = () => {
    showConfirm({
      message: 'Archive all completed/cancelled tasks older than 24h?',
      confirmText: 'Bulk Archive',
      isDanger: true,
      onConfirm: async () => {
        try {
          await api.bulkArchiveTasks({ older_than_hours: 24, status: 'completed' });
          await api.bulkArchiveTasks({ older_than_hours: 24, status: 'cancelled' });
          await fetchTasks();
        } catch {
          // refreshed
        }
      },
    });
  };

  const cardStyle: CSSProperties = {
    backgroundColor: colors.cardBg,
    borderRadius: '12px',
    border: `1px solid ${colors.border}`,
    padding: isMobile ? '12px' : '14px',
    marginBottom: '10px',
    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
  };

  if (loading && tasks.length === 0) {
    return (
      <div style={{ padding: isMobile ? '12px' : '16px' }}>
        <div style={cardStyle}>
          <div style={{ color: colors.textMuted, textAlign: 'center', padding: '20px' }}>Loading tasks...</div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      {/* Header */}
      <div style={cardStyle}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '8px' }}>
          <div>
            <h2 style={{ margin: 0, fontSize: isMobile ? '16px' : '18px', color: colors.text }}>
              Task Management
            </h2>
            <p style={{ margin: '4px 0 0', fontSize: '13px', color: colors.textMuted }}>
              <span style={{ fontWeight: '700', fontSize: '14px', color: colors.accent }}>{statusCounts.pending + statusCounts.in_progress}</span>
              <span> active / {statusCounts.all} total</span>
            </p>
          </div>
          <div style={{ display: 'flex', gap: '6px', alignItems: 'center', flexWrap: 'wrap' }}>
            {error && <span style={{ fontSize: '12px', color: '#ef4444' }}>{error}</span>}
            <label style={{
              display: 'flex', alignItems: 'center', gap: '4px', fontSize: '12px',
              color: colors.textSecondary, cursor: 'pointer',
            }}>
              <input
                type="checkbox"
                checked={showArchived}
                onChange={(e) => { setShowArchived(e.target.checked); setPage(1); }}
                style={{ accentColor: colors.accent }}
              />
              Archived
            </label>
            <button
              onClick={handleBulkArchive}
              style={{
                padding: '6px 10px', border: `1px solid ${colors.border}`, borderRadius: '6px',
                backgroundColor: colors.inputBg, color: colors.textSecondary, cursor: 'pointer', fontSize: '12px',
                display: 'flex', alignItems: 'center', gap: '4px',
              }}
            >
              <Archive size={12} /> Bulk Archive
            </button>
            <button
              onClick={fetchTasks}
              style={{
                padding: '6px 10px', border: `1px solid ${colors.border}`, borderRadius: '6px',
                backgroundColor: colors.inputBg, color: colors.text, cursor: 'pointer', fontSize: '12px',
                display: 'flex', alignItems: 'center', gap: '4px',
              }}
            >
              <RefreshCw size={12} /> Refresh
            </button>
          </div>
        </div>

        {/* Status filter tabs */}
        <div style={{ display: 'flex', gap: '4px', marginTop: '12px', flexWrap: 'wrap' }}>
          {FILTERS.map(tab => {
            const isActive = statusFilter === tab.value;
            const count = statusCounts[tab.value];
            return (
              <button
                key={tab.value}
                onClick={() => setStatusFilter(tab.value)}
                style={{
                  padding: '6px 14px', borderRadius: '6px',
                  border: `1px solid ${isActive ? '#3b82f6' : colors.border}`,
                  backgroundColor: isActive ? (isDark ? 'rgba(59,130,246,0.15)' : '#eff6ff') : 'transparent',
                  color: isActive ? '#3b82f6' : colors.textSecondary,
                  cursor: 'pointer', fontSize: '13px', fontWeight: isActive ? 600 : 400,
                }}
              >
                {tab.label} ({count})
              </button>
            );
          })}
        </div>
      </div>

      {/* Task List */}
      {filteredTasks.length === 0 ? (
        <div style={cardStyle}>
          <div style={{ textAlign: 'center', padding: '40px 20px', color: colors.textMuted, fontSize: '14px' }}>
            {tasks.length === 0 ? 'No tasks found' : `No ${statusFilter} tasks`}
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {filteredTasks.map(task => {
            const sc = STATUS_CONFIG[task.status] || STATUS_CONFIG.pending;
            const pc = PRIORITY_CONFIG[task.priority] || PRIORITY_CONFIG.medium;
            const isExpanded = expandedTask === task.task_id;
            const isLoading = actionLoading[task.task_id];

            return (
              <div
                key={task.task_id}
                style={{
                  backgroundColor: colors.cardBg,
                  borderRadius: '12px',
                  border: `1px solid ${colors.border}`,
                  borderLeft: `4px solid ${sc.color}`,
                  boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
                  opacity: task.archived ? 0.6 : 1,
                }}
              >
                {/* Task header */}
                <div
                  onClick={() => setExpandedTask(isExpanded ? null : task.task_id)}
                  style={{
                    padding: isMobile ? '10px' : '12px 14px',
                    cursor: 'pointer',
                    display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                  }}
                >
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap', marginBottom: '4px' }}>
                      <span style={{
                        fontSize: '11px', padding: '2px 8px', borderRadius: '10px',
                        backgroundColor: isDark ? sc.bgDark : sc.bgLight,
                        color: sc.color, fontWeight: 500,
                        display: 'inline-flex', alignItems: 'center', gap: '4px',
                      }}>
                        <span style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: sc.color, display: 'inline-block' }} />
                        {sc.label}
                      </span>
                      <span style={{
                        fontSize: '10px', padding: '2px 6px', borderRadius: '4px',
                        backgroundColor: isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.04)',
                        color: pc.color, fontWeight: 600, textTransform: 'uppercase',
                      }}>
                        {task.priority}
                      </span>
                      {task.type && (
                        <span style={{
                          fontSize: '10px', padding: '2px 6px', borderRadius: '4px',
                          backgroundColor: isDark ? 'rgba(59,130,246,0.1)' : 'rgba(59,130,246,0.08)',
                          color: '#3b82f6',
                        }}>
                          {task.type}
                        </span>
                      )}
                      {task.archived && (
                        <span style={{
                          fontSize: '10px', padding: '2px 6px', borderRadius: '4px',
                          backgroundColor: isDark ? 'rgba(107,114,128,0.2)' : 'rgba(107,114,128,0.1)',
                          color: '#6b7280',
                        }}>
                          archived
                        </span>
                      )}
                    </div>
                    <div style={{
                      fontSize: '13px', color: colors.text,
                      overflow: 'hidden', textOverflow: 'ellipsis',
                      whiteSpace: isExpanded ? 'normal' : 'nowrap',
                    }}>
                      {task.description}
                    </div>
                    <div style={{ fontSize: '11px', color: colors.textMuted, marginTop: '2px', display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                      <span>{new Date(task.created_at).toLocaleString('ja-JP', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                      {task.assigned_to && (
                        <>
                          <span>|</span>
                          <span style={{ color: colors.accent }}>{task.assigned_to}</span>
                        </>
                      )}
                      {task.source && (
                        <>
                          <span>|</span>
                          <span>{task.source}</span>
                        </>
                      )}
                    </div>
                  </div>
                  <span style={{ display: 'inline-flex', color: colors.textMuted, flexShrink: 0, marginLeft: '8px' }}>
                    {isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                  </span>
                </div>

                {/* Expanded actions */}
                {isExpanded && (
                  <div style={{
                    padding: `0 ${isMobile ? '10px' : '14px'} ${isMobile ? '10px' : '14px'}`,
                    borderTop: `1px solid ${colors.border}`,
                    display: 'flex', gap: '8px', paddingTop: '10px', flexWrap: 'wrap',
                  }}>
                    <div style={{ fontSize: '11px', color: colors.textMuted, fontFamily: 'monospace', marginBottom: '6px', width: '100%' }}>
                      ID: {task.task_id}
                    </div>
                    {task.status !== 'completed' && task.status !== 'cancelled' && (
                      <>
                        <button
                          onClick={(e) => { e.stopPropagation(); handleComplete(task.task_id); }}
                          disabled={isLoading}
                          style={{
                            padding: '6px 14px', borderRadius: '8px', border: 'none', cursor: isLoading ? 'default' : 'pointer',
                            fontSize: '12px', fontWeight: 500, backgroundColor: '#22c55e', color: '#fff',
                            display: 'flex', alignItems: 'center', gap: '4px', opacity: isLoading ? 0.6 : 1,
                          }}
                        >
                          <CheckCircle size={14} /> Complete
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); handleCancel(task.task_id); }}
                          disabled={isLoading}
                          style={{
                            padding: '6px 14px', borderRadius: '8px', border: 'none', cursor: isLoading ? 'default' : 'pointer',
                            fontSize: '12px', fontWeight: 500, backgroundColor: isDark ? '#7f1d1d' : '#fef2f2', color: '#ef4444',
                            display: 'flex', alignItems: 'center', gap: '4px', opacity: isLoading ? 0.6 : 1,
                          }}
                        >
                          <XCircle size={14} /> Cancel
                        </button>
                      </>
                    )}
                    {!task.archived && (
                      <button
                        onClick={(e) => { e.stopPropagation(); handleArchive(task.task_id); }}
                        disabled={isLoading}
                        style={{
                          padding: '6px 14px', borderRadius: '8px', border: `1px solid ${colors.border}`, cursor: isLoading ? 'default' : 'pointer',
                          fontSize: '12px', fontWeight: 500, backgroundColor: 'transparent', color: colors.textSecondary,
                          display: 'flex', alignItems: 'center', gap: '4px', opacity: isLoading ? 0.6 : 1,
                        }}
                      >
                        <Archive size={14} /> Archive
                      </button>
                    )}
                    {task.archived && (
                      <button
                        onClick={async (e) => {
                          e.stopPropagation();
                          setActionLoading(prev => ({ ...prev, [task.task_id]: true }));
                          try { await api.unarchiveTask(task.task_id); await fetchTasks(); }
                          finally { setActionLoading(prev => ({ ...prev, [task.task_id]: false })); }
                        }}
                        disabled={isLoading}
                        style={{
                          padding: '6px 14px', borderRadius: '8px', border: `1px solid ${colors.border}`, cursor: isLoading ? 'default' : 'pointer',
                          fontSize: '12px', fontWeight: 500, backgroundColor: 'transparent', color: colors.textSecondary,
                          display: 'flex', alignItems: 'center', gap: '4px', opacity: isLoading ? 0.6 : 1,
                        }}
                      >
                        <Archive size={14} /> Unarchive
                      </button>
                    )}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        showConfirm({
                          message: `Permanently delete task "${task.task_id.substring(0, 8)}..."?`,
                          confirmText: 'Delete',
                          isDanger: true,
                          onConfirm: async () => {
                            setActionLoading(prev => ({ ...prev, [task.task_id]: true }));
                            try { await api.deleteTask(task.task_id); await fetchTasks(); }
                            finally { setActionLoading(prev => ({ ...prev, [task.task_id]: false })); }
                          },
                        });
                      }}
                      disabled={isLoading}
                      style={{
                        padding: '6px 14px', borderRadius: '8px', border: `1px solid ${isDark ? '#7f1d1d' : '#fecaca'}`, cursor: isLoading ? 'default' : 'pointer',
                        fontSize: '12px', fontWeight: 500, backgroundColor: 'transparent', color: '#ef4444',
                        display: 'flex', alignItems: 'center', gap: '4px', opacity: isLoading ? 0.6 : 1,
                      }}
                    >
                      <Trash2 size={14} /> Delete
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination */}
      {pagination && pagination.total_pages > 1 && (
        <div style={{
          display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '8px',
          marginTop: '16px', fontSize: '13px', color: colors.textSecondary,
        }}>
          <button
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page === 1}
            style={{
              padding: '6px 12px', borderRadius: '6px', border: `1px solid ${colors.border}`,
              backgroundColor: colors.inputBg, color: colors.text, cursor: page === 1 ? 'default' : 'pointer',
              opacity: page === 1 ? 0.4 : 1,
            }}
          >
            Prev
          </button>
          <span>{page} / {pagination.total_pages}</span>
          <button
            onClick={() => setPage(p => Math.min(pagination.total_pages, p + 1))}
            disabled={page === pagination.total_pages}
            style={{
              padding: '6px 12px', borderRadius: '6px', border: `1px solid ${colors.border}`,
              backgroundColor: colors.inputBg, color: colors.text, cursor: page === pagination.total_pages ? 'default' : 'pointer',
              opacity: page === pagination.total_pages ? 0.4 : 1,
            }}
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
};
