"""DBのテスト"""

import tempfile
import time
from pathlib import Path

import pytest

from memory_engine.config import Config
from memory_engine.db import MemoryDB


@pytest.fixture
def db():
    with tempfile.TemporaryDirectory() as tmp:
        config = Config()
        config.db_path = Path(tmp) / "test.db"
        d = MemoryDB(config)
        yield d
        d.close()


def test_insert_and_stats(db):
    db.insert(
        session_id="s1",
        role="pair",
        content="テストコンテンツ",
        token_count=10,
        created_at=time.time(),
    )
    stats = db.stats()
    assert stats["total_memories"] == 1
    assert stats["total_tokens"] == 10
    assert stats["total_sessions"] == 1


def test_fts_search(db):
    db.insert("s1", "pair", "MCPサーバーの設定方法", 10, time.time())
    db.insert("s2", "pair", "Reactコンポーネントの書き方", 10, time.time())

    results = db.fts_search("MCP")
    assert len(results) >= 1
    # MCP が含まれるドキュメントがヒット
    ids = [r[0] for r in results]
    assert 1 in ids


def test_vec_search(db):
    # ダミーベクトル（768次元）
    vec1 = [1.0] * 768
    vec2 = [0.0] * 768
    vec2[0] = 1.0

    db.insert("s1", "pair", "text1", 5, time.time(), embedding=vec1)
    db.insert("s2", "pair", "text2", 5, time.time(), embedding=vec2)

    results = db.vec_search(vec1, limit=2)
    assert len(results) == 2
    # vec1に最も近いのはvec1自身
    assert results[0][0] == 1


def test_get_by_ids(db):
    db.insert("s1", "pair", "content1", 5, time.time())
    db.insert("s2", "pair", "content2", 5, time.time())

    results = db.get_by_ids([1, 2])
    assert len(results) == 2
    contents = {r["content"] for r in results}
    assert "content1" in contents
    assert "content2" in contents


def test_get_by_ids_empty(db):
    assert db.get_by_ids([]) == []


def test_multiple_agents(db):
    db.insert("s1", "pair", "root content", 5, time.time(), agent_name="root")
    db.insert("s2", "pair", "manager content", 5, time.time(), agent_name="manager")

    stats = db.stats()
    assert stats["total_agents"] == 2
