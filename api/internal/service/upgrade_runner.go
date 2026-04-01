package service

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// UpgradeRequest represents a selective upgrade execution request.
type UpgradeRequest struct {
	Layer  string `json:"layer"`
	Force  bool   `json:"force"`
	Reload bool   `json:"reload"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// UpgradeResponse represents the parsed JSON result from the upgrade script.
type UpgradeResponse struct {
	Status        string   `json:"status"`
	PreSHA        string   `json:"pre_sha,omitempty"`
	TargetSHA     string   `json:"target_sha,omitempty"`
	Commit        string   `json:"commit,omitempty"`
	LayersChanged []string `json:"layers_changed"`
	LayersUpdated []string `json:"layers_updated,omitempty"`
	StepsOK       []string `json:"steps_ok,omitempty"`
	StepsFail     []string `json:"steps_fail,omitempty"`
	Layer         string   `json:"layer,omitempty"`
	Error         string   `json:"error,omitempty"`
	Output        string   `json:"output,omitempty"`
}

// Validate normalizes and validates an upgrade request.
func (r *UpgradeRequest) Validate() error {
	if r.Layer == "" {
		r.Layer = "all"
	}

	validLayers := map[string]bool{"tmux": true, "ui": true, "api": true, "all": true}
	if !validLayers[r.Layer] {
		return fmt.Errorf("invalid layer: %s (valid: tmux, ui, api, all)", r.Layer)
	}

	if r.Reload && r.Layer != "tmux" {
		return fmt.Errorf("reload is only supported with layer=tmux")
	}

	return nil
}

// RunSelectiveUpgrade executes the selective upgrade script and parses its JSON result.
func RunSelectiveUpgrade(projectDir string, req UpgradeRequest) (UpgradeResponse, error) {
	if err := req.Validate(); err != nil {
		return UpgradeResponse{Status: "error", Error: err.Error()}, err
	}

	scriptPath := filepath.Join(projectDir, "scripts", "claude-upgrade-selective.sh")
	args := []string{scriptPath, "--layer", req.Layer}
	if req.Force {
		args = append(args, "--force")
	}
	if req.Reload {
		args = append(args, "--reload")
	}
	if req.DryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.Command("bash", args...)
	cmd.Dir = projectDir

	output, err := cmd.CombinedOutput()
	result := parseUpgradeOutput(output)
	if err != nil {
		if result.Error == "" {
			result.Error = strings.TrimSpace(string(output))
			if result.Error == "" {
				result.Error = err.Error()
			}
		}
		result.Status = "error"
		return result, fmt.Errorf("upgrade script failed: %w", err)
	}

	return result, nil
}

func parseUpgradeOutput(output []byte) UpgradeResponse {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var result UpgradeResponse
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			if result.Output == "" {
				result.Output = strings.TrimSpace(string(output))
			}
			return result
		}
	}

	trimmed := strings.TrimSpace(string(output))
	return UpgradeResponse{
		Status: "completed",
		Output: trimmed,
		Error:  trimmed,
	}
}
