"""CLIエントリーポイント — MCP起動 + save/search/save-transcriptコマンド"""

from __future__ import annotations

import argparse
import sys
import time

from .chunker import chunk_conversation
from .config import Config
from .db import MemoryDB
from .embedder import Embedder
from .redact import redact
from .search import MemorySearcher
from .summarizer import summarize_conversation
from .transcript_parser import parse_transcript, turns_to_conversation_text


def cmd_serve(args):
    """MCPサーバー起動"""
    from .server import mcp
    mcp.run(transport="stdio")


def cmd_save(args):
    """stdinからテキストを読み取り保存"""
    config = Config()
    db = MemoryDB(config)
    embedder = Embedder(config)

    text = sys.stdin.read()
    if not text.strip():
        print("空の入力です。", file=sys.stderr)
        sys.exit(1)

    text = redact(text, config)
    chunks = chunk_conversation(
        text,
        agent_name=args.agent,
        session_id=args.session,
        max_tokens=config.max_chunk_tokens,
    )

    now = time.time()
    saved = 0
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

    print(f"{saved}件保存 (session={chunk.session_id}, agent={args.agent})")
    db.close()


def cmd_search(args):
    """検索"""
    config = Config()
    db = MemoryDB(config)
    embedder = Embedder(config)
    searcher = MemorySearcher(db, embedder, config)

    results = searcher.search(args.query, top_k=args.top_k, agent_name=args.agent)
    if not results:
        print("関連する記憶が見つかりませんでした。")
        return

    for i, m in enumerate(results, 1):
        ts = time.strftime("%Y-%m-%d %H:%M", time.localtime(m["created_at"]))
        print(f"--- #{i} (score: {m['score']}, session: {m['session_id']}, "
              f"agent: {m['agent_name']}, date: {ts}) ---")
        print(m["content"])
        print()

    db.close()


def cmd_save_transcript(args):
    """JSONL transcriptファイルから会話を読み取り、チャンク+サマリーを保存"""
    config = Config()
    db = MemoryDB(config)
    embedder = Embedder(config)

    transcript_path = args.transcript
    if not transcript_path:
        print("--transcript が必要です。", file=sys.stderr)
        sys.exit(1)

    turns = parse_transcript(transcript_path)
    if not turns:
        print(f"会話ターンが見つかりません: {transcript_path}", file=sys.stderr)
        sys.exit(0)

    agent = args.agent
    session = args.session or f"transcript-{int(time.time())}"
    now = time.time()

    # 1. 会話テキストをチャンク分割して保存
    conv_text = turns_to_conversation_text(turns)
    conv_text = redact(conv_text, config)
    chunks = chunk_conversation(
        conv_text,
        agent_name=agent,
        session_id=session,
        max_tokens=config.max_chunk_tokens,
    )

    saved = 0
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

    # 2. サマリーを生成して保存
    summary = summarize_conversation(
        turns,
        agent_name=agent,
        session_id=session,
        timestamp=now,
    )
    if summary:
        summary_text = summary.to_text()
        summary_text = redact(summary_text, config)
        try:
            summary_embedding = embedder.embed(summary_text)
        except Exception:
            summary_embedding = None

        from .chunker import estimate_tokens
        db.insert(
            session_id=session,
            role="summary",
            content=summary_text,
            token_count=estimate_tokens(summary_text),
            created_at=now,
            embedding=summary_embedding,
            agent_name=agent,
        )
        saved += 1

    print(f"{saved}件保存 (session={session}, agent={agent}, turns={len(turns)})")
    db.close()


def cmd_stats(args):
    """統計情報"""
    config = Config()
    db = MemoryDB(config)
    s = db.stats()
    print(f"総メモリ数: {s['total_memories']}")
    print(f"総トークン数: {s['total_tokens']}")
    print(f"セッション数: {s['total_sessions']}")
    print(f"エージェント数: {s['total_agents']}")
    print(f"DBサイズ: {s['db_size_mb']} MB")
    db.close()


def main():
    parser = argparse.ArgumentParser(description="Claude Code長期記憶エンジン")
    sub = parser.add_subparsers(dest="command")

    # serve
    sub.add_parser("serve", help="MCPサーバー起動")

    # save
    sp_save = sub.add_parser("save", help="stdinからテキストを保存")
    sp_save.add_argument("--agent", default="root", help="エージェント名")
    sp_save.add_argument("--session", default=None, help="セッションID")

    # save-transcript
    sp_transcript = sub.add_parser("save-transcript", help="JSONL transcriptから保存")
    sp_transcript.add_argument("--transcript", required=True, help="JSONL transcriptファイルパス")
    sp_transcript.add_argument("--agent", default="root", help="エージェント名")
    sp_transcript.add_argument("--session", default=None, help="セッションID")

    # search
    sp_search = sub.add_parser("search", help="記憶を検索")
    sp_search.add_argument("query", help="検索クエリ")
    sp_search.add_argument("--top-k", type=int, default=5)
    sp_search.add_argument("--agent", default=None, help="エージェント名フィルタ")

    # stats
    sub.add_parser("stats", help="統計情報")

    args = parser.parse_args()
    if args.command == "serve":
        cmd_serve(args)
    elif args.command == "save":
        cmd_save(args)
    elif args.command == "save-transcript":
        cmd_save_transcript(args)
    elif args.command == "search":
        cmd_search(args)
    elif args.command == "stats":
        cmd_stats(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
