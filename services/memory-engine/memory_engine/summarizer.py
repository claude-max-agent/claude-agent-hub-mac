"""会話サマリー生成（LLM不使用、ルールベース抽出）"""

from __future__ import annotations

import time
from dataclasses import dataclass

from .transcript_parser import Turn, extract_tools_used


@dataclass
class ConversationSummary:
    date: str
    agent_name: str
    session_id: str
    task_description: str
    outcome: str
    topics: list[str]
    tools_used: list[str]
    turn_count: int
    user_turn_count: int

    def to_text(self) -> str:
        """検索可能なサマリーテキストを生成

        session_id/agent_nameはDB構造化メタデータとして保持。
        本文ではタスク内容・日時・トピック・結果を優先表示し、
        セッション情報は末尾の低優先度メタデータに配置。
        """
        lines = [
            f"[Session Summary] {self.date}",
            f"Turns: {self.turn_count} ({self.user_turn_count} user messages)",
            "",
            f"Task: {self.task_description}",
        ]

        if self.topics:
            lines.append(f"Topics: {', '.join(self.topics)}")

        if self.tools_used:
            lines.append(f"Tools: {', '.join(self.tools_used[:10])}")

        if self.outcome:
            lines.append("")
            lines.append(f"Outcome: {self.outcome}")

        # Low-priority metadata (DB structured fields take precedence)
        meta = []
        if self.agent_name:
            meta.append(f"agent={self.agent_name}")
        if self.session_id:
            meta.append(f"session={self.session_id}")
        if meta:
            lines.append("")
            lines.append(f"Meta: {', '.join(meta)}")

        return "\n".join(lines)


def summarize_conversation(
    turns: list[Turn],
    agent_name: str = "root",
    session_id: str = "",
    timestamp: float | None = None,
) -> ConversationSummary | None:
    """会話ターンからサマリーを生成。

    抽出ルール:
    - task_description: 最初のuserメッセージ（先頭500文字）
    - outcome: 最後のassistantメッセージ（先頭500文字）
    - topics: 最初のuserメッセージからキーワード抽出
    - tools_used: 全ターンから使用ツールを集約
    """
    if not turns:
        return None

    ts = timestamp or time.time()
    date_str = time.strftime("%Y-%m-%d %H:%M", time.localtime(ts))

    # task_description: 最初のuserメッセージ
    first_user = ""
    for t in turns:
        if t.role == "user":
            first_user = t.text[:500]
            break

    # outcome: 最後のassistantメッセージ
    last_assistant = ""
    for t in reversed(turns):
        if t.role == "assistant":
            last_assistant = t.text[:500]
            break

    # turn counts
    user_count = sum(1 for t in turns if t.role == "user")

    # tools used
    tools = extract_tools_used(turns)

    # topics: 最初のuserメッセージから抽出（シンプルなキーワード）
    topics = _extract_topics(first_user)

    return ConversationSummary(
        date=date_str,
        agent_name=agent_name,
        session_id=session_id,
        task_description=first_user,
        outcome=last_assistant,
        topics=topics,
        tools_used=tools,
        turn_count=len(turns),
        user_turn_count=user_count,
    )


def _extract_topics(text: str) -> list[str]:
    """テキストからトピックキーワードを抽出（ルールベース）"""
    import re

    topics: list[str] = []

    # Issue番号
    issues = re.findall(r'#(\d+)', text)
    for num in issues[:3]:
        topics.append(f"Issue #{num}")

    # URL内のリポジトリ名
    repos = re.findall(r'github\.com/[\w-]+/([\w-]+)', text)
    for repo in repos[:2]:
        if repo not in topics:
            topics.append(repo)

    # ファイルパス
    paths = re.findall(r'[\w/]+\.(?:go|py|ts|tsx|js|jsx|sh|yaml|yml|md|json)\b', text)
    for p in paths[:3]:
        topics.append(p)

    # 日本語のキーワードパターン（〜を実装、〜を修正 等）
    ja_actions = re.findall(r'([\w\-]+(?:を|の)(?:実装|修正|追加|削除|更新|調査|確認|設定|改善))', text)
    for action in ja_actions[:3]:
        if action not in topics:
            topics.append(action)

    return topics[:8]
