package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Deprecated: TeamLeadConfig - legacy, kept for backward compatibility. Will be removed in Phase 8.
type TeamLeadConfig struct {
	Prompt string `yaml:"prompt,omitempty"`
	Model  string `yaml:"model,omitempty"`
}

// TeammatesConfig represents teammates settings (legacy)
type TeammatesConfig struct {
	Prompt   string `yaml:"prompt,omitempty"`
	Model    string `yaml:"model,omitempty"`
	MinCount int    `yaml:"min_count,omitempty"`
	MaxCount int    `yaml:"max_count,omitempty"`
}

// MemoryMonitorConfig represents memory monitoring settings for Manager + tmp agent processes
type MemoryMonitorConfig struct {
	Enabled               bool    `yaml:"enabled"`
	CheckIntervalSeconds  int     `yaml:"check_interval_seconds"`
	ThresholdMB           int     `yaml:"threshold_mb"`             // Used when ThresholdType = "fixed"
	ThresholdType         string  `yaml:"threshold_type"`           // "fixed" or "usage_rate"
	SystemMemoryUsageRate float64 `yaml:"system_memory_usage_rate"` // e.g., 0.8 = 80%
	MinFreeMemoryMB       int     `yaml:"min_free_memory_mb"`       // Minimum free memory in MB
	GracePeriodSeconds    int     `yaml:"grace_period_seconds"`
	NotifyDiscord         bool    `yaml:"notify_discord"`
}

// StartupHealthCheckConfig represents startup health verification settings
type StartupHealthCheckConfig struct {
	Enabled            bool `yaml:"enabled"`
	TimeoutSeconds     int  `yaml:"timeout_seconds"`
	CheckClaudeProcess bool `yaml:"check_claude_process"`
	NotifyDiscord      bool `yaml:"notify_discord"`
}

// AgentTeamsConfig represents Agent Teams mode settings (legacy, kept for backward compatibility)
type AgentTeamsConfig struct {
	Enabled            bool                      `yaml:"enabled"`
	MaxTeammates       int                       `yaml:"max_teammates"`
	TeamName           string                    `yaml:"team_name,omitempty"`
	TeamLead           TeamLeadConfig            `yaml:"team_lead"`
	Teammates          TeammatesConfig           `yaml:"teammates"`
	DisplayMode        string                    `yaml:"display_mode,omitempty"`
	PermissionMode     string                    `yaml:"permission_mode,omitempty"`
	IdleShutdown       *IdleShutdownConfig       `yaml:"idle_shutdown,omitempty"`
	MemoryMonitor      *MemoryMonitorConfig      `yaml:"memory_monitor,omitempty"`
	StartupHealthCheck *StartupHealthCheckConfig `yaml:"startup_health_check,omitempty"`
}

// IdleShutdownConfig represents idle auto-shutdown settings
type IdleShutdownConfig struct {
	Enabled              bool     `yaml:"enabled"`
	CheckIntervalMinutes int      `yaml:"check_interval_minutes"`
	IdleTimeoutMinutes   int      `yaml:"idle_timeout_minutes"`
	Exclude              []string `yaml:"exclude"`
	NotifyDiscord        bool     `yaml:"notify_discord"`
}

// OllamaOptionsConfig represents Ollama generation parameters
type OllamaOptionsConfig struct {
	Temperature   float64 `yaml:"temperature"`
	TopK          int     `yaml:"top_k"`
	TopP          float64 `yaml:"top_p"`
	NumCtx        int     `yaml:"num_ctx"`
	NumPredict    int     `yaml:"num_predict"`
	RepeatPenalty float64 `yaml:"repeat_penalty"`
}

// OllamaConfig represents Ollama service configuration
type OllamaConfig struct {
	Enabled                 bool                 `yaml:"enabled"`
	Port                    int                  `yaml:"port"`
	AutoStart               bool                 `yaml:"auto_start"`
	StartupTimeoutSeconds   int                  `yaml:"startup_timeout_seconds"`
	HealthcheckEndpoint     string               `yaml:"healthcheck_endpoint"`
	Description             string               `yaml:"description"`
	Model                   string               `yaml:"model"`
	SystemPrompt            string               `yaml:"system_prompt"`
	Options                 *OllamaOptionsConfig `yaml:"options,omitempty"`
	ConversationHistorySize int                  `yaml:"conversation_history_size"`
}

// ManagerServiceConfig represents Manager settings in services.yaml
type ManagerServiceConfig struct {
	Enabled      bool   `yaml:"enabled"`
	WorkingDir   string `yaml:"working_dir"`
	Model        string `yaml:"model"`
	MemoryMB     int    `yaml:"memory_mb"`
	TmuxSession  string `yaml:"tmux_session"`
	AutoStart    bool   `yaml:"auto_start"`
	MaxTmpAgents int    `yaml:"max_tmp_agents"`
	MaxTeammates int    `yaml:"max_teammates"`
}

// ServicesConfig represents the services.yaml configuration (Phase 7: Manager-Only)
type ServicesConfig struct {
	Manager            *ManagerServiceConfig     `yaml:"manager,omitempty"`
	IdleShutdown       *IdleShutdownConfig       `yaml:"idle_shutdown,omitempty"`
	StartupHealthCheck *StartupHealthCheckConfig `yaml:"startup_health_check,omitempty"`
	MemoryMonitor      *MemoryMonitorConfig      `yaml:"memory_monitor,omitempty"`
	DisplayMode        string                    `yaml:"display_mode,omitempty"`
	PermissionMode     string                    `yaml:"permission_mode,omitempty"`
	Ollama             *OllamaConfig             `yaml:"ollama,omitempty"`
}

// AgentConfig is a backward-compatible view built from ServicesConfig for main.go consumption
type AgentConfig struct {
	Mode       string                  `yaml:"mode,omitempty"`
	AgentTeams *AgentTeamsConfig       `yaml:"agent_teams,omitempty"`
	Manager    ManagerConfig           `yaml:"manager"`
	Workers    map[string]WorkerConfig `yaml:"workers"`
	Ollama     *OllamaConfig           `yaml:"ollama,omitempty"`
}

// ManagerConfig represents manager settings (legacy)
type ManagerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Role        string `yaml:"role"`
	Description string `yaml:"description"`
	Pane        int    `yaml:"pane"`
}

// CoordinatorConfig is a backward compatibility alias for ManagerConfig
type CoordinatorConfig = ManagerConfig

// WorkerConfig represents a worker's settings (legacy)
type WorkerConfig struct {
	Enabled     bool    `yaml:"enabled"`
	Role        string  `yaml:"role"`
	Description string  `yaml:"description"`
	Pair        *string `yaml:"pair"`
	Pane        int     `yaml:"pane"`
}

// LimitsConfig represents the limits.yaml configuration
type LimitsConfig struct {
	Agents        AgentLimits         `yaml:"agents"`
	Tasks         TaskLimits          `yaml:"tasks"`
	RateLimits    RateLimitsConfig    `yaml:"rate_limits"`
	Communication CommunicationLimits `yaml:"communication"`
	Resources     ResourceLimits      `yaml:"resources"`
}

// AgentLimits represents agent count limits
type AgentLimits struct {
	MaxTmpAgents int `yaml:"max_tmp_agents"`
	MaxInstances int `yaml:"max_instances"`
	// Deprecated: use MaxTmpAgents / MaxInstances
	MaxWorkers int `yaml:"max_workers"`
	MinWorkers int `yaml:"min_workers"`
}

// TaskLimits represents task limits
type TaskLimits struct {
	MaxConcurrent      int `yaml:"max_concurrent"`
	MaxPending         int `yaml:"max_pending"`
	MaxDurationMinutes int `yaml:"max_duration_minutes"`
}

// RateLimitsConfig represents rate limiting settings
type RateLimitsConfig struct {
	TaskCreatePerMinute      int `yaml:"task_create_per_minute"`
	MessagePerMinutePerAgent int `yaml:"message_per_minute_per_agent"`
	DiscordNotifyPerMinute   int `yaml:"discord_notify_per_minute"`
	APIRequestsPerSecond     int `yaml:"api_requests_per_second"`
}

// CommunicationLimits represents agent communication limits
type CommunicationLimits struct {
	PairMaxRounds            int `yaml:"pair_max_rounds"`
	BroadcastIntervalSeconds int `yaml:"broadcast_interval_seconds"`
	MaxMessageSize           int `yaml:"max_message_size"`
}

// ResourceLimits represents system resource limits
type ResourceLimits struct {
	MemoryWarningMB    int       `yaml:"memory_warning_mb"`
	CPUWarningPercent  int       `yaml:"cpu_warning_percent"`
	DiskWarningPercent int       `yaml:"disk_warning_percent"`
	GPU                GPULimits `yaml:"gpu"`
}

// GPULimits represents GPU resource limits
type GPULimits struct {
	VRAMUsageLimit float64 `yaml:"vram_usage_limit"`
	MinVRAMMB      int     `yaml:"min_vram_mb"`
}

// Config holds all configuration
type Config struct {
	Services  *ServicesConfig
	Agents    *AgentConfig // Backward-compat view built from Services
	Limits    *LimitsConfig
	configDir string
}

// Load loads configuration from the config directory
func Load(configDir string) (*Config, error) {
	cfg := &Config{configDir: configDir}

	// Load services.yaml (the single source of truth since #856)
	servicesPath := filepath.Join(configDir, "services.yaml")

	data, err := os.ReadFile(servicesPath)
	if err != nil {
		return nil, fmt.Errorf("services.yaml not found in %s: %w", configDir, err)
	}

	var svc ServicesConfig
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return nil, fmt.Errorf("failed to parse services.yaml: %w", err)
	}
	cfg.Services = &svc
	// Build AgentConfig for backward compatibility with main.go
	cfg.Agents = buildAgentConfigFromServices(&svc)

	// Load limits.yaml
	limitsConfig, err := loadLimitsConfig(filepath.Join(configDir, "limits.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to load limits.yaml: %w", err)
	}
	cfg.Limits = limitsConfig

	return cfg, nil
}

// buildAgentConfigFromServices creates a backward-compatible AgentConfig from ServicesConfig
func buildAgentConfigFromServices(svc *ServicesConfig) *AgentConfig {
	maxTeammates := 6
	if svc.Manager != nil && svc.Manager.MaxTeammates > 0 {
		maxTeammates = svc.Manager.MaxTeammates
	}

	managerEnabled := true
	if svc.Manager != nil {
		managerEnabled = svc.Manager.Enabled
	}

	return &AgentConfig{
		Mode: "agent_teams",
		AgentTeams: &AgentTeamsConfig{
			Enabled:            true,
			MaxTeammates:       maxTeammates,
			DisplayMode:        svc.DisplayMode,
			PermissionMode:     svc.PermissionMode,
			IdleShutdown:       svc.IdleShutdown,
			MemoryMonitor:      svc.MemoryMonitor,
			StartupHealthCheck: svc.StartupHealthCheck,
		},
		Manager: ManagerConfig{
			Enabled:     managerEnabled,
			Role:        "manager",
			Description: "Task Judgment + tmp Agent Lifecycle Management",
			Pane:        0,
		},
		Ollama: svc.Ollama,
	}
}

// LoadWithDefaults loads configuration with defaults if files don't exist
func LoadWithDefaults(configDir string) *Config {
	cfg, err := Load(configDir)
	if err != nil {
		return defaultConfig()
	}
	return cfg
}

func loadLimitsConfig(path string) (*LimitsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config LimitsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func defaultConfig() *Config {
	svc := &ServicesConfig{
		StartupHealthCheck: &StartupHealthCheckConfig{
			Enabled:            true,
			TimeoutSeconds:     60,
			CheckClaudeProcess: true,
			NotifyDiscord:      true,
		},
		MemoryMonitor: &MemoryMonitorConfig{
			Enabled:               true,
			CheckIntervalSeconds:  30,
			ThresholdType:         "usage_rate",
			SystemMemoryUsageRate: 0.8,
			MinFreeMemoryMB:       1000,
			GracePeriodSeconds:    30,
			NotifyDiscord:         true,
		},
	}

	return &Config{
		Services: svc,
		Agents:   buildAgentConfigFromServices(svc),
		Limits: &LimitsConfig{
			Agents: AgentLimits{
				MaxTmpAgents: 3,
				MaxInstances: 10,
			},
			Tasks: TaskLimits{
				MaxConcurrent:      10,
				MaxPending:         50,
				MaxDurationMinutes: 120,
			},
			RateLimits: RateLimitsConfig{
				TaskCreatePerMinute:      10,
				MessagePerMinutePerAgent: 30,
				DiscordNotifyPerMinute:   5,
				APIRequestsPerSecond:     100,
			},
			Communication: CommunicationLimits{
				PairMaxRounds:            3,
				BroadcastIntervalSeconds: 2,
				MaxMessageSize:           65536,
			},
			Resources: ResourceLimits{
				MemoryWarningMB:    3072,
				CPUWarningPercent:  80,
				DiskWarningPercent: 90,
			},
		},
	}
}

// GetEnabledWorkers returns a list of enabled worker IDs (legacy compatibility)
func (c *Config) GetEnabledWorkers() []string {
	var workers []string
	if c.Agents == nil {
		return workers
	}
	for id, w := range c.Agents.Workers {
		if w.Enabled {
			workers = append(workers, id)
		}
	}
	return workers
}

// GetAllAgentIDs returns all enabled agent IDs (manager + workers)
func (c *Config) GetAllAgentIDs() []string {
	ids := []string{}
	if c.Agents != nil && c.Agents.Manager.Enabled {
		ids = append(ids, "manager")
	}
	for _, w := range c.GetEnabledWorkers() {
		ids = append(ids, w)
	}
	return ids
}

// IsValidAgent checks if an agent ID is valid
func (c *Config) IsValidAgent(agentID string) bool {
	if c.Agents == nil {
		return false
	}
	if agentID == "team-lead" || agentID == "manager" {
		return c.Agents.Manager.Enabled
	}
	w, exists := c.Agents.Workers[agentID]
	return exists && w.Enabled
}

// GetWorkerPaneIndex returns the tmux pane index for a worker
func (c *Config) GetWorkerPaneIndex(workerID string) int {
	if c.Agents == nil {
		return -1
	}
	if workerID == "manager" {
		return c.Agents.Manager.Pane
	}
	if w, exists := c.Agents.Workers[workerID]; exists {
		return w.Pane
	}
	return -1
}

// GetWorkerCount returns the number of enabled workers
func (c *Config) GetWorkerCount() int {
	return len(c.GetEnabledWorkers())
}
