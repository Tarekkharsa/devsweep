package scanner

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// resolvePort attempts to find the LISTENING port for a process.
// Only returns ports where the process has a LISTEN socket (not outbound connections).
// Uses gopsutil first, falls back to lsof on macOS.
// Returns 0 if port cannot be determined (graceful degradation).
func resolvePort(p *process.Process) int {
	// Try gopsutil connections first
	conns, err := p.Connections()
	if err == nil {
		for _, c := range conns {
			if c.Status == "LISTEN" {
				return int(c.Laddr.Port)
			}
		}
	}

	// Fallback: try net.ConnectionsPid (sometimes works better)
	allConns, err := net.ConnectionsPid("tcp", p.Pid)
	if err == nil {
		for _, c := range allConns {
			if c.Status == "LISTEN" {
				return int(c.Laddr.Port)
			}
		}
	}

	// macOS fallback: lsof
	// Note: lsof -i and -p flags are OR'd, not AND'd, so we must
	// filter the output by PID ourselves to avoid false matches.
	if runtime.GOOS == "darwin" {
		return resolvePortLsof(p.Pid)
	}

	return 0
}

func resolvePortLsof(pid int32) int {
	out, err := exec.Command("lsof", "-i", "-P", "-n").Output()
	if err != nil {
		return 0
	}

	pidStr := strconv.Itoa(int(pid))

	for _, line := range strings.Split(string(out), "\n") {
		// Only match lines for OUR PID and LISTEN status
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// lsof columns: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME (STATUS)
		linePID := fields[1]
		status := fields[len(fields)-1]

		if linePID != pidStr || status != "(LISTEN)" {
			continue
		}

		// Extract port from address like *:3000 or 127.0.0.1:3000
		addr := fields[len(fields)-2]
		if idx := strings.LastIndex(addr, ":"); idx >= 0 {
			if port, err := strconv.Atoi(addr[idx+1:]); err == nil {
				return port
			}
		}
	}

	return 0
}

// FindProcessesByPort finds all processes listening on a specific port.
func FindProcessesByPort(procs []ProcessInfo, port int) []ProcessInfo {
	var matches []ProcessInfo
	for _, p := range procs {
		if p.Port == port {
			matches = append(matches, p)
		}
	}
	return matches
}
