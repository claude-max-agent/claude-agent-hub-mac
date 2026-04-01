package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// HealthCheckConfig represents health check settings for an app
type HealthCheckConfig struct {
	Type string `yaml:"type" json:"type"` // http, process
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	Port int    `yaml:"port,omitempty" json:"port,omitempty"`
}

// EnvConfig represents environment variable requirements
type EnvConfig struct {
	Required []string `yaml:"required" json:"required"`
	Optional []string `yaml:"optional" json:"optional"`
}

// AppConfig represents a single application's configuration
type AppConfig struct {
	Name             string            `yaml:"name" json:"name"`
	Description      string            `yaml:"description" json:"description"`
	Repo             string            `yaml:"repo" json:"repo"`
	Language         string            `yaml:"language,omitempty" json:"language,omitempty"`
	Type             string            `yaml:"type" json:"type"` // process, docker-compose, docker, self
	Path             string            `yaml:"path" json:"path"`
	ComposeFile      string            `yaml:"compose_file,omitempty" json:"compose_file,omitempty"`       // TODO: Docker support (disabled for memory)
	Dockerfile       string            `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`           // TODO: Docker support
	ClaudeHubDockerfile  string            `yaml:"claude_hub_dockerfile,omitempty" json:"claude_hub_dockerfile,omitempty"` // TODO: Docker support
	BuildCommand     string            `yaml:"build_command,omitempty" json:"build_command,omitempty"`
	StartCommand     string            `yaml:"start_command,omitempty" json:"start_command,omitempty"`
	DashboardCommand string            `yaml:"dashboard_command,omitempty" json:"dashboard_command,omitempty"`
	WorkDir          string            `yaml:"work_dir,omitempty" json:"work_dir,omitempty"`
	LogFile          string            `yaml:"log_file,omitempty" json:"log_file,omitempty"`
	PidFile          string            `yaml:"pid_file,omitempty" json:"pid_file,omitempty"`
	EnvFile          string            `yaml:"env_file,omitempty" json:"env_file,omitempty"`
	Services         []string          `yaml:"services,omitempty" json:"services,omitempty"`
	Port             int               `yaml:"port" json:"port"`
	HealthCheck      HealthCheckConfig `yaml:"healthcheck" json:"healthcheck"`
	Env              EnvConfig         `yaml:"env,omitempty" json:"env,omitempty"`
	Dependencies     []string          `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Volumes          []string          `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Schedule         string            `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Status           string            `yaml:"status,omitempty" json:"status,omitempty"`
	AutoStart        bool              `yaml:"auto_start" json:"auto_start"`
	UsesClaude       bool              `yaml:"uses_claude,omitempty" json:"uses_claude,omitempty"`
	Model            string            `yaml:"model,omitempty" json:"model,omitempty"`
}

// AppsConfig represents the apps.yaml configuration
type AppsConfig struct {
	Apps        map[string]AppConfig `yaml:"apps" json:"apps"`
	AppsBaseDir string               `yaml:"apps_base_dir" json:"apps_base_dir"`
}

// GetAppPath returns the resolved absolute path for an app
func (ac *AppsConfig) GetAppPath(appID string) string {
	app, exists := ac.Apps[appID]
	if !exists {
		return ""
	}
	if app.Path != "" {
		return app.Path
	}
	baseDir := ac.AppsBaseDir
	if envDir := os.Getenv("APPS_BASE_DIR"); envDir != "" {
		baseDir = envDir
	}
	if baseDir == "" {
		return ""
	}
	return filepath.Join(baseDir, appID)
}

// LoadAppsConfig loads the apps.yaml configuration file
func LoadAppsConfig(configDir string) (*AppsConfig, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "apps.yaml"))
	if err != nil {
		return nil, err
	}

	var cfg AppsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if envDir := os.Getenv("APPS_BASE_DIR"); envDir != "" {
		cfg.AppsBaseDir = envDir
	}

	return &cfg, nil
}

// LoadAppsConfigWithDefaults loads apps config with empty defaults if file doesn't exist
func LoadAppsConfigWithDefaults(configDir string) *AppsConfig {
	cfg, err := LoadAppsConfig(configDir)
	if err != nil {
		return &AppsConfig{
			Apps: make(map[string]AppConfig),
		}
	}
	return cfg
}
