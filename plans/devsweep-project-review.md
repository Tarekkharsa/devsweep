# DevSweep Project Review

## Executive Summary

DevSweep is already a useful **alpha-quality CLI** with a strong core idea: help developers find and clean up runaway dev processes, especially the mess left behind by AI agents, dev servers, and related tooling.

The project is promising and already demonstrates real value, but it is **not yet fully ready as a broadly trusted open-source cleanup utility**. The biggest gap is that the documentation and vision are currently ahead of the implementation.

## Overall Opinion

### What is already good

- The problem is real and worth solving.
- The CLI is small, understandable, and focused.
- The codebase structure is clean and easy to navigate.
- The current commands already provide value:
  - `scan`
  - `detect`
  - `clean`
  - `kill-port`
  - `watch`
  - `report`
- The current scan output already detects real developer processes well enough to prove the concept.

### What keeps it from being “done”

DevSweep is useful as-is for:
- personal use
- early adopters
- proving the concept
- showing the direction publicly

But it is not yet strong enough to be the default open-source tool people will trust to kill processes on their machines.

## Main Findings

### 1. Rules and config are not truly driving behavior yet

The repo presents YAML-configurable rules as a major feature, but the real categorization and detection logic is still mostly hardcoded.

#### Why this matters

For an open-source tool in this category, users will want to:
- add support for new AI agents
- tune memory / CPU thresholds
- define their own duplicate rules
- adapt detection to their own workflows

Without this, extending DevSweep requires editing Go code instead of editing config.

### 2. The trust layer is still missing

A cleanup tool must be trusted before people will adopt it.

Current gaps include:
- no real unit tests
- `go test ./...` is not clean yet
- safety claims in docs are stronger than the current implementation
- cleanup is still mostly PID-based instead of process-tree-aware

#### Why this matters

The target problem is not just “kill one heavy process.”
It is usually:
- one parent tool
- spawning multiple children
- leaving helpers, watchers, MCP servers, workers, or runtimes behind

A tool in this space has to understand **process trees**, not only individual PIDs.

### 3. Duplicate detection is too narrow for the actual goal

Current duplicate logic mainly depends on multiple processes of the same tool sharing the same listening port.

That works well for dev servers, but not for many AI-tooling or browser-helper cases.

#### Missing duplicate patterns

DevSweep should eventually detect duplicates like:
- same executable path
- same normalized command line
- same workspace / cwd
- same parent lineage
- same tool role but no port

This is especially important for:
- MCP helpers
- agent harness subprocesses
- browser / Chromium helpers
- orphaned Node workers

### 4. The default UX is a little noisy

The current scan output is informative, but it surfaces a lot of generic runtime processes.

That is technically correct, but the best user value is not:

> show me every Node process

It is:

> show me the suspicious things I can safely clean

#### Better default behavior

The product should bias toward:
- suspicious clusters
- duplicate groups
- orphaned children
- stale roots
- memory / CPU outliers

And keep raw runtime listings behind flags like:
- `--all`
- `--verbose`

### 5. Some documented features are partially implemented or not fully wired

Examples include:
- `watch.auto_clean` exists in config but is not clearly active in daemon behavior
- retention is effectively hardcoded in the watcher instead of using config
- boolean config merging is limited and may not properly support explicit `false` overrides
- `rules/default.yml` is presented as core product behavior, but is not fully powering detection

## Recommendation

## Yes, the project is useful

It is worth continuing.

The concept is strong, and the current prototype already proves there is value here.

## No, it should not be considered complete as-is

Before pushing hard for broad adoption, DevSweep should focus on becoming:
- safer
- more explainable
- more configurable
- less noisy
- more process-tree-aware

## Strategic Product Direction

DevSweep should focus on this promise:

> Safe, explainable cleanup of developer-related process clutter.

That is much stronger than trying to become a generic system cleaner.

The project should especially double down on:
- AI agent leftovers
- MCP servers
- dev server duplication
- orphaned workers
- process lineage and accountability

## What to Add Next

## Highest-priority additions

### 1. Make rules truly data-driven

Use config and built-in YAML to drive:
- categorization
- signatures
- thresholds
- duplicate policy
- auto-clean safety

This should allow the community to contribute support for new tools without changing Go code.

### 2. Add process-tree and process-group cleanup

This is one of the most important missing capabilities.

Instead of only killing a single PID, DevSweep should:
- identify the root process
- preview descendants
- kill the process tree or group safely
- avoid leaving children behind

This directly supports the stated goal of cleaning AI harness leftovers.

### 3. Add confidence-based cleanup

DevSweep should classify findings by confidence level.

Examples:
- **High confidence**: duplicate dev server on same port
- **Medium confidence**: orphaned MCP helper from dead parent
- **Low confidence**: generic Node runtime with high memory

Then:
- auto-clean only high-confidence cases by default
- require confirmation for medium/low confidence

### 4. Add tests for core logic

At minimum:
- categorizer tests
- duplicate detection tests
- orphan detection tests
- config merge tests
- cleanup target selection tests

This is essential for trust.

### 5. Add JSON output

Suggested commands:
- `devsweep scan --json`
- `devsweep detect --json`
- `devsweep clean --dry-run --json`

This will make DevSweep easier to integrate with:
- scripts
- CI
- editors
- agent harnesses
- future UI layers

## High-value product features for this specific goal

### 6. Detect duplicate helpers without relying on ports

This is likely the most important product improvement after process-tree cleanup.

Add heuristics for duplicates based on:
- normalized cmdline
- executable path
- cwd / repo root
- parent tool
- temporal proximity

This will catch the kinds of clutter AI agents often leave behind.

### 7. Group results by workspace or repo

Example output should eventually look like:

- `~/work/app-a`
  - Next.js on `:3000`
  - npm dev process
  - 3 stale node children
- `~/work/tool-b`
  - 2 MCP helpers
  - 1 orphaned agent child

This is much more actionable than a flat process list.

### 8. Add a `blame` mode

Potential commands:
- `devsweep blame`
- `devsweep blame opencode`

This should show:
- which tool spawned the clutter
- how many children it left behind
- how often it happens
- estimated RAM wasted

This would be a differentiating feature.

### 9. Add curated signatures for popular tools

Especially:
- Claude Code
- Cursor
- OpenCode
- Copilot
- MCP servers
- Playwright
- Vite
- Next.js
- Webpack
- Turbopack
- Electron-based dev tools

These signatures should live in config or YAML, not only in code.

### 10. Make the default UX more opinionated

Good high-level UX focus:
- problems first
- explain why something is suspicious
- show confidence and cleanup impact
- reduce runtime noise by default

## Chrome extensions / browser processes

This is a valid future direction, but should probably be treated as:
- experimental
- opt-in
- lower confidence

### Why

Browser and Chrome extension processes can consume a lot of CPU and RAM, but they are riskier to classify correctly.

### Better approach

Treat it as a later extension, for example:
- `devsweep browser scan`
- `devsweep browser detect`

Initially:
- warn first
- avoid auto-clean
- clearly mark results as experimental

## Things Not Worth Prioritizing Yet

These should probably wait until the core is more trustworthy:
- menu bar app
- rich TUI
- graphs/dashboard
- plugin SDK
- broad system cleanup features
- aggressive auto-kill automation

The main value is the core cleanup engine, not a bigger UI surface.

## Recommended Roadmap

## Phase 1 — Make it trustworthy

1. Fix `go test ./...`
2. Add unit tests
3. Wire config/rules into actual detection
4. Implement process-tree cleanup
5. Add JSON output
6. Tighten README so it exactly matches implementation

## Phase 2 — Make it differentiated

7. Add duplicate detection without port reliance
8. Group by workspace / root process
9. Add `blame` / lineage-focused UX
10. Add curated signatures for major AI/dev tools

## Phase 3 — Expand carefully

11. Add experimental browser / Chrome process support
12. Add optional TUI or menu bar integrations
13. Add community rule packs / shared signatures

## Suggested Initial GitHub Issues

### Core reliability
- Fix `go test ./...` and vet/build issues
- Add unit tests for categorizer and rules engine
- Add tests for config merging and explicit false values

### Core product
- Load built-in detection rules from `rules/default.yml`
- Support user and repo overrides for categorization and thresholds
- Implement process-tree cleanup instead of single-PID-only cleanup
- Add confidence levels to issues
- Add machine-readable JSON output

### Product differentiation
- Detect duplicates without ports
- Group process output by workspace / cwd
- Add `blame` command using lineage data
- Improve scan output to hide generic runtimes by default

### Future exploration
- Add experimental Chromium / browser helper detection
- Add signature packs for major agent ecosystems

## Final Verdict

DevSweep is already a good and useful prototype.

The idea is strong, the structure is solid, and the project is worth continuing.

The most important next step is not adding more UI or more commands. It is making the existing cleanup engine:
- safer
- more configurable
- more explainable
- more process-tree-aware

If DevSweep does that well, it can become a genuinely compelling small open-source CLI for cleaning the process clutter left behind by modern development tools and AI agent harnesses.
