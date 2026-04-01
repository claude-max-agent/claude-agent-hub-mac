package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/zono819/claude-agent-hub/api/internal/config"
	"github.com/zono819/claude-agent-hub/api/internal/service"
)

// StrategyHandler handles strategy management API endpoints
type StrategyHandler struct {
	configDir       string
	opService       *service.OnePasswordService
	mu              sync.RWMutex
	cfg             *config.StrategiesConfig
}

// NewStrategyHandler creates a new StrategyHandler
func NewStrategyHandler(configDir string) *StrategyHandler {
	saToken := os.Getenv("OP_SA_TOKEN_BOT_PARAMS")
	h := &StrategyHandler{
		configDir: configDir,
		opService: service.NewOnePasswordService(saToken),
	}
	if err := h.loadConfig(); err != nil {
		log.Printf("Warning: Failed to load strategies.yaml: %v", err)
		h.cfg = &config.StrategiesConfig{
			Strategies: make(map[string]config.Strategy),
		}
	}
	return h
}

func (h *StrategyHandler) loadConfig() error {
	cfg, err := config.LoadStrategiesConfig(filepath.Join(h.configDir, "strategies.yaml"))
	if err != nil {
		return err
	}
	h.cfg = cfg
	return nil
}

func (h *StrategyHandler) saveConfig() error {
	return config.SaveStrategiesConfig(filepath.Join(h.configDir, "strategies.yaml"), h.cfg)
}

// StrategyListItem represents a strategy in the list response
type StrategyListItem struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	App         string                `json:"app"`
	Status      string                `json:"status"`
	Active      bool                  `json:"active"`
	Exchange    string                `json:"exchange,omitempty"`
	Pairs       []string              `json:"pairs,omitempty"`
	ParamCount  int                   `json:"param_count"`
	Params      []config.StrategyParam `json:"params"`
}

// normalizeStatus returns a valid status string, defaulting to "active" for legacy bool values
func normalizeStatus(s config.Strategy) string {
	status := s.Status
	if status == "" {
		return "active"
	}
	switch status {
	case "active", "inactive", "archived":
		return status
	default:
		return "active"
	}
}

// HandleList returns all strategies, optionally filtered by ?status=active|inactive|archived
func (h *StrategyHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	statusFilter := r.URL.Query().Get("status")

	items := make([]StrategyListItem, 0, len(h.cfg.Strategies))
	for id, s := range h.cfg.Strategies {
		status := normalizeStatus(s)
		if statusFilter != "" && status != statusFilter {
			continue
		}
		items = append(items, StrategyListItem{
			ID:          id,
			Name:        s.Name,
			Description: s.Description,
			App:         s.App,
			Status:      status,
			Active:      status == "active",
			Exchange:    s.Exchange,
			Pairs:       s.Pairs,
			ParamCount:  len(s.Params),
			Params:      s.Params,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"strategies": items,
		"count":      len(items),
	})
}

// HandleGet returns a single strategy with its parameter definitions
func (h *StrategyHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "strategyID")

	h.mu.RLock()
	s, exists := h.cfg.Strategies[id]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"strategy %q not found"}`, id), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       id,
		"strategy": s,
	})
}

// HandleToggle toggles a strategy's active state (legacy, calls HandleSetStatus internally)
func (h *StrategyHandler) HandleToggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "strategyID")

	h.mu.Lock()
	defer h.mu.Unlock()

	s, exists := h.cfg.Strategies[id]
	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"strategy %q not found"}`, id), http.StatusNotFound)
		return
	}

	oldStatus := normalizeStatus(s)
	if oldStatus == "active" {
		s.Status = "inactive"
	} else {
		s.Status = "active"
	}
	h.cfg.Strategies[id] = s

	if err := h.saveConfig(); err != nil {
		log.Printf("Failed to save strategies.yaml: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  s.Status,
		"active":  s.Status == "active",
	})
}

// HandleSetStatus sets a strategy's status (active/inactive/archived)
func (h *StrategyHandler) HandleSetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "strategyID")

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	switch req.Status {
	case "active", "inactive", "archived":
		// valid
	default:
		http.Error(w, `{"error":"status must be active, inactive, or archived"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	s, exists := h.cfg.Strategies[id]
	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"strategy %q not found"}`, id), http.StatusNotFound)
		return
	}

	s.Status = req.Status
	h.cfg.Strategies[id] = s

	if err := h.saveConfig(); err != nil {
		log.Printf("Failed to save strategies.yaml: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  s.Status,
	})
}

// HandleGetParams reads strategy parameters from 1Password
func (h *StrategyHandler) HandleGetParams(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "strategyID")

	h.mu.RLock()
	s, exists := h.cfg.Strategies[id]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"strategy %q not found"}`, id), http.StatusNotFound)
		return
	}

	fields, err := h.opService.GetItemFields(s.OpVault, s.OpItem)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("failed to read 1Password: %v", err),
		})
		return
	}

	// Build params with current values, filtered by defined params
	type ParamWithValue struct {
		config.StrategyParam
		Value string `json:"value"`
	}
	params := make([]ParamWithValue, 0, len(s.Params))
	for _, p := range s.Params {
		params = append(params, ParamWithValue{
			StrategyParam: p,
			Value:         fields[p.Key],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     id,
		"params": params,
	})
}

// HandleUpdateParams writes strategy parameters to 1Password
func (h *StrategyHandler) HandleUpdateParams(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "strategyID")

	h.mu.RLock()
	s, exists := h.cfg.Strategies[id]
	h.mu.RUnlock()

	if !exists {
		http.Error(w, fmt.Sprintf(`{"error":"strategy %q not found"}`, id), http.StatusNotFound)
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate: only allow updating defined params
	validKeys := make(map[string]bool)
	for _, p := range s.Params {
		validKeys[p.Key] = true
	}

	// Validate all params first
	validFields := make(map[string]string)
	var errors []string
	for key, value := range req {
		if !validKeys[key] {
			errors = append(errors, fmt.Sprintf("unknown param: %s", key))
			continue
		}
		validFields[key] = value
	}

	// Batch update all valid fields in a single op command
	var updated []string
	if len(validFields) > 0 {
		if err := h.opService.SetItemFields(s.OpVault, s.OpItem, validFields); err != nil {
			errors = append(errors, fmt.Sprintf("batch update failed: %v", err))
		} else {
			for key := range validFields {
				updated = append(updated, key)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if len(errors) > 0 && len(updated) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"errors":  errors,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"updated": updated,
		"errors":  errors,
	})
}
