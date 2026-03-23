package scanner

import (
	"path/filepath"
	"strings"
)

// categoryRule defines matching patterns for a process category.
type categoryRule struct {
	category Category
	tool     string
	// matchBinary checks the process binary name
	binaries []string
	// matchCmdline checks substrings in the full command line
	cmdPatterns []string
}

// rules are ordered by priority — first match wins.
// More specific rules come before generic ones.
var rules = []categoryRule{
	// === Protected (highest priority — never touch) ===
	{CategoryProtected, "System", []string{"ninjarmm", "jamf", "launchd", "kernel_task", "WindowServer"}, nil},

	// === Dev Servers ===
	{CategoryDevServer, "Vite", []string{"vite"}, []string{"vite dev", "vite serve", "vite preview", "vite/bin/vite"}},
	{CategoryDevServer, "Next.js", []string{"next-server"}, []string{"next dev", "next start", "next-server", ".next/server"}},
	{CategoryDevServer, "Webpack", []string{"webpack-dev-server"}, []string{"webpack serve", "webpack-dev-server"}},
	{CategoryDevServer, "Remix", []string{"remix-serve"}, []string{"remix dev"}},
	{CategoryDevServer, "Nuxt", []string{"nuxt"}, []string{"nuxt dev"}},
	{CategoryDevServer, "Astro", []string{"astro"}, []string{"astro dev"}},
	{CategoryDevServer, "Gatsby", []string{"gatsby"}, []string{"gatsby develop"}},
	{CategoryDevServer, "Turbopack", []string{"turbopack"}, []string{"turbopack", "turbo dev"}},

	// === AI Agents / MCP ===
	{CategoryAIAgent, "MCP Server", []string{"mcp-remote", "mcp-server"}, []string{"mcp-remote", "mcp-server", "modelcontextprotocol"}},
	{CategoryAIAgent, "Codex", []string{"codex"}, []string{"codex"}},
	{CategoryAIAgent, "Claude", nil, []string{"claude", "pi-coding-agent"}},
	{CategoryAIAgent, "Copilot", nil, []string{"copilot"}},
	{CategoryAIAgent, "OpenCode", nil, []string{"opencode"}},
	{CategoryAIAgent, "Cursor", nil, []string{"cursor-agent"}},

	// === Package Managers ===
	{CategoryPackageManager, "npm", nil, []string{"npm install", "npm ci", "npm run"}},
	{CategoryPackageManager, "pnpm", nil, []string{"pnpm install", "pnpm run", "pnpm dev"}},
	{CategoryPackageManager, "yarn", nil, []string{"yarn install", "yarn run", "yarn dev"}},
	{CategoryPackageManager, "bun", nil, []string{"bun install", "bun run"}},

	// === Runtimes (lowest priority — fallback) ===
	{CategoryRuntime, "Node.js", []string{"node"}, nil},
	{CategoryRuntime, "Deno", []string{"deno"}, nil},
	{CategoryRuntime, "Bun", []string{"bun"}, nil},
	{CategoryRuntime, "Python", []string{"python", "python3"}, nil},
}

// Categorize identifies what a process actually is using layered matching.
// Returns the category and the specific tool name.
func Categorize(binaryName, cmdline string) (Category, string) {
	binaryName = strings.ToLower(filepath.Base(binaryName))
	cmdlineLower := strings.ToLower(cmdline)

	for _, rule := range rules {
		// Priority 1: Check binary name
		for _, bin := range rule.binaries {
			if strings.Contains(binaryName, strings.ToLower(bin)) {
				return rule.category, rule.tool
			}
		}

		// Priority 2: Check cmdline patterns
		for _, pattern := range rule.cmdPatterns {
			if strings.Contains(cmdlineLower, strings.ToLower(pattern)) {
				return rule.category, rule.tool
			}
		}
	}

	return CategoryUnknown, ""
}
