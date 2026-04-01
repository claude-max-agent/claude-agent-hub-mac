#!/bin/bash
# PostToolUse hook: ファイル変更数のスコープ制限チェック
# 変更ファイル数が limits.yaml の max_files_per_task を超えたら警告

if [[ "${CLAUDE_TOOL_NAME:-}" != "Edit" && "${CLAUDE_TOOL_NAME:-}" != "Write" ]]; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 現在の変更ファイル数
CHANGED_FILES=$(git diff --name-only 2>/dev/null | wc -l | tr -d ' ')
STAGED_FILES=$(git diff --cached --name-only 2>/dev/null | wc -l | tr -d ' ')
TOTAL=$((CHANGED_FILES + STAGED_FILES))

# 上限読み取り
MAX_FILES=15
if [ -f "$REPO_ROOT/config/limits.yaml" ] && command -v python3 &>/dev/null; then
    MAX_FILES=$(python3 -c "
import yaml
with open('$REPO_ROOT/config/limits.yaml') as f:
    d = yaml.safe_load(f)
print(d.get('autonomy', {}).get('max_files_per_task', 15))
" 2>/dev/null || echo 15)
fi

if [ "$TOTAL" -gt "$MAX_FILES" ]; then
    echo "WARNING: File change count ($TOTAL) exceeds limit ($MAX_FILES). Consider splitting the task." >&2
    # 警告のみ、ブロックはしない
fi

exit 0
