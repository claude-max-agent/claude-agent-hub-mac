# Architecture Diagram Prompt

anygen.io 用のプロンプト。

## Prompt

```
Create an architecture diagram for a multi-agent orchestration system called "claude-agent-hub-mac".

Layout (top-down hierarchy):

Top layer - "Admin (Human)":
- Terminal (tmux session: manager)
- Connected to Manager

Middle layer - "Manager (Claude Code - always running)":
- Receives instructions from Admin via terminal
- Manages tmp-agent lifecycle
- Connected to:
  - Go API Server (localhost:18080) - Task CRUD, Agent State
  - Obsidian Vault (~/Obsidian/claude_work/) - Dashboard auto-update
  - Claude Code Hooks (SessionEnd, PostToolUse, etc.)
  - Memory Engine (MCP) - Long-term context

Bottom layer - "tmp-agent pool (max 3)":
- Each agent is an independent Claude Code process in its own tmux session
- Templates: dev (code changes, PRs) / research (investigation)
- Disposable: created for a task, destroyed on completion
- Reports back to Manager via tmux send-keys

Side connections:
- GitHub (claude-max-agent org) - PRs, Issues, repos
- Codex CLI (GPT-5.4) - Second opinion, rescue tasks

Key data flows (use arrows):
1. Admin → Manager: task instructions
2. Manager → tmp-agent: launch with template + briefing
3. tmp-agent → GitHub: commits, PRs
4. tmp-agent → Manager: completion report
5. Manager → Obsidian: dashboard.md update (via hooks)
6. Manager → Go API: task state persistence

Style: clean, minimal, dark theme, rounded boxes, use icons where possible.
```
