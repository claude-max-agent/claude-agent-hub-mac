#!/bin/bash
# MacBook版 Claude Agent Hub — 初回セットアップ
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"

echo "=== Claude Agent Hub (Mac) Setup ==="
echo ""

# 前提チェック
echo "Checking prerequisites..."
missing=()
for cmd in claude tmux jq git gh sqlite3; do
    if command -v "$cmd" &>/dev/null; then
        echo "  [OK] $cmd"
    else
        echo "  [NG] $cmd"
        missing+=("$cmd")
    fi
done

# オプショナルツール
for cmd in go node python3 ruff mlx_lm.server; do
    if command -v "$cmd" &>/dev/null; then
        echo "  [OK] $cmd (optional)"
    else
        echo "  [--] $cmd (optional, not found)"
    fi
done

if [ ${#missing[@]} -gt 0 ]; then
    echo ""
    echo "ERROR: Missing required commands: ${missing[*]}"
    echo "  brew install ${missing[*]}"
    exit 1
fi
echo ""

# ディレクトリ作成
echo "Creating directories..."
mkdir -p "$REPO_ROOT"/{.claude/state,agents/tmp-agents,context/state,context/learnings,data,logs}
touch "$REPO_ROOT/agents/tmp-agents/.gitkeep"
touch "$REPO_ROOT/context/state/.gitkeep"
touch "$REPO_ROOT/context/learnings/.gitkeep"
touch "$REPO_ROOT/data/.gitkeep"
touch "$REPO_ROOT/logs/.gitkeep"
echo "  [OK] Directories created"

# worktreeディレクトリ
mkdir -p "$HOME/hub-worktrees"
echo "  [OK] Worktree directory: $HOME/hub-worktrees"

# 状態ファイル初期化
if [ ! -f "$REPO_ROOT/.claude/state/managed-instances.json" ]; then
    echo "[]" > "$REPO_ROOT/.claude/state/managed-instances.json"
fi

# スクリプトに実行権限
chmod +x "$REPO_ROOT"/scripts/*.sh 2>/dev/null || true
chmod +x "$REPO_ROOT"/scripts/hooks/*.sh 2>/dev/null || true
echo "  [OK] Script permissions set"

# RAM検出 & limits.yaml 自動調整
echo ""
echo "Detecting system resources..."
if [ "$(uname)" = "Darwin" ]; then
    TOTAL_MEM_MB=$(sysctl -n hw.memsize 2>/dev/null | awk '{print int($1/1024/1024)}')
else
    TOTAL_MEM_MB=$(free -m | awk '/^Mem:/{print $2}')
fi
echo "  RAM: ${TOTAL_MEM_MB} MB"

if [ "$TOTAL_MEM_MB" -le 16384 ]; then
    echo "  Profile: 16GB (conservative)"
    echo "  Recommendation: max_instances=2, memory_spawn_block_mb=2048"
elif [ "$TOTAL_MEM_MB" -le 32768 ]; then
    echo "  Profile: 32GB (default)"
    echo "  Using default limits.yaml settings"
else
    echo "  Profile: 64GB+ (generous)"
    echo "  Recommendation: Consider increasing max_instances in config/limits.yaml"
fi

# Obsidian Vault確認
echo ""
VAULT_PATH="$HOME/Documents/Obsidian/claude_work"
if [ -d "$VAULT_PATH" ]; then
    echo "  [OK] Obsidian Vault found: $VAULT_PATH"
else
    echo "  [--] Obsidian Vault not found: $VAULT_PATH"
    echo "       Create it manually or update config/services.yaml"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Start:  bash scripts/start.sh"
echo "  2. Attach: tmux attach -t manager"
echo "  3. Status: bash scripts/status.sh"
echo ""
echo "Optional:"
echo "  - Set up Go API (Phase 2): See api/ directory"
echo "  - Set up MLX server: pip install mlx-lm"
echo "  - Configure .env for secrets"
