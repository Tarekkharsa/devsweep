package scanner

import "testing"

func TestFilterByWorkingDirMatchesParentsChildrenAndCmdline(t *testing.T) {
	target := "/Users/test/work/repo/apps/web"
	procs := []ProcessInfo{
		{PID: 1, Cwd: "/Users/test/work/repo"},
		{PID: 2, Cwd: "/Users/test/work/repo/apps/web"},
		{PID: 3, Cwd: "/Users/test/work/repo/apps/web/subdir"},
		{PID: 4, Cwd: "/Users/test/other"},
		{PID: 5, Cmdline: "node /Users/test/work/repo/apps/web/script.js"},
	}

	filtered := FilterByWorkingDir(procs, target)
	if len(filtered) != 4 {
		t.Fatalf("expected 4 matching processes, got %d", len(filtered))
	}
	if filtered[0].PID != 1 || filtered[1].PID != 2 || filtered[2].PID != 3 || filtered[3].PID != 5 {
		t.Fatalf("unexpected filtered processes: %+v", filtered)
	}
}
