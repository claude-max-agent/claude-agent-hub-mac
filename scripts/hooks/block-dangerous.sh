#!/bin/bash
# PreToolUse hook: 危険なBashコマンドをブロック
# 入力: stdin から JSON (tool_input)

if [ "${CLAUDE_TOOL_NAME:-}" != "Bash" ]; then
    exit 0
fi

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)

if [ -z "$COMMAND" ]; then
    exit 0
fi

# 危険パターン
BLOCKED_PATTERNS=(
    'rm -rf /'
    'rm -rf ~'
    'rm -rf \$HOME'
    'mkfs\.'
    'dd if=.* of=/dev/'
    '> /dev/sd'
    'chmod -R 777 /'
    'curl .* | bash'
    'wget .* | bash'
    'eval.*\$\(curl'
    'git push.*--force.*main'
    'git push.*--force.*master'
    'git push.*-f.*main'
    'git push.*-f.*master'
)

for pattern in "${BLOCKED_PATTERNS[@]}"; do
    if echo "$COMMAND" | grep -qE "$pattern"; then
        echo "BLOCKED: Dangerous command pattern detected: $pattern" >&2
        # Hook failure blocks the tool use
        exit 2
    fi
done

exit 0
