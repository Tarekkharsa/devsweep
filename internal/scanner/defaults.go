package scanner

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed default_rules.yml
var embeddedDefaultRules []byte

type signatureSpec struct {
	Category string   `yaml:"category"`
	Tool     string   `yaml:"tool"`
	Binary   []string `yaml:"binary"`
	Cmdline  []string `yaml:"cmdline"`
}

type defaultRulesFile struct {
	Signatures []signatureSpec `yaml:"signatures"`
	Rules      struct {
		Stale struct {
			MaxAge string `yaml:"max_age"`
		} `yaml:"stale"`
		Duplicates struct {
			MaxDuplicates int `yaml:"max_duplicates"`
		} `yaml:"duplicates"`
		CPUHog struct {
			CPUPercent float64 `yaml:"cpu_percent"`
			Duration   string  `yaml:"duration"`
		} `yaml:"cpu_hog"`
		MemoryBloat struct {
			MaxMemory          string `yaml:"max_memory"`
			DevServerMaxMemory string `yaml:"dev_server_max_memory"`
		} `yaml:"memory_bloat"`
		Orphan struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"orphan"`
	} `yaml:"rules"`
}

// DefaultRuleValues contains built-in rule defaults loaded from YAML.
type DefaultRuleValues struct {
	MaxDuplicates     int
	StaleMaxAge       time.Duration
	CPUHogPercent     float64
	CPUHogDuration    time.Duration
	MemoryBloatMB     float64
	DevServerMemoryMB float64
	OrphanEnabled     bool
}

var (
	defaultsOnce          sync.Once
	loadedDefaults        defaultRulesFile
	loadedCategoryRules   []categoryRule
	loadedDefaultRuleVals DefaultRuleValues
)

func defaultCategoryRules() []categoryRule {
	defaultsOnce.Do(loadDefaults)
	if len(loadedCategoryRules) > 0 {
		return loadedCategoryRules
	}
	return fallbackCategoryRules
}

// BuiltInRuleValues returns default thresholds loaded from the built-in rule file.
func BuiltInRuleValues() DefaultRuleValues {
	defaultsOnce.Do(loadDefaults)
	if loadedDefaultRuleVals.MaxDuplicates > 0 {
		return loadedDefaultRuleVals
	}
	return fallbackDefaultRuleValues()
}

func loadDefaults() {
	data := readDefaultRulesData()
	if len(data) == 0 {
		loadedCategoryRules = fallbackCategoryRules
		loadedDefaultRuleVals = fallbackDefaultRuleValues()
		return
	}

	var parsed defaultRulesFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		loadedCategoryRules = fallbackCategoryRules
		loadedDefaultRuleVals = fallbackDefaultRuleValues()
		return
	}

	loadedDefaults = parsed
	loadedCategoryRules = compileSignatureRules(parsed.Signatures)
	if len(loadedCategoryRules) == 0 {
		loadedCategoryRules = fallbackCategoryRules
	}
	loadedDefaultRuleVals = DefaultRuleValues{
		MaxDuplicates:     maxInt(parsed.Rules.Duplicates.MaxDuplicates, 1),
		StaleMaxAge:       parseDurationOr(parsed.Rules.Stale.MaxAge, 24*time.Hour),
		CPUHogPercent:     maxFloat(parsed.Rules.CPUHog.CPUPercent, 50),
		CPUHogDuration:    parseDurationOr(parsed.Rules.CPUHog.Duration, 5*time.Minute),
		MemoryBloatMB:     parseMemoryMBOr(parsed.Rules.MemoryBloat.MaxMemory, 500),
		DevServerMemoryMB: parseMemoryMBOr(parsed.Rules.MemoryBloat.DevServerMaxMemory, 1500),
		OrphanEnabled:     parsed.Rules.Orphan.Enabled,
	}
	if !parsed.Rules.Orphan.Enabled {
		loadedDefaultRuleVals.OrphanEnabled = false
	}
}

func fallbackDefaultRuleValues() DefaultRuleValues {
	return DefaultRuleValues{
		MaxDuplicates:     1,
		StaleMaxAge:       24 * time.Hour,
		CPUHogPercent:     50,
		CPUHogDuration:    5 * time.Minute,
		MemoryBloatMB:     500,
		DevServerMemoryMB: 1500,
		OrphanEnabled:     true,
	}
}

func readDefaultRulesData() []byte {
	for _, path := range defaultRuleCandidates() {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data
		}
	}
	return embeddedDefaultRules
}

func defaultRuleCandidates() []string {
	candidates := []string{filepath.Join("rules", "default.yml")}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "rules", "default.yml"))
	}
	return candidates
}

func compileSignatureRules(specs []signatureSpec) []categoryRule {
	compiled := make([]categoryRule, 0, len(specs))
	for _, spec := range specs {
		cat, ok := parseCategory(spec.Category)
		if !ok || strings.TrimSpace(spec.Tool) == "" {
			continue
		}
		compiled = append(compiled, categoryRule{
			category:    cat,
			tool:        spec.Tool,
			binaries:    spec.Binary,
			cmdPatterns: spec.Cmdline,
		})
	}
	return compiled
}

func parseCategory(v string) (Category, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "protected":
		return CategoryProtected, true
	case "dev_server", "dev_servers":
		return CategoryDevServer, true
	case "ai_agent", "ai_agents":
		return CategoryAIAgent, true
	case "package_manager", "package_managers":
		return CategoryPackageManager, true
	case "runtime", "runtimes":
		return CategoryRuntime, true
	default:
		return CategoryUnknown, false
	}
}

func parseDurationOr(v string, fallback time.Duration) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func parseMemoryMBOr(v string, fallback float64) float64 {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "" {
		return fallback
	}
	multiplier := 1.0
	switch {
	case strings.HasSuffix(v, "GB"):
		multiplier = 1024
		v = strings.TrimSuffix(v, "GB")
	case strings.HasSuffix(v, "G"):
		multiplier = 1024
		v = strings.TrimSuffix(v, "G")
	case strings.HasSuffix(v, "MB"):
		v = strings.TrimSuffix(v, "MB")
	case strings.HasSuffix(v, "M"):
		v = strings.TrimSuffix(v, "M")
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return fallback
	}
	return f * multiplier
}

func maxInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func maxFloat(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}
