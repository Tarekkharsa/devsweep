package store

import (
	"time"

	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

// UpdateLineage records or updates parent-child relationships for all processes.
func (db *DB) UpdateLineage(procs []scanner.ProcessInfo, at time.Time) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build a PID→ProcessInfo lookup for this scan
	pidMap := make(map[int32]scanner.ProcessInfo)
	for _, p := range procs {
		pidMap[p.PID] = p
	}

	for _, child := range procs {
		if child.PPID <= 1 || child.Category == scanner.CategoryProtected {
			continue
		}

		parent, parentAlive := pidMap[child.PPID]
		parentName := ""
		parentTool := ""
		if parentAlive {
			parentName = parent.Name
			parentTool = parent.Tool
		}

		// Check if this relationship already exists
		var id int64
		err := tx.QueryRow(`
			SELECT id FROM lineage WHERE child_pid = ? AND parent_pid = ?
		`, child.PID, child.PPID).Scan(&id)

		if err != nil {
			// New relationship
			_, err = tx.Exec(`
				INSERT INTO lineage (child_pid, child_name, child_tool, parent_pid, parent_name, parent_tool, first_seen, last_seen)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, child.PID, child.Name, child.Tool, child.PPID, parentName, parentTool, at, at)
			if err != nil {
				return err
			}
		} else {
			// Update last_seen
			_, err = tx.Exec(`UPDATE lineage SET last_seen = ? WHERE id = ?`, at, id)
			if err != nil {
				return err
			}
		}
	}

	// Mark orphans: children whose parents disappeared since last scan
	for _, child := range procs {
		if child.IsOrphan {
			_, err := tx.Exec(`
				UPDATE lineage SET orphaned = 1, orphaned_at = ?
				WHERE child_pid = ? AND orphaned = 0
			`, at, child.PID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// OrphansByParent returns orphan counts grouped by parent tool for the given period.
type OrphanStat struct {
	ParentTool  string
	OrphanCount int
	TotalMemMB  float64
}

func (db *DB) OrphansByParent(since time.Time) ([]OrphanStat, error) {
	rows, err := db.conn.Query(`
		SELECT
			COALESCE(NULLIF(parent_tool, ''), parent_name, 'Unknown') as parent,
			COUNT(DISTINCT child_pid) as orphan_count,
			COALESCE(SUM(s.memory_mb), 0) as total_mem
		FROM lineage l
		LEFT JOIN snapshots s ON s.pid = l.child_pid AND s.scanned_at = (
			SELECT MAX(scanned_at) FROM snapshots WHERE pid = l.child_pid
		)
		WHERE l.orphaned = 1 AND l.orphaned_at >= ?
		GROUP BY parent
		ORDER BY orphan_count DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []OrphanStat
	for rows.Next() {
		var s OrphanStat
		if err := rows.Scan(&s.ParentTool, &s.OrphanCount, &s.TotalMemMB); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// KillStats returns aggregated kill statistics for the given period.
type KillStat struct {
	Tool      string
	Count     int
	TotalMem  float64
	TotalCPU  float64
}

func (db *DB) KillStats(since time.Time) ([]KillStat, error) {
	rows, err := db.conn.Query(`
		SELECT
			COALESCE(NULLIF(tool, ''), 'Unknown') as tool,
			COUNT(*) as count,
			SUM(memory_mb) as total_mem,
			SUM(cpu_pct) as total_cpu
		FROM kills
		WHERE killed_at >= ?
		GROUP BY tool
		ORDER BY count DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []KillStat
	for rows.Next() {
		var s KillStat
		if err := rows.Scan(&s.Tool, &s.Count, &s.TotalMem, &s.TotalCPU); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// PortConflictCount returns how many times duplicate processes were detected on the same port.
func (db *DB) PortConflictCount(since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM kills WHERE reason LIKE '%duplicate%' AND killed_at >= ?
	`, since).Scan(&count)
	return count, err
}
