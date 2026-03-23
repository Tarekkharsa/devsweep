package ui

import (
	"fmt"
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
func PrintScanResults(procs []scanner.ProcessInfo) {
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

		fmt.Println(headerStyle.Render(fmt.Sprintf("  %s %s (%d)", categoryIcon(cat), string(cat), len(group))))
		fmt.Println()

		for _, p := range group {
			portStr := ""
			if p.Port > 0 {
				portStr = fmt.Sprintf(":%d", p.Port)
			}

			orphanTag := ""
			if p.IsOrphan {
				orphanTag = errorStyle.Render(" [ORPHAN]")
			}

			fmt.Printf("    %s %-20s PID %-7d CPU %5.1f%%  MEM %6.0f MB  Up %s%s%s\n",
				dimStyle.Render("│"),
				boldStyle.Render(p.Tool),
				p.PID,
				p.CPUPercent,
				p.MemoryMB,
				scanner.FormatUptime(p.Uptime),
				portStr,
				orphanTag,
			)

			cmdPreview := scanner.TruncateCmdline(p.Cmdline, 60)
			fmt.Printf("    %s %s\n", dimStyle.Render("│"), dimStyle.Render(cmdPreview))
		}
		fmt.Println()
	}

	fmt.Printf("  %s %d dev processes found\n\n", dimStyle.Render("Total:"), len(procs))
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
