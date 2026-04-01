#!/bin/bash
# MacBook版 tmp-agent ライフサイクル管理
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
STATE_FILE="$REPO_ROOT/.claude/state/managed-instances.json"
WORKTREE_DIR="$HOME/hub-worktrees"
LIMITS_FILE="$REPO_ROOT/config/limits.yaml"
SPAWN_LOG="$REPO_ROOT/logs/spawn-history.jsonl"

# --- Utility Functions ---

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

get_limit() {
    local key="$1"
    local default="$2"
    if [ -f "$LIMITS_FILE" ] && command -v python3 &>/dev/null; then
        python3 -c "
import yaml, sys
with open('$LIMITS_FILE') as f:
    d = yaml.safe_load(f)
keys = '$key'.split('.')
for k in keys:
    d = d.get(k, {})
print(d if d else '$default')
" 2>/dev/null || echo "$default"
    else
        echo "$default"
    fi
}

ensure_state_file() {
    mkdir -p "$(dirname "$STATE_FILE")"
    if [ ! -f "$STATE_FILE" ] || [ ! -s "$STATE_FILE" ]; then
        echo "[]" > "$STATE_FILE"
    fi
}

log_spawn() {
    local action="$1" name="$2" template="${3:-}" extra="${4:-}"
    mkdir -p "$(dirname "$SPAWN_LOG")"
    jq -n --arg action "$action" --arg name "$name" --arg template "$template" \
        --arg extra "$extra" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        '{timestamp: $ts, action: $action, name: $name, template: $template, extra: $extra}' \
        >> "$SPAWN_LOG"
}

get_instance_count() {
    ensure_state_file
    jq 'length' "$STATE_FILE"
}

# --- Commands ---

cmd_start() {
    local template="dev"
    local name=""
    local cli_opts=""
    local issue=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --template) template="$2"; shift 2 ;;
            --cli-opts) cli_opts="$2"; shift 2 ;;
            --issue) issue="$2"; shift 2 ;;
            *) name="$1"; shift ;;
        esac
    done

    if [ -z "$name" ]; then
        echo "ERROR: Agent name required"
        echo "Usage: $0 start --template <type> <name> [--cli-opts \"...\"]"
        exit 1
    fi

    # 重複チェック
    if tmux has-session -t "$name" 2>/dev/null; then
        echo "ERROR: Session '$name' already exists"
        exit 1
    fi

    # インスタンス上限チェック
    local max_instances
    max_instances=$(get_limit "agents.max_instances" 3)
    local current_count
    current_count=$(get_instance_count)
    if [ "$current_count" -ge "$max_instances" ]; then
        echo "ERROR: Instance limit reached ($current_count/$max_instances)"
        echo "  Stop an existing agent first: $0 stop <name>"
        exit 1
    fi

    # メモリチェック
    local memory_floor
    memory_floor=$(get_limit "autonomy.memory_spawn_block_mb" 4096)
    local free_mem
    free_mem=$(get_free_memory_mb)
    if [ "$free_mem" -lt "$memory_floor" ]; then
        echo "ERROR: Insufficient memory (${free_mem}MB free, need ${memory_floor}MB)"
        exit 1
    fi

    # テンプレートチェック
    local template_dir="$REPO_ROOT/agents/templates/$template"
    if [ ! -d "$template_dir" ]; then
        echo "ERROR: Template '$template' not found at $template_dir"
        exit 1
    fi

    # 作業ディレクトリ生成
    local agent_dir="$REPO_ROOT/agents/tmp-agents/$name"
    mkdir -p "$agent_dir"

    # テンプレートマージ (base + specific)
    if [ -x "$SCRIPT_DIR/merge-template.sh" ]; then
        bash "$SCRIPT_DIR/merge-template.sh" "$template" "$name"
    else
        # 簡易マージ: base → template の順でコピー
        local base_dir="$REPO_ROOT/agents/templates/base"
        if [ -d "$base_dir" ]; then
            cp -r "$base_dir"/. "$agent_dir/" 2>/dev/null || true
        fi
        cp -r "$template_dir"/. "$agent_dir/" 2>/dev/null || true
    fi

    # tmuxセッション作成
    tmux new-session -d -s "$name" -c "$agent_dir"

    # Claude Code起動
    local claude_cmd="claude"
    [ -n "$cli_opts" ] && claude_cmd="claude $cli_opts"
    [ -n "$issue" ] && claude_cmd="$claude_cmd --issue $issue"
    tmux send-keys -t "$name" "$claude_cmd" Enter

    # 状態ファイル更新
    ensure_state_file
    local new_entry
    new_entry=$(jq -n \
        --arg name "$name" \
        --arg template "$template" \
        --arg status "running" \
        --arg started "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --arg dir "$agent_dir" \
        --arg issue "$issue" \
        '{name: $name, template: $template, status: $status, started_at: $started, dir: $dir, issue: $issue}')
    jq --argjson entry "$new_entry" '. += [$entry]' "$STATE_FILE" > "${STATE_FILE}.tmp" \
        && mv "${STATE_FILE}.tmp" "$STATE_FILE"

    log_spawn "start" "$name" "$template" "$issue"

    echo "Started tmp-agent: $name (template: $template)"
    echo "  Attach: tmux attach -t $name"
    echo "  Stop:   $0 stop $name"
}

cmd_stop() {
    local name="$1"

    if [ -z "$name" ]; then
        echo "ERROR: Agent name required"
        exit 1
    fi

    # Claude Codeに終了を送信
    if tmux has-session -t "$name" 2>/dev/null; then
        tmux send-keys -t "$name" "/exit" Enter
        sleep 2
        tmux kill-session -t "$name" 2>/dev/null || true
        echo "[OK] Session '$name' stopped"
    else
        echo "[--] Session '$name' not found"
    fi

    # 作業ディレクトリ削除
    local agent_dir="$REPO_ROOT/agents/tmp-agents/$name"
    if [ -d "$agent_dir" ]; then
        rm -rf "$agent_dir"
        echo "[OK] Working directory cleaned up"
    fi

    # 状態ファイルから削除
    ensure_state_file
    jq --arg name "$name" 'map(select(.name != $name))' "$STATE_FILE" > "${STATE_FILE}.tmp" \
        && mv "${STATE_FILE}.tmp" "$STATE_FILE"

    log_spawn "stop" "$name"

    echo "Stopped tmp-agent: $name"
}

cmd_list() {
    ensure_state_file
    local count
    count=$(jq 'length' "$STATE_FILE")
    local max_instances
    max_instances=$(get_limit "agents.max_instances" 3)

    echo "Tmp Agents ($count/$max_instances):"
    echo ""

    if [ "$count" -eq 0 ]; then
        echo "  (none)"
        return
    fi

    jq -r '.[] | "  \(.name)\t[\(.template)]\t\(.status)\tstarted: \(.started_at)"' "$STATE_FILE"
}

cmd_cleanup() {
    ensure_state_file
    local cleaned=0

    # 状態ファイルにあるがtmuxセッションが存在しないエントリを検出
    local names
    names=$(jq -r '.[].name' "$STATE_FILE")

    for name in $names; do
        if ! tmux has-session -t "$name" 2>/dev/null; then
            echo "[CLEANUP] Orphaned entry: $name"
            jq --arg name "$name" 'map(select(.name != $name))' "$STATE_FILE" > "${STATE_FILE}.tmp" \
                && mv "${STATE_FILE}.tmp" "$STATE_FILE"

            # 作業ディレクトリも削除
            local agent_dir="$REPO_ROOT/agents/tmp-agents/$name"
            [ -d "$agent_dir" ] && rm -rf "$agent_dir"

            log_spawn "cleanup" "$name"
            cleaned=$((cleaned + 1))
        fi
    done

    if [ "$cleaned" -eq 0 ]; then
        echo "No orphans found."
    else
        echo "Cleaned up $cleaned orphaned entries."
    fi
}

# --- Main ---

usage() {
    echo "Usage: $0 <command> [args...]"
    echo ""
    echo "Commands:"
    echo "  start --template <type> <name> [--cli-opts \"...\"] [--issue <URL>]"
    echo "  stop <name>"
    echo "  list"
    echo "  cleanup"
    echo ""
    echo "Templates: dev, research"
}

case "${1:-}" in
    start)   shift; cmd_start "$@" ;;
    stop)    shift; cmd_stop "$@" ;;
    list)    cmd_list ;;
    cleanup) cmd_cleanup ;;
    *)       usage ;;
esac
