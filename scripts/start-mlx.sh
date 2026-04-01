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

mkdir -p "$REPO_ROOT/logs"

# 既に起動中か確認
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "[OK] MLX server already running (PID: $(cat "$PID_FILE"), port: $PORT)"
    exit 0
fi

# mlx_lm.server の存在確認
if ! command -v mlx_lm.server &>/dev/null; then
    echo "ERROR: mlx_lm.server not found"
    echo "  Install: pip install mlx-lm"
    exit 1
fi

echo "[..] Starting MLX server (model: $MODEL, port: $PORT)..."
mlx_lm.server --model "$MODEL" --port "$PORT" > "$LOG_FILE" 2>&1 &
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
