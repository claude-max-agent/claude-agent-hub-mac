"""chunkerのテスト"""

from memory_engine.chunker import chunk_conversation, estimate_tokens


def test_estimate_tokens_ascii():
    assert estimate_tokens("hello world") == 2  # 11 ascii / 4 = 2


def test_estimate_tokens_japanese():
    assert estimate_tokens("こんにちは") == 5  # 5 non-ascii chars


def test_estimate_tokens_mixed():
    text = "Hello こんにちは"
    tokens = estimate_tokens(text)
    # "Hello " = 6 ascii -> 1, "こんにちは" = 5 non-ascii -> 5, total = 6
    assert tokens == 6


def test_chunk_simple_conversation():
    text = "Human: こんにちは\n\nAssistant: はい、何かお手伝いしましょうか？"
    chunks = chunk_conversation(text, session_id="test1")
    assert len(chunks) == 1
    assert chunks[0].role == "pair"
    assert chunks[0].session_id == "test1"
    assert "こんにちは" in chunks[0].content
    assert "お手伝い" in chunks[0].content


def test_chunk_multi_turn():
    text = (
        "Human: Q1\n\nAssistant: A1\n\n"
        "Human: Q2\n\nAssistant: A2"
    )
    chunks = chunk_conversation(text, session_id="test2")
    assert len(chunks) == 2
    assert "Q1" in chunks[0].content
    assert "A1" in chunks[0].content
    assert "Q2" in chunks[1].content
    assert "A2" in chunks[1].content


def test_chunk_no_turns():
    text = "Just some random text without any turn markers."
    chunks = chunk_conversation(text, session_id="test3")
    assert len(chunks) >= 1
    assert chunks[0].role == "pair"


def test_chunk_long_text_split():
    # 2000トークン超のチャンクは分割される
    long_text = "Human: " + "あ" * 3000 + "\n\nAssistant: OK"
    chunks = chunk_conversation(long_text, max_tokens=2000, session_id="test4")
    assert len(chunks) >= 2


def test_chunk_agent_name():
    text = "Human: test\n\nAssistant: response"
    chunks = chunk_conversation(text, agent_name="manager", session_id="test5")
    assert all(c.agent_name == "manager" for c in chunks)


def test_chunk_user_prefix():
    text = "User: test\n\nAssistant: response"
    chunks = chunk_conversation(text, session_id="test6")
    assert len(chunks) == 1
    assert chunks[0].role == "pair"
