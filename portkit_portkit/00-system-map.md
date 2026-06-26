# System Map — ae-port

## What this system is

`ae-port` is a **Claude Code plugin marketplace** that packages agentic software-porting
workflows. It is not a conventional compiled/served application: its "code" is a set of
Markdown command/agent definitions, JavaScript Workflow orchestration scripts, and JSON
manifests that Claude Code loads and executes. (`.claude-plugin/marketplace.json:1-20`)

The marketplace exposes two plugins (`.claude-plugin/marketplace.json:8-19`):

- **`cli`** (`./src/cli`) — an agent team + multi-phase workflow for faithfully porting a CLI to
  another language with byte-identical output, verified by differential testing.
- **`portkit`** (`./src/portkit`) — analyzes a codebase into a target-neutral, vertical-slice
  build kit so a weaker downstream model can recreate the project in another language/framework.

## Languages

- **JavaScript (ESM)** — the executable Workflow scripts run by Claude Code's Workflow runtime:
  - `src/portkit/workflows/portkit.js:1` (576 lines; 7-phase pipeline)
  - `src/cli/workflows/port-fanout.workflow.js:1` (87 lines)
  - `src/cli/workflows/adversarial-sweep.workflow.js:1` (116 lines)
- **Markdown** — slash-command definitions (`src/*/commands/*.md`) and agent definitions
  (`src/cli/agents/*.md`), plus docs/templates.
- **JSON** — plugin/marketplace manifests and JSON Schemas
  (`src/cli/templates/port.config.schema.json:1`, `src/cli/examples/seeds-go/port.config.json:1`).
- **Go** — present ONLY as a test/demo fixture to be ported, NOT system code:
  `src/examples/seeds-go/` (a tested Go HTTP JSON service). Cited by `src/portkit/VERIFICATION.md:42-45`.

## Build system

There is **no compiler, bundler, or package build**. Distribution is via the Claude Code plugin
marketplace mechanism: a user runs `/plugin marketplace add russelltsherman/ae-port` (`README.md:10-12`)
and Claude Code auto-discovers commands/agents/workflows from each plugin directory.

The Workflow scripts only run when the gated Workflow tool is enabled
(`CLAUDE_CODE_WORKFLOWS=1`); the `/portkit` command STOPs if it is unset
(`src/portkit/commands/portkit.md:37-44`).

## Tests / verification

No automated unit-test suite covers the system's own code. Verification is **tiered and manual**
(`src/portkit/VERIFICATION.md:1-73`):

- **Tier 1 — static**: `node --check` on the workflow, the workflow-creator linter, and JSON-manifest
  parse checks (`src/portkit/VERIFICATION.md:11-25`).
- **Tier 2 — runtime**: run `/portkit <target> src/examples/seeds-go` and check acceptance gates 1–7
  (`src/portkit/VERIFICATION.md:32-69`). Marked "Not yet executed" (`src/portkit/VERIFICATION.md:33`).

The only place with executable tests is the **Go fixture** `src/examples/seeds-go/`, which uses the
Go standard `testing` package: `src/examples/seeds-go/server_test.go:1`,
`src/examples/seeds-go/store_test.go:1` (run via `go test ./...`). These tests belong to the sample
being ported, not to ae-port itself.

## Dependency manifests

- `.claude-plugin/marketplace.json` — marketplace + plugin source list.
- `src/cli/.claude-plugin/plugin.json` — `cli` plugin manifest.
- `src/portkit/.claude-plugin/plugin.json` — `portkit` plugin manifest.
- `src/examples/seeds-go/go.mod` — Go module manifest for the fixture only.

There are no `package.json`, lockfiles, or runtime npm/Go dependencies for the system itself; the
JS workflows use only the Workflow runtime's injected globals (`agent`, `parallel`, `phase`, `log`,
`budget`, `args`).

## Capability / epic inventory

These are the externally-observable capabilities — the slash commands a user invokes and the
Workflow scripts they drive. Each is a vertical thread.

| id | name | kind | entry point |
|----|------|------|-------------|
| ep-portkit-analyze | `/portkit` codebase-to-buildkit analysis | cli (slash command) | `src/portkit/commands/portkit.md:1`, `src/portkit/workflows/portkit.js:257` |
| ep-cli-init | `/cli-port:init` — generate port.config.json + scaffold | cli (slash command) | `src/cli/commands/port-init.md:1` |
| ep-cli-map | `/cli-port:map` — Phase 0 cartography of every command | cli (slash command) | `src/cli/commands/port-map.md:1`, `src/cli/agents/cartographer.md:1` |
| ep-cli-harness | `/cli-port:harness` — Phase 1 determinism contract + differential harness | cli (slash command) | `src/cli/commands/port-harness.md:1` |
| ep-cli-run | `/cli-port:run` — Phase 2 port fan-out to byte-parity | cli (slash command) | `src/cli/commands/port-run.md:1`, `src/cli/workflows/port-fanout.workflow.js:59` |
| ep-cli-verify | `/cli-port:verify` — Phase 3 adversarial sweep + sign-off | cli (slash command) | `src/cli/commands/port-verify.md:1`, `src/cli/workflows/adversarial-sweep.workflow.js:68` |
| ep-cli-run-all | `/cli-port:run-all` — full pipeline init→map→harness→run→verify | cli (slash command) | `src/cli/commands/port-run-all.md:1` |
| ep-cli-status | `/cli-port:status` — evidence-based port status report | cli (slash command) | `src/cli/commands/port-status.md:1` |
| ep-marketplace-install | Marketplace install / plugin discovery | public-api (manifest) | `.claude-plugin/marketplace.json:8`, `README.md:10` |

### Notes on the `portkit.js` pipeline (ep-portkit-analyze)

The `/portkit` workflow is a 7-phase agent pipeline (`src/portkit/workflows/portkit.js:5-13`):
Preflight (`:257`), Map (`:300`), Discover slices (`:342`), Synthesize (`:389`), Write slices
(`:449`), Target mapping (`:482`, only when a target lang is given), Critic + bounded gap-fill
(`:510`). It aborts loudly if the input dir is missing/empty (`:280-294`) and caps fan-out per axis
to avoid silent truncation (`:61-81`).

### Notes on the `cli` port pipeline

`/cli-port:run` drives `port-fanout.workflow.js`: one `port-engineer` per command in an isolated
git worktree, TDD against the differential harness, commit-on-green
(`src/cli/workflows/port-fanout.workflow.js:59-75`). `/cli-port:verify` drives
`adversarial-sweep.workflow.js`: per-category `adversarial-differ` rounds hunting byte divergences,
looping until N consecutive dry rounds, fixing each finding serially
(`src/cli/workflows/adversarial-sweep.workflow.js:68-108`). Supporting agents live in
`src/cli/agents/` (cartographer, determinism-auditor, harness-engineer, port-engineer,
parity-specialist, adversarial-differ, integrator).
