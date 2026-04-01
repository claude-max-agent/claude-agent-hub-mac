"""ハイブリッド検索 + RRF + 時間減衰"""

from __future__ import annotations

import math
import time

from .config import Config
from .db import MemoryDB
from .embedder import Embedder


def _rrf_merge(
    fts_results: list[tuple[int, float]],
    vec_results: list[tuple[int, float]],
    fts_weight: float,
    vec_weight: float,
    k: int,
) -> dict[int, float]:
    """Reciprocal Rank Fusion でスコア統合"""
    scores: dict[int, float] = {}

    for rank, (doc_id, _) in enumerate(fts_results):
        scores[doc_id] = scores.get(doc_id, 0) + fts_weight / (k + rank + 1)

    for rank, (doc_id, _) in enumerate(vec_results):
        scores[doc_id] = scores.get(doc_id, 0) + vec_weight / (k + rank + 1)

    return scores


def _apply_time_decay(
    scores: dict[int, float],
    memories: list[dict],
    half_life_days: float,
    now: float | None = None,
) -> dict[int, float]:
    """時間減衰を適用"""
    now = now or time.time()
    ts_map = {m["id"]: m["created_at"] for m in memories}

    for doc_id in scores:
        if doc_id in ts_map:
            days = (now - ts_map[doc_id]) / 86400
            decay = math.pow(0.5, days / half_life_days)
            scores[doc_id] *= decay

    return scores


class MemorySearcher:
    def __init__(
        self,
        db: MemoryDB,
        embedder: Embedder,
        config: Config | None = None,
    ):
        self.db = db
        self.embedder = embedder
        self.config = config or Config()

    def search(
        self,
        query: str,
        top_k: int | None = None,
        agent_name: str | None = None,
    ) -> list[dict]:
        """ハイブリッド検索を実行"""
        top_k = top_k or self.config.default_top_k
        fetch_limit = top_k * 4  # 候補を多めに取得

        # FTS5 検索（キーワード主軸）
        fts_results = self.db.fts_search(query, limit=fetch_limit)

        # ベクトル検索（補助）
        try:
            query_embedding = self.embedder.embed(query)
            vec_results = self.db.vec_search(query_embedding, limit=fetch_limit)
        except Exception:
            # Ollama未起動時はFTSのみで検索
            vec_results = []

        if not fts_results and not vec_results:
            return []

        # RRF スコア統合
        scores = _rrf_merge(
            fts_results,
            vec_results,
            self.config.fts_weight,
            self.config.vec_weight,
            self.config.rrf_k,
        )

        # メモリ取得
        all_ids = list(scores.keys())
        memories = self.db.get_by_ids(all_ids)

        # agent_nameフィルタ
        if agent_name:
            memories = [m for m in memories if m["agent_name"] == agent_name]
            scores = {m["id"]: scores[m["id"]] for m in memories}

        # 時間減衰
        scores = _apply_time_decay(
            scores, memories, self.config.decay_half_life_days
        )

        # スコア順でソート
        sorted_ids = sorted(scores, key=lambda x: scores[x], reverse=True)[:top_k]
        id_set = set(sorted_ids)
        result = [m for m in memories if m["id"] in id_set]
        result.sort(key=lambda m: scores[m["id"]], reverse=True)

        # スコアを付与
        for m in result:
            m["score"] = round(scores[m["id"]], 6)

        return result
