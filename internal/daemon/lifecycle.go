package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile manages the daemon PID file at ~/.devsweep/daemon.pid
func pidFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".devsweep", "daemon.pid"), nil
}

// WritePID writes the current process PID to the PID file.
func WritePID() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// ReadPID reads the daemon PID from the PID file. Returns 0 if not found.
func ReadPID() (int, error) {
	path, err := pidFilePath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// RemovePID removes the PID file.
func RemovePID() error {
	path, err := pidFilePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// IsRunning checks if the daemon is currently running.
func IsRunning() (bool, int) {
	pid, err := ReadPID()
	if err != nil || pid == 0 {
		return false, 0
	}

	// Check if process is actually alive
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// Signal 0 checks if process exists without actually sending a signal
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		// Process is dead, clean up stale PID file
		RemovePID()
		return false, 0
	}

	return true, pid
}

// StopDaemon sends SIGTERM to the running daemon.
func StopDaemon() error {
	running, pid := IsRunning()
	if !running {
		return fmt.Errorf("daemon is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop daemon (PID %d): %w", pid, err)
	}

	RemovePID()
	return nil
}

// InstallLaunchAgent creates a macOS LaunchAgent plist for auto-start.
func InstallLaunchAgent(binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return err
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>dev.devsweep.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>watch</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>StandardOutPath</key>
	<string>%s/.devsweep/daemon.log</string>
	<key>StandardErrorPath</key>
	<string>%s/.devsweep/daemon.log</string>
</dict>
</plist>`, binaryPath, home, home)

	plistPath := filepath.Join(plistDir, "dev.devsweep.daemon.plist")
	return os.WriteFile(plistPath, []byte(plist), 0644)
}

// UninstallLaunchAgent removes the macOS LaunchAgent plist.
func UninstallLaunchAgent() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "dev.devsweep.daemon.plist")
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
