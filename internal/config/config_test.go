package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeFromFileSupportsExplicitFalseValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devsweep.yml")

	content := []byte(`watch:
  notify: false
  auto_clean: false
report:
  retention_days: 14
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Watch.Notify = true
	cfg.Watch.AutoClean = true

	if err := mergeFromFile(cfg, path); err != nil {
		t.Fatalf("merge config: %v", err)
	}

	if cfg.Watch.Notify {
		t.Fatalf("expected notify=false after merge")
	}
	if cfg.Watch.AutoClean {
		t.Fatalf("expected auto_clean=false after merge")
	}
	if cfg.Report.RetentionDays != 14 {
		t.Fatalf("expected retention_days=14, got %d", cfg.Report.RetentionDays)
	}
}
