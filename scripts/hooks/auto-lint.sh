#!/bin/bash
# PostToolUse hook: 変更されたファイルに対して自動lint
# Edit/Write ツール使用後に実行

if [[ "${CLAUDE_TOOL_NAME:-}" != "Edit" && "${CLAUDE_TOOL_NAME:-}" != "Write" ]]; then
    exit 0
fi

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)

if [ -z "$FILE_PATH" ] || [ ! -f "$FILE_PATH" ]; then
    exit 0
fi

EXT="${FILE_PATH##*.}"

case "$EXT" in
    py)
        if command -v ruff &>/dev/null; then
            ruff check --fix "$FILE_PATH" 2>/dev/null || true
            ruff format "$FILE_PATH" 2>/dev/null || true
        fi
        ;;
    go)
        if command -v gofmt &>/dev/null; then
            gofmt -w "$FILE_PATH" 2>/dev/null || true
        fi
        ;;
    ts|tsx|js|jsx)
        # プロジェクトにeslint設定があれば実行
        PROJECT_DIR=$(dirname "$FILE_PATH")
        while [ "$PROJECT_DIR" != "/" ]; do
            if [ -f "$PROJECT_DIR/.eslintrc.json" ] || [ -f "$PROJECT_DIR/.eslintrc.js" ] || [ -f "$PROJECT_DIR/eslint.config.js" ]; then
                npx eslint --fix "$FILE_PATH" 2>/dev/null || true
                break
            fi
            PROJECT_DIR=$(dirname "$PROJECT_DIR")
        done
        ;;
esac

exit 0
