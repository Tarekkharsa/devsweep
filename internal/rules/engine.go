package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

// IssueType identifies the kind of problem detected.
type IssueType string

const (
	IssueDuplicate   IssueType = "duplicate"
	IssueStale       IssueType = "stale"
	IssueOrphan      IssueType = "orphan"
	IssueCPUHog      IssueType = "cpu_hog"
	IssueMemoryBloat IssueType = "memory_bloat"
)

// Confidence indicates how safe an issue is to auto-clean.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Issue represents a detected problem with one or more processes.
type Issue struct {
	Type        IssueType             `json:"type"`
	Description string                `json:"description"`
	Processes   []scanner.ProcessInfo `json:"processes"`
	TotalCPU    float64               `json:"totalCpu"`
	TotalMemMB  float64               `json:"totalMemMb"`
	Suggestion  string                `json:"suggestion"`
	Confidence  Confidence            `json:"confidence"`
}

// DetectAll runs all rule checks against the scanned processes using default thresholds.
func DetectAll(procs []scanner.ProcessInfo) []Issue {
	return DetectAllWithConfig(procs, nil)
}

// DetectAllWithConfig runs all rule checks against the scanned processes using config overrides.
func DetectAllWithConfig(procs []scanner.ProcessInfo, cfg *config.Config) []Issue {
	issues := make([]Issue, 0)
	issues = append(issues, detectDuplicates(procs, cfg)...)
	issues = append(issues, detectStale(procs, cfg)...)
	issues = append(issues, detectOrphans(procs)...)
	issues = append(issues, detectCPUHogs(procs, cfg)...)
	issues = append(issues, detectMemoryBloat(procs, cfg)...)

	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Confidence != issues[j].Confidence {
			return confidenceRank(issues[i].Confidence) < confidenceRank(issues[j].Confidence)
		}
		if issues[i].Type != issues[j].Type {
			return issues[i].Type < issues[j].Type
		}
		return issues[i].Description < issues[j].Description
	})

	return issues
}

// detectDuplicates finds multiple instances of the same tool on the same port,
// or identical AI helper processes without ports.
func detectDuplicates(procs []scanner.ProcessInfo, cfg *config.Config) []Issue {
	var issues []Issue

	type portKey struct {
		tool string
		port int
	}
	portGroups := make(map[portKey][]scanner.ProcessInfo)
	for _, p := range procs {
		if p.Tool == "" || p.Category == scanner.CategoryProtected || p.Port == 0 {
			continue
		}
		k := portKey{tool: p.Tool, port: p.Port}
		portGroups[k] = append(portGroups[k], p)
	}

	for k, group := range portGroups {
		maxDuplicates := maxDuplicatesForGroup(group, cfg)
		if len(group) <= maxDuplicates {
			continue
		}

		totalCPU, totalMem := totals(group)
		issues = append(issues, Issue{
			Type:        IssueDuplicate,
			Description: fmt.Sprintf("%d× %s on :%d (only %d needed)", len(group), k.tool, k.port, maxDuplicates),
			Processes:   group,
			TotalCPU:    totalCPU,
			TotalMemMB:  totalMem,
			Suggestion:  fmt.Sprintf("Keep newest %d, kill %d others", maxDuplicates, len(group)-maxDuplicates),
			Confidence:  ConfidenceHigh,
		})
	}

	type helperKey struct {
		tool   string
		cwd    string
		normal string
	}
	helperGroups := make(map[helperKey][]scanner.ProcessInfo)
	for _, p := range procs {
		if p.Category != scanner.CategoryAIAgent || p.Category == scanner.CategoryProtected || p.Port != 0 {
			continue
		}
		normalized := normalizeCmdline(p.Cmdline)
		if normalized == "" {
			continue
		}
		k := helperKey{tool: p.Tool, cwd: p.Cwd, normal: normalized}
		helperGroups[k] = append(helperGroups[k], p)
	}

	for k, group := range helperGroups {
		maxDuplicates := maxDuplicatesForGroup(group, cfg)
		if len(group) <= maxDuplicates || len(group) < 2 {
			continue
		}

		totalCPU, totalMem := totals(group)
		desc := fmt.Sprintf("%d× %s helpers with identical command line", len(group), k.tool)
		if k.cwd != "" {
			desc = fmt.Sprintf("%s in %s", desc, k.cwd)
		}
		issues = append(issues, Issue{
			Type:        IssueDuplicate,
			Description: desc,
			Processes:   group,
			TotalCPU:    totalCPU,
			TotalMemMB:  totalMem,
			Suggestion:  fmt.Sprintf("Keep newest %d, kill %d identical helpers", maxDuplicates, len(group)-maxDuplicates),
			Confidence:  ConfidenceMedium,
		})
	}

	return issues
}

// detectStale finds dev servers running longer than the configured max age with low activity.
func detectStale(procs []scanner.ProcessInfo, cfg *config.Config) []Issue {
	var issues []Issue

	for _, p := range procs {
		if p.Category != scanner.CategoryDevServer || p.Category == scanner.CategoryProtected {
			continue
		}

		maxAge := maxAgeForProcess(p, cfg)
		if p.Uptime > maxAge && p.CPUPercent < 5.0 {
			issues = append(issues, Issue{
				Type:        IssueStale,
				Description: fmt.Sprintf("%s running for %s with ~0%% CPU", p.Tool, scanner.FormatUptime(p.Uptime)),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Likely forgotten — safe to kill",
				Confidence:  ConfidenceHigh,
			})
		}
	}

	return issues
}

// detectOrphans finds processes whose parent has died (PPID=1).
func detectOrphans(procs []scanner.ProcessInfo) []Issue {
	defaults := scanner.BuiltInRuleValues()
	if !defaults.OrphanEnabled {
		return nil
	}

	var issues []Issue

	for _, p := range procs {
		if p.IsOrphan && p.Category != scanner.CategoryProtected && p.Category != scanner.CategoryRuntime {
			issues = append(issues, Issue{
				Type:        IssueOrphan,
				Description: fmt.Sprintf("Orphaned %s (parent dead, PPID=1)", p.Tool),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Parent process exited without cleanup",
				Confidence:  ConfidenceHigh,
			})
		}
	}

	return issues
}

// detectCPUHogs finds processes using high CPU for extended periods.
func detectCPUHogs(procs []scanner.ProcessInfo, cfg *config.Config) []Issue {
	var issues []Issue
	defaults := scanner.BuiltInRuleValues()

	for _, p := range procs {
		if p.Category == scanner.CategoryProtected {
			continue
		}

		cpuThreshold := cpuThresholdForProcess(p, cfg)
		if p.CPUPercent > cpuThreshold && p.Uptime > defaults.CPUHogDuration {
			issues = append(issues, Issue{
				Type:        IssueCPUHog,
				Description: fmt.Sprintf("%s using %.0f%% CPU for %s", p.Tool, p.CPUPercent, scanner.FormatUptime(p.Uptime)),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Possible runaway process",
				Confidence:  ConfidenceMedium,
			})
		}
	}

	return issues
}

// detectMemoryBloat finds processes using excessive RAM while idle.
// Only flags orphaned processes or processes NOT in an active session.
func detectMemoryBloat(procs []scanner.ProcessInfo, cfg *config.Config) []Issue {
	var issues []Issue

	for _, p := range procs {
		if p.Category == scanner.CategoryProtected {
			continue
		}

		threshold := memoryThresholdForProcess(p, cfg)
		if p.MemoryMB > threshold {
			if p.SessionIsActive {
				continue
			}

			isIdle := p.CPUPercent < 5.0
			isOrphan := p.IsOrphan

			suggestion := "Possible memory leak"
			confidence := ConfidenceLow
			if isOrphan {
				suggestion = "Orphaned process with high memory"
				confidence = ConfidenceMedium
			} else if isIdle {
				suggestion = "Idle process with high memory"
				confidence = ConfidenceMedium
			}

			issues = append(issues, Issue{
				Type:        IssueMemoryBloat,
				Description: fmt.Sprintf("%s using %.0f MB RAM", p.Tool, p.MemoryMB),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  suggestion,
				Confidence:  confidence,
			})
		}
	}

	return issues
}

func totals(group []scanner.ProcessInfo) (cpu float64, mem float64) {
	for _, p := range group {
		cpu += p.CPUPercent
		mem += p.MemoryMB
	}
	return cpu, mem
}

func confidenceRank(c Confidence) int {
	switch c {
	case ConfidenceHigh:
		return 0
	case ConfidenceMedium:
		return 1
	default:
		return 2
	}
}

func maxDuplicatesForGroup(group []scanner.ProcessInfo, cfg *config.Config) int {
	maxDuplicates := scanner.BuiltInRuleValues().MaxDuplicates
	for _, p := range group {
		if override, ok := matchingOverride(p, cfg); ok && override.MaxDuplicates != nil && *override.MaxDuplicates > 0 {
			if *override.MaxDuplicates < maxDuplicates || maxDuplicates == 1 {
				maxDuplicates = *override.MaxDuplicates
			}
		}
	}
	if maxDuplicates < 1 {
		return 1
	}
	return maxDuplicates
}

func maxAgeForProcess(p scanner.ProcessInfo, cfg *config.Config) time.Duration {
	defaultMaxAge := scanner.BuiltInRuleValues().StaleMaxAge
	if override, ok := matchingOverride(p, cfg); ok {
		if d, ok := parseDuration(override.MaxAge); ok {
			return d
		}
	}
	return defaultMaxAge
}

func cpuThresholdForProcess(p scanner.ProcessInfo, cfg *config.Config) float64 {
	defaultCPU := scanner.BuiltInRuleValues().CPUHogPercent
	if override, ok := matchingOverride(p, cfg); ok {
		if v, ok := parsePercent(override.CPU); ok {
			return v
		}
	}
	return defaultCPU
}

func memoryThresholdForProcess(p scanner.ProcessInfo, cfg *config.Config) float64 {
	defaults := scanner.BuiltInRuleValues()
	threshold := defaults.MemoryBloatMB
	if p.Category == scanner.CategoryDevServer {
		threshold = defaults.DevServerMemoryMB
	}
	if override, ok := matchingOverride(p, cfg); ok {
		if v, ok := parseMemoryMB(override.Memory); ok {
			return v
		}
	}
	return threshold
}

func matchingOverride(p scanner.ProcessInfo, cfg *config.Config) (config.RuleOverride, bool) {
	if cfg == nil {
		return config.RuleOverride{}, false
	}

	haystack := strings.ToLower(strings.Join([]string{p.Tool, p.Name, p.Cmdline, p.Cwd}, " "))
	for _, rule := range cfg.Rules {
		needle := strings.ToLower(strings.TrimSpace(rule.Match))
		if needle == "" {
			continue
		}
		if strings.Contains(haystack, needle) {
			return rule, true
		}
	}
	return config.RuleOverride{}, false
}

func parseDuration(v string) (time.Duration, bool) {
	if strings.TrimSpace(v) == "" {
		return 0, false
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, false
	}
	return d, true
}

func parsePercent(v string) (float64, bool) {
	v = strings.TrimSpace(strings.TrimSuffix(v, "%"))
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseMemoryMB(v string) (float64, bool) {
	v = strings.TrimSpace(strings.ToUpper(v))
	if v == "" {
		return 0, false
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
		return 0, false
	}
	return f * multiplier, true
}

func normalizeCmdline(cmd string) string {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	if cmd == "" {
		return ""
	}
	return strings.Join(strings.Fields(cmd), " ")
}
