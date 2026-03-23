package scanner

import "testing"

func TestBuiltInRuleValuesLoaded(t *testing.T) {
	defaults := BuiltInRuleValues()

	if defaults.MaxDuplicates != 1 {
		t.Fatalf("expected max duplicates 1, got %d", defaults.MaxDuplicates)
	}
	if defaults.CPUHogPercent != 50 {
		t.Fatalf("expected cpu threshold 50, got %.0f", defaults.CPUHogPercent)
	}
	if defaults.MemoryBloatMB != 500 {
		t.Fatalf("expected memory threshold 500MB, got %.0f", defaults.MemoryBloatMB)
	}
	if defaults.DevServerMemoryMB != 1500 {
		t.Fatalf("expected dev server memory threshold 1500MB, got %.0f", defaults.DevServerMemoryMB)
	}
	if !defaults.OrphanEnabled {
		t.Fatalf("expected orphan detection to be enabled")
	}
}
