"""Q&Aチャンク分割（ルールベース、LLM不使用）"""

from __future__ import annotations

import re
import uuid
from dataclasses import dataclass, field


@dataclass
class Chunk:
    content: str
    role: str  # 'human' | 'assistant' | 'pair'
    session_id: str
    agent_name: str = "root"
    token_estimate: int = 0


def estimate_tokens(text: str) -> int:
    """簡易トークン推定（英語4文字≒1トークン、日本語1文字≒1トークン）"""
    ascii_chars = sum(1 for c in text if ord(c) < 128)
    non_ascii = len(text) - ascii_chars
    return ascii_chars // 4 + non_ascii


def _split_long_chunk(text: str, max_tokens: int) -> list[str]:
    """長いチャンクを段落境界で分割"""
    paragraphs = re.split(r'\n\n+', text)
    result: list[str] = []
    current = ""

    for para in paragraphs:
        candidate = (current + "\n\n" + para).strip() if current else para
        if estimate_tokens(candidate) > max_tokens and current:
            result.append(current)
            current = para
        else:
            current = candidate

    if current:
        result.append(current)
    return result


def chunk_conversation(
    text: str,
    agent_name: str = "root",
    session_id: str | None = None,
    max_tokens: int = 2000,
) -> list[Chunk]:
    """会話テキストをQ&Aペアのチャンクに分割

    フォーマット:
    - "Human: ...\n\nAssistant: ..." (標準)
    - "User: ...\n\nAssistant: ..." (代替)
    - JSONL形式 {"role": "human", "content": "..."} も対応
    """
    sid = session_id or uuid.uuid4().hex[:12]
    chunks: list[Chunk] = []

    # Human/Assistant ターン分割
    turns = re.split(
        r'\n*(?=(?:Human|User|Assistant|System):\s)',
        text,
        flags=re.IGNORECASE,
    )
    turns = [t.strip() for t in turns if t.strip()]

    if len(turns) < 2:
        # ターン分割できない場合は全体を1チャンクとして扱う
        for part in _split_long_chunk(text, max_tokens):
            chunks.append(Chunk(
                content=part,
                role="pair",
                session_id=sid,
                agent_name=agent_name,
                token_estimate=estimate_tokens(part),
            ))
        return chunks

    # Human+Assistantペアを作成
    i = 0
    while i < len(turns):
        human_turn = turns[i]
        is_human = re.match(r'(?:Human|User):\s', human_turn, re.IGNORECASE)

        if is_human and i + 1 < len(turns):
            assistant_turn = turns[i + 1]
            pair_text = f"{human_turn}\n\n{assistant_turn}"
            i += 2
        else:
            pair_text = human_turn
            i += 1

        role = "pair" if is_human else "assistant"

        for part in _split_long_chunk(pair_text, max_tokens):
            chunks.append(Chunk(
                content=part,
                role=role,
                session_id=sid,
                agent_name=agent_name,
                token_estimate=estimate_tokens(part),
            ))

    return chunks
