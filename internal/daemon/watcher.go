package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
	"github.com/tarek-k-devs/devsweep/internal/store"
)

// Watcher runs the background monitoring loop.
type Watcher struct {
	db       *store.DB
	interval time.Duration
	notify   bool
	stopCh   chan struct{}
}

// NewWatcher creates a new background watcher.
func NewWatcher(db *store.DB, intervalSec int, notify bool) *Watcher {
	if intervalSec <= 0 {
		intervalSec = 30
	}
	return &Watcher{
		db:       db,
		interval: time.Duration(intervalSec) * time.Second,
		notify:   notify,
		stopCh:   make(chan struct{}),
	}
}

// Run starts the polling loop. Blocks until stopped.
func (w *Watcher) Run() error {
	log.Printf("DevSweep daemon started (interval: %s, notifications: %v)", w.interval, w.notify)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Run first scan immediately
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

	// Record snapshot and lineage
	if err := w.db.RecordSnapshot(procs, now); err != nil {
		log.Printf("Failed to record snapshot: %v", err)
	}
	if err := w.db.UpdateLineage(procs, now); err != nil {
		log.Printf("Failed to update lineage: %v", err)
	}

	// Detect issues
	issues := rules.DetectAll(procs)
	if len(issues) > 0 && w.notify {
		w.sendNotification(issues)
	}

	// Purge old data (30 days)
	if err := w.db.Purge(30 * 24 * time.Hour); err != nil {
		log.Printf("Failed to purge old data: %v", err)
	}

	log.Printf("Scan complete: %d processes, %d issues", len(procs), len(issues))
}

func (w *Watcher) sendNotification(issues []rules.Issue) {
	var totalMem float64
	for _, issue := range issues {
		totalMem += issue.TotalMemMB
	}

	title := "DevSweep"
	msg := fmt.Sprintf("%d issues detected — %.0f MB reclaimable. Run `devsweep clean`", len(issues), totalMem)

	SendNotification(title, msg)
}
