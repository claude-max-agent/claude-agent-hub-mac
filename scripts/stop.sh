#!/bin/bash
# MacBook版 Claude Agent Hub — 全サービス停止
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Claude Agent Hub (Mac) Stopping ==="

# tmp-agents停止
if [ -x "$SCRIPT_DIR/instance-manager.sh" ]; then
    "$SCRIPT_DIR/instance-manager.sh" cleanup 2>/dev/null || true
fi

# Managerセッション停止
if tmux has-session -t manager 2>/dev/null; then
    tmux send-keys -t manager "/exit" Enter
    sleep 2
    tmux kill-session -t manager 2>/dev/null || true
    echo "[OK] Manager session stopped"
else
    echo "[--] Manager session not running"
fi

# Go API停止
API_PID_FILE="$REPO_ROOT/logs/api.pid"
if [ -f "$API_PID_FILE" ]; then
    API_PID="$(cat "$API_PID_FILE")"
    if kill -0 "$API_PID" 2>/dev/null; then
        kill "$API_PID"
        echo "[OK] Go API stopped (PID: $API_PID)"
    fi
    rm -f "$API_PID_FILE"
else
    echo "[--] Go API not running"
fi

# MLX Server停止
MLX_PID_FILE="$REPO_ROOT/logs/mlx.pid"
if [ -f "$MLX_PID_FILE" ]; then
    MLX_PID="$(cat "$MLX_PID_FILE")"
    if kill -0 "$MLX_PID" 2>/dev/null; then
        kill "$MLX_PID"
        echo "[OK] MLX server stopped (PID: $MLX_PID)"
    fi
    rm -f "$MLX_PID_FILE"
else
    echo "[--] MLX server not running"
fi

echo "=== All Services Stopped ==="
