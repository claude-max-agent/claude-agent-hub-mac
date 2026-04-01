package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/zono819/claude-agent-hub/api/internal/database"
)

// RevenueHandler handles revenue, KPI, activity, and targets API endpoints
type RevenueHandler struct {
	repo *database.RevenueRepository
}

// NewRevenueHandler creates a new RevenueHandler
func NewRevenueHandler(repo *database.RevenueRepository) *RevenueHandler {
	return &RevenueHandler{repo: repo}
}

// --- Revenue ---

// HandleGetRevenue returns revenue data (monthly or daily)
func (h *RevenueHandler) HandleGetRevenue(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	w.Header().Set("Content-Type", "application/json")

	if period == "monthly" {
		monthly, err := h.repo.GetMonthlyRevenue()
		if err != nil {
			log.Printf("Error getting monthly revenue: %v", err)
			http.Error(w, `{"error":"failed to get revenue"}`, http.StatusInternalServerError)
			return
		}
		if monthly == nil {
			monthly = []database.MonthlyRevenue{}
		}
		json.NewEncoder(w).Encode(monthly)
		return
	}

	entries, err := h.repo.ListRevenue(period)
	if err != nil {
		log.Printf("Error listing revenue: %v", err)
		http.Error(w, `{"error":"failed to list revenue"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []database.RevenueEntry{}
	}
	json.NewEncoder(w).Encode(entries)
}

// HandleCreateRevenue creates a new revenue entry
func (h *RevenueHandler) HandleCreateRevenue(w http.ResponseWriter, r *http.Request) {
	var entry database.RevenueEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if entry.Date == "" || entry.Source == "" {
		http.Error(w, `{"error":"date and source are required"}`, http.StatusBadRequest)
		return
	}
	if entry.Currency == "" {
		entry.Currency = "JPY"
	}

	if err := h.repo.CreateRevenue(&entry); err != nil {
		log.Printf("Error creating revenue: %v", err)
		http.Error(w, `{"error":"failed to create revenue"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// --- KPI ---

// HandleGetKpi returns latest KPI snapshots
func (h *RevenueHandler) HandleGetKpi(w http.ResponseWriter, r *http.Request) {
	snapshots, err := h.repo.GetLatestKpi()
	if err != nil {
		log.Printf("Error getting KPI: %v", err)
		http.Error(w, `{"error":"failed to get KPI"}`, http.StatusInternalServerError)
		return
	}
	if snapshots == nil {
		snapshots = []database.KpiSnapshot{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshots)
}

// HandleCreateKpi creates a new KPI snapshot
func (h *RevenueHandler) HandleCreateKpi(w http.ResponseWriter, r *http.Request) {
	var snapshot database.KpiSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if snapshot.Metric == "" {
		http.Error(w, `{"error":"metric is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.CreateKpi(&snapshot); err != nil {
		log.Printf("Error creating KPI: %v", err)
		http.Error(w, `{"error":"failed to create KPI"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(snapshot)
}

// --- Activity ---

// HandleGetActivity returns recent activity log entries
func (h *RevenueHandler) HandleGetActivity(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	entries, err := h.repo.ListActivity(limit)
	if err != nil {
		log.Printf("Error getting activity: %v", err)
		http.Error(w, `{"error":"failed to get activity"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []database.ActivityEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleCreateActivity creates a new activity log entry
func (h *RevenueHandler) HandleCreateActivity(w http.ResponseWriter, r *http.Request) {
	var entry database.ActivityEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if entry.Agent == "" || entry.Action == "" {
		http.Error(w, `{"error":"agent and action are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.CreateActivity(&entry); err != nil {
		log.Printf("Error creating activity: %v", err)
		http.Error(w, `{"error":"failed to create activity"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// --- Targets ---

// HandleGetTargets returns targets filtered by optional month
func (h *RevenueHandler) HandleGetTargets(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")

	targets, err := h.repo.ListTargets(month)
	if err != nil {
		log.Printf("Error getting targets: %v", err)
		http.Error(w, `{"error":"failed to get targets"}`, http.StatusInternalServerError)
		return
	}
	if targets == nil {
		targets = []database.Target{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(targets)
}

// HandleCreateTarget creates or updates a target
func (h *RevenueHandler) HandleCreateTarget(w http.ResponseWriter, r *http.Request) {
	var target database.Target
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if target.Month == "" || target.Source == "" {
		http.Error(w, `{"error":"month and source are required"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.UpsertTarget(&target); err != nil {
		log.Printf("Error creating target: %v", err)
		http.Error(w, `{"error":"failed to create target"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(target)
}
