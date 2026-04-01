"""Claude Code JSONL transcript → 会話テキスト変換"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Turn:
    role: str  # 'user' | 'assistant'
    text: str
    timestamp: str = ""
    tools_used: list[str] | None = None


def parse_transcript(path: str | Path) -> list[Turn]:
    """JSONL transcriptファイルを読み込み、user/assistantターンのリストを返す。

    Claude Code transcriptのJSONL形式:
    - type: "user" | "assistant" | "system" | "file-history-snapshot"
    - message.role: "user" | "assistant"
    - message.content: str (user) | list[{type, text, ...}] (assistant)
    """
    path = Path(path)
    if not path.exists():
        return []

    turns: list[Turn] = []

    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue

            entry_type = entry.get("type", "")
            if entry_type not in ("user", "assistant"):
                continue

            msg = entry.get("message", {})
            if not msg:
                continue

            role = msg.get("role", entry_type)
            content = msg.get("content", "")
            timestamp = entry.get("timestamp", "")

            text = ""
            tools: list[str] = []

            if isinstance(content, str):
                text = content.strip()
            elif isinstance(content, list):
                text_parts = []
                for block in content:
                    if not isinstance(block, dict):
                        continue
                    block_type = block.get("type", "")
                    if block_type == "text":
                        text_parts.append(block.get("text", ""))
                    elif block_type == "tool_use":
                        name = block.get("name", "")
                        if name:
                            tools.append(name)
                    # thinking, tool_result はスキップ
                text = "\n".join(t for t in text_parts if t).strip()

            if not text:
                continue

            turns.append(Turn(
                role=role,
                text=text,
                timestamp=str(timestamp),
                tools_used=tools if tools else None,
            ))

    return turns


def turns_to_conversation_text(turns: list[Turn]) -> str:
    """TurnリストをHuman/Assistant形式の会話テキストに変換。
    chunker.pyが期待するフォーマットに合わせる。
    """
    parts = []
    for turn in turns:
        label = "Human" if turn.role == "user" else "Assistant"
        parts.append(f"{label}: {turn.text}")
    return "\n\n".join(parts)


def extract_tools_used(turns: list[Turn]) -> list[str]:
    """全ターンから使用されたツール名を重複除去で抽出"""
    seen: set[str] = set()
    tools: list[str] = []
    for turn in turns:
        if turn.tools_used:
            for t in turn.tools_used:
                if t not in seen:
                    seen.add(t)
                    tools.append(t)
    return tools
