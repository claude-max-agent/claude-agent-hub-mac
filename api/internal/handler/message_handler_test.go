package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sendCall struct {
	session string
	pane    int
	message string
}

func TestHandleSend_Success(t *testing.T) {
	var calls []sendCall
	mockSend := func(session string, pane int, message string) error {
		calls = append(calls, sendCall{session, pane, message})
		return nil
	}
	mockHasSession := func(session string) bool {
		return true
	}
	mockWriteToFile := func(from, to, message string) (string, error) {
		return "", nil
	}

	h := NewMessageHandler(mockSend, mockHasSession, mockWriteToFile)

	body := `{"to":"codex","from":"tmp-agent-xyz","message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSend(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "sent" {
		t.Errorf("expected status=sent, got %v", resp["status"])
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(calls))
	}
	if calls[0].session != "codex" {
		t.Errorf("expected session=codex, got %s", calls[0].session)
	}
	if calls[0].pane != 0 {
		t.Errorf("expected pane=0, got %d", calls[0].pane)
	}
}

func TestHandleSend_MissingFields(t *testing.T) {
	h := NewMessageHandler(nil, nil, nil)

	tests := []struct {
		name string
		body string
	}{
		{"missing to", `{"from":"a","message":"b"}`},
		{"missing from", `{"to":"a","message":"b"}`},
		{"missing message", `{"to":"a","from":"b"}`},
		{"empty message", `{"to":"a","from":"b","message":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			h.HandleSend(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleSend_InvalidSessionName(t *testing.T) {
	h := NewMessageHandler(nil, nil, nil)

	body := `{"to":"codex; rm -rf /","from":"agent","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSend_InvalidFromName(t *testing.T) {
	h := NewMessageHandler(nil, nil, nil)

	body := `{"to":"codex","from":"agent; rm -rf /","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSend_SessionNotFound(t *testing.T) {
	mockSend := func(session string, pane int, message string) error { return nil }
	mockHasSession := func(session string) bool { return false }
	mockWriteToFile := func(from, to, message string) (string, error) { return "", nil }

	h := NewMessageHandler(mockSend, mockHasSession, mockWriteToFile)

	body := `{"to":"nonexistent","from":"agent","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleSend_SendError(t *testing.T) {
	mockSend := func(session string, pane int, message string) error {
		return fmt.Errorf("tmux send failed")
	}
	mockHasSession := func(session string) bool { return true }
	mockWriteToFile := func(from, to, message string) (string, error) { return "", nil }

	h := NewMessageHandler(mockSend, mockHasSession, mockWriteToFile)

	body := `{"to":"codex","from":"agent","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleSend_MalformedJSON(t *testing.T) {
	h := NewMessageHandler(nil, nil, nil)

	body := `{"to":"codex","from":"agent","message":}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleSend_WriteToFileError(t *testing.T) {
	mockSend := func(session string, pane int, message string) error { return nil }
	mockHasSession := func(session string) bool { return true }
	mockWriteToFile := func(from, to, message string) (string, error) {
		return "", fmt.Errorf("disk full")
	}

	h := NewMessageHandler(mockSend, mockHasSession, mockWriteToFile)

	longMsg := strings.Repeat("a", 17000)
	body := fmt.Sprintf(`{"to":"codex","from":"agent","message":"%s"}`, longMsg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleSend_LongMessage(t *testing.T) {
	var calls []sendCall
	mockSend := func(session string, pane int, message string) error {
		calls = append(calls, sendCall{session, pane, message})
		return nil
	}
	mockHasSession := func(session string) bool { return true }
	mockWriteToFile := func(from, to, message string) (string, error) {
		return "/tmp/claude-hub-messages/msg-test.txt", nil
	}

	h := NewMessageHandler(mockSend, mockHasSession, mockWriteToFile)

	longMsg := strings.Repeat("a", 17000)
	body := fmt.Sprintf(`{"to":"codex","from":"agent","message":"%s"}`, longMsg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.HandleSend(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(calls))
	}
	if strings.Contains(calls[0].message, longMsg) {
		t.Error("expected long message to be written to file, not sent directly")
	}
	if !strings.Contains(calls[0].message, "/tmp/claude-hub-messages/") {
		t.Error("expected file path in sent message")
	}
}
