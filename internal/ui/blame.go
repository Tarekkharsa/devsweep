package ui

import (
	"fmt"

	"github.com/tarek-k-devs/devsweep/internal/store"
)

// PrintBlame displays which tools are leaving the most cleanup behind.
func PrintBlame(stats []store.BlameStat, days int, filter string) {
	title := fmt.Sprintf("  🧾 DevSweep Blame (last %d days)", days)
	if filter != "" {
		title = fmt.Sprintf("%s — %s", title, filter)
	}
	fmt.Println(title)
	fmt.Println()

	if len(stats) == 0 {
		fmt.Println("  No blame data recorded yet. Run `devsweep watch start` or `devsweep clean` first.")
		fmt.Println()
		return
	}

	fmt.Println("  ┌──────────────────────┬───────────┬──────────────┬──────────────┬──────────────┐")
	fmt.Println("  │ Tool                 │ Orphans   │ RAM Wasted   │ Kills        │ RAM Reclaimed│")
	fmt.Println("  ├──────────────────────┼───────────┼──────────────┼──────────────┼──────────────┤")
	for _, s := range stats {
		name := s.Tool
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		fmt.Printf("  │ %-20s │ %-9d │ %8.0f MB   │ %-12d │ %8.0f MB   │\n",
			name,
			s.OrphanCount,
			s.RAMWastedMB,
			s.KillCount,
			s.ReclaimedMB,
		)
	}
	fmt.Println("  └──────────────────────┴───────────┴──────────────┴──────────────┴──────────────┘")
	fmt.Println()
}
