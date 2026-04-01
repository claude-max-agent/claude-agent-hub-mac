"""機密情報フィルタのテスト"""

from memory_engine.redact import redact


def test_redact_api_key():
    text = 'api_key = "sk-1234567890abcdefghijklmnop"'
    assert "[REDACTED]" in redact(text)


def test_redact_github_token():
    text = "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
    assert "[REDACTED]" in redact(text)


def test_redact_slack_token():
    text = "SLACK_TOKEN=xoxb-1234567890-abcdefghij"
    assert "[REDACTED]" in redact(text)


def test_redact_op_reference():
    text = "password from op://claudebot/postgres/password"
    assert "[REDACTED]" in redact(text)


def test_redact_preserves_normal_text():
    text = "これは通常のテキストです。MCPサーバーを設定します。"
    assert redact(text) == text


def test_redact_bearer_token():
    text = 'bearer: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.something.very.long.token"'
    assert "[REDACTED]" in redact(text)
