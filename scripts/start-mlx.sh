#!/bin/bash
# MLX LLM Server 起動スクリプト
# mlx_lm.server で OpenAI互換APIを提供
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PORT="${MLX_PORT:-11434}"
MODEL="${MLX_MODEL:-mlx-community/Qwen2.5-7B-Instruct-4bit}"
PID_FILE="$REPO_ROOT/logs/mlx.pid"
LOG_FILE="$REPO_ROOT/logs/mlx.log"
VENV_DIR="$REPO_ROOT/services/memory-engine/.venv"

mkdir -p "$REPO_ROOT/logs"

# 既に起動中か確認
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "[OK] MLX server already running (PID: $(cat "$PID_FILE"), port: $PORT)"
    exit 0
fi

# venv作成 (なければ)
if [ ! -d "$VENV_DIR" ]; then
    echo "[..] Creating Python venv..."
    python3 -m venv "$VENV_DIR"
    "$VENV_DIR/bin/pip" install -q -e "$REPO_ROOT/services/memory-engine"
fi

# mlx-lm インストール (なければ)
MLX_SERVER="$VENV_DIR/bin/mlx_lm.server"
if [ ! -x "$MLX_SERVER" ]; then
    echo "[..] Installing mlx-lm..."
    "$VENV_DIR/bin/pip" install -q mlx-lm
fi

if [ ! -x "$MLX_SERVER" ]; then
    echo "ERROR: mlx_lm.server not found after install"
    exit 1
fi

echo "[..] Starting MLX server (model: $MODEL, port: $PORT)..."
"$MLX_SERVER" --model "$MODEL" --port "$PORT" > "$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"

# 起動待ち
for i in $(seq 1 30); do
    if curl -s "http://localhost:$PORT/v1/models" &>/dev/null; then
        echo "[OK] MLX server started (PID: $(cat "$PID_FILE"), port: $PORT)"
        exit 0
    fi
    sleep 1
done

echo "[WARN] MLX server started but health check pending (PID: $(cat "$PID_FILE"))"
echo "  Check: tail -f $LOG_FILE"
