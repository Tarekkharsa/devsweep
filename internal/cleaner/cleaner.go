package cleaner

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	gprocess "github.com/shirou/gopsutil/v4/process"
	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

const (
	gracefulTimeout = 5 * time.Second
)

// CleanResult records what happened during cleanup.
type CleanResult struct {
	Killed     int
	CPUSaved   float64
	MemSavedMB float64
	Errors     []string
}

// IsProtected checks if a process is in the protected list.
func IsProtected(p scanner.ProcessInfo, cfg *config.Config) bool {
	if p.Category == scanner.CategoryProtected {
		return true
	}
	for _, prot := range cfg.Protected {
		if prot.Name != "" {
			name := strings.ToLower(p.Name)
			pattern := strings.ToLower(strings.TrimSuffix(prot.Name, "*"))
			if strings.HasPrefix(name, pattern) {
				return true
			}
		}
		if prot.Port > 0 && p.Port == prot.Port {
			return true
		}
	}
	return false
}

// KillProcess gracefully kills a process and any descendants it spawned.
func KillProcess(pid int32, dryRun bool) error {
	if dryRun {
		return nil
	}

	targets := processTree(pid)
	if len(targets) == 0 {
		targets = []int32{pid}
	}

	for _, target := range targets {
		_ = signalIfAlive(target, syscall.SIGTERM)
	}

	deadline := time.Now().Add(gracefulTimeout)
	for time.Now().Before(deadline) {
		if allExited(targets) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	var lastErr error
	for _, target := range targets {
		if err := signalIfAlive(target, syscall.SIGKILL); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func processTree(root int32) []int32 {
	procs, err := gprocess.Processes()
	if err != nil {
		return nil
	}

	children := make(map[int32][]int32)
	for _, p := range procs {
		ppid, err := p.Ppid()
		if err != nil {
			continue
		}
		children[ppid] = append(children[ppid], p.Pid)
	}

	visited := map[int32]bool{}
	var ordered []int32
	var walk func(int32)
	walk = func(pid int32) {
		if visited[pid] {
			return
		}
		visited[pid] = true
		for _, child := range children[pid] {
			walk(child)
		}
		ordered = append(ordered, pid)
	}
	walk(root)

	return ordered
}

func allExited(pids []int32) bool {
	for _, pid := range pids {
		if processAlive(pid) {
			return false
		}
	}
	return true
}

func processAlive(pid int32) bool {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func signalIfAlive(pid int32, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return nil
	}
	if err := proc.Signal(sig); err != nil {
		return nil
	}
	return nil
}

func keepCountFromSuggestion(s string) int {
	keep := 1
	if _, err := fmt.Sscanf(strings.ToLower(s), "keep newest %d", &keep); err == nil && keep > 0 {
		return keep
	}
	return 1
}

// ResolveKillTargets determines which processes to kill for an issue.
// For duplicates: keep the newest, kill the rest.
// For everything else: kill all flagged processes.
func ResolveKillTargets(issue rules.Issue) (keep []scanner.ProcessInfo, kill []scanner.ProcessInfo) {
	if issue.Type == rules.IssueDuplicate && len(issue.Processes) > 1 {
		sorted := make([]scanner.ProcessInfo, len(issue.Processes))
		copy(sorted, issue.Processes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].StartTime.After(sorted[j].StartTime)
		})

		keepCount := keepCountFromSuggestion(issue.Suggestion)
		if keepCount >= len(sorted) {
			return sorted, nil
		}
		return sorted[:keepCount], sorted[keepCount:]
	}

	return nil, issue.Processes
}
