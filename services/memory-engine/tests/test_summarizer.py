"""summarizer テスト"""

from memory_engine.summarizer import (
    ConversationSummary,
    _extract_topics,
    summarize_conversation,
)
from memory_engine.transcript_parser import Turn


def test_summarize_basic():
    turns = [
        Turn(role="user", text="Fix the login bug in auth.go, see Issue #123"),
        Turn(role="assistant", text="Looking at auth.go...", tools_used=["Read"]),
        Turn(role="user", text="Looks good"),
        Turn(role="assistant", text="The login bug has been fixed and tests pass."),
    ]
    summary = summarize_conversation(
        turns, agent_name="pool-1", session_id="test-001", timestamp=1743465600.0,
    )
    assert summary is not None
    assert summary.agent_name == "pool-1"
    assert summary.session_id == "test-001"
    assert summary.turn_count == 4
    assert summary.user_turn_count == 2
    assert "login bug" in summary.task_description
    assert "fixed" in summary.outcome
    assert "Read" in summary.tools_used

    text = summary.to_text()
    assert "[Session Summary]" in text
    assert "Task:" in text
    assert "Outcome:" in text
    # session_id/agent_name are low-priority metadata at the end
    assert "Meta: agent=pool-1, session=test-001" in text


def test_summarize_empty():
    result = summarize_conversation([])
    assert result is None


def test_summarize_single_user_turn():
    turns = [Turn(role="user", text="What is the status?")]
    summary = summarize_conversation(turns, agent_name="manager")
    assert summary is not None
    assert summary.task_description == "What is the status?"
    assert summary.outcome == ""
    assert summary.turn_count == 1


def test_extract_topics_issue():
    topics = _extract_topics("Fix #123 and #456 in the repo")
    assert "Issue #123" in topics
    assert "Issue #456" in topics


def test_extract_topics_repo():
    topics = _extract_topics("See https://github.com/claude-max-agent/claude-agent-hub/issues/100")
    assert "claude-agent-hub" in topics


def test_extract_topics_files():
    topics = _extract_topics("Edit services/memory-engine/server.py and config.yaml")
    assert any("server.py" in t for t in topics)
    assert any("config.yaml" in t for t in topics)


def test_extract_topics_japanese():
    topics = _extract_topics("SessionEnd hookを改善してmemory-saveを修正する")
    ja_topics = [t for t in topics if "を" in t]
    assert len(ja_topics) >= 1


def test_summary_to_text_format():
    summary = ConversationSummary(
        date="2026-04-01 12:00",
        agent_name="pool-1",
        session_id="sess-001",
        task_description="Implement feature X",
        outcome="Feature X implemented successfully",
        topics=["Issue #100", "feature.go"],
        tools_used=["Read", "Edit", "Bash"],
        turn_count=10,
        user_turn_count=3,
    )
    text = summary.to_text()
    assert "2026-04-01 12:00" in text
    assert "Implement feature X" in text
    assert "Feature X implemented successfully" in text
    assert "Issue #100" in text
    assert "Read" in text
    # session_id/agent_name are in Meta line, not at the top
    assert "Meta: agent=pool-1, session=sess-001" in text
    # Verify they don't appear as top-level "Agent:" / "Session:" lines
    assert "Agent: pool-1" not in text
    assert "Session: sess-001" not in text
