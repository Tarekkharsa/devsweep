package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tarek-k-devs/devsweep/internal/rules"
	"github.com/tarek-k-devs/devsweep/internal/scanner"
)

var (
	// Colors
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6B6B"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4ECDC4"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFE66D"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#95E06C"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	boldStyle    = lipgloss.NewStyle().Bold(true)
)

// PrintBanner prints the DevSweep header.
func PrintBanner() {
	fmt.Println(titleStyle.Render("🧹 DevSweep"))
	fmt.Println()
}

// PrintScanResults displays scanned processes grouped by category.
func PrintScanResults(procs []scanner.ProcessInfo, flagged map[int32]rules.IssueType, hiddenRuntimes int, includeRuntimes bool) {
	if len(procs) == 0 {
		fmt.Println(successStyle.Render("✅ No dev processes found running."))
		return
	}

	groups := scanner.GroupByCategory(procs)
	categoryOrder := []scanner.Category{
		scanner.CategoryDevServer,
		scanner.CategoryAIAgent,
		scanner.CategoryPackageManager,
		scanner.CategoryRuntime,
		scanner.CategoryProtected,
	}

	for _, cat := range categoryOrder {
		group, ok := groups[cat]
		if !ok {
			continue
		}

		sort.SliceStable(group, func(i, j int) bool {
			return scanPriority(group[i], flagged) > scanPriority(group[j], flagged)
		})

		fmt.Println(headerStyle.Render(fmt.Sprintf("  %s %s (%d)", categoryIcon(cat), string(cat), len(group))))
		fmt.Println()

		for _, p := range group {
			portStr := ""
			if p.Port > 0 {
				portStr = fmt.Sprintf(":%d", p.Port)
			}

			badges := ""
			if p.IsOrphan {
				badges += errorStyle.Render(" [ORPHAN]")
			}
			if issueType, ok := flagged[p.PID]; ok {
				badges += issueBadge(issueType)
			}

			fmt.Printf("    %s %-20s PID %-7d CPU %5.1f%%  MEM %6.0f MB  Up %s%s%s\n",
				dimStyle.Render("│"),
				boldStyle.Render(p.Tool),
				p.PID,
				p.CPUPercent,
				p.MemoryMB,
				scanner.FormatUptime(p.Uptime),
				portStr,
				badges,
			)

			cmdPreview := scanner.TruncateCmdline(p.Cmdline, 60)
			fmt.Printf("    %s %s\n", dimStyle.Render("│"), dimStyle.Render(cmdPreview))
		}
		fmt.Println()
	}

	fmt.Printf("  %s %d dev processes shown\n", dimStyle.Render("Total:"), len(procs))
	if hiddenRuntimes > 0 && !includeRuntimes {
		fmt.Printf("  %s %d generic runtime processes hidden (use %s to show them)\n", dimStyle.Render("Note:"), hiddenRuntimes, boldStyle.Render("scan --all"))
	}
	fmt.Println()
}

// PrintIssues displays detected problems.
func PrintIssues(issues []rules.Issue) {
	if len(issues) == 0 {
		fmt.Println(successStyle.Render("✅ No issues detected. All clean!"))
		return
	}

	fmt.Print(warnStyle.Render(fmt.Sprintf("🔍 Found %d issues:\n\n", len(issues))))

	for i, issue := range issues {
		icon := issueIcon(issue.Type)
		fmt.Printf(" %s  %s %s\n", icon, boldStyle.Render(issue.Description), confidenceBadge(issue.Confidence))

		pids := make([]string, len(issue.Processes))
		for j, p := range issue.Processes {
			pids[j] = fmt.Sprintf("%d", p.PID)
		}
		fmt.Printf("    PIDs: %s  |  CPU: %.0f%%  |  RAM: %.1f MB\n",
			strings.Join(pids, ", "),
			issue.TotalCPU,
			issue.TotalMemMB,
		)
		fmt.Printf("    → %s\n", dimStyle.Render(issue.Suggestion))

		if i < len(issues)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

// PrintCleanPlan shows what DevSweep intends to keep and kill for an issue.
func PrintCleanPlan(issue rules.Issue, keep []scanner.ProcessInfo, kill []scanner.ProcessInfo, protectedSkipped int) {
	fmt.Printf("  %s %s\n", boldStyle.Render("Plan:"), issue.Description)
	if len(keep) > 0 {
		fmt.Printf("    keep: %s\n", describeProcesses(keep))
	}
	if len(kill) > 0 {
		fmt.Printf("    kill: %s\n", describeProcesses(kill))
	}
	if protectedSkipped > 0 {
		fmt.Printf("    skip: %s\n", dimStyle.Render(fmt.Sprintf("%d protected process(es)", protectedSkipped)))
	}
}

// PrintCleanResult displays the cleanup summary.
func PrintCleanResult(result int, cpuSaved float64, memSavedMB float64) {
	fmt.Println()
	fmt.Printf(" %s Cleaned: %d processes | Freed: ~%.1f MB RAM | Saved: ~%.0f%% CPU\n",
		successStyle.Render("✅"),
		result,
		memSavedMB,
		cpuSaved,
	)
	fmt.Println()
}

func categoryIcon(cat scanner.Category) string {
	switch cat {
	case scanner.CategoryDevServer:
		return "🖥"
	case scanner.CategoryAIAgent:
		return "🤖"
	case scanner.CategoryPackageManager:
		return "📦"
	case scanner.CategoryRuntime:
		return "⚙️"
	case scanner.CategoryProtected:
		return "🛡"
	default:
		return "❓"
	}
}

func issueIcon(t rules.IssueType) string {
	switch t {
	case rules.IssueDuplicate:
		return warnStyle.Render("⚠")
	case rules.IssueStale:
		return warnStyle.Render("⏰")
	case rules.IssueOrphan:
		return errorStyle.Render("👻")
	case rules.IssueCPUHog:
		return errorStyle.Render("🔥")
	case rules.IssueMemoryBloat:
		return errorStyle.Render("💾")
	default:
		return "❓"
	}
}

func confidenceBadge(c rules.Confidence) string {
	switch c {
	case rules.ConfidenceHigh:
		return successStyle.Render("[high confidence]")
	case rules.ConfidenceMedium:
		return warnStyle.Render("[medium confidence]")
	case rules.ConfidenceLow:
		return dimStyle.Render("[low confidence]")
	default:
		return ""
	}
}

func issueBadge(t rules.IssueType) string {
	switch t {
	case rules.IssueDuplicate:
		return warnStyle.Render(" [DUPLICATE]")
	case rules.IssueStale:
		return warnStyle.Render(" [STALE]")
	case rules.IssueOrphan:
		return errorStyle.Render(" [ISSUE]")
	case rules.IssueCPUHog:
		return errorStyle.Render(" [CPU]")
	case rules.IssueMemoryBloat:
		return errorStyle.Render(" [MEM]")
	default:
		return warnStyle.Render(" [ISSUE]")
	}
}

func scanPriority(p scanner.ProcessInfo, flagged map[int32]rules.IssueType) float64 {
	score := p.CPUPercent + (p.MemoryMB / 100.0)
	if p.Port > 0 {
		score += 20
	}
	if p.IsOrphan {
		score += 1000
	}
	if _, ok := flagged[p.PID]; ok {
		score += 500
	}
	return score
}

func describeProcesses(procs []scanner.ProcessInfo) string {
	parts := make([]string, 0, len(procs))
	for _, p := range procs {
		parts = append(parts, fmt.Sprintf("%s (PID %d)", p.Tool, p.PID))
	}
	return strings.Join(parts, ", ")
}
