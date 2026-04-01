# Claude Agent Hub (Mac)

MacBook向け Claude Code マルチエージェントシステム。
[claude-agent-hub](https://github.com/claude-max-agent/claude-agent-hub) (Linux版) の軽量版。

## 主な違い（Linux版との比較）

| 項目 | Linux版 | Mac版 |
|------|---------|-------|
| タスク指示 | Discord/Slack | ターミナル + Admin UI |
| 外部連携 | Discord Bot, Slack Bot | なし |
| LLMバックエンド | Ollama | MLX (Apple Silicon最適化) |
| 常駐化 | systemd | launchd (オプション) |
| エージェント規模 | Lead + 4-6体 | Lead + 2-3体 |
| Obsidian | なし | あり |

## クイックスタート

```bash
# 1. クローン
git clone https://github.com/claude-max-agent/claude-agent-hub-mac.git
cd claude-agent-hub-mac

# 2. セットアップ
bash setup.sh

# 3. 起動
bash scripts/start.sh

# 4. Managerに接続
tmux attach -t manager
# 抜けるとき: Ctrl+b → d
```

## ステータス確認

```bash
bash scripts/status.sh
```

## 停止

```bash
bash scripts/stop.sh
```

## Admin UI

起動後、ブラウザで http://localhost:18080 にアクセス。

## memory-engine セットアップ

```bash
# venvと依存のセットアップ
bash scripts/setup-memory-engine.sh

# MLXサーバー起動 (オプション)
bash scripts/start-mlx.sh
```

## tmp-agent 管理

```bash
# 起動
./scripts/instance-manager.sh start --template dev my-task

# 一覧
./scripts/instance-manager.sh list

# 停止
./scripts/instance-manager.sh stop my-task
```

## launchd 常駐化 (オプション)

```bash
# 登録
cp launchd/com.claude-agent-hub.manager.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/com.claude-agent-hub.manager.plist

# 手動起動
launchctl start com.claude-agent-hub.manager

# 登録解除
launchctl unload ~/Library/LaunchAgents/com.claude-agent-hub.manager.plist
```

## ディレクトリ構成

```
claude-agent-hub-mac/
├── CLAUDE.md                    # Manager指示書
├── setup.sh                     # 初回セットアップ
├── .claude/                     # hooks, rules, state
├── api/                         # Go APIサーバー
├── frontend/                    # React Admin UI
├── agents/templates/            # エージェントテンプレート
├── services/memory-engine/      # RAG検索エンジン
├── config/                      # limits, services, projects
├── scripts/                     # 起動・管理スクリプト
├── data/                        # SQLite DB
├── logs/                        # ログ
└── launchd/                     # macOS常駐化
```

## 要件

- macOS (Apple Silicon)
- Claude Code CLI
- tmux, jq, git, gh, sqlite3
- Go 1.25+ (API Server)
- Node.js 22+ (Frontend)
- Python 3.10+ (memory-engine)

## 会社PCへの移植

```bash
git clone https://github.com/claude-max-agent/claude-agent-hub-mac.git
cd claude-agent-hub-mac
bash setup.sh
# config/limits.yaml をRAMに応じて調整
```
