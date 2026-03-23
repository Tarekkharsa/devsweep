package scanner

// FilterForScanDisplay removes generic runtime noise from the default scan view.
// Flagged runtimes are always retained so suspicious processes stay visible.
func FilterForScanDisplay(procs []ProcessInfo, flagged map[int32]bool, includeRuntimes bool) (visible []ProcessInfo, hiddenRuntimes int) {
	if includeRuntimes {
		return procs, 0
	}

	visible = make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		if p.Category == CategoryRuntime && !flagged[p.PID] {
			hiddenRuntimes++
			continue
		}
		visible = append(visible, p)
	}
	return visible, hiddenRuntimes
}
