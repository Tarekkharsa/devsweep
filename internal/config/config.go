package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the merged user configuration.
type Config struct {
	Protected []ProtectedEntry `yaml:"protected,omitempty"`
	Rules     []RuleOverride   `yaml:"rules,omitempty"`
	Watch     WatchConfig      `yaml:"watch,omitempty"`
	Report    ReportConfig     `yaml:"report,omitempty"`
}

type ProtectedEntry struct {
	Name string `yaml:"name,omitempty"`
	Port int    `yaml:"port,omitempty"`
}

type RuleOverride struct {
	Match         string `yaml:"match"`
	MaxDuplicates *int   `yaml:"max_duplicates,omitempty"`
	MaxAge        string `yaml:"max_age,omitempty"`
	CPU           string `yaml:"cpu,omitempty"`
	Memory        string `yaml:"memory,omitempty"`
}

type WatchConfig struct {
	Interval  int  `yaml:"interval"`
	Notify    bool `yaml:"notify"`
	AutoClean bool `yaml:"auto_clean"`
}

type ReportConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Watch: WatchConfig{
			Interval:  30,
			Notify:    true,
			AutoClean: false,
		},
		Report: ReportConfig{
			RetentionDays: 30,
		},
	}
}

// Load loads configuration from ~/.devsweep.yml and ./.devsweep.yml,
// merging them with built-in defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load user-level config
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".devsweep.yml")
		if err := mergeFromFile(cfg, userPath); err != nil {
			return nil, err
		}
	}

	// Load repo-level config (overrides user-level)
	repoPath := ".devsweep.yml"
	if err := mergeFromFile(cfg, repoPath); err != nil {
		return nil, err
	}

	return cfg, nil
}

func mergeFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var overlay Config
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return err
	}

	// Merge protected lists (append, don't replace)
	cfg.Protected = append(cfg.Protected, overlay.Protected...)

	// Merge rules (overlay rules take precedence by match key)
	for _, or := range overlay.Rules {
		found := false
		for i, er := range cfg.Rules {
			if er.Match == or.Match {
				cfg.Rules[i] = or
				found = true
				break
			}
		}
		if !found {
			cfg.Rules = append(cfg.Rules, or)
		}
	}

	// Override watch/report if specified
	if overlay.Watch.Interval > 0 {
		cfg.Watch.Interval = overlay.Watch.Interval
	}
	if overlay.Watch.Notify {
		cfg.Watch.Notify = overlay.Watch.Notify
	}
	if overlay.Watch.AutoClean {
		cfg.Watch.AutoClean = overlay.Watch.AutoClean
	}
	if overlay.Report.RetentionDays > 0 {
		cfg.Report.RetentionDays = overlay.Report.RetentionDays
	}

	return nil
}
