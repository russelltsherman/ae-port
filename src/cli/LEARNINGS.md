# cli-port — Learnings from the seeds (`sd`) TypeScript→Go port

Captured from a full end-to-end run of the plugin porting the `seeds` CLI
(TypeScript/bun, 24 commands) to Go, byte-for-byte, verified by the differential
harness. Phases 0–3 ran to a 188-case corpus at 188/188 byte-identical with the
gate proven stable. This doc records what broke, what was missing, and what
worked — so the plugin gets better for the next port.

Severity legend: **[BUG]** ships broken · **[GAP]** missing guidance/coverage ·
**[KEEP]** worked well, preserve/reinforce.

---

## A. Workflow bugs — the shipped workflows do not run as documented

### A1. [BUG] `args` does not forward to workflows invoked via `scriptPath`
Both `port-fanout.workflow.js` and `adversarial-sweep.workflow.js` read all
their config from the `args` global (`commands`, `harnessCmd`, `categories`,
`dryRoundsToStop`). When a workflow is launched via `{scriptPath: ...}` (which is
how the driving `/cli-port:*` commands invoke them after editing), `args` arrives
`undefined`, so the scripts hit their `commands.length === 0` /
`cfg || {}` fallback and return `{ported: [], note: 'empty command list'}` —
doing nothing. Observed twice in this run; both shipped workflows were unusable
until forked with config baked into the script body.

**Fix options** (pick one and make it the documented contract):
- Have the driving command WRITE a config file (e.g. `port.run.json`) into the
  working tree and have the workflow READ it from disk, not from `args`.
- Or bake the command-list/config into a generated script the command emits.
- Either way: stop depending on `args` surviving `scriptPath`, and add a loud
  `log()` + early validation that distinguishes "no config found" from
  "config says zero commands".

### A2. [BUG] `agentType` is not plugin-qualified
`port-fanout.workflow.js` spawns `agentType: 'port-engineer'`, which throws
`agent type 'port-engineer' not found`. The registry requires the qualified
`cli-port:port-engineer`. Same applies anywhere a workflow names an agent.
Audit every `agentType:` in every workflow for the `cli-port:` prefix.

### A3. [BUG/GAP] `isolation: 'worktree'` assumes platform auto-worktree is available
`port-fanout` uses `isolation: 'worktree'`. If the session did not start inside a
git repo (or no `WorktreeCreate` hook is configured), this fails:
`Cannot create agent worktree: not in a git repository and no WorktreeCreate
hooks are configured`. A mid-session `git init` does NOT retroactively enable the
platform feature. Workaround used: pre-create real git worktrees
(`git worktree add`) and bind each engineer to its absolute path (distinct path →
distinct `repoRoot()` → distinct harness scratch → safe parallel).

**Fix:** `/cli-port:init` should `git init` the output dir and make an initial
commit, so worktree isolation is available from the start. The fanout should
detect the no-auto-worktree case and fall back to pre-created worktrees +
explicit paths rather than dying.

### A4. [BUG] The fanout "Verify" stage and the sweep "Fix" stage are wrong for worktree isolation
- **Fanout verify stage** runs in the BASE tree, but the port code an engineer
  wrote lives in its isolated WORKTREE and is not visible in base until merged —
  so the verify stage false-reds every command. The authoritative verification is
  the **integrator's post-merge harness run**, not a separate base-tree verify
  agent. The verify stage as written tests code that isn't in the tree it runs in.
- **Sweep fix stage** runs fixers in `parallel()` in the base tree with no
  isolation and no commit step — concurrent edits to shared files
  (`internal/cli/`, `internal/serial/`, `main.go`) collide and nothing is
  committed/integrated.

**Fix:** model the real, safe loop: `find (read-only, parallel) → add fixtures →
fix in isolated worktrees (one per file-disjoint cluster) → integrator merges +
re-runs the FULL gate → loop`. Fixing must never run parallel-in-base-tree.

---

## B. Method gaps the agent prompts should encode

### B5. [GAP] The determinism gate must be proven STABLE UNDER LOAD, not green-once
The single most important correctness lesson. Phase 1 reported "112/112,
reproduced 4×" and was signed off — but the gate was **flaky**: under full-gate
CPU load it intermittently dropped to 110–111/112, always on multi-record-update
cases. Root cause: the ISO8601 mask was **ordinal-by-distinct-value**
(`<TS:n>` in first-seen order). Source operations that update ≥2 records call
`new Date().toISOString()` separately per record (e.g. `dep add` updates both
endpoints); under load those sibling timestamps land 1 ms apart, changing the
COUNT of distinct timestamp values in the file, which shifts the ordinal
numbering → false divergence vs a Go port that writes both from one clock read.

**Generalizable rule for the determinism-auditor:**
- Ordinal-by-distinct-value masking is only sound when the **count** of distinct
  entropy values is structurally fixed (true for random ids — one per record;
  FALSE for timestamps — two same-operation writes may or may not share a ms).
- Default timestamps to a **single constant token** (`<TS>`), keeping the format
  assertion and a per-record chronology assertion (`createdAt ≤ updatedAt`,
  `createdAt ≤ closedAt` — NOT `updatedAt ≤ closedAt`, which update-after-close
  legitimately violates).
- **Acceptance = N consecutive full-gate runs (≥20) under deliberate CPU load**,
  not a single green run. A single green run is how this flake hid through
  Phase-2 sign-off. The harness-engineer should bake "loop the gate under load"
  into Phase-1 acceptance.

### B6. [GAP] The cartographer under-maps the framework surface
The largest real divergence clusters were NOT in the Phase-0 contracts or the
seed corpus, because they are **framework-generated, not command-authored**, and
thus easy to miss when mapping per-command behavior:
- bare invocation (`sd` with no command) and `-h`/`--help` at EVERY level
  (top-level, per-command, nested subcommand);
- the arg-parser's error-layer ordering (e.g. commander checks required options
  BEFORE unknown options) and flag-parsing edges (`--opt=val` on a boolean,
  combined short flags `-qx`, attached `-vx`, `--` terminator, `suggestSimilar`
  "Did you mean", empty `--format=`).

**Fix:** the cartographer prompt must explicitly require mapping: bare/`--help`
at all levels, version, unknown-command suggestion, and the framework's
error/flag-parsing semantics — as first-class IO contracts, with corpus cases.

### B7. [GAP] Ship a JS→Go parity-hazard checklist (reusable up-front reference)
~80% of the adversarial findings across three rounds were predictable Go-vs-JS
semantic gaps. A checklist consulted up front (by auditor + port-engineers) would
have pre-empted most of them. See `templates/js-to-go-parity-hazards.md` (added
alongside this doc).

### B8. [GAP] Adversarial sweep needs two more categories + an entropy filter + the JSONPARSE-tail trap
- Add default categories **`cli_surface` (framework parsing / error layers)** and
  **`file_effects` (compare the bytes WRITTEN to disk, not just stdout)** — both
  were highly productive and absent from the shipped six.
- The differ MUST **TS-vs-TS-verify each candidate fixture** (run it source-vs-
  source) before reporting, to filter non-reproducible entropy. This is how a
  `--timing` finding (`⏱ <N>ms`, genuinely nondeterministic, §7-excluded) was
  correctly discarded instead of chased.
- **JSONPARSE-tail trap:** a mask bounded at the first quote
  (`JSON Parse error: [^"\n]*`, bounded so it doesn't eat JSON-string closing
  quotes in e.g. doctor-json) cannot neutralize a runtime error that echoes an
  input token in quotes (JSC: `Unexpected identifier "not"` leaves `"not"` past
  the mask). For regression fixtures, pick inputs whose runtime error has NO
  double-quote (`{` → `Expected '}'`); and the PORT must emit a fixed,
  quote-free detail rather than echoing raw input.

### B9. [GAP] Don't hardcode the oracle source path in the harness
The harness `-source` default was a hardcoded absolute path. When the workspace
relocated mid-run, the gate broke (`oracle entrypoint not found`) and produced a
spurious one-off TS-vs-TS failure. Derive the oracle path as a repo sibling
(`../<source>` next to the port repo) or from `port.config.json`, never hardcode.

---

## C. What worked — preserve and reinforce

### C-KEEP. The independent integrator that VERIFIES rather than trusts
The integrator repeatedly caught the COORDINATOR's (orchestrator's) own errors by
re-deriving from evidence instead of relaying claims:
- a wrong `registerAll()` tail order (verified against oracle source before merge);
- a bogus "this helper has 3 duplicate copies" claim (grep showed 2, and one was
  a differently-named function);
- a "all colliding helpers are byte-identical, safe to dedup" claim that was
  FALSE for several pairs (different signatures/return types) — it diffed each
  pair and renamed instead of deduping, avoiding silent behavior changes.
The determinism-auditor likewise caught two errors in a proposed timestamp-fix
(an over-strict chronology term; an over-optimistic "zero detection lost" claim).

**This "re-derive, don't relay" discipline is the plugin's strongest property.**
Reinforce in the integrator and auditor prompts: treat relayed coordinator claims
as hypotheses to verify, quote tool output verbatim, never round up a pass rate.

### C-KEEP. Find/fix separation under controlled integration
Splitting the sweep into a read-only parallel FIND phase + controlled,
worktree-isolated FIX clusters + a single integrator merge was far safer than the
shipped parallel-fix-in-base-tree, and made "loop until dry" actually converge.

---

## Appendix: numbers from this run
- Corpus grew 112 (seed) → 137 (help surface) → 171 (sweep round 1) → 188
  (sweep round 2). Round 3 surfaced ~23 more (a BOM/NEL trim cluster) — fixing
  in progress at time of writing.
- Divergences found+fixed by adversarial sweep: round 1 = 34, round 2 = 17
  (+1 discarded as `--timing` entropy). Trend not yet dry → the loop continues
  until two consecutive empty rounds.
- Every fix made the port MORE faithful to the oracle; the divergence ledger
  records zero intentional deviations.
</content>
