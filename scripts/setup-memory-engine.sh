#!/bin/bash
# memory-engine セットアップスクリプト
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ME_DIR="$REPO_ROOT/services/memory-engine"

echo "=== Memory Engine Setup ==="

# Python確認
if ! command -v python3 &>/dev/null; then
    echo "ERROR: python3 not found"
    exit 1
fi

PYTHON_VERSION=$(python3 --version 2>&1)
echo "Python: $PYTHON_VERSION"

# venv作成
if [ ! -d "$ME_DIR/.venv" ]; then
    echo "[..] Creating virtual environment..."
    python3 -m venv "$ME_DIR/.venv"
    echo "[OK] venv created"
fi

# 依存インストール
echo "[..] Installing dependencies..."
"$ME_DIR/.venv/bin/pip" install -q -e "$ME_DIR" 2>&1 | tail -3
echo "[OK] Dependencies installed"

# DB初期化確認
DB_PATH="${MEMORY_ENGINE_DB:-$HOME/.claude/memory.db}"
if [ -f "$DB_PATH" ]; then
    echo "[OK] Database exists: $DB_PATH"
else
    echo "[--] Database will be created on first use: $DB_PATH"
fi

# MLX確認
if command -v mlx_lm.server &>/dev/null; then
    echo "[OK] MLX server available"
else
    echo "[--] MLX server not found (pip install mlx-lm for local LLM)"
fi

echo ""
echo "=== Setup Complete ==="
echo "  Start MCP server: cd $ME_DIR && .venv/bin/python -m memory_engine.cli serve"
echo "  Start MLX server: bash scripts/start-mlx.sh"
