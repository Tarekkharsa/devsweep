package rules

import (
	"testing"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

func TestDetectAllWithConfigHonorsMaxDuplicatesOverride(t *testing.T) {
	two := 2
	cfg := &config.Config{
		Rules: []config.RuleOverride{{Match: "vite", MaxDuplicates: &two}},
	}

	procs := []scanner.ProcessInfo{
		{PID: 1, Tool: "Vite", Category: scanner.CategoryDevServer, Port: 3000, StartTime: time.Now().Add(-3 * time.Minute)},
		{PID: 2, Tool: "Vite", Category: scanner.CategoryDevServer, Port: 3000, StartTime: time.Now().Add(-2 * time.Minute)},
	}

	issues := DetectAllWithConfig(procs, cfg)
	for _, issue := range issues {
		if issue.Type == IssueDuplicate {
			t.Fatalf("expected duplicate issue to be suppressed by max_duplicates=2")
		}
	}
}

func TestDetectDuplicateHelpersWithoutPorts(t *testing.T) {
	procs := []scanner.ProcessInfo{
		{PID: 10, Tool: "MCP Server", Name: "node", Cmdline: "node mcp-remote jira", Cwd: "/tmp/work", Category: scanner.CategoryAIAgent, StartTime: time.Now().Add(-4 * time.Minute)},
		{PID: 11, Tool: "MCP Server", Name: "node", Cmdline: "node   mcp-remote   jira", Cwd: "/tmp/work", Category: scanner.CategoryAIAgent, StartTime: time.Now().Add(-2 * time.Minute)},
	}

	issues := DetectAllWithConfig(procs, nil)
	for _, issue := range issues {
		if issue.Type == IssueDuplicate {
			if issue.Confidence != ConfidenceMedium {
				t.Fatalf("expected medium confidence for helper duplicates, got %s", issue.Confidence)
			}
			return
		}
	}
	t.Fatalf("expected duplicate helper issue to be detected")
}

func TestDetectMemoryBloatHonorsOverride(t *testing.T) {
	cfg := &config.Config{
		Rules: []config.RuleOverride{{Match: "next", Memory: "2GB"}},
	}

	procs := []scanner.ProcessInfo{{
		PID:      20,
		Tool:     "Next.js",
		Name:     "node",
		Cmdline:  "next dev",
		Category: scanner.CategoryDevServer,
		MemoryMB: 1600,
	}}

	issues := DetectAllWithConfig(procs, cfg)
	for _, issue := range issues {
		if issue.Type == IssueMemoryBloat {
			t.Fatalf("expected memory bloat issue to be suppressed by 2GB override")
		}
	}
}
