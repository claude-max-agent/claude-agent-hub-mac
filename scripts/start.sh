#!/bin/bash
# MacBook版 Claude Agent Hub — 全サービス起動
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SESSION_NAME="manager"

cd "$REPO_ROOT"

echo "=== Claude Agent Hub (Mac) Starting ==="

# 前提チェック
missing=()
for cmd in claude tmux jq sqlite3 git gh; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
done
if [ ${#missing[@]} -gt 0 ]; then
    echo "ERROR: Missing commands: ${missing[*]}"
    echo "  brew install ${missing[*]}"
    exit 1
fi

# ディレクトリ確保
mkdir -p "$REPO_ROOT"/{.claude/state,agents/tmp-agents,data,logs}

# SQLite DB初期化
if [ -f "$REPO_ROOT/scripts/schema.sql" ] && [ ! -f "$REPO_ROOT/data/claude-hub.db" ]; then
    sqlite3 "$REPO_ROOT/data/claude-hub.db" < "$REPO_ROOT/scripts/schema.sql"
    echo "[OK] Database initialized"
fi

# Go API起動 (バックグラウンド)
if [ -f "$REPO_ROOT/api/cmd/server/main.go" ]; then
    API_PID_FILE="$REPO_ROOT/logs/api.pid"
    if [ -f "$API_PID_FILE" ] && kill -0 "$(cat "$API_PID_FILE")" 2>/dev/null; then
        echo "[OK] Go API already running (PID: $(cat "$API_PID_FILE"))"
    else
        echo "[..] Building and starting Go API..."
        cd "$REPO_ROOT/api"
        go build -o "$REPO_ROOT/api/hub-api" ./cmd/server/ 2>/dev/null && {
            "$REPO_ROOT/api/hub-api" > "$REPO_ROOT/logs/api.log" 2>&1 &
            echo $! > "$API_PID_FILE"
            echo "[OK] Go API started (PID: $!, port: 8080)"
        } || echo "[SKIP] Go API build failed (will set up in Phase 2)"
        cd "$REPO_ROOT"
    fi
else
    echo "[SKIP] Go API not yet available"
fi

# MLX LLM Server起動 (バックグラウンド)
if command -v mlx_lm.server &>/dev/null; then
    MLX_PID_FILE="$REPO_ROOT/logs/mlx.pid"
    if [ -f "$MLX_PID_FILE" ] && kill -0 "$(cat "$MLX_PID_FILE")" 2>/dev/null; then
        echo "[OK] MLX server already running (PID: $(cat "$MLX_PID_FILE"))"
    else
        echo "[..] Starting MLX LLM server..."
        mlx_lm.server --model mlx-community/Qwen2.5-7B-Instruct-4bit --port 11434 \
            > "$REPO_ROOT/logs/mlx.log" 2>&1 &
        echo $! > "$MLX_PID_FILE"
        echo "[OK] MLX server started (PID: $!, port: 11434)"
    fi
else
    echo "[SKIP] MLX server not available (pip install mlx-lm)"
fi

# Manager tmux セッション
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo "[OK] Manager session already running"
    echo ""
    echo "  Attach: tmux attach -t $SESSION_NAME"
else
    tmux new-session -d -s "$SESSION_NAME" -c "$REPO_ROOT"
    tmux send-keys -t "$SESSION_NAME" "claude" Enter
    echo "[OK] Manager started in tmux session: $SESSION_NAME"
    echo ""
    echo "  Attach: tmux attach -t $SESSION_NAME"
    echo "  Detach: Ctrl+b → d"
fi

echo ""
echo "  Status: bash scripts/status.sh"
echo "  Stop:   bash scripts/stop.sh"
echo "=== Startup Complete ==="
