package store

import "testing"

func TestMergeBlameStatsCombinesOrphansAndKills(t *testing.T) {
	orphans := []OrphanStat{{ParentTool: "OpenCode", OrphanCount: 3, TotalMemMB: 120}}
	kills := []KillStat{{Tool: "OpenCode", Count: 2, TotalMem: 80, TotalCPU: 25}}

	stats := MergeBlameStats(orphans, kills)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat row, got %d", len(stats))
	}
	if stats[0].Tool != "OpenCode" {
		t.Fatalf("expected tool OpenCode, got %s", stats[0].Tool)
	}
	if stats[0].OrphanCount != 3 || stats[0].KillCount != 2 {
		t.Fatalf("unexpected merged counts: %+v", stats[0])
	}
	if stats[0].RAMWastedMB != 120 || stats[0].ReclaimedMB != 80 {
		t.Fatalf("unexpected merged memory stats: %+v", stats[0])
	}
}
