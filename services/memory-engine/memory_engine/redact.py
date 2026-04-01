"""機密情報フィルタ"""

import re

from .config import Config


def redact(text: str, config: Config | None = None) -> str:
    """APIキー・トークン等を[REDACTED]に置換"""
    config = config or Config()
    for pattern in config.redact_patterns:
        text = re.sub(pattern, "[REDACTED]", text, flags=re.IGNORECASE)
    return text
