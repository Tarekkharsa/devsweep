package cleaner

import (
	"testing"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

func TestResolveKillTargetsKeepsNewestDuplicate(t *testing.T) {
	oldest := scanner.ProcessInfo{PID: 1, StartTime: time.Now().Add(-10 * time.Minute)}
	newest := scanner.ProcessInfo{PID: 2, StartTime: time.Now().Add(-1 * time.Minute)}

	keep, kill := ResolveKillTargets(rules.Issue{
		Type:       rules.IssueDuplicate,
		Suggestion: "Keep newest 1, kill 1 others",
		Processes:  []scanner.ProcessInfo{oldest, newest},
	})

	if len(keep) != 1 || keep[0].PID != 2 {
		t.Fatalf("expected newest process to be kept, got %+v", keep)
	}
	if len(kill) != 1 || kill[0].PID != 1 {
		t.Fatalf("expected oldest process to be killed, got %+v", kill)
	}
}
