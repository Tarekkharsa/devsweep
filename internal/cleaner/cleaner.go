package cleaner

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

const (
	gracefulTimeout = 5 * time.Second
)

// CleanResult records what happened during cleanup.
type CleanResult struct {
	Killed    int
	CPUSaved  float64
	MemSavedMB float64
	Errors    []string
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

// KillProcess gracefully kills a process (SIGTERM → wait → SIGKILL).
func KillProcess(pid int32, dryRun bool) error {
	if dryRun {
		return nil
	}

	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	// Try graceful shutdown first
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		return nil
	}

	// Wait for graceful shutdown
	done := make(chan struct{})
	go func() {
		// Poll to check if process is still alive
		for i := 0; i < 50; i++ {
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				close(done)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(gracefulTimeout):
		// Force kill
		return proc.Signal(syscall.SIGKILL)
	}
}

// ResolveKillTargets determines which processes to kill for an issue.
// For duplicates: keep the newest, kill the rest.
// For everything else: kill all flagged processes.
func ResolveKillTargets(issue rules.Issue) (keep []scanner.ProcessInfo, kill []scanner.ProcessInfo) {
	if issue.Type == rules.IssueDuplicate && len(issue.Processes) > 1 {
		// Sort by start time descending (newest first)
		sorted := make([]scanner.ProcessInfo, len(issue.Processes))
		copy(sorted, issue.Processes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].StartTime.After(sorted[j].StartTime)
		})
		return sorted[:1], sorted[1:]
	}

	return nil, issue.Processes
}
