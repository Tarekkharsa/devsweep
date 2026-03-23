# 🧹 DevSweep — Smart Process Cleanup for Developers

A lightweight **CLI tool** (with an optional **macOS menu bar app** later) that detects, monitors, and cleans up runaway dev processes — specifically targeting the chaos that AI agents, dev servers, and Node.js tools leave behind.

## Why This is Unique

Tools like `htop` and Activity Monitor show _everything_ but understand _nothing_. An AI agent can kill a process when you ask, but only **after** you've noticed the problem. DevSweep is **developer-aware and always-on** — it knows what Vite, Next.js, webpack, MCP servers, and AI agents are, watches them continuously, learns your patterns over time, and cleans up before you even notice something's wrong.

**The moat:**

- **Continuous monitoring** — catches problems at minute 1, not minute 30
- **Trend tracking** — knows your MCP servers leak 50 MB/hour, that you've killed 47 orphans this week
- **Process lineage** — knows *which tool* left the mess (OpenCode left 23 orphans this week, Claude Code left 12)
- **Team-shared rules** — `.devsweep.yml` in your repo = ESLint for processes
- **Safety guarantees** — graceful shutdown, protected process lists, dry-run mode

---

## Architecture

```
Phase 1: CLI + Background Daemon (Go binary — small, fast, no runtime deps)
Phase 2: macOS Menu Bar app (Swift wrapper around CLI) + Terminal Dashboard
Phase 3: Ecosystem (VS Code/Zed extensions, plugin system)
```

**Why Go?** Tiny binary (~5 MB), no dependencies (no irony of a Node process killing Node processes), fast process scanning, easy cross-compilation for macOS + Linux.

### Distribution Strategy — Zero-Friction Entry

**Primary: `npx devsweep`** — try it instantly, no install required.

The npm package is a thin JS wrapper (~20 lines) that downloads the correct Go binary for the user's OS/arch on first run, then proxies all commands. This is the same proven pattern used by esbuild, Turbo, SWC, Tailwind CSS, and Prisma.

```bash
# Zero install. Works immediately.
npx devsweep scan
npx devsweep clean
npx devsweep detect
```

| Channel | Purpose | When |
|---|---|---|
| **`npx devsweep`** | Try it instantly, zero friction | Day one (MVP) |
| **`brew install devsweep`** | Permanent install (macOS) — needed for `devsweep watch` | At launch |
| **`apt` / GitHub releases** | Linux permanent install | At launch |
| **`go install`** | Go developers | Free (comes with Go module) |

> **Note:** `npx` is for one-shot commands (`scan`, `detect`, `clean`). For the background daemon (`devsweep watch`), users should permanently install via `brew` or `apt` so the binary persists across sessions.

### Cross-Platform Strategy

- Use **`gopsutil`** for all process discovery (abstracts OS-specific APIs)
- Runtime OS detection via `runtime.GOOS` for platform-specific behavior
- **Port detection with graceful degradation:**
  1. Try `gopsutil` `ConnectionsPid()` — works without root on Linux
  2. On macOS, fall back to `lsof -i -P -n` — shows current user's ports without root
  3. If neither works, show `port: unknown`, skip port-based rules — never require `sudo`

---

## Full Plan

### Phase 1 — CLI + Daemon (MVP) — _~3 weeks_

#### 1.1 Core Scanner (`devsweep scan`)

- Scan all running processes via `gopsutil` (cross-platform)
- Categorize each process using **layered cmdline matching** (see 1.2)
- Categories:
  - **Dev Servers**: Vite, Next.js, webpack-dev-server, Turbopack, Remix
  - **AI Agents**: MCP servers, Codex, Copilot, Claude, pi, OpenCode, Cursor
  - **Package Managers**: npm, pnpm, yarn (stuck installs)
  - **Runtimes**: Node, Deno, Bun, Python (fallback when no specific tool matched)
  - **System/IT**: NinjaRMM, Jamf, etc. (flag but don't touch)
- For each process, collect: PID, CPU%, MEM%, uptime, port (graceful degradation), working directory, parent PID, full cmdline

#### 1.2 Process Categorizer

A process named `node` could be Vite, Next.js, an MCP server, or anything. The categorizer uses **layered matching** to identify what a process actually is:

| Priority | Strategy | Example |
|---|---|---|
| 1. **Known binary name** | Process binary itself is recognizable | `next-server` → Next.js, `vite` → Vite |
| 2. **Cmdline pattern** | Inspect full command-line arguments | `node .../vite/bin/vite.js` → Vite |
| 3. **Keyword matching** | Match known keywords in cmdline | `mcp-remote`, `mcp-server` → MCP Server |
| 4. **Port heuristic** | Known default ports for tools | `:3000` + Node → likely dev server |
| 5. **Runtime fallback** | Categorize by runtime only | `node` with no match → "Node.js (unknown)" |

Matching patterns are defined in `rules/default.yml` and extensible via user config:

```yaml
# rules/default.yml (built-in, ships with binary)
categories:
  dev_servers:
    - binary: ["vite", "next-server", "webpack-dev-server", "remix-serve"]
    - cmdline: ["vite dev", "vite serve", "next dev", "webpack serve", "remix dev"]
  ai_agents:
    - binary: ["mcp-remote", "mcp-server", "codex"]
    - cmdline: ["mcp-remote", "mcp-server", "claude", "copilot", "opencode"]
  package_managers:
    - binary: ["npm", "pnpm", "yarn", "bun"]
    - cmdline: ["npm install", "pnpm install", "yarn install"]
  protected:
    - binary: ["ninjarmm*", "jamf*"]
```

This means the community can contribute new patterns via PRs to `rules/default.yml` without touching Go code.

#### 1.3 Smart Detection Rules (`devsweep detect`)

Six **hardcoded rule types** in Go, each with **configurable thresholds** via YAML:

| Rule                                                | Default Threshold              | Example                          |
| --------------------------------------------------- | ------------------------------ | -------------------------------- |
| **Duplicate dev servers** on same port               | `max_duplicates: 1`            | 3 Vite servers all on `:3000`    |
| **Stale dev servers** (running N+ hours with 0 net)  | `max_age: "24h"`               | Vite from 3 days ago             |
| **Orphaned child processes** (parent already dead)    | always flagged                 | Node workers with PPID=1         |
| **CPU hogs** (>N% for M+ min, not user-focused)      | `cpu: 50%, duration: "5m"`     | Runaway background process       |
| **Zombie agents** (MCP/LSP servers with no client)   | `max_duplicates: 1`            | 4 duplicate `mcp-remote`         |
| **Memory bloat** (>N MB for a background process)     | `memory: "500MB"`              | Electron app leak                |

**Rule engine design:** No generic DSL or expression engine — the six rule types are hardcoded in Go for reliability. The YAML only controls thresholds. User config overrides built-in defaults per-rule:

```yaml
# User override in ~/.devsweep.yml
rules:
  - match: "vite dev"
    max_duplicates: 1
    max_age: "12h"       # stricter than default 24h
  - match: "mcp-remote"
    max_duplicates: 1
    max_age: "4h"
  - match: "node"
    cpu: 80%             # more lenient for Node
    memory: "1GB"
```

#### 1.4 Interactive Cleanup (`devsweep clean`)

```
$ devsweep clean

🔍 Found 7 issues:

 ⚠  3× Vite dev server on :3000 (only 1 needed)
    PIDs: 98921, 50804, 51634  |  CPU: 156%  |  RAM: 2.1 GB
    → Keep newest (51634), kill 2 others?  [Y/n/skip]

 ⚠  4× mcp-remote (Atlassian) — duplicates
    PIDs: 9788, 7851, 5057, 92275  |  RAM: 132 MB
    → Keep 1, kill 3 others?  [Y/n/skip]

 ✅ Cleaned: 5 processes | Freed: ~2.3 GB RAM | Saved: ~180% CPU
```

**Safety guarantees:**
- Graceful shutdown: SIGTERM → configurable wait → SIGKILL (not blind `kill -9`)
- Process group awareness: kill the parent, not 15 children individually
- Protected process lists (system & user-defined)
- `--dry-run` mode for paranoid users

#### 1.5 Background Daemon (`devsweep watch`)

- Lightweight background process that polls every **30 seconds** (configurable)
  - 30s default balances catching short-lived parents for lineage tracking vs. overhead
  - The scan is cheap (~5ms via gopsutil), so 30s adds negligible load
- Sends **native OS notifications** when issues are detected:
  > _"3 stale Vite servers detected — 2.1 GB reclaimable. Run `devsweep clean`"_
- macOS: `osascript` notifications (or `terminal-notifier`)
- Linux: `notify-send` (freedesktop)
- Logs every scan snapshot to **local SQLite DB** for trend tracking
- **Records parent-child process relationships in real time** (see 1.6 Process Lineage)
- Optional auto-cleanup mode for known-safe rules

**Lifecycle management:**
- **PID file** at `~/.devsweep/daemon.pid` — simple, cross-platform
- `devsweep watch start` — forks to background, writes PID file
- `devsweep watch stop` — reads PID file, sends SIGTERM
- `devsweep watch status` — checks if PID is still alive, shows last scan time
- **Does not survive reboots by default** (no auto-installing system services)
- **Opt-in system integration** via `devsweep watch install`:
  - macOS: generates `~/Library/LaunchAgents/dev.devsweep.daemon.plist`
  - Linux: generates `~/.config/systemd/user/devsweep.service`
  - `devsweep watch uninstall` to remove

```bash
devsweep watch start       # Start background daemon
devsweep watch stop        # Stop it
devsweep watch status      # Is it running? Last scan time?
devsweep watch install     # Set up auto-start on login (launchd/systemd)
devsweep watch uninstall   # Remove auto-start
```

#### 1.6 Process Lineage Tracking

The daemon continuously records parent-child process relationships **while parents are still alive**. This solves the problem where orphaned processes (PPID=1) lose all trace of which tool spawned them.

**How it works:**
- Each scan, the daemon records `{PID, PPID, parent_name, cmdline, timestamp}` to SQLite
- When a parent (e.g., OpenCode PID 5000) spawns 6 MCP servers, the daemon sees the relationship
- When OpenCode exits, the MCP servers become orphans (PPID → 1) — but DevSweep already knows they belonged to OpenCode
- Lineage data is surfaced in `devsweep report` and exportable for bug reports

**Why this matters:** No other tool tells you *who left the mess*. This holds AI coding tools, editors, and dev servers accountable for their process hygiene.

#### 1.7 Trend Tracking & Reporting (`devsweep report`)

- Local **SQLite database** (`~/.devsweep/history.db`) stores process snapshots + lineage
- `devsweep report` shows:
  ```
  📊 DevSweep Report (last 7 days)

  Processes killed:       47 orphaned Node workers, 12 stale Vite servers
  RAM reclaimed:          18.3 GB total
  Port 3000 conflicts:    12 times
  Top offender:           mcp-remote (Atlassian) — 23 duplicates killed
  Memory trend:           MCP servers leak ~50 MB/hour on average

  🔗 Orphans by Parent Tool:
  ┌──────────────────┬─────────────┬───────────────┐
  │ Tool             │ Orphans Left│ RAM Wasted    │
  ├──────────────────┼─────────────┼───────────────┤
  │ OpenCode         │ 23          │ 1.8 GB        │
  │ Claude Code (pi) │ 12          │ 640 MB        │
  │ Cursor           │ 8           │ 320 MB        │
  └──────────────────┴─────────────┴───────────────┘

  💡 Tip: Add max_age: "4h" rule for mcp-remote to auto-clean sooner.
  ```
- Data retained for 30 days by default (configurable)

#### 1.8 Exportable Report (`devsweep report --export`)

- `devsweep report --export <tool>` generates a Markdown snippet ready to paste into a GitHub issue:
  ```
  $ devsweep report --export opencode

  📋 Copied to clipboard! Paste into a GitHub issue:

  ## DevSweep: Process Cleanup Report for OpenCode

  **Environment:** macOS 15.2, OpenCode v0.3.1
  **Period:** March 16–23, 2026

  OpenCode left **23 orphaned processes** across 9 sessions:
  - 18× `mcp-remote` (avg 56 MB each, not cleaned up on exit)
  - 5× `node --inspect` (debug workers, never terminated)

  **Total RAM wasted:** 1.8 GB
  **Suggested fix:** Send SIGTERM to child process group on CLI exit.

  _Generated by [DevSweep](https://github.com/user/devsweep)_
  ```
- Local-only — DevSweep never sends data anywhere. The user decides whether to share.
- Helps tool maintainers get actionable, structured bug reports instead of vague "it's leaking" complaints.

#### 1.9 Config File (`~/.devsweep.yml` + per-repo `.devsweep.yml`)

```yaml
# Processes to never touch
protected:
  - name: "ninjarmm*"
  - name: "jamf*"
  - port: 8080 # my main API

# Custom rules
rules:
  - match: "vite dev"
    max_duplicates: 1
    max_age: "12h"
  - match: "mcp-remote"
    max_duplicates: 1

# Daemon settings
watch:
  interval: 30        # seconds between scans (default 30)
  notify: true         # send OS notifications
  auto_clean: false    # auto-kill known-safe matches

# Report settings
report:
  retention_days: 30
```

**Config resolution order:** per-repo `.devsweep.yml` > user `~/.devsweep.yml` > built-in defaults. Per-repo configs are merged (not replaced), so teams can commit shared rules while individuals keep their own protected lists.

#### 1.10 Quick Commands

```bash
devsweep scan              # Show all dev processes grouped
devsweep detect            # Show problems only
devsweep clean             # Interactive cleanup
devsweep clean --auto      # Auto-clean (use rules, no prompts)
devsweep clean --dry-run   # Show what would be killed, don't kill
devsweep kill-port 3000    # Kill everything on a port
devsweep report            # Trends, stats & orphan lineage
devsweep report --export opencode  # Generate Markdown bug report for a tool
devsweep watch start       # Start background daemon
devsweep watch stop        # Stop background daemon
devsweep watch status      # Daemon status
devsweep watch install     # Auto-start on login (launchd/systemd)
devsweep watch uninstall   # Remove auto-start
```

---

### Phase 2 — macOS Menu Bar App + Terminal Dashboard — _~2 weeks_

#### 2.1 Terminal Dashboard (`devsweep dashboard`)

- Live-updating TUI built with `bubbletea` (already in dependency tree via `lipgloss`)
- Real-time process list grouped by category with CPU/RAM/port/age
- Keyboard-driven: `k` to kill, `s` to skip, `a` to auto-clean, `q` to quit
- Color-coded status per process (🟢 healthy, 🟡 stale, 🔴 problem)

#### 2.2 Swift Menu Bar Agent

- Lightweight menu bar icon (broom 🧹) with color states:
  - 🟢 Green — all clean
  - 🟡 Yellow — minor issues detected
  - 🔴 Red — high CPU/RAM from dev processes
- Click to see summary dropdown
- "Clean Now" button triggers `devsweep clean --auto`
- Native macOS notifications (replaces `osascript` approach from Phase 1)

#### 2.3 Static HTML Report (`devsweep report --html`)

- Generates a one-shot HTML file with trend charts (memory over time, CPU spikes, top offenders)
- Opens in default browser — no server, no persistent process
- Uses embedded chart library (e.g., Chart.js via Go template)

#### 2.4 Integration with CLI

- Menu bar app calls the Go CLI binary under the hood (no duplicated logic)
- Reads from the same SQLite history DB for trend display
- Shares the same config files

---

### Phase 3 — Ecosystem — _Later_

- VS Code / Zed extension that shows status in the editor
- Plugin system for custom process rules (community-contributed)
- Windows support via `gopsutil` (already cross-platform)

---

## Project Structure

```
devsweep/
├── cmd/
│   └── devsweep/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── scanner/
│   │   ├── process.go           # OS process listing via gopsutil
│   │   ├── categorizer.go       # Map process → category
│   │   └── ports.go             # Port resolution with graceful degradation
│   ├── rules/
│   │   ├── engine.go            # Rule evaluation
│   │   ├── duplicates.go        # Duplicate detection
│   │   ├── stale.go             # Stale/zombie detection
│   │   └── cpu_hog.go           # CPU hog detection
│   ├── cleaner/
│   │   ├── cleaner.go           # Process killing logic (SIGTERM → SIGKILL)
│   │   └── safety.go            # Protected process checks
│   ├── daemon/
│   │   ├── watcher.go           # Background polling loop
│   │   ├── lifecycle.go         # PID file, start/stop, install/uninstall
│   │   └── notifier.go          # OS-native notifications (macOS/Linux)
│   ├── store/
│   │   ├── db.go                # SQLite schema & connection
│   │   ├── history.go           # Snapshot logging & trend queries
│   │   └── lineage.go           # Parent-child relationship tracking & queries
│   ├── config/
│   │   └── config.go            # YAML config loader (merge logic)
│   └── ui/
│       ├── terminal.go          # Colored interactive output
│       └── export.go            # Markdown report generator for --export
├── npm/                         # npx distribution package
│   ├── package.json             # "devsweep" on npm
│   ├── bin.js                   # Downloads correct Go binary + proxies commands
│   └── install.js               # postinstall: fetch binary for current OS/arch
├── macos/                       # Phase 2: Swift menu bar app
│   ├── DevSweep/
│   │   ├── AppDelegate.swift
│   │   ├── StatusBarController.swift
│   │   └── ProcessBridge.swift  # Calls Go CLI
│   └── DevSweep.xcodeproj
├── rules/
│   └── default.yml              # Built-in detection rules
├── .goreleaser.yml              # Release automation (Go binary + npm publish)
├── LICENSE                      # MIT
└── README.md
```

---

## Tech Decisions

| Choice                        | Why                                                  |
| ----------------------------- | ---------------------------------------------------- |
| **Go** for CLI + daemon       | Single binary, no deps, fast, easy cross-compile     |
| **`gopsutil`** library        | Cross-platform process info (macOS + Linux)          |
| **SQLite** for history        | Zero-config, embedded, perfect for local trend data  |
| **npm** for distribution      | `npx devsweep` = zero-install trial (esbuild pattern)|
| **Swift** for menu bar        | Native macOS look & feel, low overhead (Phase 2)     |
| **`bubbletea`** for TUI       | Live-updating terminal dashboard (Phase 2)           |
| **YAML** for config           | Developer-friendly, easy to version control          |
| **`charmbracelet/lipgloss`**  | Beautiful terminal UI (colored tables, prompts)      |

---

## MVP Scope (v0.1.0)

1. ✅ `devsweep scan` — list all dev processes, grouped & colored
2. ✅ `devsweep detect` — flag duplicates, stale servers, CPU hogs
3. ✅ `devsweep clean` — interactive kill with confirmation + `--dry-run`
4. ✅ `devsweep kill-port <port>` — quick port cleanup
5. ✅ `devsweep watch` — background daemon with native OS notifications
6. ✅ `devsweep report` — trends, stats & orphan-by-tool lineage from SQLite
7. ✅ `devsweep report --export <tool>` — generate Markdown bug report for tool maintainers
8. ✅ Process lineage tracking — records parent-child relationships before parents die
9. ✅ Config file with protected processes + per-repo `.devsweep.yml` support
10. ✅ Graceful shutdown (SIGTERM → SIGKILL), never requires `sudo`
11. ✅ Cross-platform: macOS + Linux from day one
12. ✅ `npx devsweep` — zero-install distribution via npm
