package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TradingSchedule represents a scheduled trade configuration
type TradingSchedule struct {
	ID          string     `json:"id"`
	Coin        string     `json:"coin"`
	Side        string     `json:"side"` // buy or sell
	OrderType   string     `json:"order_type"` // market, limit
	Price       *float64   `json:"price,omitempty"`
	Amount      float64    `json:"amount"`
	Leverage    int        `json:"leverage"`
	Cron        string     `json:"cron"`
	Enabled     bool       `json:"enabled"`
	LastRunAt   *time.Time `json:"last_run_at"`
	NextRunAt   *time.Time `json:"next_run_at"`
	RunCount    int        `json:"run_count"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TradingScheduleHandler handles CRUD for trading schedules (JSON file-based)
type TradingScheduleHandler struct {
	mu       sync.RWMutex
	filePath string
}

// NewTradingScheduleHandler creates a new handler with the given state file path
func NewTradingScheduleHandler(stateDir string) *TradingScheduleHandler {
	return &TradingScheduleHandler{
		filePath: filepath.Join(stateDir, "trading-schedules.json"),
	}
}

func (h *TradingScheduleHandler) load() ([]TradingSchedule, error) {
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []TradingSchedule{}, nil
		}
		return nil, err
	}
	var schedules []TradingSchedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		return nil, err
	}
	return schedules, nil
}

func (h *TradingScheduleHandler) save(schedules []TradingSchedule) error {
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.filePath, data, 0644)
}

// HandleList returns all trading schedules
func (h *TradingScheduleHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	schedules, err := h.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"schedules": schedules,
	})
}

type tradingScheduleCreateRequest struct {
	Coin        string   `json:"coin"`
	Side        string   `json:"side"`
	OrderType   string   `json:"order_type"`
	Price       *float64 `json:"price"`
	Amount      float64  `json:"amount"`
	Leverage    int      `json:"leverage"`
	Cron        string   `json:"cron"`
	Enabled     bool     `json:"enabled"`
	Description string   `json:"description"`
}

// HandleCreate creates a new trading schedule
func (h *TradingScheduleHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req tradingScheduleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Coin == "" || req.Side == "" || req.Cron == "" || req.Amount <= 0 {
		http.Error(w, "coin, side, cron, and amount (>0) are required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	schedule := TradingSchedule{
		ID:          uuid.New().String()[:8],
		Coin:        req.Coin,
		Side:        req.Side,
		OrderType:   req.OrderType,
		Price:       req.Price,
		Amount:      req.Amount,
		Leverage:    req.Leverage,
		Cron:        req.Cron,
		Enabled:     req.Enabled,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if schedule.OrderType == "" {
		schedule.OrderType = "market"
	}
	if schedule.Leverage == 0 {
		schedule.Leverage = 1
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	schedules, err := h.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	schedules = append(schedules, schedule)
	if err := h.save(schedules); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(schedule)
}

type tradingScheduleUpdateRequest struct {
	Coin        *string  `json:"coin"`
	Side        *string  `json:"side"`
	OrderType   *string  `json:"order_type"`
	Price       *float64 `json:"price"`
	Amount      *float64 `json:"amount"`
	Leverage    *int     `json:"leverage"`
	Cron        *string  `json:"cron"`
	Enabled     *bool    `json:"enabled"`
	Description *string  `json:"description"`
}

// HandleUpdate partially updates an existing trading schedule
func (h *TradingScheduleHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")

	var req tradingScheduleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	schedules, err := h.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idx := -1
	for i, s := range schedules {
		if s.ID == scheduleID {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	s := &schedules[idx]
	if req.Coin != nil {
		s.Coin = *req.Coin
	}
	if req.Side != nil {
		s.Side = *req.Side
	}
	if req.OrderType != nil {
		s.OrderType = *req.OrderType
	}
	if req.Price != nil {
		s.Price = req.Price
	}
	if req.Amount != nil {
		s.Amount = *req.Amount
	}
	if req.Leverage != nil {
		s.Leverage = *req.Leverage
	}
	if req.Cron != nil {
		s.Cron = *req.Cron
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if req.Description != nil {
		s.Description = *req.Description
	}
	s.UpdatedAt = time.Now().UTC()

	if err := h.save(schedules); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// HandleDelete removes a trading schedule
func (h *TradingScheduleHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	scheduleID := chi.URLParam(r, "scheduleID")

	h.mu.Lock()
	defer h.mu.Unlock()

	schedules, err := h.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	found := false
	filtered := make([]TradingSchedule, 0, len(schedules))
	for _, s := range schedules {
		if s.ID == scheduleID {
			found = true
			continue
		}
		filtered = append(filtered, s)
	}

	if !found {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	if err := h.save(filtered); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"id":     scheduleID,
	})
}
