package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zono819/claude-agent-hub/api/internal/service"
)

// RAGHandler handles RAG query API requests
type RAGHandler struct {
	client *service.RAGClient
}

// RAGQueryResponse represents the API response for RAG queries
type RAGQueryResponse struct {
	Query   string   `json:"query"`
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
	Model   string   `json:"model"`
}

// NewRAGHandler creates a new RAG handler
func NewRAGHandler(client *service.RAGClient) *RAGHandler {
	return &RAGHandler{client: client}
}

// HandleQuery handles GET /api/v1/rag/query?q=<question>
func (h *RAGHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"query parameter 'q' is required"}`, http.StatusBadRequest)
		return
	}

	result, err := h.client.Query(query)
	if err != nil {
		errMsg := err.Error()
		if strings.HasPrefix(errMsg, "ollama_unavailable:") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Ollama service is not available",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "RAG query failed: " + errMsg,
		})
		return
	}

	resp := RAGQueryResponse{
		Query:   query,
		Answer:  result.Answer,
		Sources: result.Sources,
		Model:   result.Model,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
