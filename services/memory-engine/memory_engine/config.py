"""設定"""

from dataclasses import dataclass, field
from pathlib import Path
import os


@dataclass
class Config:
    db_path: Path = field(default_factory=lambda: Path(
        os.environ.get("MEMORY_ENGINE_DB", str(Path.home() / ".claude" / "memory.db"))
    ))
    # LLMバックエンド: "ollama" or "mlx"
    llm_backend: str = field(default_factory=lambda: os.environ.get(
        "MEMORY_ENGINE_BACKEND", "mlx"
    ))
    ollama_url: str = field(default_factory=lambda: os.environ.get(
        "OLLAMA_URL", "http://localhost:11434"
    ))
    # MLX設定
    mlx_model: str = field(default_factory=lambda: os.environ.get(
        "MLX_MODEL", "mlx-community/Qwen2.5-7B-Instruct-4bit"
    ))
    embed_model: str = "nomic-embed-text"
    embed_dim: int = 768

    # 検索パラメータ
    fts_weight: float = 0.7
    vec_weight: float = 0.3
    rrf_k: int = 60
    decay_half_life_days: float = 30.0
    default_top_k: int = 5
    max_chunk_tokens: int = 2000

    # 機密情報フィルタパターン
    redact_patterns: list[str] = field(default_factory=lambda: [
        r'(?:sk|pk|api[_-]?key|token|secret|password|bearer)\s*[:=]\s*["\']?[\w\-\.]{20,}',
        r'(?:ghp|gho|ghs|ghr)_[A-Za-z0-9_]{36,}',
        r'xox[bpsar]-[\w\-]{10,}',
        r'op://[\w/\-]+',
    ])
