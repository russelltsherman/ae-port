---
description: "Run the full port pipeline (init → map → harness → run → verify) in sequence, enforcing gates and auto-resuming from the last incomplete phase."
argument-hint: [--auto] <inputdir> [<outputdir>]
---

Run the **entire cli-port pipeline** end-to-end from a single command. This
orchestrates all phases — init, map, harness, port fan-out, adversarial sweep,
and sign-off — in sequence, enforcing each phase's gate before proceeding.

Arguments: `$ARGUMENTS`

- **`--auto`** (optional flag): when present, skips confirmation prompts
  (overwrite existing config, next-step handoff). Does **not** skip the
  interview — if required information is missing, the agent still asks.
- **`<inputdir>`** (first positional arg, required): path to the source CLI
  repo being ported.
- **`<outputdir>`** (second positional arg, optional): where the working tree
  is created. If omitted, the system infers it as `<inputdir>_<target.lang>`
  (e.g., `~/src/seeds` + `_go` → `~/src/seeds_go`). The target.lang is
  determined from the interview during init (or from an existing config).

If `<inputdir>` is missing, STOP and state the required form
(`/cli-port:run-all <inputdir> [<outputdir>]`).

## Method

You are the **Integrator**. Orchestrate the full pipeline from a single
invocation. Once the project is initialized, do not hand off to the user
*between phases* — run each phase, check its gate, and proceed (or escalate)
autonomously.

This autonomy applies to **phase transitions only**. It does **not** authorize
skipping the Step 0 init interview. Required fields with no default — above all
`target.lang` — MUST be obtained by asking the user before any phase runs. Never
infer `target.lang` from an `outputdir` suffix, the source repo, or convention,
and never pick a default. If it is unknown, STOP and ask.

### Step 0 — Ensure config exists (init)

1. Check if `port.config.json` exists in `<outputdir>`.
2. **If it exists**: load and validate it. Proceed to Step 1.
3. **If it does not exist**: run the init steps from `/cli-port:init`:
   - Inspect `<inputdir>` to infer source fields (lang, runtime, entrypoint,
     binary).
   - **Interview the user (blocking gate).** This interview is mandatory and
     must complete before Step 1. `target.lang` has no default — you may not
     guess it, infer it from the `outputdir` suffix, or proceed without it.
     Batch the questions, propose inferred defaults for everything that has one,
     and wait for the user's answers. The `--auto` flag does NOT skip this; it
     only skips overwrite/handoff confirmations.
   - Generate `port.config.json` with sensible defaults for everything else.
   - Validate against `templates/port.config.schema.json`.
   - Scaffold the working tree (`contracts/`, `harness/`, `corpus/`, `port/`,
     `divergence-ledger.md`).
   - Verify the oracle actually runs.
   - If init fails (oracle won't run, config won't validate), STOP and
     report why — the pipeline cannot proceed without a valid config.

### Step 1 — Detect resume point

If `port.config.json` exists and some phases are already complete, auto-detect
the resume point from artifacts:

- No `contracts/INDEX.md` → start from **Phase 0 (map)**.
- `contracts/INDEX.md` exists but no `determinism.md` or `harness/` → start
  from **Phase 1 (harness)**.
- `determinism.md` and `harness/` exist but no `port/` or partial port → start
  from **Phase 2 (run)**.
- Everything present → start from **Phase 3 (verify)**.

### Step 2 — Phase 0: Map

Run `/cli-port:map` from the working tree. Gate: every command in the registry
has a complete contract (`contracts/INDEX.md` with all seven sections checked).
If the gate fails, report the gaps and STOP — ask the user whether to retry
or abort.

### Step 3 — Phase 1: Harness

Run `/cli-port:harness` from the working tree. Gate: TS-vs-TS stability test
is green across the full corpus. If not green, loop the determinism-auditor
and harness-engineer until it passes. If the auditor and engineer cannot
resolve it after two loops, STOP and report the exact entropy sources that
cannot be pinned.

### Step 4 — Phase 2: Port fan-out

Run `/cli-port:run` from the working tree. This fans out port-engineers per
command (via `workflows/port-fanout.workflow.js`), each in an isolated
worktree, with the integrator merging and re-running the full harness after
each merge. Gate: every command is byte-identical (post-canonicalization)
across the corpus. If the gate fails, report the diverging commands and
STOP — ask the user whether to retry or abort.

### Step 5 — Phase 3: Adversarial sweep

Run `/cli-port:verify` from the working tree. This runs
`workflows/adversarial-sweep.workflow.js` until consecutive dry rounds are
reached, then performs the Phase 4 sign-off. Gate: adversarial corpus is
byte-identical, `divergence-ledger.md` is empty or all entries are justified.
If a genuine irreducible divergence remains, STOP and surface it to the user
with the byte diff — do not paper it over.

### Step 6 — Final report

When all phases complete, print a compact summary:
- Commands ported (count)
- Corpus size (seed + adversarial cases)
- Total adversarial rounds run
- Divergences (open / justified / zero)
- Any residual notes from the sign-off

## Automation behavior

- **`--auto` flag**: when present, skips confirmation prompts:
  - Overwrite existing `port.config.json` without asking.
  - No "next step" handoff after each phase — just proceed.
  - If the pipeline completes successfully, print a one-line summary and
    exit rather than prompting for further action.
- **Without `--auto`**: after each phase gate passes, briefly report the
  result and proceed (no user confirmation needed — the orchestrator handles
  the flow). Only stop for gate failures that require human judgment.

## Gate failure protocol

When a gate fails, STOP and report:
1. Which phase and which gate failed.
2. The specific evidence (byte diffs, missing contracts, etc.).
3. Ask the user whether to **retry** (fix and re-run the phase) or **abort**.

Never weaken a gate to proceed. Never paper over a divergence. Be honest
about readiness.