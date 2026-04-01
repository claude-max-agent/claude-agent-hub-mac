"""埋め込みラッパー（Ollama / MLX対応）

MLXバックエンドの場合:
  - mlx_lm.serverがOpenAI互換APIを提供するが、embeddings APIはない
  - FTS（全文検索）のみで動作し、ベクトル検索は無効化
  - Ollamaが利用可能になれば自動的にベクトル検索も有効化

Ollamaバックエンドの場合:
  - nomic-embed-text でベクトル埋め込みを生成
  - FTS + ベクトルのハイブリッド検索
"""

import logging

import httpx

from .config import Config

log = logging.getLogger(__name__)


class Embedder:
    def __init__(self, config: Config | None = None):
        self.config = config or Config()
        self._client = httpx.Client(timeout=30.0)
        self._available: bool | None = None

    @property
    def is_available(self) -> bool:
        """埋め込みサーバーが利用可能かチェック"""
        if self._available is not None:
            return self._available

        if self.config.llm_backend == "mlx":
            # MLXバックエンドではembeddings APIがないためFTSのみ
            log.info("MLX backend: embedding disabled, using FTS-only search")
            self._available = False
            return False

        try:
            resp = self._client.get(
                f"{self.config.ollama_url}/api/tags",
                timeout=5.0,
            )
            self._available = resp.status_code == 200
        except Exception:
            log.warning("Ollama not reachable, embedding disabled")
            self._available = False

        return self._available

    def embed(self, text: str) -> list[float]:
        """単一テキストの埋め込みベクトルを取得"""
        if not self.is_available:
            return []

        resp = self._client.post(
            f"{self.config.ollama_url}/api/embeddings",
            json={"model": self.config.embed_model, "prompt": text},
        )
        resp.raise_for_status()
        return resp.json()["embedding"]

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """複数テキストの埋め込みを取得（逐次実行）"""
        if not self.is_available:
            return [[] for _ in texts]
        return [self.embed(t) for t in texts]

    def close(self):
        self._client.close()
