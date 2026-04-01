package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// StrategiesConfig represents the strategies.yaml configuration
type StrategiesConfig struct {
	Strategies map[string]Strategy `yaml:"strategies" json:"strategies"`
}

// Strategy represents a single trading strategy definition
type Strategy struct {
	Name        string          `yaml:"name" json:"name"`
	Description string          `yaml:"description" json:"description"`
	App         string          `yaml:"app" json:"app"`
	OpVault     string          `yaml:"op_vault" json:"op_vault"`
	OpItem      string          `yaml:"op_item" json:"op_item"`
	Status      string          `yaml:"status" json:"status"`
	Exchange    string          `yaml:"exchange,omitempty" json:"exchange,omitempty"`
	Pairs       []string        `yaml:"pairs,omitempty" json:"pairs,omitempty"`
	Params      []StrategyParam `yaml:"params" json:"params"`
}

// StrategyParam represents a configurable parameter for a strategy
type StrategyParam struct {
	Key         string   `yaml:"key" json:"key"`
	Label       string   `yaml:"label" json:"label"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Type        string   `yaml:"type" json:"type"` // number, string, boolean
	Min         *float64 `yaml:"min,omitempty" json:"min,omitempty"`
	Max         *float64 `yaml:"max,omitempty" json:"max,omitempty"`
	Step        *float64 `yaml:"step,omitempty" json:"step,omitempty"`
	Placeholder string   `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Default     string   `yaml:"default,omitempty" json:"default,omitempty"`
}

// LoadStrategiesConfig loads the strategies.yaml configuration file
func LoadStrategiesConfig(path string) (*StrategiesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg StrategiesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveStrategiesConfig writes the strategies config back to a YAML file
func SaveStrategiesConfig(path string, cfg *StrategiesConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
