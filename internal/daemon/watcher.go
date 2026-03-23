package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/cleaner"
	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
	"github.com/tarek-k-devs/devsweep/internal/store"
)

// Watcher runs the background monitoring loop.
type Watcher struct {
	db        *store.DB
	cfg       *config.Config
	interval  time.Duration
	notify    bool
	autoClean bool
	retention time.Duration
	stopCh    chan struct{}
}

// NewWatcher creates a new background watcher.
func NewWatcher(db *store.DB, cfg *config.Config) *Watcher {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	interval := cfg.Watch.Interval
	if interval <= 0 {
		interval = 30
	}

	retentionDays := cfg.Report.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	return &Watcher{
		db:        db,
		cfg:       cfg,
		interval:  time.Duration(interval) * time.Second,
		notify:    cfg.Watch.Notify,
		autoClean: cfg.Watch.AutoClean,
		retention: time.Duration(retentionDays) * 24 * time.Hour,
		stopCh:    make(chan struct{}),
	}
}

// Run starts the polling loop. Blocks until stopped.
func (w *Watcher) Run() error {
	log.Printf("DevSweep daemon started (interval: %s, notifications: %v, auto-clean: %v)", w.interval, w.notify, w.autoClean)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	w.scan()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.scan()
		case <-sigCh:
			log.Println("DevSweep daemon stopping (received signal)")
			return nil
		case <-w.stopCh:
			log.Println("DevSweep daemon stopping")
			return nil
		}
	}
}

// Stop signals the watcher to stop.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) scan() {
	now := time.Now()

	procs, err := scanner.ScanAll()
	if err != nil {
		log.Printf("Scan error: %v", err)
		return
	}

	if err := w.db.RecordSnapshot(procs, now); err != nil {
		log.Printf("Failed to record snapshot: %v", err)
	}
	if err := w.db.UpdateLineage(procs, now); err != nil {
		log.Printf("Failed to update lineage: %v", err)
	}

	issues := rules.DetectAllWithConfig(procs, w.cfg)
	if len(issues) > 0 && w.notify {
		w.sendNotification(issues)
	}
	if len(issues) > 0 && w.autoClean {
		w.autoCleanIssues(issues)
	}

	if err := w.db.Purge(w.retention); err != nil {
		log.Printf("Failed to purge old data: %v", err)
	}

	log.Printf("Scan complete: %d processes, %d issues", len(procs), len(issues))
}

func (w *Watcher) autoCleanIssues(issues []rules.Issue) {
	killed := map[int32]bool{}
	for _, issue := range issues {
		if issue.Confidence != rules.ConfidenceHigh {
			continue
		}

		_, targets := cleaner.ResolveKillTargets(issue)
		for _, target := range targets {
			if killed[target.PID] || cleaner.IsProtected(target, w.cfg) {
				continue
			}
			if err := cleaner.KillProcess(target.PID, false); err != nil {
				log.Printf("Auto-clean failed for PID %d: %v", target.PID, err)
				continue
			}
			killed[target.PID] = true
			if err := w.db.RecordKill(target, string(issue.Type)); err != nil {
				log.Printf("Failed to record auto-clean kill for PID %d: %v", target.PID, err)
			}
		}
	}

	if len(killed) > 0 {
		log.Printf("Auto-cleaned %d high-confidence processes", len(killed))
	}
}

func (w *Watcher) sendNotification(issues []rules.Issue) {
	var totalMem float64
	var highConfidence int
	for _, issue := range issues {
		totalMem += issue.TotalMemMB
		if issue.Confidence == rules.ConfidenceHigh {
			highConfidence++
		}
	}

	title := "DevSweep"
	msg := fmt.Sprintf("%d issues detected (%d high-confidence) — %.0f MB reclaimable. Run `devsweep clean`", len(issues), highConfidence, totalMem)

	SendNotification(title, msg)
}
