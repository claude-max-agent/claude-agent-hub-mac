#!/bin/bash
# PreToolUse hook: Agent spawn制限
# メモリ不足やインスタンス上限時にspawnを拒否

if [ "${CLAUDE_TOOL_NAME:-}" != "Agent" ]; then
    exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# メモリチェック (macOS対応)
get_free_memory_mb() {
    if [ "$(uname)" = "Darwin" ]; then
        local page_size
        page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)
        local free_pages inactive
        free_pages=$(vm_stat | awk '/Pages free/ {gsub(/\./, "", $3); print $3}')
        inactive=$(vm_stat | awk '/Pages inactive/ {gsub(/\./, "", $3); print $3}')
        echo $(( (free_pages + inactive) * page_size / 1024 / 1024 ))
    else
        free -m | awk '/^Mem:/{print $7}'
    fi
}

FREE_MEM=$(get_free_memory_mb)
MEMORY_FLOOR=4096  # デフォルト

# limits.yaml から読み取り
if [ -f "$REPO_ROOT/config/limits.yaml" ] && command -v python3 &>/dev/null; then
    MEMORY_FLOOR=$(python3 -c "
import yaml
with open('$REPO_ROOT/config/limits.yaml') as f:
    d = yaml.safe_load(f)
print(d.get('autonomy', {}).get('memory_spawn_block_mb', 4096))
" 2>/dev/null || echo 4096)
fi

if [ "$FREE_MEM" -lt "$MEMORY_FLOOR" ]; then
    echo "BLOCKED: Insufficient memory for spawn (${FREE_MEM}MB free, need ${MEMORY_FLOOR}MB)" >&2
    exit 2
fi

exit 0
