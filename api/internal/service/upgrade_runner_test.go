package service

import "testing"

func TestUpgradeRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     UpgradeRequest
		wantErr bool
	}{
		{
			name:    "default layer",
			req:     UpgradeRequest{},
			wantErr: false,
		},
		{
			name:    "valid reload",
			req:     UpgradeRequest{Layer: "tmux", Reload: true},
			wantErr: false,
		},
		{
			name:    "invalid layer",
			req:     UpgradeRequest{Layer: "bad"},
			wantErr: true,
		},
		{
			name:    "reload only for tmux",
			req:     UpgradeRequest{Layer: "api", Reload: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseUpgradeOutput(t *testing.T) {
	output := []byte("[✓] test\n{\"status\":\"success\",\"pre_sha\":\"abc\",\"target_sha\":\"def\",\"layers_changed\":[\"tmux\"],\"layers_updated\":[\"tmux\"]}\n")

	result := parseUpgradeOutput(output)

	if result.Status != "success" {
		t.Fatalf("Status = %q, want success", result.Status)
	}
	if len(result.LayersUpdated) != 1 || result.LayersUpdated[0] != "tmux" {
		t.Fatalf("LayersUpdated = %#v, want [tmux]", result.LayersUpdated)
	}
	if result.Output == "" {
		t.Fatal("Output should be preserved")
	}
}
