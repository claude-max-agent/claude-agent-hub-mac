#!/bin/bash
# SessionStart hook: 前回セッションの状態復元

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STATE_DIR="$REPO_ROOT/.claude/state"

mkdir -p "$STATE_DIR"

# managed-instances.json の存在確認
if [ ! -f "$STATE_DIR/managed-instances.json" ]; then
    echo "[]" > "$STATE_DIR/managed-instances.json"
fi

# セッション履歴ログ
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) session_start event=${CLAUDE_SESSION_EVENT:-unknown}" \
    >> "$STATE_DIR/session_history.log"

exit 0
