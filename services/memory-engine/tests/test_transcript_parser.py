"""transcript_parser テスト"""

import json
import tempfile
from pathlib import Path

from memory_engine.transcript_parser import (
    Turn,
    extract_tools_used,
    parse_transcript,
    turns_to_conversation_text,
)


def _write_jsonl(entries: list[dict]) -> Path:
    f = tempfile.NamedTemporaryFile(mode="w", suffix=".jsonl", delete=False)
    for entry in entries:
        f.write(json.dumps(entry, ensure_ascii=False) + "\n")
    f.close()
    return Path(f.name)


def test_parse_basic_conversation():
    entries = [
        {
            "type": "system",
            "message": {"role": "system", "content": "system prompt"},
            "timestamp": "2026-04-01T00:00:00Z",
        },
        {
            "type": "user",
            "message": {"role": "user", "content": "Hello, fix the bug in auth.go"},
            "timestamp": "2026-04-01T00:01:00Z",
        },
        {
            "type": "assistant",
            "message": {
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "Let me look at auth.go"},
                    {"type": "text", "text": "I'll fix the authentication bug."},
                    {"type": "tool_use", "name": "Read", "id": "tool_1", "input": {}},
                ],
            },
            "timestamp": "2026-04-01T00:02:00Z",
        },
        {
            "type": "assistant",
            "message": {
                "role": "assistant",
                "content": [
                    {"type": "text", "text": "The bug has been fixed. Auth now works correctly."},
                ],
            },
            "timestamp": "2026-04-01T00:03:00Z",
        },
    ]
    path = _write_jsonl(entries)
    turns = parse_transcript(path)

    assert len(turns) == 3
    assert turns[0].role == "user"
    assert "fix the bug" in turns[0].text
    assert turns[1].role == "assistant"
    assert "I'll fix" in turns[1].text
    assert turns[1].tools_used == ["Read"]
    assert turns[2].role == "assistant"
    assert "bug has been fixed" in turns[2].text

    path.unlink()


def test_parse_empty_file():
    path = _write_jsonl([])
    turns = parse_transcript(path)
    assert turns == []
    path.unlink()


def test_parse_nonexistent_file():
    turns = parse_transcript("/nonexistent/file.jsonl")
    assert turns == []


def test_parse_skips_system_messages():
    entries = [
        {"type": "system", "message": {"role": "system", "content": "sys"}},
        {"type": "file-history-snapshot", "message": {}},
        {"type": "user", "message": {"role": "user", "content": "hello"}},
    ]
    path = _write_jsonl(entries)
    turns = parse_transcript(path)
    assert len(turns) == 1
    assert turns[0].role == "user"
    path.unlink()


def test_parse_skips_empty_content():
    entries = [
        {"type": "user", "message": {"role": "user", "content": ""}},
        {"type": "assistant", "message": {"role": "assistant", "content": []}},
        {"type": "user", "message": {"role": "user", "content": "real message"}},
    ]
    path = _write_jsonl(entries)
    turns = parse_transcript(path)
    assert len(turns) == 1
    assert turns[0].text == "real message"
    path.unlink()


def test_turns_to_conversation_text():
    turns = [
        Turn(role="user", text="What is 2+2?"),
        Turn(role="assistant", text="2+2 = 4"),
    ]
    text = turns_to_conversation_text(turns)
    assert "Human: What is 2+2?" in text
    assert "Assistant: 2+2 = 4" in text


def test_extract_tools_used():
    turns = [
        Turn(role="assistant", text="...", tools_used=["Read", "Edit"]),
        Turn(role="assistant", text="...", tools_used=["Read", "Bash"]),
        Turn(role="user", text="ok"),
    ]
    tools = extract_tools_used(turns)
    assert tools == ["Read", "Edit", "Bash"]


def test_parse_assistant_thinking_only():
    """thinking のみの assistant メッセージはスキップされる"""
    entries = [
        {
            "type": "assistant",
            "message": {
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "internal thought"},
                ],
            },
        },
    ]
    path = _write_jsonl(entries)
    turns = parse_transcript(path)
    assert len(turns) == 0
    path.unlink()
