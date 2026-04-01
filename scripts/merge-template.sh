#!/bin/bash
# テンプレートマージ: base + specific → agents/tmp-agents/<name>/
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TEMPLATE="${1:?Usage: $0 <template> <name>}"
NAME="${2:?Usage: $0 <template> <name>}"

BASE_DIR="$REPO_ROOT/agents/templates/base"
TEMPLATE_DIR="$REPO_ROOT/agents/templates/$TEMPLATE"
TARGET_DIR="$REPO_ROOT/agents/tmp-agents/$NAME"

if [ ! -d "$TEMPLATE_DIR" ]; then
    echo "ERROR: Template '$TEMPLATE' not found" >&2
    exit 1
fi

mkdir -p "$TARGET_DIR"

# base テンプレートをコピー
if [ -d "$BASE_DIR" ]; then
    cp -r "$BASE_DIR"/. "$TARGET_DIR/"
fi

# specific テンプレートを上書きコピー（CLAUDE.mdは追記）
if [ -f "$TEMPLATE_DIR/CLAUDE.md" ] && [ -f "$TARGET_DIR/CLAUDE.md" ]; then
    echo "" >> "$TARGET_DIR/CLAUDE.md"
    cat "$TEMPLATE_DIR/CLAUDE.md" >> "$TARGET_DIR/CLAUDE.md"
else
    cp -r "$TEMPLATE_DIR"/. "$TARGET_DIR/" 2>/dev/null || true
fi

# .claude/settings.json は specific が優先（存在する場合）
if [ -f "$TEMPLATE_DIR/.claude/settings.json" ]; then
    mkdir -p "$TARGET_DIR/.claude"
    cp "$TEMPLATE_DIR/.claude/settings.json" "$TARGET_DIR/.claude/settings.json"
fi

echo "Merged template: base + $TEMPLATE → $TARGET_DIR"
