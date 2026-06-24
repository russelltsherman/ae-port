# cli

A Claude Code plugin: an agent team and workflow for **faithfully porting a CLI
to another language with bit-by-bit identical output**, verified by differential
testing.

## The idea

"Bit-by-bit identical output given identical input" is impossible to take
literally when the source CLI emits entropy: random IDs, wall-clock timestamps,
locale-sensitive ordering, ANSI color. So the harness runs **both** binaries on
identical input in identical sandboxes, **canonicalizes** the nondeterministic
fields (ordinally — preserving cardinality, order, and references), and then
**byte-compares**. The whole port is test-driven against that comparison.

The contract is defined up front and validated by a hard gate: **TS-vs-TS** — the
source run against a second run of itself, canonicalized, must be byte-identical
*before any port code exists*. If entropy isn't neutralized, every later
divergence is just noise.

A deliberate guard against the weakness of masking: because IDs/timestamps are
blanked, format bugs there could hide — so the harness **asserts the pre-mask
shape** (regex/length/count) before substitution. The value is masked; the format
is not.

## The team (`agents/`)

| Agent | Role |
|---|---|
| `cartographer` | Maps every source command into a precise IO contract (read-only) |
| `determinism-auditor` | Inventories entropy; authors the canonicalization contract (read-only) |
| `harness-engineer` | Builds the differential harness; turns the TS-vs-TS gate green |
| `port-engineer` | Ports one module/command to byte-parity, TDD, isolated worktree |
| `parity-specialist` | Owns byte-exact JSON/YAML/number/ANSI + the format-assertion guard |
| `adversarial-differ` | Generates hostile inputs, loops until divergences run dry |
| `integrator` | Sequences phases, enforces gates, maintains the divergence ledger |

## The workflow (`commands/`)

```
/cli:port-run-all [--auto] <inputdir> [<outputdir>]   run full pipeline (init → map → harness → run → verify)
/cli:port-init [--auto] <inputdir> [<outputdir>]      inspect source + interview -> generate config + scaffold
/cli:port-map       Phase 0 — cartographer fans out -> contracts/
/cli:port-harness   Phase 1 — auditor + harness; GATE: TS-vs-TS byte-stable
/cli:port-run       Phase 2 — Workflow fan-out: port -> verify per command (TDD)
/cli:port-verify    Phase 3 — adversarial sweep (loop until dry) + Phase 4 sign-off
/cli:port-status    report corpus pass rate, phase, open divergences, next gate
```

`init` reads the source CLI at `inputdir` to infer the source fields, interviews
you for only what it can't read (target language/module/binary, oracle command),
writes standard determinism defaults (Phase 1 finalizes the mask rules), then
**generates `port.config.json` into `outputdir`** (or `<inputdir>_<target.lang>`
if omitted) and scaffolds the working tree there. It verifies the oracle runs,
then tells you to `cd` into `outputdir`. Every later command operates on the
current directory, so run them from the working tree — there is no config to
hand-author and no path to repeat per command.

**`--auto` flag** on `init` and `run-all` skips confirmation prompts (overwrite
existing config, next-step handoff) but does not skip the interview — if required
information is missing, the agent still asks.

**`/cli:port-run-all`** orchestrates the entire pipeline from a single command:
init (if config is missing), map, harness, port fan-out, adversarial sweep, and
sign-off. It auto-detects the resume point from artifacts, enforces each gate,
and escalates on failure. Use it for fully automated runs.

Phases 2–3 are driven by the Workflow scripts in `workflows/` (`port-fanout`,
`adversarial-sweep`), invoked by the commands via `scriptPath`.

## Phases and gates

0. **Map** — every command has a complete contract.
1. **Contract + harness** — `determinism.md` written; **TS-vs-TS byte-stable**.
2. **Port fan-out** — every command byte-identical (post-canon) across the corpus.
3. **Adversarial sweep** — consecutive dry rounds reached.
4. **Sign-off** — full + adversarial corpus byte-identical; coverage at parity;
   divergence ledger empty or every entry justified.

No phase starts until the prior gate is genuinely green. Nobody reaches green by
weakening the harness, a mask, or a format assertion — an irreducible divergence
is a finding to surface, not something to paper over.

## Configure a port (`templates/port.config.schema.json`)

A `port.config.json` declares the source, the target, the live oracle command,
and the determinism contract (env pins + ordinal mask rules + format assertions).
`/cli:port-init` **generates** this file into the output dir from the source repo
plus a short interview — you don't author it by hand. See
`examples/seeds-go/port.config.json` for a finished seeds → Go instance (the shape
init produces) and `templates/port.config.schema.json` for the full contract.

## Status

This is the **scaffold**: the agent prompts, commands, workflow scripts, harness
specs, and the seeds example config. Running it (generating the config, building
the harness and corpus, and the Go port) happens when the live `bun`/`sd` oracle
is available, starting with
`/cli-port:init ~/src/github.com/jayminwest/seeds ~/src/seeds-go` — init inspects
the seeds checkout, interviews for the target, writes the config into
`~/src/seeds-go`, and you `cd` there for the rest.

## Caveat

Auto-discovery of plugin-shipped `workflows/*.js` by name is not relied upon; the
commands invoke the workflow scripts by explicit `scriptPath`. Confirm the
plugin-manifest and command/agent discovery behavior against your Claude Code
version before publishing.
