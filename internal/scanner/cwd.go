package scanner

import (
	"path/filepath"
	"strings"
)

// FilterByWorkingDir keeps processes that belong to the current working tree.
// A process matches when its cwd is the same directory, a parent directory,
// or a child directory of the target. If cwd is unavailable, cmdline is used
// as a best-effort fallback.
func FilterByWorkingDir(procs []ProcessInfo, target string) []ProcessInfo {
	target = normalizePath(target)
	if target == "" {
		return procs
	}

	filtered := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		procCwd := normalizePath(p.Cwd)
		if sameWorkingTree(target, procCwd) || cmdlineReferencesPath(p.Cmdline, target) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func sameWorkingTree(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return hasPathPrefix(a, b) || hasPathPrefix(b, a)
}

func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	prefix = strings.TrimSuffix(prefix, string(filepath.Separator)) + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}

func normalizePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	clean := filepath.Clean(path)
	abs, err := filepath.Abs(clean)
	if err == nil {
		clean = abs
	}
	return filepath.Clean(clean)
}

func cmdlineReferencesPath(cmdline, target string) bool {
	if target == "" || strings.TrimSpace(cmdline) == "" {
		return false
	}
	cmdline = strings.ReplaceAll(cmdline, "\\", "/")
	target = strings.ReplaceAll(target, "\\", "/")
	return strings.Contains(cmdline, target)
}
