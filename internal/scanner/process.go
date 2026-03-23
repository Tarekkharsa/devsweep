package scanner

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Category represents the type of dev process.
type Category string

const (
	CategoryDevServer      Category = "Dev Server"
	CategoryAIAgent        Category = "AI Agent"
	CategoryPackageManager Category = "Package Manager"
	CategoryRuntime        Category = "Runtime"
	CategoryProtected      Category = "Protected"
	CategoryUnknown        Category = "Unknown"
)

// ProcessInfo holds enriched information about a running process.
type ProcessInfo struct {
	PID        int32
	PPID       int32
	Name       string
	Cmdline    string
	Category   Category
	Tool       string // e.g., "Vite", "Next.js", "mcp-remote"
	CPUPercent float64
	MemoryMB   float64
	Port       int
	Cwd        string
	StartTime  time.Time
	Uptime     time.Duration
	IsOrphan   bool
}

// ScanAll discovers all running processes and returns those relevant to developers.
func ScanAll() ([]ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	myPID := int32(os.Getpid())
	var results []ProcessInfo

	for _, p := range procs {
		if p.Pid == myPID || p.Pid == 0 {
			continue
		}

		info, relevant := inspectProcess(p)
		if relevant {
			results = append(results, info)
		}
	}

	return results, nil
}

func inspectProcess(p *process.Process) (ProcessInfo, bool) {
	name, err := p.Name()
	if err != nil {
		return ProcessInfo{}, false
	}

	cmdline, _ := p.Cmdline()
	if cmdline == "" {
		cmdline = name
	}

	// Categorize first — skip irrelevant processes early
	cat, tool := Categorize(name, cmdline)
	if cat == CategoryUnknown {
		return ProcessInfo{}, false
	}

	ppid, _ := p.Ppid()
	cpuPct, _ := p.CPUPercent()
	memInfo, _ := p.MemoryInfo()

	var memMB float64
	if memInfo != nil {
		memMB = float64(memInfo.RSS) / 1024 / 1024
	}

	cwd, _ := p.Cwd()

	createTime, _ := p.CreateTime() // milliseconds since epoch
	startTime := time.UnixMilli(createTime)
	uptime := time.Since(startTime)

	// Detect orphans: parent PID is 1 (init/launchd) but process is a dev tool
	isOrphan := false
	if runtime.GOOS == "darwin" {
		isOrphan = ppid == 1 && cat != CategoryProtected
	} else {
		isOrphan = ppid == 1 && cat != CategoryProtected
	}

	port := resolvePort(p)

	info := ProcessInfo{
		PID:        p.Pid,
		PPID:       ppid,
		Name:       name,
		Cmdline:    cmdline,
		Category:   cat,
		Tool:       tool,
		CPUPercent: cpuPct,
		MemoryMB:   memMB,
		Port:       port,
		Cwd:        cwd,
		StartTime:  startTime,
		Uptime:     uptime,
		IsOrphan:   isOrphan,
	}

	return info, true
}

// FormatUptime returns a human-readable uptime string.
func FormatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, h)
}

// GroupByCategory groups processes by their category.
func GroupByCategory(procs []ProcessInfo) map[Category][]ProcessInfo {
	groups := make(map[Category][]ProcessInfo)
	for _, p := range procs {
		groups[p.Category] = append(groups[p.Category], p)
	}
	return groups
}

// TruncateCmdline shortens a command line for display.
func TruncateCmdline(cmd string, maxLen int) string {
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}
