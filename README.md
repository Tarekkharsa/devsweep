# 🧹 DevSweep

Smart process cleanup for developers. Detects, monitors, and cleans up runaway dev processes — Vite servers, MCP agents, orphaned Node workers, and all the chaos AI tools leave behind.

**Single binary. Zero dependencies. Works immediately.**

## Install

```bash
# Option 1: Run directly from the project
cd ~/Desktop/devsweep
./devsweep scan

# Option 2: Make it available everywhere (one-time setup)
sudo ln -s /Users/tarek.k/Desktop/devsweep/devsweep /usr/local/bin/devsweep
```

## Commands

### See what's running

```bash
devsweep scan
devsweep scan --json      # Machine-readable output
```

Lists all dev processes on your machine, grouped and color-coded:
- 🖥 **Dev Servers** — Vite, Next.js, Webpack, Remix, etc.
- 🤖 **AI Agents** — MCP servers, Claude, Copilot, OpenCode
- 📦 **Package Managers** — npm, pnpm, yarn, bun
- ⚙️ **Runtimes** — Node.js, Deno, Bun, Python
- 🛡 **Protected** — System processes (never touched)

### Find problems

```bash
devsweep detect
devsweep detect --json    # Machine-readable issues
```

Flags issues automatically:
- ⚠️ Duplicate dev servers on the same port
- 🤖 Duplicate AI helpers with identical command lines
- ⏰ Stale servers running 24+ hours with no CPU usage
- 👻 Orphaned processes (parent died, child still running)
- 🔥 CPU hogs (>50% for 5+ minutes)
- 💾 Memory bloat (>500 MB)

Every issue is labeled with a confidence level so auto-clean can stay conservative.

### Clean up

```bash
devsweep clean              # Interactive — asks before killing each group
devsweep clean --auto       # Auto-clean, no prompts
devsweep clean --dry-run    # Preview what would be killed (safe)
devsweep clean --dry-run --json
```

For duplicates, DevSweep keeps the **newest** process and kills the rest. Cleanup now targets the selected process **and its descendants**, so leftover helper children are less likely to survive a cleanup pass.

### Kill a specific port

```bash
devsweep kill-port 3000
devsweep kill-port 3000 --dry-run   # Preview first
```

### Background monitoring

```bash
devsweep watch start       # Start the daemon (scans every 30 seconds)
devsweep watch stop        # Stop it
devsweep watch status      # Check if it's running
```

The daemon runs in the background, records process snapshots to a local database, tracks parent-child relationships (so it knows *who* left orphans), and sends you a native macOS notification when problems are found.

If `watch.auto_clean: true` is enabled, the daemon will only auto-clean **high-confidence** issues by default.

**Auto-start on login (optional):**

```bash
devsweep watch install     # Add to macOS LaunchAgents
devsweep watch uninstall   # Remove it
```

### Reports & trends

```bash
devsweep report                      # Stats from the last 7 days
devsweep report --export opencode    # Generate a Markdown bug report for a tool
devsweep blame                       # Which tools leave the most clutter
devsweep blame opencode              # Focus on one tool
devsweep blame --json                # Machine-readable blame stats
```

Shows:
- How many processes were killed and RAM reclaimed
- Port conflicts over time
- Orphan counts grouped by which tool left them (e.g., "OpenCode left 23 orphans")

The `blame` command merges orphan and cleanup history into a single table so you can quickly see which tools are the repeat offenders.

The `--export` flag generates a ready-to-paste GitHub issue so tool maintainers get actionable bug reports instead of vague complaints.

## Config

Create `~/.devsweep.yml` for personal settings, or `.devsweep.yml` in a repo for team-shared rules.

```yaml
# Processes to never touch
protected:
  - name: "my-important-server"
  - port: 8080

# Custom rules
rules:
  - match: "vite"
    max_duplicates: 1
    max_age: "12h"
    cpu: "80%"
    memory: "2GB"
  - match: "mcp-remote"
    max_duplicates: 1

# Daemon settings
watch:
  interval: 30         # seconds between scans
  notify: true         # send OS notifications
  auto_clean: false    # auto-kill (use with caution)
```

Repo configs merge with your personal config — teams can share rules while you keep your own protected list.

## Safety

- **Tree-aware cleanup** — kills the selected process and any descendants it spawned
- **Graceful shutdown** — always SIGTERM first, waits 5 seconds, then SIGKILL
- **Protected processes** — system processes are never touched
- **Dry-run mode** — preview everything before killing
- **No sudo required** — works entirely in user space
- **Local only** — all data stays on your machine, nothing is sent anywhere

## How it works

DevSweep uses **layered matching** to figure out what a process actually is (a `node` process could be Vite, Next.js, an MCP server, or anything):

1. **Binary name** — `next-server` → Next.js
2. **Command line** — `node .../vite/bin/vite.js` → Vite
3. **Keywords** — `mcp-remote` in cmdline → MCP Server
4. **Port heuristic** — `:3000` + Node → likely a dev server
5. **Runtime fallback** — just `node` with no match → "Node.js (unknown)"

## Data storage

Everything is stored locally at `~/.devsweep/`:

```
~/.devsweep/
├── history.db    # SQLite — snapshots, lineage, kill records
├── daemon.pid    # PID file for the background daemon
└── daemon.log    # Daemon logs
```

Data is retained for 30 days by default (configurable via `report.retention_days` in config).

## Build from source

Requires Go 1.21+:

```bash
git clone https://github.com/tarek-k-devs/devsweep.git
cd devsweep
go build -o devsweep ./cmd/devsweep/
./devsweep scan
```
