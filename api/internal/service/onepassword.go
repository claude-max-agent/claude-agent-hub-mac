package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// OnePasswordService handles 1Password CLI operations
type OnePasswordService struct {
	saToken string // Service Account Token for bot-params vault
}

// NewOnePasswordService creates a new OnePasswordService
func NewOnePasswordService(saToken string) *OnePasswordService {
	return &OnePasswordService{saToken: saToken}
}

type opField struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Section *struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"section,omitempty"`
}

type opItem struct {
	Fields []opField `json:"fields"`
}

// GetItemFields reads all fields from a 1Password item and returns a map of label→value
func (s *OnePasswordService) GetItemFields(vault, item string) (map[string]string, error) {
	cmd := exec.Command("op", "item", "get", item, "--vault", vault, "--format=json")
	if s.saToken != "" {
		cmd.Env = append(os.Environ(), "OP_SERVICE_ACCOUNT_TOKEN="+s.saToken)
	}
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("op item get failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("op item get failed: %w", err)
	}

	var parsed opItem
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse op output: %w", err)
	}

	result := make(map[string]string)
	for _, f := range parsed.Fields {
		if f.Label != "" && f.Type != "CONCEALED" {
			result[f.Label] = f.Value
		}
	}
	return result, nil
}

// GetItemField reads a single field value from a 1Password item
func (s *OnePasswordService) GetItemField(vault, item, field string) (string, error) {
	cmd := exec.Command("op", "read", fmt.Sprintf("op://%s/%s/%s", vault, item, field))
	if s.saToken != "" {
		cmd.Env = append(os.Environ(), "OP_SERVICE_ACCOUNT_TOKEN="+s.saToken)
	}
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("op read failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("op read failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SetItemField updates a single field in a 1Password item
func (s *OnePasswordService) SetItemField(vault, item, field, value string) error {
	cmd := exec.Command("op", "item", "edit", item, fmt.Sprintf("%s=%s", field, value), "--vault", vault)
	if s.saToken != "" {
		cmd.Env = append(os.Environ(), "OP_SERVICE_ACCOUNT_TOKEN="+s.saToken)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("op item edit failed: %s", string(output))
	}
	return nil
}

// SetItemFields updates multiple fields in a 1Password item in a single command.
// This is significantly faster than calling SetItemField for each field individually.
func (s *OnePasswordService) SetItemFields(vault, item string, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}

	args := []string{"item", "edit", item}
	for field, value := range fields {
		args = append(args, fmt.Sprintf("%s=%s", field, value))
	}
	args = append(args, "--vault", vault)

	cmd := exec.Command("op", args...)
	if s.saToken != "" {
		cmd.Env = append(os.Environ(), "OP_SERVICE_ACCOUNT_TOKEN="+s.saToken)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("op item edit (batch) failed: %s", string(output))
	}
	return nil
}
