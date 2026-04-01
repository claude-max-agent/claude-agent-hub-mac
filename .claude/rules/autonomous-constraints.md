# 自律スコープ制約

すべてのエージェント（Manager, tmp agent）はこの制約に従う。
具体的な数値は `config/limits.yaml` を参照。

## 制約テーブル

| 制約 | 設定キー | 説明 |
|------|---------|------|
| 最大ファイル変更数 | `autonomy.max_files_per_task` | 1タスクで変更できるファイル数上限 |
| spawn深度 | `autonomy.max_spawn_depth` | エージェントのネスト深度上限 |
| コスト予算 | `autonomy.max_cost_per_task_usd` | 1タスクあたりのLLMコスト上限 |
| メモリ下限 | `autonomy.memory_spawn_block_mb` | これ未満でspawn拒否 |
| イテレーション時間 | `autonomy.max_iteration_minutes` | 1イテレーションの最大実行時間 |

## 判断基準

制約を超える場合は、Adminに確認を取ること。
自律的に制約を緩和してはならない。
