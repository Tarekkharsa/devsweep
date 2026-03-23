package store

import (
	"time"

	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

// RecordSnapshot saves a batch of scanned processes.
func (db *DB) RecordSnapshot(procs []scanner.ProcessInfo, at time.Time) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO snapshots (pid, ppid, name, cmdline, category, tool, cpu_pct, memory_mb, port, cwd, is_orphan, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range procs {
		_, err := stmt.Exec(
			p.PID, p.PPID, p.Name, p.Cmdline,
			string(p.Category), p.Tool,
			p.CPUPercent, p.MemoryMB, p.Port, p.Cwd,
			p.IsOrphan, at,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RecordKill logs a process that was killed.
func (db *DB) RecordKill(p scanner.ProcessInfo, reason string) error {
	_, err := db.conn.Exec(`
		INSERT INTO kills (pid, tool, category, cpu_pct, memory_mb, reason, killed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, p.PID, p.Tool, string(p.Category), p.CPUPercent, p.MemoryMB, reason, time.Now())
	return err
}
