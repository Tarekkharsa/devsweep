package scanner

import "testing"

func TestFilterForScanDisplayHidesUnflaggedRuntimes(t *testing.T) {
	procs := []ProcessInfo{
		{PID: 1, Category: CategoryRuntime, Tool: "Node.js"},
		{PID: 2, Category: CategoryRuntime, Tool: "Node.js"},
		{PID: 3, Category: CategoryAIAgent, Tool: "OpenCode"},
	}

	visible, hidden := FilterForScanDisplay(procs, map[int32]bool{2: true}, false)
	if hidden != 1 {
		t.Fatalf("expected 1 hidden runtime, got %d", hidden)
	}
	if len(visible) != 2 {
		t.Fatalf("expected 2 visible processes, got %d", len(visible))
	}
	if visible[0].PID != 2 || visible[1].PID != 3 {
		t.Fatalf("unexpected visible processes: %+v", visible)
	}
}
