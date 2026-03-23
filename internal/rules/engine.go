package rules

import (
	"fmt"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

// IssueType identifies the kind of problem detected.
type IssueType string

const (
	IssueDuplicate  IssueType = "duplicate"
	IssueStale      IssueType = "stale"
	IssueOrphan     IssueType = "orphan"
	IssueCPUHog     IssueType = "cpu_hog"
	IssueMemoryBloat IssueType = "memory_bloat"
)

// Issue represents a detected problem with one or more processes.
type Issue struct {
	Type        IssueType
	Description string
	Processes   []scanner.ProcessInfo
	TotalCPU    float64
	TotalMemMB  float64
	Suggestion  string
}

// DetectAll runs all rule checks against the scanned processes.
func DetectAll(procs []scanner.ProcessInfo) []Issue {
	var issues []Issue
	issues = append(issues, detectDuplicates(procs)...)
	issues = append(issues, detectStale(procs)...)
	issues = append(issues, detectOrphans(procs)...)
	issues = append(issues, detectCPUHogs(procs)...)
	issues = append(issues, detectMemoryBloat(procs)...)
	return issues
}

// detectDuplicates finds multiple instances of the same tool on the same port.
func detectDuplicates(procs []scanner.ProcessInfo) []Issue {
	var issues []Issue

	// Group by tool+port
	type key struct {
		tool string
		port int
	}
	groups := make(map[key][]scanner.ProcessInfo)
	for _, p := range procs {
		if p.Tool == "" || p.Category == scanner.CategoryProtected {
			continue
		}
		// Only group by port when the process is actually LISTENING on one.
		// Processes with port=0 are not servers and shouldn't be flagged as duplicates.
		if p.Port == 0 {
			continue
		}
		k := key{tool: p.Tool, port: p.Port}
		groups[k] = append(groups[k], p)
	}

	for k, group := range groups {
		if len(group) <= 1 {
			continue
		}

		var totalCPU, totalMem float64
		for _, p := range group {
			totalCPU += p.CPUPercent
			totalMem += p.MemoryMB
		}

		portStr := ""
		if k.port > 0 {
			portStr = fmt.Sprintf(" on :%d", k.port)
		}

		issues = append(issues, Issue{
			Type:        IssueDuplicate,
			Description: fmt.Sprintf("%d× %s%s (only 1 needed)", len(group), k.tool, portStr),
			Processes:   group,
			TotalCPU:    totalCPU,
			TotalMemMB:  totalMem,
			Suggestion:  fmt.Sprintf("Keep newest, kill %d others", len(group)-1),
		})
	}

	return issues
}

// detectStale finds dev servers running longer than 24 hours with low activity.
func detectStale(procs []scanner.ProcessInfo) []Issue {
	var issues []Issue
	maxAge := 24 * time.Hour

	for _, p := range procs {
		if p.Category != scanner.CategoryDevServer || p.Category == scanner.CategoryProtected {
			continue
		}
		if p.Uptime > maxAge && p.CPUPercent < 5.0 {
			issues = append(issues, Issue{
				Type:        IssueStale,
				Description: fmt.Sprintf("%s running for %s with ~0%% CPU", p.Tool, scanner.FormatUptime(p.Uptime)),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Likely forgotten — safe to kill",
			})
		}
	}

	return issues
}

// detectOrphans finds processes whose parent has died (PPID=1).
func detectOrphans(procs []scanner.ProcessInfo) []Issue {
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
			})
		}
	}

	return issues
}

// detectCPUHogs finds processes using >50% CPU for extended periods.
func detectCPUHogs(procs []scanner.ProcessInfo) []Issue {
	var issues []Issue
	const cpuThreshold = 50.0

	for _, p := range procs {
		if p.Category == scanner.CategoryProtected {
			continue
		}
		if p.CPUPercent > cpuThreshold && p.Uptime > 5*time.Minute {
			issues = append(issues, Issue{
				Type:        IssueCPUHog,
				Description: fmt.Sprintf("%s using %.0f%% CPU for %s", p.Tool, p.CPUPercent, scanner.FormatUptime(p.Uptime)),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Possible runaway process",
			})
		}
	}

	return issues
}

// detectMemoryBloat finds processes using excessive RAM.
// Dev servers (Vite, Next.js, etc.) get a higher threshold since they
// legitimately use more memory for hot reload, source maps, etc.
func detectMemoryBloat(procs []scanner.ProcessInfo) []Issue {
	var issues []Issue

	for _, p := range procs {
		if p.Category == scanner.CategoryProtected {
			continue
		}

		// Dev servers legitimately use more memory — use a higher threshold
		threshold := 500.0 // default for background processes
		if p.Category == scanner.CategoryDevServer {
			threshold = 1500.0 // 1.5 GB — Next.js/Webpack can easily use 800 MB+
		}

		if p.MemoryMB > threshold {
			issues = append(issues, Issue{
				Type:        IssueMemoryBloat,
				Description: fmt.Sprintf("%s using %.0f MB RAM", p.Tool, p.MemoryMB),
				Processes:   []scanner.ProcessInfo{p},
				TotalCPU:    p.CPUPercent,
				TotalMemMB:  p.MemoryMB,
				Suggestion:  "Possible memory leak",
			})
		}
	}

	return issues
}
