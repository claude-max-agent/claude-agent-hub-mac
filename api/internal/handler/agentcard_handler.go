package handler

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

//go:embed agent.json
var embeddedAgentCard []byte

// AgentCardHandler handles A2A Protocol Agent Card requests
type AgentCardHandler struct {
	once  sync.Once
	cache []byte
	err   error
}

// NewAgentCardHandler creates a new AgentCardHandler
func NewAgentCardHandler() *AgentCardHandler {
	return &AgentCardHandler{}
}

func (h *AgentCardHandler) init() {
	var card map[string]interface{}
	if err := json.Unmarshal(embeddedAgentCard, &card); err != nil {
		h.err = err
		return
	}

	if agentURL := os.Getenv("AGENT_CARD_URL"); agentURL != "" {
		card["url"] = agentURL
	}

	data, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		h.err = err
		return
	}
	h.cache = data
}

// HandleGetAgentCard handles GET /.well-known/agent.json
func (h *AgentCardHandler) HandleGetAgentCard(w http.ResponseWriter, r *http.Request) {
	h.once.Do(h.init)

	if h.err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to load agent card",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(h.cache)
}
