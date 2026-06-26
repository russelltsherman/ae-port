# Cross-cutting conventions — rules every slice obeys

These are **rules**, not a layer to build. There is no shared library to implement
first. Each slice doc is self-contained; when a slice's behavior is governed by one of
these conventions, the slice doc states the concrete behavior and cites the rule number
here instead of re-deriving the policy. A weak model rebuilding a slice applies these
rules locally.

Every rule cites a `path:line`. Claims not grounded are marked `[UNVERIFIED]`.

---

## CC-1 — Packaging is convention, not configuration

No manifest enumerates components. A file's **location** registers it:
`commands/*.md` → slash command (name = filename stem), `agents/*.md` → subagent,
`workflows/*.js` exporting `meta` → workflow (id = `meta.name`). A plugin dir is
loadable iff it contains `.claude-plugin/plugin.json`, whose `name` MUST equal the
marketplace catalog entry's `name` or install fails to resolve. Manifest paths are
fixed (`.claude-plugin/marketplace.json`, `<plugin>/.claude-plugin/plugin.json`); a
manifest anywhere else is invisible. All manifests must be valid JSON — one parse error
fails the whole marketplace load.
(`.claude-plugin/marketplace.json:1`, `src/cli/.claude-plugin/plugin.json:1`,
`src/cli/commands/port-init.md:1`)

## CC-2 — Workflow runtime is gated and filesystem-blind

JS workflows run only when `CLAUDE_CODE_WORKFLOWS=1`; a command that drives one STOPs
and tells the user to enable it if unset
(`src/portkit/commands/portkit.md:37`). Inside a workflow there is **no filesystem
access** — any disk read/write/existence check is delegated to a subagent (e.g.
portkit's preflight probe runs `test -e`/`test -d`/`find … | wc -l` via an agent,
`src/portkit/workflows/portkit.js:267`).

## CC-3 — `args` does not forward through `{scriptPath}`; bake config (LEARNINGS A1)

When a workflow is launched by `{scriptPath}`, the `args` global is **undefined**.
Every driving command MUST overwrite the `BAKED_CONFIG` const in the script body before
launch; the workflow reads config args-first only when launched by `{name}` with
forwarded args. A workflow that finds an empty command list logs a loud "config not
baked" diagnostic and returns a no-op, so misconfiguration never reads as a clean pass.
(`src/cli/workflows/port-fanout.workflow.js:15`, `:27`;
`src/cli/workflows/adversarial-sweep.workflow.js:16`)

## CC-4 — Input normalization happens once, at a single point

A workflow normalizes its input exactly once and everything downstream assumes the
normalized shape. portkit: `args` may be object / JSON string / undefined → parse only
if string → `cfg = (typeof input==='object' && input) ? input : {}`; a bare string or
null collapses to `{}` (`src/portkit/workflows/portkit.js:21`). cli workflows: `cfg`/
`project` chosen args-first only when args is an object carrying the expected key,
else `BAKED_CONFIG` (`src/cli/workflows/port-fanout.workflow.js:22`,
`src/cli/workflows/adversarial-sweep.workflow.js:21`). Config reads use `Number(x)||def`
/ `x||fallback`, so `0`/`NaN`/empty fall back to the documented default — a slice MUST
reproduce that exact coalescing, not a stricter parse.

## CC-5 — Concurrency is bounded and order-stable

Every agent fan-out routes through a bounded pool, never raw unbounded parallelism, to
stay under runtime caps (~1000 agents, 4096 items/call, `min(16,cores-2)` in flight)
and avoid API rate limits. portkit uses `pooled(thunks, MAX_CONCURRENCY=8)`
(`src/portkit/workflows/portkit.js:92`); cli uses `parallel()` as a **barrier** so the
integrator has every branch at once (`src/cli/workflows/port-fanout.workflow.js:59`).
Both contracts: order-stable by original index, a throwing thunk → `null`, never
rejects. A `parallel`/`pooled` barrier resolves only after every thunk finishes.

## CC-6 — Truncation and caps are never silent

Any time a list is capped to a fan-out limit, the drop is logged AND recorded so it
surfaces in the final result / RISKS doc — a silent cap must never read as "complete".
portkit's `cap()` logs `⚠️` and pushes to `dropped[]`
(`src/portkit/workflows/portkit.js:74`). The adversarial sweep's `maxRounds` backstop is
reported DISTINCTLY from a genuine dry stop (`hitMaxRounds` vs `dryReached`)
(`src/cli/workflows/adversarial-sweep.workflow.js:110`).

## CC-7 — Logging conventions

Live progress via `log()`. Prefix `⚠️` for warnings/truncation, `❌` for a loud abort.
Each pipeline stage is wrapped in `phase('<Title>')` for `/workflows` progress.
(`src/portkit/workflows/portkit.js:78`, `:257`, `:295`)

## CC-8 — Loud abort over silent drift; isolated early-return shapes

A precondition failure STOPs the whole run with an explicit message and a distinct
return shape, rather than proceeding against the wrong target. portkit's preflight
aborts when the input dir is missing/empty and returns `{ok:false, error, inputDir,
preflight}` — distinct from the success `{ok:true, …}` and from the no-epics / no-slices
aborts (`src/portkit/workflows/portkit.js:280`, `:554`). cli phase gates STOP and ask
retry/abort; the gate-failure protocol requires human judgment and is never auto-skipped
(`src/cli/commands/port-run-all.md:124`).

## CC-9 — Grounding is mandatory; honesty over progress

Every nontrivial factual claim about the source cites `path:line` or is marked
`[UNVERIFIED]`; never invent behavior. The downstream consumer is a LESS CAPABLE model
rebuilding without the source, so docs are explicit, prescriptive, and exhaustive about
errors/edge-cases/ordering. This is encoded as portkit's `GROUND_RULE` appended to
nearly every agent prompt (`src/portkit/workflows/portkit.js:108`) and as the cli
"read from evidence, never optimism / re-derive don't relay" discipline
(`src/cli/commands/port-status.md:5`, `src/cli/agents/integrator.md:59`). An unverifiable
section is reported as such, never fabricated (`src/cli/agents/cartographer.md:50`).

## CC-10 — The contract is fixed; code conforms to it (never weaken the gate)

No engineer, fixer, or gap-filler reaches green by weakening the harness, a mask, a
format assertion, or any gate. The determinism contract and IO contracts are authority;
the port conforms. A genuine irreducible divergence is surfaced WITH its byte diff, never
papered over. (`src/cli/agents/port-engineer.md:1`, `src/cli/commands/port-run.md:26`,
`src/cli/commands/port-verify.md:25`, `src/cli/agents/integrator.md:41`)

## CC-11 — Determinism contract is the single source of canonicalization rules

The determinism-auditor authors the rules; the harness-engineer's canonicalizer
implements them EXACTLY and invents none of its own. Sandbox-path rewrite runs FIRST so
later masks can't match inside paths; format assertions run BEFORE masking (a mask can
hide a format bug); ordinal masking is per-capture independent (a cross-capture mismatch
IS the divergence and is never realigned away); ordering assertions run after ordinals.
JS→target semantic differences (case-folding, UTF-16-vs-byte sort, number formatting)
are **port rules**, not masks. (`src/cli/templates/harness/canonicalize.md:6`,
`src/cli/agents/determinism-auditor.md:84`)

## CC-12 — The live oracle is ground truth; goldens are advisory

The live oracle (source CLI) is always authority. The golden cache is an optimization +
regression layer keyed by a deterministic case hash, refreshable via `--refresh`, never
the sole source of truth. Engineer self-reports of "green" are advisory; the integrator's
post-merge full-harness run (re-run after EACH merge, repeatedly under load) is
authoritative. (`src/cli/templates/harness/runner.md:46`,
`src/cli/workflows/port-fanout.workflow.js:81`, `src/cli/agents/integrator.md:68`)

## CC-13 — Isolation makes parallel work safe; fixes that share a tree go serial (LEARNINGS A3/A4)

Parallel writers must not collide. Distinct output paths are safe to fan out without
isolation (portkit slice docs each have a unique path,
`src/portkit/workflows/portkit.js:450`). Port engineers each work in a dedicated git
worktree — platform auto-worktree (`isolation:'worktree'`) when available, else a
pre-created worktree per command bound by `worktreeBase`
(`src/cli/workflows/port-fanout.workflow.js:53`). Adversarial differs explore in their
OWN scratch corpus dir, never the real `corpus/`/`port/`
(`src/cli/workflows/adversarial-sweep.workflow.js:76`). When fixers must edit a shared
base tree, they run **strictly serial**, one awaited at a time
(`src/cli/workflows/adversarial-sweep.workflow.js:100`).

## CC-14 — Stability is proven under load, not green-once (LEARNINGS B5)

A single green gate is not a stable gate. Byte-stability gates (TS-vs-TS, the final
sign-off) require repeated full-gate runs under deliberate CPU load before sign-off,
because load-dependent canonicalization flakes (sibling timestamps 1ms apart shifting
ordinals) hide behind one green run. Re-run before declaring red too.
(`src/cli/agents/determinism-auditor.md:74`, `src/cli/agents/integrator.md:76`)

## CC-15 — `--auto` suppresses confirmations, never the interview

The `--auto` flag suppresses only confirmation prompts (overwrite-existing-config,
next-step handoff) and collapses success to a one-line summary. It NEVER skips the Step-0
interview: if a required no-default field (above all `target.lang`) is unknown, the agent
still STOPs and asks. `target.lang` is never inferred from an outputdir suffix, the
source repo, or convention. (`src/cli/commands/port-init.md:12`,
`src/cli/commands/port-run-all.md:13`, `:32`)

## CC-16 — Phase/build order is planning, enforced by gates, not by the fan-out

Work is ordered by dependency (foundations — types, id, store/IO, serializers — before
the commands that use them), derived from contracts + determinism hazards. The fan-out
itself runs all units in one parallel barrier; ordering is a planning concern applied
before launch and enforced by phase gates, not by per-unit sequencing inside the
workflow. (`src/cli/commands/port-run.md:10`, `src/cli/agents/integrator.md:26`)
