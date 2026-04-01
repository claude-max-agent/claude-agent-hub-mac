"""MCP Server — memory_search, memory_save, memory_stats"""

from __future__ import annotations

import time

from mcp.server import FastMCP

from .chunker import chunk_conversation
from .config import Config
from .db import MemoryDB
from .embedder import Embedder
from .redact import redact
from .search import MemorySearcher

config = Config()
db = MemoryDB(config)
embedder = Embedder(config)
searcher = MemorySearcher(db, embedder, config)

mcp = FastMCP(
    "memory-engine",
    instructions="Claude Code長期記憶エンジン。セッション間の会話文脈を保持・検索する。",
)


@mcp.tool()
def memory_search(
    query: str,
    top_k: int = 5,
    agent_name: str | None = None,
) -> str:
    """過去のセッション記憶を検索する。

    Args:
        query: 検索クエリ（自然言語）
        top_k: 返却件数（デフォルト5）
        agent_name: エージェント名でフィルタ（省略時は全エージェント）
    """
    results = searcher.search(query, top_k=top_k, agent_name=agent_name)
    if not results:
        return "関連する記憶が見つかりませんでした。"

    parts = []
    for i, m in enumerate(results, 1):
        ts = time.strftime("%Y-%m-%d %H:%M", time.localtime(m["created_at"]))
        parts.append(
            f"--- Memory #{i} (score: {m['score']}, session: {m['session_id']}, "
            f"agent: {m['agent_name']}, date: {ts}) ---\n{m['content']}"
        )
    return "\n\n".join(parts)


@mcp.tool()
def memory_save(
    text: str,
    agent_name: str = "root",
    session_id: str = "manual",
) -> str:
    """テキストを記憶として保存する。

    Args:
        text: 保存するテキスト（会話形式 or 自由テキスト）
        agent_name: エージェント名（デフォルト: root）
        session_id: セッションID（デフォルト: manual）
    """
    # 機密情報フィルタ
    text = redact(text, config)

    chunks = chunk_conversation(
        text,
        agent_name=agent_name,
        session_id=session_id,
        max_tokens=config.max_chunk_tokens,
    )

    saved = 0
    now = time.time()
    for chunk in chunks:
        try:
            embedding = embedder.embed(chunk.content)
        except Exception:
            embedding = None

        db.insert(
            session_id=chunk.session_id,
            role=chunk.role,
            content=chunk.content,
            token_count=chunk.token_estimate,
            created_at=now,
            embedding=embedding,
            agent_name=chunk.agent_name,
        )
        saved += 1

    return f"{saved}件のチャンクを保存しました。(session: {session_id}, agent: {agent_name})"


@mcp.tool()
def memory_stats() -> str:
    """記憶エンジンの統計情報を表示する。"""
    s = db.stats()
    return (
        f"総メモリ数: {s['total_memories']}\n"
        f"総トークン数: {s['total_tokens']}\n"
        f"セッション数: {s['total_sessions']}\n"
        f"エージェント数: {s['total_agents']}\n"
        f"DBサイズ: {s['db_size_mb']} MB"
    )
