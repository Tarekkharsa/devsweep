package store

import "sort"

// BlameStat combines orphan and cleanup stats by tool.
type BlameStat struct {
	Tool         string  `json:"tool"`
	OrphanCount  int     `json:"orphanCount"`
	RAMWastedMB  float64 `json:"ramWastedMb"`
	KillCount    int     `json:"killCount"`
	ReclaimedMB  float64 `json:"reclaimedMb"`
	CPUReclaimed float64 `json:"cpuReclaimed"`
}

// MergeBlameStats combines orphan and kill statistics keyed by tool.
func MergeBlameStats(orphans []OrphanStat, kills []KillStat) []BlameStat {
	merged := map[string]*BlameStat{}

	for _, o := range orphans {
		tool := o.ParentTool
		if tool == "" {
			tool = "Unknown"
		}
		row := ensureBlameRow(merged, tool)
		row.OrphanCount += o.OrphanCount
		row.RAMWastedMB += o.TotalMemMB
	}

	for _, k := range kills {
		tool := k.Tool
		if tool == "" {
			tool = "Unknown"
		}
		row := ensureBlameRow(merged, tool)
		row.KillCount += k.Count
		row.ReclaimedMB += k.TotalMem
		row.CPUReclaimed += k.TotalCPU
	}

	rows := make([]BlameStat, 0, len(merged))
	for _, row := range merged {
		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].OrphanCount != rows[j].OrphanCount {
			return rows[i].OrphanCount > rows[j].OrphanCount
		}
		if rows[i].KillCount != rows[j].KillCount {
			return rows[i].KillCount > rows[j].KillCount
		}
		return rows[i].Tool < rows[j].Tool
	})

	return rows
}

func ensureBlameRow(rows map[string]*BlameStat, tool string) *BlameStat {
	if row, ok := rows[tool]; ok {
		return row
	}
	row := &BlameStat{Tool: tool}
	rows[tool] = row
	return row
}
