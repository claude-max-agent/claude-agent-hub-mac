import React, { useState, useCallback } from 'react';
import { RefreshCw } from 'lucide-react';
import { useTheme } from '../theme';
import { useIsMobile } from '../hooks/useIsMobile';
import { useAppData } from '../hooks/useAppData';

const HoverCard: React.FC<{
  children: React.ReactNode;
  style: React.CSSProperties;
  isDark: boolean;
}> = ({ children, style, isDark }) => {
  const [hovered, setHovered] = useState(false);
  const onEnter = useCallback(() => setHovered(true), []);
  const onLeave = useCallback(() => setHovered(false), []);
  return (
    <div
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
      style={{
        ...style,
        transition: 'transform 0.2s ease, box-shadow 0.2s ease',
        transform: hovered ? 'translateY(-2px)' : 'translateY(0)',
        boxShadow: hovered
          ? (isDark ? '0 4px 12px rgba(0,0,0,0.4)' : '0 4px 12px rgba(0,0,0,0.15)')
          : (style.boxShadow || (isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)')),
      }}
    >
      {children}
    </div>
  );
};

export const Dashboard: React.FC = () => {
  const { colors, isDark } = useTheme();
  const isMobile = useIsMobile();
  const { githubSummary, refreshGitHub, githubRefreshing } = useAppData();

  const recentPRs = githubSummary?.repos.flatMap(repo =>
    repo.recent_prs.map(pr => ({ ...pr, repo: repo.repo }))
  ).slice(0, 5) || [];
  const recentCommits = githubSummary?.repos.flatMap(repo =>
    repo.recent_commits.map(commit => ({ ...commit, repo: repo.repo }))
  ).slice(0, 5) || [];

  return (
    <div style={{ padding: isMobile ? '12px' : '16px', maxWidth: '100%', overflow: 'hidden', boxSizing: 'border-box' }}>
      <h2 style={{ fontSize: '18px', fontWeight: '600', color: colors.text, marginBottom: '16px' }}>Dashboard</h2>

      {/* Repository Status */}
      {githubSummary && githubSummary.repos.length > 0 && (
        <div style={{ marginBottom: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <h3 style={{ fontSize: '14px', fontWeight: '600', color: colors.textMuted, margin: 0, textTransform: 'uppercase', letterSpacing: '0.5px', paddingBottom: '8px', borderBottom: `2px solid ${colors.accent}`, display: 'inline-block' }}>
              Repositories
            </h3>
            <button
              onClick={refreshGitHub}
              disabled={githubRefreshing}
              style={{
                background: 'none',
                border: `1px solid ${colors.border}`,
                borderRadius: '4px',
                padding: '4px 8px',
                fontSize: '11px',
                color: colors.textMuted,
                cursor: githubRefreshing ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                opacity: githubRefreshing ? 0.6 : 1,
              }}
            >
              <RefreshCw size={12} style={{
                animation: githubRefreshing ? 'spin 1s linear infinite' : 'none',
              }} />
              {githubRefreshing ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(auto-fit, minmax(300px, 1fr))', gap: '12px', minWidth: 0 }}>
            {[...githubSummary.repos].sort((a, b) => a.repo.localeCompare(b.repo)).map((repo) => {
              const pendingPRs = repo.recent_prs.filter(pr => pr.state === 'open' && !pr.merged).length;
              const repoName = repo.repo.split('/').pop() || repo.repo;
              const repoUrl = `https://github.com/${repo.repo}`;
              return (
                <HoverCard
                  key={repo.repo}
                  isDark={isDark}
                  style={{
                    backgroundColor: colors.cardBg,
                    borderRadius: '12px',
                    padding: '14px',
                    border: `1px solid ${colors.border}`,
                    borderLeft: `4px solid ${colors.accent}`,
                    boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
                    minWidth: 0,
                    overflow: 'hidden',
                  }}
                >
                  <a
                    href={repoUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ fontSize: '13px', fontWeight: '600', color: colors.accent, textDecoration: 'none', display: 'block', marginBottom: '8px' }}
                  >
                    {repoName}
                  </a>
                  <div style={{ display: 'flex', gap: '12px', fontSize: '12px' }}>
                    <a
                      href={`${repoUrl}/pulls`}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ color: '#10b981', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '6px' }}
                    >
                      <span style={{ fontWeight: '700', fontSize: '18px' }}>{pendingPRs}</span>
                      <span style={{ color: colors.textMuted }}>Pending PRs</span>
                    </a>
                    <a
                      href={`${repoUrl}/issues`}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ color: '#3b82f6', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '6px' }}
                    >
                      <span style={{ fontWeight: '700', fontSize: '18px' }}>{repo.open_issues}</span>
                      <span style={{ color: colors.textMuted }}>Open Issues</span>
                    </a>
                  </div>
                </HoverCard>
              );
            })}
          </div>
        </div>
      )}

      {/* Recent PRs */}
      {githubSummary && recentPRs.length > 0 && (
        <div style={{ marginBottom: '20px' }}>
          <h3 style={{ fontSize: '14px', fontWeight: '600', color: colors.textMuted, marginBottom: '12px', textTransform: 'uppercase', letterSpacing: '0.5px', paddingBottom: '8px', borderBottom: `2px solid ${colors.accent}`, display: 'inline-block' }}>
            Recent Pull Requests
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', minWidth: 0 }}>
            {recentPRs.map((pr: any) => (
              <HoverCard
                key={`${pr.repo}-${pr.number}`}
                isDark={isDark}
                style={{
                  backgroundColor: colors.cardBg,
                  borderRadius: '12px',
                  padding: '14px',
                  border: `1px solid ${colors.border}`,
                  borderLeft: `4px solid ${pr.merged ? '#8b5cf6' : pr.state === 'open' ? '#10b981' : '#6b7280'}`,
                  boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
                  minWidth: 0,
                  overflow: 'hidden',
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '4px', gap: '8px' }}>
                  <a
                    href={pr.html_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ fontSize: '13px', fontWeight: '500', color: colors.text, textDecoration: 'none', overflow: 'hidden', textOverflow: 'ellipsis', flex: 1 }}
                  >
                    {pr.title}
                  </a>
                  <span style={{
                    fontSize: '10px',
                    padding: '2px 6px',
                    borderRadius: '10px',
                    backgroundColor: pr.merged ? '#8b5cf6' : pr.state === 'open' ? '#10b981' : '#6b7280',
                    color: '#fff',
                    whiteSpace: 'nowrap',
                  }}>
                    {pr.merged ? 'merged' : pr.state}
                  </span>
                </div>
                <div style={{ fontSize: '11px', color: colors.textMuted, display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  <span>{pr.repo.split('/').pop()} #{pr.number}</span>
                  <span>•</span>
                  <span>{pr.user}</span>
                  <span>•</span>
                  <span>{new Date(pr.updated_at).toLocaleDateString()}</span>
                </div>
              </HoverCard>
            ))}
          </div>
        </div>
      )}

      {/* Recent Commits */}
      {githubSummary && recentCommits.length > 0 && (
        <div style={{ marginBottom: '20px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <h3 style={{ fontSize: '14px', fontWeight: '600', color: colors.textMuted, margin: 0, textTransform: 'uppercase', letterSpacing: '0.5px', paddingBottom: '8px', borderBottom: `2px solid ${colors.accent}`, display: 'inline-block' }}>
              Recent Commits
            </h3>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', minWidth: 0 }}>
            {recentCommits.map((commit: any) => (
              <HoverCard
                key={`${commit.repo}-${commit.sha}`}
                isDark={isDark}
                style={{
                  backgroundColor: colors.cardBg,
                  borderRadius: '12px',
                  padding: '14px',
                  border: `1px solid ${colors.border}`,
                  borderLeft: `4px solid ${colors.accent}`,
                  boxShadow: isDark ? '0 1px 3px rgba(0,0,0,0.3)' : '0 1px 3px rgba(0,0,0,0.1)',
                  minWidth: 0,
                  overflow: 'hidden',
                }}
              >
                <a
                  href={commit.html_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{ fontSize: '13px', fontWeight: '500', color: colors.text, textDecoration: 'none', display: 'block', marginBottom: '4px', overflow: 'hidden', textOverflow: 'ellipsis' }}
                >
                  {commit.message}
                </a>
                <div style={{ fontSize: '11px', color: colors.textMuted, display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                  <span>{commit.repo.split('/').pop()}</span>
                  <span>•</span>
                  <span>{commit.author}</span>
                  <span>•</span>
                  <span>{new Date(commit.date).toLocaleDateString()}</span>
                  <span>•</span>
                  <span style={{ fontFamily: 'monospace', fontSize: '10px' }}>{commit.sha.substring(0, 7)}</span>
                </div>
              </HoverCard>
            ))}
          </div>
        </div>
      )}

    </div>
  );
};
