package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection for DevSweep history.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the DevSweep database.
func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find home directory: %w", err)
	}

	dir := filepath.Join(home, ".devsweep")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create data directory: %w", err)
	}

	dbPath := filepath.Join(dir, "history.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	db := &DB{conn: conn, path: dbPath}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS snapshots (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		pid        INTEGER NOT NULL,
		ppid       INTEGER NOT NULL,
		name       TEXT NOT NULL,
		cmdline    TEXT,
		category   TEXT,
		tool       TEXT,
		cpu_pct    REAL,
		memory_mb  REAL,
		port       INTEGER,
		cwd        TEXT,
		is_orphan  BOOLEAN,
		scanned_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS lineage (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		child_pid   INTEGER NOT NULL,
		child_name  TEXT,
		child_tool  TEXT,
		parent_pid  INTEGER NOT NULL,
		parent_name TEXT,
		parent_tool TEXT,
		first_seen  DATETIME NOT NULL,
		last_seen   DATETIME NOT NULL,
		orphaned    BOOLEAN DEFAULT 0,
		orphaned_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS kills (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		pid       INTEGER NOT NULL,
		tool      TEXT,
		category  TEXT,
		cpu_pct   REAL,
		memory_mb REAL,
		reason    TEXT,
		killed_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_snapshots_scanned ON snapshots(scanned_at);
	CREATE INDEX IF NOT EXISTS idx_lineage_child ON lineage(child_pid);
	CREATE INDEX IF NOT EXISTS idx_lineage_parent ON lineage(parent_pid);
	CREATE INDEX IF NOT EXISTS idx_kills_time ON kills(killed_at);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Purge removes data older than the given retention period.
func (db *DB) Purge(retention time.Duration) error {
	cutoff := time.Now().Add(-retention)
	for _, table := range []string{"snapshots", "lineage", "kills"} {
		var col string
		switch table {
		case "snapshots":
			col = "scanned_at"
		case "lineage":
			col = "last_seen"
		case "kills":
			col = "killed_at"
		}
		if _, err := db.conn.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE %s < ?", table, col), cutoff,
		); err != nil {
			return err
		}
	}
	return nil
}
