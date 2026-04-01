"""SQLite + FTS5 + sqlite-vec データベース"""

from __future__ import annotations

import sqlite3
import struct
from pathlib import Path

import sqlite_vec

from .config import Config


def _serialize_f32(vec: list[float]) -> bytes:
    """float listをsqlite-vec用バイナリに変換"""
    return struct.pack(f"{len(vec)}f", *vec)


def _deserialize_f32(blob: bytes, dim: int) -> list[float]:
    """sqlite-vecバイナリをfloat listに変換"""
    return list(struct.unpack(f"{dim}f", blob))


class MemoryDB:
    def __init__(self, config: Config | None = None):
        self.config = config or Config()
        self.config.db_path.parent.mkdir(parents=True, exist_ok=True)
        self._conn = sqlite3.connect(str(self.config.db_path))
        self._conn.enable_load_extension(True)
        sqlite_vec.load(self._conn)
        self._conn.enable_load_extension(False)
        self._init_schema()

    def _init_schema(self):
        cur = self._conn.cursor()
        cur.executescript(f"""
            CREATE TABLE IF NOT EXISTS memories (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                session_id TEXT NOT NULL,
                agent_name TEXT DEFAULT 'root',
                role TEXT NOT NULL,
                content TEXT NOT NULL,
                token_count INTEGER DEFAULT 0,
                created_at REAL NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_memories_session
                ON memories(session_id);
            CREATE INDEX IF NOT EXISTS idx_memories_agent
                ON memories(agent_name);
            CREATE INDEX IF NOT EXISTS idx_memories_created
                ON memories(created_at);

            CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
                content,
                content_rowid='id',
                tokenize='trigram'
            );
        """)

        # sqlite-vec テーブル（別途作成、IF NOT EXISTS未対応の場合がある）
        try:
            cur.execute(f"""
                CREATE VIRTUAL TABLE memories_vec USING vec0(
                    id INTEGER PRIMARY KEY,
                    embedding float[{self.config.embed_dim}]
                )
            """)
        except sqlite3.OperationalError:
            pass  # already exists

        self._conn.commit()

    def insert(
        self,
        session_id: str,
        role: str,
        content: str,
        token_count: int,
        created_at: float,
        embedding: list[float] | None = None,
        agent_name: str = "root",
    ) -> int:
        """メモリチャンクを挿入"""
        cur = self._conn.cursor()
        cur.execute(
            """INSERT INTO memories
               (session_id, agent_name, role, content, token_count, created_at)
               VALUES (?, ?, ?, ?, ?, ?)""",
            (session_id, agent_name, role, content, token_count, created_at),
        )
        row_id = cur.lastrowid

        # FTS5同期
        cur.execute(
            "INSERT INTO memories_fts(rowid, content) VALUES (?, ?)",
            (row_id, content),
        )

        # ベクトル挿入
        if embedding:
            cur.execute(
                "INSERT INTO memories_vec(id, embedding) VALUES (?, ?)",
                (row_id, _serialize_f32(embedding)),
            )

        self._conn.commit()
        return row_id

    def fts_search(self, query: str, limit: int = 20) -> list[tuple[int, float]]:
        """FTS5 trigram検索。(id, bm25_score)のリストを返す"""
        cur = self._conn.cursor()
        # trigram tokenizeではBM25が使えないため、matchのrank疑似スコアを使用
        cur.execute(
            """SELECT rowid, rank
               FROM memories_fts
               WHERE memories_fts MATCH ?
               ORDER BY rank
               LIMIT ?""",
            (f'"{query}"', limit),
        )
        return cur.fetchall()

    def vec_search(
        self, embedding: list[float], limit: int = 20
    ) -> list[tuple[int, float]]:
        """ベクトル近傍検索。(id, distance)のリストを返す"""
        cur = self._conn.cursor()
        cur.execute(
            """SELECT id, distance
               FROM memories_vec
               WHERE embedding MATCH ?
               ORDER BY distance
               LIMIT ?""",
            (_serialize_f32(embedding), limit),
        )
        return cur.fetchall()

    def get_by_ids(self, ids: list[int]) -> list[dict]:
        """IDリストからメモリを取得"""
        if not ids:
            return []
        placeholders = ",".join("?" * len(ids))
        cur = self._conn.cursor()
        cur.execute(
            f"""SELECT id, session_id, agent_name, role, content,
                       token_count, created_at
                FROM memories WHERE id IN ({placeholders})""",
            ids,
        )
        cols = ["id", "session_id", "agent_name", "role", "content",
                "token_count", "created_at"]
        return [dict(zip(cols, row)) for row in cur.fetchall()]

    def stats(self) -> dict:
        """統計情報"""
        cur = self._conn.cursor()
        cur.execute("SELECT COUNT(*), COALESCE(SUM(token_count), 0) FROM memories")
        count, tokens = cur.fetchone()
        cur.execute("SELECT COUNT(DISTINCT session_id) FROM memories")
        sessions = cur.fetchone()[0]
        cur.execute("SELECT COUNT(DISTINCT agent_name) FROM memories")
        agents = cur.fetchone()[0]

        db_size = self.config.db_path.stat().st_size if self.config.db_path.exists() else 0

        return {
            "total_memories": count,
            "total_tokens": tokens,
            "total_sessions": sessions,
            "total_agents": agents,
            "db_size_mb": round(db_size / 1024 / 1024, 2),
        }

    def close(self):
        self._conn.close()
