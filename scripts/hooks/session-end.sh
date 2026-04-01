#!/bin/bash
# SessionEnd hook: セッション状態保存

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STATE_DIR="$REPO_ROOT/.claude/state"

mkdir -p "$STATE_DIR"

# セッション履歴ログ
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) session_end" \
    >> "$STATE_DIR/session_history.log"

exit 0
