# KERNEL — ae-port shared glossary

This is the **thin shared kernel** every slice doc references but does NOT restate.
A slice doc names a kernel term; the definition lives here. The kernel holds only
naming, types, and domain vocabulary that are genuinely cross-cutting. Everything
behavioral stays inside its slice so each slice is self-contained for a weak model.

Grounding: every term below cites a `path:line` in the source. A downstream model
rebuilding this system reads the kernel once, then rebuilds each slice from its own
doc plus this glossary.

---

## 1. What ae-port is

`ae-port` is a **Claude Code plugin marketplace** (not a compiled/served app). Its
"code" is Markdown command/agent definitions, JS Workflow orchestration scripts, and
JSON manifests that the Claude Code host loads and runs
(`.claude-plugin/marketplace.json:1`, `00-system-map.md`). There is no compiler or
bundler. It ships **two plugins**:

- **`cli`** (`./src/cli`) — a 7-agent team + 4-phase pipeline that faithfully ports a
  CLI to another language with **byte-identical** output, verified by differential
  testing (`.claude-plugin/marketplace.json:11`).
- **`portkit`** (`./src/portkit`) — analyzes a codebase into a target-neutral,
  vertical-slice build kit (`.claude-plugin/marketplace.json:16`).

The two plugins are independent products; the kernel is shared vocabulary, not shared
runtime code.

---

## 2. Plugin packaging vocabulary (epic `ep-marketplace-install`)

| Term | Definition | Citation |
|------|-----------|----------|
| **marketplace manifest** | `.claude-plugin/marketplace.json` — fixed discovery path; JSON object `{name, description, owner, plugins[]}`. Marketplace `name` = `"ae-port"`. Entry point the host reads on `/plugin marketplace add`. | `.claude-plugin/marketplace.json:1` |
| **catalog entry** | One object in `plugins[]`: `{name, source, description}`. `source` is a repo-root-relative path beginning `./` pointing at a dir containing that plugin's `.claude-plugin/plugin.json`. | `.claude-plugin/marketplace.json:9` |
| **plugin manifest** | `<plugin>/.claude-plugin/plugin.json` — `{name, version, description, author, …}`. Convention-fixed path; its parent dir roots auto-discovery. `name` MUST equal the catalog entry's `name`. | `src/cli/.claude-plugin/plugin.json:1`, `src/portkit/.claude-plugin/plugin.json:1` |
| **auto-discovery** | No manifest lists components. Placing a file in the convention dir registers it: `commands/*.md` → one slash command (name = filename stem); `agents/*.md` → one subagent; `workflows/*.js` exporting `meta` → one workflow (id = `meta.name`). | `src/cli/commands/port-init.md:1`, `src/cli/agents/cartographer.md:1`, `src/cli/workflows/port-fanout.workflow.js:1` |
| **command frontmatter** | YAML between `---` lines at top of a `commands/*.md`. Keys: `description`, `argument-hint`, `allowed-tools`. Body after frontmatter is the prompt template; `$ARGUMENTS` is the user's raw arg string. | `src/cli/commands/port-init.md:1`, `src/portkit/commands/portkit.md:4` |
| **agent frontmatter** | YAML in an `agents/*.md`. Keys: `name`, `description` (routing: what + when), `tools` (comma list), `model` (e.g. `opus`). Body = the agent system prompt. | `src/cli/agents/cartographer.md:1` |
| **`meta` export** | First statement of a workflow JS file: `export const meta = { name, description, whenToUse?, phases:[{title,detail}] }`. `meta.name` is the registration id (need not equal filename). | `src/portkit/workflows/portkit.js:1`, `src/cli/workflows/port-fanout.workflow.js:1` |

---

## 3. Workflow-runtime vocabulary (cross-cutting for all JS workflows)

The JS workflows run ONLY under the gated Workflow runtime (`CLAUDE_CODE_WORKFLOWS=1`).
The runtime injects globals; workflow scripts have **no filesystem access** of their own.

| Global / term | Definition | Citation |
|---------------|-----------|----------|
| **`args`** | The workflow input. May be an object, a JSON string, or undefined. **KNOWN LIMITATION (LEARNINGS A1):** when a workflow is launched by `{scriptPath}`, `args` arrives **undefined** — the driving command MUST bake config into a `BAKED_CONFIG` const instead. | `src/portkit/workflows/portkit.js:21`, `src/cli/workflows/port-fanout.workflow.js:15` |
| **`agent(prompt, opts)`** | Spawn one fresh-context subagent. `opts`: `{label, phase, agentType, schema, isolation?}`. Returns the agent's structured result (or falsy on empty/fail). `agentType` MUST be plugin-qualified (e.g. `cli-port:port-engineer`); a bare name throws "agent type not found" (LEARNINGS A2). | `src/cli/workflows/port-fanout.workflow.js:64` |
| **`parallel(thunks)`** | Run an array of zero-arg thunks concurrently; **barrier** — resolves only when all finish; order-stable; a throwing thunk → `null`. | `src/cli/workflows/port-fanout.workflow.js:59` |
| **`pooled(thunks, limit)`** | portkit's bounded-concurrency replacement for `parallel()`: same contract, but ≤ `limit` thunks in flight via a rolling worker pool. Use for every fan-out to avoid API rate limits. | `src/portkit/workflows/portkit.js:92` |
| **`phase(title)`** | Marks a pipeline stage for live `/workflows` progress. | `src/portkit/workflows/portkit.js:257` |
| **`log(msg)`** | Emit a live progress line. Convention: prefix `⚠️` for warnings, `❌` for aborts. | `src/portkit/workflows/portkit.js:78` |
| **`budget`** | Token budget. `budget.total`, `budget.remaining()`. Used to stop a loop that can't finish (portkit gap-fill floors at 50k). | `src/portkit/workflows/portkit.js:533` |
| **schema** | A JSON-Schema object passed as `opts.schema` so the runtime validates the agent's structured return. | `src/portkit/workflows/portkit.js:118` |
| **runtime caps** | A run is capped at ~1000 agents total and 4096 items per `parallel`/`pooled` call; in-flight agents at `min(16, cores-2)`. Drives every scale-guard/cap. | `src/portkit/workflows/portkit.js:56` |

---

## 4. portkit shared helpers (epic `ep-portkit-analyze`)

Pure functions referenced across portkit slices. Reproduce EXACTLY.

| Helper | Contract | Citation |
|--------|----------|----------|
| **`slug(s)`** | `String(s).toLowerCase()` → replace runs of `[^a-z0-9]+` with one `-` → strip leading/trailing `-` → `.slice(0,48)` → if empty return literal `'slice'`. `'Go/Rust!'`→`go-rust`; `''`→`slice`. | `src/portkit/workflows/portkit.js:83` |
| **`pad(n)`** | `String(n).padStart(4,'0')`. `1`→`0001`; 5+ digits returned unpadded. | `src/portkit/workflows/portkit.js:86` |
| **`cap(list,max,what)`** | If `len ≤ max` return unchanged. Else keep first `max`, log `⚠️ Capped ${what}: kept ${max} of ${len} (dropped ${len-max}).`, push that note to module-level `dropped[]`, return kept. Truncation is never silent. | `src/portkit/workflows/portkit.js:74` |
| **`dropped[]`** | Module-level truncation ledger; surfaced as `truncations` in the final result and fed to RISKS-AND-GAPS. | `src/portkit/workflows/portkit.js:73` |
| **`GROUND_RULE`** | The mandatory grounding instruction appended to nearly every agent prompt: cite `path:line` or mark `[UNVERIFIED]`; never invent; consumer is a weaker model rebuilding without the source. | `src/portkit/workflows/portkit.js:108` |
| **`MAX_*` knobs** | `MAX_EPICS`(40) `MAX_SLICES_TOTAL`(120) `MAX_HINTS_PER_TARGET`(80) `MAX_GAPFILL_ROUNDS`(2) via `Number(cfg.x)||default`; `MAX_CONCURRENCY=Math.max(1,Number(cfg.maxConcurrency)||8)`. `0`/`NaN` fall back to default. | `src/portkit/workflows/portkit.js:61` |
| **`OUT` derivation** | Output dir is a **sibling** of the input, never nested: `${inputDir}_${slug(target)||'portkit'}`; `.`/empty base → `portkit_<suffix>` in cwd. Explicit `outputDir`/`outDir` overrides. | `src/portkit/workflows/portkit.js:47` |

---

## 5. cli-port domain vocabulary (epics `ep-cli-*`)

The `cli` plugin ports a source CLI to a target language with byte-identical output.
Shared nouns:

| Term | Definition | Citation |
|------|-----------|----------|
| **`port.config.json`** | The single config the whole cli pipeline reads. Required keys `source, target, oracle, determinism, corpus`; `additionalProperties:false` at every level (unknown key = hard error). `adversarial` optional. Validated against `templates/port.config.schema.json`. | `src/cli/templates/port.config.schema.json:7` |
| **working tree** | The output dir created by init that CONTAINS `port.config.json`. Every later path (`contracts/`, `determinism.md`, `harness/`, `corpus/`, `port/`, `divergence-ledger.md`) is relative to it. | `src/cli/commands/port-init.md:76` |
| **oracle** | The source CLI run as ground truth. `oracle.command` (e.g. `bun src/index.ts`, args appended) executed with `cwd = source.repo`. `oracle.mode`: `live` (run source per case) or `snapshot` (diff committed goldens). | `src/cli/templates/port.config.schema.json:32` |
| **target** | The port being built. `target.binary` is the compiled command compared against the oracle. | `src/cli/templates/port.config.schema.json:22` |
| **determinism contract** | `determinism.md` + the `determinism` config block: the read-only inventory of every entropy source plus the env pins, mask rules, format assertions, and ordering hazards that make byte-comparison possible. Both harness and port are bound by it. | `src/cli/agents/determinism-auditor.md:22` |
| **`envPins`** | String→string map applied to the child env of BOTH runs; the only allowed env difference. Standard four: `NO_COLOR=1, FORCE_COLOR=0, TZ=UTC, LC_ALL=C`. Each pin must be *verified by observation* to neutralize what it claims (LC_ALL=C does NOT fix ICU localeCompare). | `src/cli/templates/port.config.schema.json:53`, `src/cli/agents/determinism-auditor.md:24` |
| **mask rule** | `{name, pattern, mode:'ordinal', assertFormat?, ordered?}`. **Ordinal** = the Nth *distinct* matching token → `<NAME:n>` in first-seen order, same value → same token within a capture (preserves references). | `src/cli/templates/port.config.schema.json:58`, `src/cli/agents/determinism-auditor.md:36` |
| **`<TS>` timestamp rule (LEARNINGS B5)** | Timestamps default to a single **constant** token `<TS>`, NOT per-distinct ordinals — sibling timestamps can land 1ms apart under load, shifting ordinals → false divergence. Keep the format assertion + chronology assertions. | `src/cli/agents/determinism-auditor.md:65` |
| **`sandboxPathVar`** | Placeholder (e.g. `<ROOT>`) that the absolute temp sandbox path is rewritten to, FIRST in canonicalization, before any masking. | `src/cli/templates/port.config.schema.json:84`, `src/cli/templates/harness/canonicalize.md:13` |
| **corpus / case** | `corpus/` holds `*.json` cases. A **case** = `{name, steps:[{args}]}` — an ordered sequence of invocations sharing ONE sandbox (stateful flows). A step arg may reference a prior step's parsed output (`$1.id`). Seed cases + adversarial cases. | `src/cli/templates/harness/runner.md:9` |
| **capture object** | Per (case, impl): `{steps:[{stdout,stderr,exit}], files:{path:bytes}}`. Everything is raw **bytes**, never decoded strings. Files sorted by path. | `src/cli/templates/harness/canonicalize.md:11` |
| **canonicalization** | The fixed 4-step pipeline run on BOTH captures before comparison: (1) sandbox-path rewrite, (2) format assertions, (3) ordinal masking, (4) ordering assertions. Defines what "byte-parity" means. A format/ordering assertion failure is a **test failure**, not a mask. | `src/cli/templates/harness/canonicalize.md:6` |
| **harness / `harnessCmd`** | The differential harness. `harnessCmd <command>` runs only that command's cases and **exits non-zero on any byte divergence** (the TDD signal). Full-corpus run omits the command arg. | `src/cli/templates/harness/runner.md:52`, `src/cli/workflows/port-fanout.workflow.js:12` |
| **divergence report** | On any byte diff: case, step index, stream (`stdout`/`stderr`/`exit`/`file:<path>`), first differing byte offset, windowed hex+printable diff. Labels **A = oracle**, **B = port**. Copy-pasteable. | `src/cli/templates/harness/runner.md:38` |
| **TS-vs-TS gate (Phase-1 acceptance)** | Run the source against a second run of itself, canonicalize both, require byte-identical — proves the contract neutralizes all entropy BEFORE any port exists. Acceptance = ≥20 consecutive full-gate runs **under deliberate CPU load**, not green-once. | `src/cli/agents/determinism-auditor.md:54` |
| **`divergence-ledger.md`** | Created empty by init, owned by the integrator. Each entry records what deviates, where, why, how the harness accounts for it. Sign-off needs it empty OR every entry justified. | `src/cli/commands/port-init.md:83`, `src/cli/commands/port-verify.md:23` |
| **`BAKED_CONFIG`** | The const a driving command overwrites before launching a workflow by `{scriptPath}` (because `args` does not forward). Empty command list → loud no-op return. | `src/cli/workflows/port-fanout.workflow.js:21` |
| **VERDICT / FINDINGS schemas** | Per-agent structured returns. VERDICT (port-engineer): `{command, byteIdentical, branch, …}`. FINDINGS (adversarial-differ): `{divergencesFound, findings:[{command, category, …, reproducible}]}`. | `src/cli/workflows/port-fanout.workflow.js:32`, `src/cli/workflows/adversarial-sweep.workflow.js:40` |

---

## 6. The cli-port agent team (epics `ep-cli-*`)

Seven specialist subagents under `src/cli/agents/` (auto-discovered):

| Agent | Role | Citation |
|-------|------|----------|
| **cartographer** | Phase 0. Read-only. Reverse-engineers each command into one precise IO contract. | `src/cli/agents/cartographer.md:1` |
| **determinism-auditor** | Phase 1. Read-only. Authors `determinism.md` (entropy inventory + canonicalization rules). | `src/cli/agents/determinism-auditor.md:1` |
| **harness-engineer** | Phase 1. Builds `harness/` implementing the auditor's rules exactly. | `src/cli/agents/harness-engineer.md:1` |
| **port-engineer** | Phase 2. TDD red→green per command against the harness; never weakens it. | `src/cli/agents/port-engineer.md:1` |
| **parity-specialist** | Owns serialization/format byte-parity (foundations: types, id, store, serializers). | `src/cli/agents/parity-specialist.md:1` |
| **adversarial-differ** | Phase 3. Read-only. Hunts byte divergences the corpus misses, per category, with the entropy filter. | `src/cli/agents/adversarial-differ.md:1` |
| **integrator** | Authoritative verifier. Merges branches, re-runs the FULL harness after each merge, owns the gates and the ledger. Self-reports are advisory only (LEARNINGS C-KEEP). | `src/cli/agents/integrator.md:1` |

---

## 7. Vertical-slice IR vocabulary (portkit output, the thing this kit IS)

| Term | Definition | Citation |
|------|-----------|----------|
| **epic** | A coarse, externally-observable capability (a slash command, an HTTP surface, a workflow) — a vertical thread, never a horizontal layer ("the models"). Has `id, name, kind, entry anchors`. | `src/portkit/workflows/portkit.js:300` |
| **slice** | A function/unit-sized vertical thread: `{id, name, epicId, capability, thread[], behaviorSummary, dependsOn[]}`. A weak model must rebuild it from its doc + this kernel ALONE. | `src/portkit/workflows/portkit.js:144` |
| **thread** | The end-to-end component list of a slice (entry → validation → rule → data → persistence → response), each with a `path:line` citation. | `src/portkit/workflows/portkit.js:144` |
| **`mergedFrom`** | Records that this normalized slice absorbed same-thread slices discovered under other epics. | this synthesis |
| **buildOrder** | A deterministic topological order over slice ids; dependencies precede dependents. | `epics/INDEX.md` |
| **coverage** | Each slice's behavioral-spec coverage rating: `good`/`thin`/`none`. `thin`/`none` flagged LOUDLY as a rebuild risk; defaults to `none` when absent. | `src/portkit/workflows/portkit.js:430` |
