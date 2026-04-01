#!/usr/bin/env bash
# update-obsidian-dashboard.sh
# SessionEnd hook から呼ばれ、Obsidian vault にダッシュボードを生成・更新する

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECTS_YAML="${PROJECT_DIR}/config/projects.yaml"
VAULT_DIR="${HOME}/Documents/Obsidian/claude_work"
DASHBOARD_FILE="${VAULT_DIR}/ダッシュボード.md"

# ---- 前提チェック ----

if [[ ! -f "${PROJECTS_YAML}" ]]; then
  echo "[dashboard] config/projects.yaml が見つかりません。スキップします。" >&2
  exit 0
fi

if [[ ! -d "${VAULT_DIR}" ]]; then
  echo "[dashboard] Obsidian vault が見つかりません (${VAULT_DIR})。スキップします。" >&2
  exit 0
fi

# ---- gh CLI チェック ----
GH_AVAILABLE=false
if command -v gh &>/dev/null; then
  GH_AVAILABLE=true
fi

# ---- projects.yaml からリポジトリ一覧を取得 ----
# yq → python3+PyYAML → awk の順で試行
get_repos() {
  if command -v yq &>/dev/null; then
    yq -r '.repositories[] | .org + "/" + .name' "${PROJECTS_YAML}" 2>/dev/null
  elif python3 -c "import yaml" &>/dev/null 2>&1; then
    python3 - "${PROJECTS_YAML}" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f:
    data = yaml.safe_load(f)
for r in data.get('repositories', []):
    print(r['org'] + '/' + r['name'])
PYEOF
  else
    # フォールバック: awk でシンプルなYAMLをパース
    # repositories: ブロック内の org/name ペアを抽出
    awk '
      /^repositories:/ { in_repos=1; next }
      /^[a-zA-Z]/ && !/^repositories:/ { in_repos=0 }
      in_repos && /^[[:space:]]+- name:/ { name=$NF }
      in_repos && /^[[:space:]]+org:/ {
        org=$NF
        if (org != "" && name != "") print org "/" name
        name=""
      }
    ' "${PROJECTS_YAML}"
  fi
}

REPOS=$(get_repos 2>/dev/null || true)

if [[ -z "${REPOS}" ]]; then
  echo "[dashboard] リポジトリ一覧の取得に失敗しました。" >&2
  exit 0
fi

# ---- タイムスタンプ ----
# macOS の date は %:z 非対応のため手動でコロンを挿入
TZ_OFFSET=$(date '+%z')  # 例: +0900
TZ_ISO="${TZ_OFFSET:0:3}:${TZ_OFFSET:3:2}"  # 例: +09:00
UPDATED_ISO=$(date "+%Y-%m-%dT%H:%M:%S${TZ_ISO}")
UPDATED_DISPLAY=$(date '+%Y-%m-%d %H:%M')

# ---- データ収集 ----
REVIEW_PR_LINES=""
ADMIN_ISSUE_LINES=""
SUMMARY_ROWS=""

if [[ "${GH_AVAILABLE}" == "true" ]]; then
  for REPO in ${REPOS}; do
    # レビュー依頼中のPR
    PR_JSON=$(gh pr list \
      --repo "${REPO}" \
      --search 'review-requested:@me' \
      --json number,title,createdAt \
      --limit 20 \
      2>/dev/null || echo "[]")

    PR_COUNT=$(echo "${PR_JSON}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(len(data))
" 2>/dev/null || echo "0")

    if [[ "${PR_JSON}" != "[]" && "${PR_COUNT}" -gt 0 ]]; then
      LINES=$(echo "${PR_JSON}" | python3 -c "
import sys, json
from datetime import datetime, timezone
data = json.load(sys.stdin)
repo = sys.argv[1]
for pr in data:
    num = pr['number']
    title = pr['title']
    created = pr.get('createdAt', '')
    if created:
        dt = datetime.fromisoformat(created.replace('Z', '+00:00'))
        now = datetime.now(timezone.utc)
        diff = now - dt
        hours = int(diff.total_seconds() // 3600)
        if hours < 24:
            age = f'{hours}h ago'
        else:
            age = f'{diff.days}d ago'
    else:
        age = ''
    suffix = f' ({age})' if age else ''
    print(f'- [ ] {repo}#{num} - {title}{suffix}')
" "${REPO}" 2>/dev/null || true)
      REVIEW_PR_LINES="${REVIEW_PR_LINES}${LINES}"$'\n'
    fi

    # Admin判断待ちIssue
    ISSUE_JSON=$(gh issue list \
      --repo "${REPO}" \
      --label 'admin-decision' \
      --json number,title \
      --limit 20 \
      2>/dev/null || echo "[]")

    ISSUE_LINES=$(echo "${ISSUE_JSON}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
repo = sys.argv[1]
for issue in data:
    num = issue['number']
    title = issue['title']
    print(f'- [ ] {repo}#{num} - {title}')
" "${REPO}" 2>/dev/null || true)

    if [[ -n "${ISSUE_LINES}" ]]; then
      ADMIN_ISSUE_LINES="${ADMIN_ISSUE_LINES}${ISSUE_LINES}"$'\n'
    fi

    # サマリー: Open PR数・Open Issue数
    OPEN_PR=$(gh pr list --repo "${REPO}" --state open --json number --limit 100 2>/dev/null \
      | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "?")
    OPEN_ISSUE=$(gh issue list --repo "${REPO}" --state open --json number --limit 100 2>/dev/null \
      | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "?")

    REPO_SHORT="${REPO##*/}"
    SUMMARY_ROWS="${SUMMARY_ROWS}| ${REPO_SHORT} | ${OPEN_PR} | ${OPEN_ISSUE} |"$'\n'
  done
else
  REVIEW_PR_LINES="_(gh CLI が見つかりません — データ取得をスキップしました)_"$'\n'
  ADMIN_ISSUE_LINES="_(gh CLI が見つかりません — データ取得をスキップしました)_"$'\n'
  for REPO in ${REPOS}; do
    REPO_SHORT="${REPO##*/}"
    SUMMARY_ROWS="${SUMMARY_ROWS}| ${REPO_SHORT} | ? | ? |"$'\n'
  done
fi

# ---- レンダリング ----
if [[ -z "${REVIEW_PR_LINES// /}" ]]; then
  REVIEW_PR_SECTION="なし"
else
  REVIEW_PR_SECTION="${REVIEW_PR_LINES%$'\n'}"
fi

if [[ -z "${ADMIN_ISSUE_LINES// /}" ]]; then
  ADMIN_ISSUE_SECTION="なし"
else
  ADMIN_ISSUE_SECTION="${ADMIN_ISSUE_LINES%$'\n'}"
fi

# ---- ダッシュボード.md を書き出す ----
cat > "${DASHBOARD_FILE}" <<DASHBOARD
---
updated: ${UPDATED_ISO}
---
# ダッシュボード
最終更新: ${UPDATED_DISPLAY}

## レビュー待ちPR
${REVIEW_PR_SECTION}

## Admin判断待ちIssue
${ADMIN_ISSUE_SECTION}

## リポジトリサマリー
| リポジトリ | Open PR | Open Issue |
|---|---|---|
${SUMMARY_ROWS%$'\n'}
DASHBOARD

echo "[dashboard] ダッシュボードを更新しました: ${DASHBOARD_FILE}"
