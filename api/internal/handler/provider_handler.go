package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type LLMProvider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	APIKeyRef    string `json:"api_key_ref"`
	DefaultModel string `json:"default_model"`
	Status       string `json:"status"`
}

type AgentProviderConfig struct {
	AgentID         string `json:"agent_id"`
	ProviderID      string `json:"provider_id"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type ProviderHandler struct {
	mu           sync.RWMutex
	providers    []LLMProvider
	agentConfigs map[string]*AgentProviderConfig
}

func NewProviderHandler() *ProviderHandler {
	h := &ProviderHandler{
		agentConfigs: make(map[string]*AgentProviderConfig),
	}

	// Initialize with known providers
	h.providers = []LLMProvider{
		{
			ID:           "anthropic",
			Name:         "Anthropic",
			APIKeyRef:    "op://claudebot/anthropic/ANTHROPIC_API_KEY",
			DefaultModel: "claude-sonnet-4-6",
			Status:       "active",
		},
		{
			ID:           "openai",
			Name:         "OpenAI",
			APIKeyRef:    "op://claudebot/openai/OPENAI_API_KEY",
			DefaultModel: "gpt-5.3-codex",
			Status:       "active",
		},
	}

	return h
}

func (h *ProviderHandler) HandleListProviders(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.providers)
}

func (h *ProviderHandler) HandleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var prov LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&prov); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if prov.ID == "" || prov.Name == "" {
		http.Error(w, `{"error":"id and name are required"}`, http.StatusBadRequest)
		return
	}
	if prov.Status == "" {
		prov.Status = "active"
	}

	h.mu.Lock()
	// Check for duplicate
	for _, p := range h.providers {
		if p.ID == prov.ID {
			h.mu.Unlock()
			http.Error(w, `{"error":"provider already exists"}`, http.StatusConflict)
			return
		}
	}
	h.providers = append(h.providers, prov)
	h.mu.Unlock()

	log.Printf("Provider created: %s (%s)", prov.ID, prov.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prov)
}

func (h *ProviderHandler) HandleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	provID := chi.URLParam(r, "providerID")

	var update LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for i, p := range h.providers {
		if p.ID == provID {
			if update.Name != "" {
				h.providers[i].Name = update.Name
			}
			if update.APIKeyRef != "" {
				h.providers[i].APIKeyRef = update.APIKeyRef
			}
			if update.DefaultModel != "" {
				h.providers[i].DefaultModel = update.DefaultModel
			}
			if update.Status != "" {
				h.providers[i].Status = update.Status
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(h.providers[i])
			return
		}
	}

	http.Error(w, `{"error":"provider not found"}`, http.StatusNotFound)
}

func (h *ProviderHandler) HandleListAgentConfigs(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := make([]AgentProviderConfig, 0, len(h.agentConfigs))
	for _, c := range h.agentConfigs {
		configs = append(configs, *c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (h *ProviderHandler) HandleUpdateAgentConfig(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")

	var update struct {
		ProviderID      string `json:"provider_id"`
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	h.agentConfigs[agentID] = &AgentProviderConfig{
		AgentID:         agentID,
		ProviderID:      update.ProviderID,
		Model:           update.Model,
		ReasoningEffort: update.ReasoningEffort,
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	h.mu.Unlock()

	log.Printf("Agent config updated: %s -> provider=%s, model=%s", agentID, update.ProviderID, update.Model)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.agentConfigs[agentID])
}
