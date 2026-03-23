package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tarek-k-devs/devsweep/internal/cleaner"
	"github.com/tarek-k-devs/devsweep/internal/config"
	"github.com/tarek-k-devs/devsweep/internal/daemon"
	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
	"github.com/tarek-k-devs/devsweep/internal/store"
	"github.com/tarek-k-devs/devsweep/internal/ui"
)

var version = "0.1.0-dev"

type cleanAction struct {
	Issue rules.Issue           `json:"issue"`
	Keep  []scanner.ProcessInfo `json:"keep,omitempty"`
	Kill  []scanner.ProcessInfo `json:"kill,omitempty"`
}

type cleanOutput struct {
	DryRun     bool          `json:"dryRun"`
	Auto       bool          `json:"auto"`
	Issues     []rules.Issue `json:"issues"`
	Actions    []cleanAction `json:"actions"`
	Killed     int           `json:"killed"`
	CPUSaved   float64       `json:"cpuSaved"`
	MemSavedMB float64       `json:"memSavedMb"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	switch cmd {
	case "scan":
		cmdScan()
	case "detect":
		cmdDetect()
	case "clean":
		cmdClean()
	case "kill-port":
		cmdKillPort()
	case "watch":
		cmdWatch()
	case "report":
		cmdReport()
	case "blame":
		cmdBlame()
	case "version", "--version", "-v":
		fmt.Printf("devsweep %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	ui.PrintBanner()
	fmt.Println("  Smart process cleanup for developers")
	fmt.Println()
	fmt.Println("  Usage: devsweep <command> [flags]")
	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    scan                    Show active dev processes (generic runtimes hidden by default)")
	fmt.Println("    scan --all              Include generic runtime processes")
	fmt.Println("    scan --cwd              Limit results to the current working tree")
	fmt.Println("    scan --json             Show scan results as JSON")
	fmt.Println("    detect                  Show problems only (duplicates, stale, orphans)")
	fmt.Println("    detect --cwd            Limit results to the current working tree")
	fmt.Println("    detect --json           Show detected issues as JSON")
	fmt.Println("    clean                   Interactive cleanup with confirmation")
	fmt.Println("    clean --cwd             Only clean processes from the current working tree")
	fmt.Println("    clean --auto            Auto-clean using rules (no prompts)")
	fmt.Println("    clean --dry-run         Show what would be killed, don't kill")
	fmt.Println("    clean --dry-run --json  Show cleanup plan as JSON")
	fmt.Println("    kill-port <port>        Kill everything on a specific port")
	fmt.Println("    watch start             Start background daemon")
	fmt.Println("    watch stop              Stop background daemon")
	fmt.Println("    watch status            Show daemon status")
	fmt.Println("    watch install           Auto-start on login (launchd)")
	fmt.Println("    watch uninstall         Remove auto-start")
	fmt.Println("    report                  Trends, stats & orphan lineage")
	fmt.Println("    report --export <tool>  Generate Markdown bug report for a tool")
	fmt.Println("    blame                   Show which tools leave the most process clutter")
	fmt.Println("    blame <tool>            Show blame details for one tool")
	fmt.Println("    blame --json            Show blame stats as JSON")
	fmt.Println("    version                 Show version")
	fmt.Println()
}

func cmdScan() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	procs, err := scanner.ScanAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning processes: %v\n", err)
		os.Exit(1)
	}
	procs = maybeFilterByCWD(procs)

	if hasFlag("--json") {
		printJSON(procs)
		return
	}

	issues := rules.DetectAllWithConfig(procs, cfg)
	flagged := make(map[int32]rules.IssueType)
	for _, issue := range issues {
		for _, p := range issue.Processes {
			if _, exists := flagged[p.PID]; !exists {
				flagged[p.PID] = issue.Type
			}
		}
	}

	filtered, hiddenRuntimes := scanner.FilterForScanDisplay(procs, issuePIDSet(flagged), hasFlag("--all"))

	ui.PrintBanner()
	ui.PrintScanResults(filtered, flagged, hiddenRuntimes, hasFlag("--all"))
}

func cmdDetect() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	procs, err := scanner.ScanAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning processes: %v\n", err)
		os.Exit(1)
	}
	procs = maybeFilterByCWD(procs)
	issues := rules.DetectAllWithConfig(procs, cfg)

	if hasFlag("--json") {
		printJSON(issues)
		return
	}

	ui.PrintBanner()
	ui.PrintIssues(issues)
}

func cmdClean() {
	dryRun := hasFlag("--dry-run")
	auto := hasFlag("--auto")
	jsonOut := hasFlag("--json")

	if jsonOut && !dryRun && !auto {
		fmt.Fprintln(os.Stderr, "--json requires --dry-run or --auto for clean")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	db, _ := store.Open() // non-fatal if DB fails
	if db != nil {
		defer db.Close()
	}

	procs, err := scanner.ScanAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning processes: %v\n", err)
		os.Exit(1)
	}
	procs = maybeFilterByCWD(procs)

	issues := rules.DetectAllWithConfig(procs, cfg)
	result := cleanOutput{
		DryRun:  dryRun,
		Auto:    auto,
		Issues:  issues,
		Actions: make([]cleanAction, 0),
	}

	if len(issues) == 0 {
		if jsonOut {
			printJSON(result)
			return
		}
		ui.PrintBanner()
		fmt.Println("✅ No issues detected. All clean!")
		return
	}

	var totalKilled int
	var totalCPU, totalMem float64
	reader := bufio.NewReader(os.Stdin)

	if !jsonOut {
		ui.PrintBanner()
		ui.PrintIssues(issues)
		if dryRun {
			fmt.Println("  (dry-run mode — no processes were killed)")
		}
	}

	for _, issue := range issues {
		keepTargets, killTargets := cleaner.ResolveKillTargets(issue)

		var safeTargets []scanner.ProcessInfo
		for _, t := range killTargets {
			if !cleaner.IsProtected(t, cfg) {
				safeTargets = append(safeTargets, t)
			}
		}

		action := cleanAction{Issue: issue, Keep: keepTargets, Kill: safeTargets}
		result.Actions = append(result.Actions, action)

		if len(safeTargets) == 0 || dryRun {
			continue
		}

		if !auto {
			fmt.Printf("  Kill %d processes? [Y/n/skip] ", len(safeTargets))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input == "n" || input == "skip" || input == "s" {
				continue
			}
		}

		for _, target := range safeTargets {
			if err := cleaner.KillProcess(target.PID, false); err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to kill PID %d: %v\n", target.PID, err)
			} else {
				totalKilled++
				totalCPU += target.CPUPercent
				totalMem += target.MemoryMB
				if db != nil {
					_ = db.RecordKill(target, string(issue.Type))
				}
			}
		}
	}

	result.Killed = totalKilled
	result.CPUSaved = totalCPU
	result.MemSavedMB = totalMem

	if jsonOut {
		printJSON(result)
		return
	}

	if totalKilled > 0 {
		ui.PrintCleanResult(totalKilled, totalCPU, totalMem)
	}
}

func cmdKillPort() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: devsweep kill-port <port>")
		os.Exit(1)
	}

	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid port: %s\n", os.Args[2])
		os.Exit(1)
	}

	ui.PrintBanner()
	fmt.Printf("  Scanning for processes on port %d...\n\n", port)

	procs, err := scanner.ScanAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning processes: %v\n", err)
		os.Exit(1)
	}

	matches := scanner.FindProcessesByPort(procs, port)
	if len(matches) == 0 {
		fmt.Printf("  No dev processes found on port %d\n", port)
		return
	}

	dryRun := hasFlag("--dry-run")
	var killed int

	for _, p := range matches {
		fmt.Printf("  Killing %s (PID %d) on :%d\n", p.Tool, p.PID, port)
		if err := cleaner.KillProcess(p.PID, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed: %v\n", err)
		} else {
			killed++
		}
	}

	if dryRun {
		fmt.Printf("\n  (dry-run — would kill %d processes)\n", len(matches))
	} else {
		fmt.Printf("\n  ✅ Killed %d processes on port %d\n", killed, port)
	}
}

func cmdWatch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: devsweep watch <start|stop|status|install|uninstall>")
		os.Exit(1)
	}

	subcmd := os.Args[2]
	switch subcmd {
	case "start":
		cmdWatchStart()
	case "run":
		cmdWatchRun()
	case "stop":
		cmdWatchStop()
	case "status":
		cmdWatchStatus()
	case "install":
		cmdWatchInstall()
	case "uninstall":
		cmdWatchUninstall()
	default:
		fmt.Fprintf(os.Stderr, "Unknown watch subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func cmdWatchStart() {
	running, pid := daemon.IsRunning()
	if running {
		fmt.Printf("  Daemon already running (PID %d)\n", pid)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".devsweep")
	_ = os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(filepath.Join(logDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "watch", "run")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	_ = cmd.Process.Release()

	fmt.Printf("  🧹 DevSweep daemon started (PID %d)\n", cmd.Process.Pid)
	fmt.Println("  Monitoring dev processes every 30s...")
	fmt.Println("  Stop with: devsweep watch stop")
}

func cmdWatchRun() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := store.Open()
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := daemon.WritePID(); err != nil {
		log.Fatalf("Failed to write PID file: %v", err)
	}
	defer daemon.RemovePID()

	w := daemon.NewWatcher(db, cfg)
	if err := w.Run(); err != nil {
		log.Fatalf("Daemon error: %v", err)
	}
}

func cmdWatchStop() {
	if err := daemon.StopDaemon(); err != nil {
		fmt.Fprintf(os.Stderr, "  %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  🧹 DevSweep daemon stopped")
}

func cmdWatchStatus() {
	ui.PrintBanner()
	running, pid := daemon.IsRunning()
	if running {
		fmt.Printf("  ✅ Daemon is running (PID %d)\n", pid)
	} else {
		fmt.Println("  ⏹  Daemon is not running")
		fmt.Println("  Start with: devsweep watch start")
	}
}

func cmdWatchInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := daemon.InstallLaunchAgent(exe); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to install: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✅ LaunchAgent installed — DevSweep will start on login")
	fmt.Println("  Remove with: devsweep watch uninstall")
}

func cmdWatchUninstall() {
	if err := daemon.UninstallLaunchAgent(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to uninstall: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✅ LaunchAgent removed")
}

func cmdReport() {
	ui.PrintBanner()

	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	days := 7
	since := time.Now().AddDate(0, 0, -days)

	kills, err := db.KillStats(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading kill stats: %v\n", err)
		os.Exit(1)
	}

	orphans, err := db.OrphansByParent(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading orphan stats: %v\n", err)
		os.Exit(1)
	}

	portConflicts, _ := db.PortConflictCount(since)

	exportTool := getFlagValue("--export")
	if exportTool != "" {
		md := ui.GenerateExport(exportTool, orphans, kills, days)
		fmt.Println(md)
		return
	}

	ui.PrintReport(kills, orphans, portConflicts, days)
}

func cmdBlame() {
	db, err := store.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	days := getIntFlagValue("--days", 7)
	since := time.Now().AddDate(0, 0, -days)

	kills, err := db.KillStats(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading kill stats: %v\n", err)
		os.Exit(1)
	}

	orphans, err := db.OrphansByParent(since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading orphan stats: %v\n", err)
		os.Exit(1)
	}

	stats := store.MergeBlameStats(orphans, kills)
	filter := positionalArgAfterCommand()
	if filter != "" {
		stats = filterBlameStats(stats, filter)
	}

	if hasFlag("--json") {
		printJSON(stats)
		return
	}

	ui.PrintBanner()
	ui.PrintBlame(stats, days, filter)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

func getFlagValue(flag string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func getIntFlagValue(flag string, fallback int) int {
	value := getFlagValue(flag)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func positionalArgAfterCommand() string {
	if len(os.Args) < 3 {
		return ""
	}
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--") {
			if i+1 < len(os.Args) && !strings.HasPrefix(os.Args[i+1], "--") {
				i++
			}
			continue
		}
		return arg
	}
	return ""
}

func issuePIDSet(flagged map[int32]rules.IssueType) map[int32]bool {
	set := make(map[int32]bool, len(flagged))
	for pid := range flagged {
		set[pid] = true
	}
	return set
}

func maybeFilterByCWD(procs []scanner.ProcessInfo) []scanner.ProcessInfo {
	if !hasFlag("--cwd") {
		return procs
	}
	cwd, err := os.Getwd()
	if err != nil {
		return procs
	}
	return scanner.FilterByWorkingDir(procs, cwd)
}

func filterBlameStats(stats []store.BlameStat, filter string) []store.BlameStat {
	if filter == "" {
		return stats
	}
	filter = strings.ToLower(filter)
	var filtered []store.BlameStat
	for _, stat := range stats {
		if strings.Contains(strings.ToLower(stat.Tool), filter) {
			filtered = append(filtered, stat)
		}
	}
	return filtered
}
