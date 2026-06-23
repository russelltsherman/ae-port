# Differential runner spec

The harness-engineer builds this. It is test infrastructure, not the port — keep
it small, deterministic, and fast. Build it in whatever language is cheapest to
keep correct (often the target language's test runner, shelling out to the
oracle).

## A test case

A case is an ordered sequence of CLI invocations sharing one sandbox, so
stateful flows (create -> reference -> close) are exercised end to end:

```
case:
  name: create-then-show
  steps:
    - args: ["create", "fix the bug", "--json"]
    - args: ["show", "$1.id"]      # may reference output of a prior step
```

Cases live under the configured `corpus/` directory. Seed cases come from the
contracts; adversarial cases are added by the adversarial-differ as regression
fixtures.

## Execution (per case, per implementation)

1. Create a fresh temp sandbox directory. Initialize identical starting state for
   both implementations (same fixture files, same `.<tool>/` layout).
2. Apply `determinism.envPins` to the process environment.
3. For each step: run `<oracle.command or target.binary> <args>` with cwd =
   sandbox. Capture stdout bytes, stderr bytes, exit code. After the step,
   capture the bytes of every file under the sandbox that the tool may touch.
4. Produce a capture object: `{ steps: [{ stdout, stderr, exit }], files: {...} }`.

## Comparison

1. Canonicalize the source capture and the target capture per
   `canonicalize.md`.
2. Byte-compare. On any difference, emit a report: case name, step index,
   stream (stdout/stderr/exit/file:path), first differing byte offset, and a
   windowed diff (hex + printable). Make it copy-pasteable.

## Golden cache

In `live` oracle mode, cache the canonicalized source capture keyed by case hash
to avoid re-running the oracle every time. Support `--refresh` to re-capture.
The live oracle remains ground truth; goldens are an optimization + regression
layer, never the sole source of truth.

## Single-command mode

Support `harness <command>` to run only the cases touching one command, exiting
non-zero on any divergence — this is what port engineers loop on for TDD and what
the workflow scripts call as `harnessCmd`.

## Self-determinism

The runner's own output must be deterministic: fixed stream/file ordering, no
unpinned time, no reliance on map iteration order. The TS-vs-TS gate
(source vs a second run of source, canonicalized, must be byte-identical) is the
proof the runner + canonicalizer are themselves correct before any port exists.
