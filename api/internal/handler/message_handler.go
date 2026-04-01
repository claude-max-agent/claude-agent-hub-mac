package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"
)

const maxDirectMessageLen = 16000

var validSessionName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type MessageHandler struct {
	sendFunc    func(session string, pane int, message string) error
	hasSession  func(session string) bool
	writeToFile func(from, to, message string) (string, error)
}

func NewMessageHandler(
	sendFunc func(string, int, string) error,
	hasSession func(string) bool,
	writeToFile func(string, string, string) (string, error),
) *MessageHandler {
	return &MessageHandler{
		sendFunc:    sendFunc,
		hasSession:  hasSession,
		writeToFile: writeToFile,
	}
}

type MessageSendRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Message string `json:"message"`
}

type MessageSendResponse struct {
	Status    string `json:"status"`
	To        string `json:"to"`
	Timestamp string `json:"timestamp"`
}

type MessageErrorResponse struct {
	Error string `json:"error"`
	To    string `json:"to,omitempty"`
}

func (h *MessageHandler) HandleSend(w http.ResponseWriter, r *http.Request) {
	var req MessageSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMessageError(w, http.StatusBadRequest, "invalid request body", "")
		return
	}

	if req.To == "" {
		writeMessageError(w, http.StatusBadRequest, "missing required field: to", "")
		return
	}
	if req.From == "" {
		writeMessageError(w, http.StatusBadRequest, "missing required field: from", "")
		return
	}
	if req.Message == "" {
		writeMessageError(w, http.StatusBadRequest, "missing required field: message", "")
		return
	}

	if !validSessionName.MatchString(req.To) {
		writeMessageError(w, http.StatusBadRequest, "invalid session name: must be alphanumeric with hyphens/underscores", req.To)
		return
	}
	if !validSessionName.MatchString(req.From) {
		writeMessageError(w, http.StatusBadRequest, "invalid sender name: must be alphanumeric with hyphens/underscores", "")
		return
	}

	if !h.hasSession(req.To) {
		writeMessageError(w, http.StatusNotFound, "session not found", req.To)
		return
	}

	msgPreview := req.Message
	if len(msgPreview) > 100 {
		msgPreview = msgPreview[:100] + "..."
	}
	log.Printf("[MessageHandler] Sending message from=%s to=%s preview=%q", req.From, req.To, msgPreview)

	actualMessage := req.Message
	if len(req.Message) > maxDirectMessageLen {
		filePath, err := h.writeToFile(req.From, req.To, req.Message)
		if err != nil {
			log.Printf("[MessageHandler] Failed to write message to file: %v", err)
			writeMessageError(w, http.StatusInternalServerError, "failed to send message", req.To)
			return
		}
		actualMessage = fmt.Sprintf("[Message from %s] メッセージが長いためファイルに保存しました。以下を読んで対応してください: %s", req.From, filePath)
	}

	if err := h.sendFunc(req.To, 0, actualMessage); err != nil {
		log.Printf("[MessageHandler] Failed to send message to=%s: %v", req.To, err)
		writeMessageError(w, http.StatusInternalServerError, "failed to send message", req.To)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MessageSendResponse{
		Status:    "sent",
		To:        req.To,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func writeMessageError(w http.ResponseWriter, status int, msg string, to string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(MessageErrorResponse{Error: msg, To: to})
}
