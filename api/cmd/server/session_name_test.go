package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/zono819/claude-agent-hub/api/internal/config"
)

// ============================================================
// 1. セッション名定数テスト
// ============================================================

func TestSessionNameConstants(t *testing.T) {
	// Phase 7: Manager-Only — managerセッション名確認
	t.Run("manager session name is 'manager'", func(t *testing.T) {
		if tmuxDefaultSessionName != "manager" {
			t.Errorf("got %q, want %q", tmuxDefaultSessionName, "manager")
		}
	})
}

func TestSessionNameConstantsNoDuplicatePrefix(t *testing.T) {
	constants := map[string]string{
		"tmuxDefaultSessionName": tmuxDefaultSessionName,
	}
	for name, val := range constants {
		t.Run(name, func(t *testing.T) {
			if strings.HasPrefix(val, "hub-hub-") {
				t.Errorf("%s = %q has double hub- prefix", name, val)
			}
		})
	}
}

// ============================================================
// 2. resolveManagerSession テスト（正常系・異常系・エッジケース）
// ============================================================

func TestResolveManagerSession(t *testing.T) {
	// 元の状態を保存・復元
	origConfig := discordRoutingConfig
	defer func() { discordRoutingConfig = origConfig }()

	t.Run("returns default when config is nil", func(t *testing.T) {
		discordRoutingConfig = nil
		got := resolveManagerSession("manager")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q", got, tmuxDefaultSessionName)
		}
	})

	t.Run("returns session from config", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {
						TmuxSession: "hub-manager",
					},
				},
			},
		}
		got := resolveManagerSession("manager")
		if got != "hub-manager" {
			t.Errorf("got %q, want %q", got, "hub-manager")
		}
	})

	t.Run("returns default for unknown template", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {
						TmuxSession: "hub-manager",
					},
				},
			},
		}
		got := resolveManagerSession("nonexistent")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q", got, tmuxDefaultSessionName)
		}
	})

	t.Run("no double hub- prefix in resolved session", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {
						TmuxSession: "hub-manager",
					},
				},
			},
		}
		got := resolveManagerSession("manager")
		if strings.HasPrefix(got, "hub-hub-") {
			t.Errorf("double prefix detected: got %q", got)
		}
	})

	t.Run("uses TmuxSession value directly without adding prefix", func(t *testing.T) {
		// 修正の核心: TmuxSession が "hub-manager" なら "hub-hub-manager" にならない
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"hub": {
						TmuxSession: "hub-hub-team",
					},
				},
			},
		}
		got := resolveManagerSession("hub")
		if got != "hub-hub-team" {
			t.Errorf("got %q, want %q (should use TmuxSession as-is)", got, "hub-hub-team")
		}
	})

	t.Run("empty TmuxSession falls back to default", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {
						TmuxSession: "",
					},
				},
			},
		}
		got := resolveManagerSession("manager")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q (empty TmuxSession should fall back)", got, tmuxDefaultSessionName)
		}
	})

	t.Run("empty Templates map falls back to default", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{},
			},
		}
		got := resolveManagerSession("manager")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q", got, tmuxDefaultSessionName)
		}
	})

	t.Run("nil Templates map falls back to default", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: nil,
			},
		}
		got := resolveManagerSession("manager")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q", got, tmuxDefaultSessionName)
		}
	})

	t.Run("multiple templates resolve correctly", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {TmuxSession: "hub-manager"},
					"slack":   {TmuxSession: "hub-slack"},
					"crypto":  {TmuxSession: "hub-crypto"},
				},
			},
		}
		cases := []struct {
			template string
			expected string
		}{
			{"manager", "hub-manager"},
			{"slack", "hub-slack"},
			{"crypto", "hub-crypto"},
			{"unknown", tmuxDefaultSessionName},
		}
		for _, c := range cases {
			got := resolveManagerSession(c.template)
			if got != c.expected {
				t.Errorf("resolveManagerSession(%q) = %q, want %q", c.template, got, c.expected)
			}
		}
	})
}

// ============================================================
// 3. エッジケース — 空文字、不正なセッション名
// ============================================================

func TestResolveManagerSessionEdgeCases(t *testing.T) {
	origConfig := discordRoutingConfig
	defer func() { discordRoutingConfig = origConfig }()

	t.Run("empty template name", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {TmuxSession: "hub-manager"},
				},
			},
		}
		got := resolveManagerSession("")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q for empty template name", got, tmuxDefaultSessionName)
		}
	})

	t.Run("template name with spaces", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"bad name": {TmuxSession: "hub-bad"},
				},
			},
		}
		got := resolveManagerSession("bad name")
		// テンプレート名にスペースがあっても一致すればそのまま返す
		if got != "hub-bad" {
			t.Errorf("got %q, want %q", got, "hub-bad")
		}
	})

	t.Run("template name with special characters", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{},
			},
		}
		// 存在しないテンプレート → デフォルト
		got := resolveManagerSession("../../../../etc/passwd")
		if got != tmuxDefaultSessionName {
			t.Errorf("got %q, want %q for malicious template name", got, tmuxDefaultSessionName)
		}
	})
}

// ============================================================
// 4. isTmuxSessionExists / isDispatcherRunning / isManagerRunning
//    — tmuxセッション生存・死亡判定テスト
// ============================================================

func TestIsTmuxSessionExists(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	t.Run("returns true when tmux session exists", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			// tmux has-session -t <session> → 成功（exit 0）
			return exec.Command("true")
		}
		if !isTmuxSessionExists("hub-manager") {
			t.Error("expected true when tmux session exists")
		}
	})

	t.Run("returns false when tmux session does not exist", func(t *testing.T) {
		execCommand = func(name string, args ...string) *exec.Cmd {
			// tmux has-session -t <session> → 失敗（exit 1）
			return exec.Command("false")
		}
		if isTmuxSessionExists("hub-manager") {
			t.Error("expected false when tmux session does not exist")
		}
	})

	t.Run("passes correct session name to tmux", func(t *testing.T) {
		var capturedArgs []string
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			return exec.Command("true")
		}
		isTmuxSessionExists("hub-test-session")
		expected := []string{"tmux", "has-session", "-t", "hub-test-session"}
		if len(capturedArgs) != len(expected) {
			t.Fatalf("args length mismatch: got %v, want %v", capturedArgs, expected)
		}
		for i, v := range expected {
			if capturedArgs[i] != v {
				t.Errorf("arg[%d] = %q, want %q", i, capturedArgs[i], v)
			}
		}
	})
}

func TestIsManagerRunning(t *testing.T) {
	origExecCommand := execCommand
	origConfig := discordRoutingConfig
	defer func() {
		execCommand = origExecCommand
		discordRoutingConfig = origConfig
	}()

	t.Run("checks resolved session name from config", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"hub": {TmuxSession: "hub-manager"},
				},
			},
		}
		var capturedSession string
		execCommand = func(name string, args ...string) *exec.Cmd {
			if len(args) >= 3 && args[0] == "has-session" {
				capturedSession = args[2]
			}
			return exec.Command("true")
		}
		result := isManagerRunning("hub")
		if !result {
			t.Error("expected true when manager session is alive")
		}
		if capturedSession != "hub-manager" {
			t.Errorf("manager checked session %q, want %q", capturedSession, "hub-manager")
		}
	})

	t.Run("falls back to default when template unknown", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{},
			},
		}
		var capturedSession string
		execCommand = func(name string, args ...string) *exec.Cmd {
			if len(args) >= 3 && args[0] == "has-session" {
				capturedSession = args[2]
			}
			return exec.Command("true")
		}
		isManagerRunning("nonexistent")
		if capturedSession != tmuxDefaultSessionName {
			t.Errorf("checked session %q, want default %q", capturedSession, tmuxDefaultSessionName)
		}
	})

	t.Run("returns false when manager session is dead", func(t *testing.T) {
		discordRoutingConfig = nil
		execCommand = func(name string, args ...string) *exec.Cmd {
			return exec.Command("false")
		}
		if isManagerRunning("manager") {
			t.Error("expected false when manager session does not exist")
		}
	})
}

// ============================================================
// 5. autoStartManager — auto-start 成功/失敗テスト
// ============================================================

func TestAutoStartManager(t *testing.T) {
	origExecCommand := execCommand
	origConfig := discordRoutingConfig
	defer func() {
		execCommand = origExecCommand
		discordRoutingConfig = origConfig
	}()

	t.Run("fails when config is nil", func(t *testing.T) {
		discordRoutingConfig = nil
		err := autoStartManager("manager")
		if err == nil {
			t.Error("expected error when config is nil")
		}
		if !strings.Contains(err.Error(), "config not loaded") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fails for unknown template", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {TmuxSession: "hub-manager"},
				},
			},
		}
		err := autoStartManager("nonexistent")
		if err == nil {
			t.Error("expected error for unknown template")
		}
		if !strings.Contains(err.Error(), "unknown manager template") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("resolves correct session name in auto-start", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"hub": {TmuxSession: "hub-manager"},
				},
			},
		}
		// tmux has-session → session already running (skip actual start)
		var capturedSession string
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == "tmux" && len(args) >= 3 && args[0] == "has-session" {
				capturedSession = args[2]
				return exec.Command("true") // session exists → no-op
			}
			return exec.Command("true")
		}
		err := autoStartManager("hub")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if capturedSession != "hub-manager" {
			t.Errorf("auto-start checked session %q, want %q", capturedSession, "hub-manager")
		}
	})

	t.Run("session name does not get double hub- prefix during auto-start", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"manager": {TmuxSession: "hub-manager"},
				},
			},
		}
		var capturedSession string
		execCommand = func(name string, args ...string) *exec.Cmd {
			if name == "tmux" && len(args) >= 3 && args[0] == "has-session" {
				capturedSession = args[2]
				return exec.Command("true")
			}
			return exec.Command("true")
		}
		autoStartManager("manager")
		if strings.HasPrefix(capturedSession, "hub-hub-") {
			t.Errorf("double prefix in auto-start: session = %q", capturedSession)
		}
	})
}

// ============================================================
// 6. prefix有無の両パターン — hub-あり/なしの入力に対する正しい処理
// ============================================================

func TestResolveManagerSessionPrefixHandling(t *testing.T) {
	origConfig := discordRoutingConfig
	defer func() { discordRoutingConfig = origConfig }()

	t.Run("TmuxSession already has hub- prefix — used as-is", func(t *testing.T) {
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"mgr": {TmuxSession: "hub-manager"},
				},
			},
		}
		got := resolveManagerSession("mgr")
		if got != "hub-manager" {
			t.Errorf("got %q, want %q", got, "hub-manager")
		}
	})

	t.Run("TmuxSession without hub- prefix — used as-is (no prefix added)", func(t *testing.T) {
		// 修正後: TmuxSessionの値をそのまま使用し、プレフィックスを追加しない
		discordRoutingConfig = &config.DiscordConfig{
			Discord: config.DiscordSettings{
				Templates: map[string]config.TemplateDef{
					"custom": {TmuxSession: "custom-session"},
				},
			},
		}
		got := resolveManagerSession("custom")
		if got != "custom-session" {
			t.Errorf("got %q, want %q (should use TmuxSession as-is)", got, "custom-session")
		}
	})

	t.Run("config values preserve exact session names", func(t *testing.T) {
		// 典型的なセッション名のパターンテスト
		templates := map[string]string{
			"hub-manager": "hub-manager",
			"hub-slack":   "hub-slack",
			"manager":     "manager",
		}
		for session, expected := range templates {
			discordRoutingConfig = &config.DiscordConfig{
				Discord: config.DiscordSettings{
					Templates: map[string]config.TemplateDef{
						"test": {TmuxSession: session},
					},
				},
			}
			got := resolveManagerSession("test")
			if got != expected {
				t.Errorf("TmuxSession=%q: got %q, want %q", session, got, expected)
			}
		}
	})
}

// ============================================================
// 7. sessionPrefix 定数が削除されたことの確認（回帰テスト）
// ============================================================

func TestSessionPrefixConstantRemoved(t *testing.T) {
	// 旧コードでは sessionPrefix = "hub" 定数があり、
	// resolveManagerSession で sessionPrefix + "-" + tl.TmuxSession としていた。
	// これにより hub-hub-manager のような二重プレフィックスが発生していた。
	//
	// 修正後は TmuxSession の値をそのまま使うため、
	// resolveManagerSession の結果が config 値と一致することを検証する。
	origConfig := discordRoutingConfig
	defer func() { discordRoutingConfig = origConfig }()

	testCases := []struct {
		tmuxSession string
		expected    string
	}{
		{"hub-manager", "hub-manager"},
		{"hub-slack", "hub-slack"},
		{"hub-crypto", "hub-crypto"},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("TmuxSession=%s", tc.tmuxSession), func(t *testing.T) {
			discordRoutingConfig = &config.DiscordConfig{
				Discord: config.DiscordSettings{
					Templates: map[string]config.TemplateDef{
						"test": {TmuxSession: tc.tmuxSession},
					},
				},
			}
			got := resolveManagerSession("test")
			if got != tc.expected {
				t.Errorf("got %q, want %q — sessionPrefix may still be in use", got, tc.expected)
			}
		})
	}
}
