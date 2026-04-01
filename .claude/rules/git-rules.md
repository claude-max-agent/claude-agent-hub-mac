# Git Rules

## ブランチ戦略

- **Feature branches only** — main への直接コミット禁止
- ブランチ命名: `feat/`, `fix/`, `refactor/`, `docs/` + 概要
- 例: `feat/add-obsidian-search`, `fix/memory-leak-agent`

## コミット規約

- 日本語で記述
- フォーマット: `<type>: <summary>`
- type: feat, fix, refactor, docs, test, chore
- 例: `feat: Obsidian検索機能を追加`

## PR ワークフロー

- PRはAdmin承認が必要
- PRタイトルは簡潔に（70文字以内）
- PR本文にやったこと・テスト方法を記載

## 禁止事項

- force push 禁止
- secrets のコミット禁止
- main ブランチの直接変更禁止
