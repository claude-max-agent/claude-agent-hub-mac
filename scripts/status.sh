#!/bin/bash
# MacBook版 Claude Agent Hub — ステータス表示
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Claude Agent Hub (Mac) Status ==="
echo ""

# --- メモリ ---
echo "--- Memory ---"
if [ "$(uname)" = "Darwin" ]; then
    page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)
    free_pages=$(vm_stat | awk '/Pages free/ {gsub(/\./, "", $3); print $3}')
    inactive=$(vm_stat | awk '/Pages inactive/ {gsub(/\./, "", $3); print $3}')
    total_mb=$(sysctl -n hw.memsize 2>/dev/null | awk '{print int($1/1024/1024)}')
    avail_mb=$(( (free_pages + inactive) * page_size / 1024 / 1024 ))
    echo "Total: ${total_mb} MB | Available: ${avail_mb} MB"
else
    free -m | head -2
fi
echo ""

# --- サービス ---
echo "--- Services ---"

# Go API
API_PID_FILE="$REPO_ROOT/logs/api.pid"
if [ -f "$API_PID_FILE" ] && kill -0 "$(cat "$API_PID_FILE")" 2>/dev/null; then
    echo "Go API:     RUNNING (PID: $(cat "$API_PID_FILE"), port: 8080)"
else
    echo "Go API:     STOPPED"
fi

# MLX Server
MLX_PID_FILE="$REPO_ROOT/logs/mlx.pid"
if [ -f "$MLX_PID_FILE" ] && kill -0 "$(cat "$MLX_PID_FILE")" 2>/dev/null; then
    echo "MLX Server: RUNNING (PID: $(cat "$MLX_PID_FILE"), port: 11434)"
else
    echo "MLX Server: STOPPED"
fi
echo ""

# --- tmux Sessions ---
echo "--- tmux Sessions ---"
tmux list-sessions 2>/dev/null || echo "(none)"
echo ""

# --- Tmp Agents ---
echo "--- Tmp Agents ---"
STATE_FILE="$REPO_ROOT/.claude/state/managed-instances.json"
if [ -f "$STATE_FILE" ] && [ -s "$STATE_FILE" ]; then
    jq -r '.[] | "  \(.name) [\(.template)] — \(.status // "unknown")"' "$STATE_FILE" 2>/dev/null || echo "(parse error)"
else
    echo "(none)"
fi
echo ""

echo "=== End ==="
