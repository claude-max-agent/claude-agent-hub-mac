package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zono819/claude-agent-hub/api/internal/service"
)

// UpgradeHandler handles system upgrade operations
type UpgradeHandler struct {
	projectDir string
}

// NewUpgradeHandler creates a new UpgradeHandler
func NewUpgradeHandler(projectDir string) *UpgradeHandler {
	return &UpgradeHandler{projectDir: projectDir}
}

// HandleUpgrade handles POST /api/v1/upgrade and POST /api/v1/system/upgrade.
func (h *UpgradeHandler) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	req := service.UpgradeRequest{}

	if r.ContentLength > 0 || strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, service.UpgradeResponse{
				Status: "error",
				Error:  fmt.Sprintf("invalid request body: %v", err),
			})
			return
		}
	}

	if req.Layer == "" {
		req.Layer = r.URL.Query().Get("layer")
	}
	if !req.DryRun {
		req.DryRun = r.URL.Query().Get("dry_run") == "true"
	}
	if !req.Force {
		req.Force = r.URL.Query().Get("force") == "true"
	}
	if !req.Reload {
		req.Reload = r.URL.Query().Get("reload") == "true"
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, service.UpgradeResponse{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	result, err := service.RunSelectiveUpgrade(h.projectDir, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, result)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
