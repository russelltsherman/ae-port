---
description: "Phase 2 — fan out port engineers to port each module/command to byte-parity, TDD against the harness."
---

Run Phase 2 (Port fan-out). Run this from the working tree `/cli-port:init`
created (the directory that contains `port.config.json`); every path below is
relative to it. Precondition: the Phase 1 gate (TS-vs-TS stable) is green. If it
is not, stop and run `/cli-port:harness` first.

1. Order the work by dependency: port foundations first (types, ID generation,
   the store/IO layer, the serializers owned by the **parity-specialist**), then
   the commands that build on them. Derive the order from the contracts and
   `determinism.md`'s ordering hazards.

2. Drive the fan-out with the Workflow tool using the shipped script
   `workflows/port-fanout.workflow.js` (invoke via its `scriptPath`). It
   pipelines each command through port → verify: a **port-engineer** (in an
   isolated worktree) makes the harness green for that command, then the harness
   re-runs to confirm byte-parity across the corpus. The **parity-specialist** is
   on call for any serialization divergence.

3. As engineers finish, the **integrator** reconciles their worktrees, re-runs
   the full harness after each integration (a merge can reintroduce a
   divergence), and updates `divergence-ledger.md`.

**Gate:** every command is byte-identical (post-canonicalization) across the
corpus. Report per-command pass rate from the harness — not from engineer
self-reports. Never reach green by weakening the harness, a mask, or a format
assertion.

Next step: `/cli-port:verify`.
