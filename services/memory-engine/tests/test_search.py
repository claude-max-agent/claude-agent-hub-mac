"""検索のテスト（Ollamaモック使用）"""

import math
import tempfile
import time
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from memory_engine.config import Config
from memory_engine.db import MemoryDB
from memory_engine.search import MemorySearcher, _rrf_merge, _apply_time_decay


@pytest.fixture
def config():
    return Config()


def test_rrf_merge():
    fts = [(1, -1.0), (2, -0.5), (3, -0.3)]
    vec = [(2, 0.1), (3, 0.2), (4, 0.3)]

    scores = _rrf_merge(fts, vec, fts_weight=0.7, vec_weight=0.3, k=60)

    # doc 2 should have highest score (appears in both)
    assert scores[2] > scores[1]
    assert scores[2] > scores[4]
    # doc 1 only in fts, doc 4 only in vec
    assert 1 in scores
    assert 4 in scores


def test_rrf_empty():
    scores = _rrf_merge([], [], 0.7, 0.3, 60)
    assert scores == {}


def test_time_decay():
    now = time.time()
    memories = [
        {"id": 1, "created_at": now},  # 今
        {"id": 2, "created_at": now - 30 * 86400},  # 30日前
        {"id": 3, "created_at": now - 60 * 86400},  # 60日前
    ]
    scores = {1: 1.0, 2: 1.0, 3: 1.0}

    decayed = _apply_time_decay(scores, memories, half_life_days=30.0, now=now)

    assert decayed[1] == pytest.approx(1.0, abs=0.01)
    assert decayed[2] == pytest.approx(0.5, abs=0.01)
    assert decayed[3] == pytest.approx(0.25, abs=0.01)


def test_search_with_mock_embedder():
    with tempfile.TemporaryDirectory() as tmp:
        config = Config()
        config.db_path = Path(tmp) / "test.db"
        db = MemoryDB(config)

        # モックembedder
        embedder = MagicMock()
        # 768次元のダミーベクトル
        embedder.embed.return_value = [0.1] * 768

        # テストデータ挿入
        now = time.time()
        db.insert("s1", "pair", "MCPサーバー設定", 10, now, [0.1] * 768)
        db.insert("s2", "pair", "React開発入門", 10, now, [0.9] * 768)

        searcher = MemorySearcher(db, embedder, config)
        results = searcher.search("MCP", top_k=2)

        assert len(results) >= 1
        # FTS5がMCPにマッチするので、MCPサーバー設定が上位に来る
        assert "MCP" in results[0]["content"]

        db.close()


def test_search_agent_filter():
    with tempfile.TemporaryDirectory() as tmp:
        config = Config()
        config.db_path = Path(tmp) / "test.db"
        db = MemoryDB(config)

        embedder = MagicMock()
        embedder.embed.return_value = [0.1] * 768

        now = time.time()
        db.insert("s1", "pair", "root memory data", 10, now, [0.1] * 768, "root")
        db.insert("s2", "pair", "manager memory data", 10, now, [0.1] * 768, "manager")

        searcher = MemorySearcher(db, embedder, config)

        # managerのみ
        results = searcher.search("memory", top_k=5, agent_name="manager")
        assert all(r["agent_name"] == "manager" for r in results)

        db.close()


def test_search_no_results():
    with tempfile.TemporaryDirectory() as tmp:
        config = Config()
        config.db_path = Path(tmp) / "test.db"
        db = MemoryDB(config)

        embedder = MagicMock()
        embedder.embed.side_effect = Exception("Ollama not running")

        searcher = MemorySearcher(db, embedder, config)
        results = searcher.search("anything")
        assert results == []

        db.close()
