---
description: Report cli-port status from evidence — corpus pass rate, current phase, open divergences, next gate.
---

Report the current state of the faithful port. Read from evidence, not optimism.
Run this from the working tree `/cli-port:init` created (the directory that
contains `port.config.json`); every path below is relative to it.

1. Determine the current phase from what artifacts exist: `contracts/` (Phase 0),
   `determinism.md` + `harness/` (Phase 1), `port/` progress (Phase 2),
   adversarial fixtures in `corpus/` (Phase 3).

2. Run the differential harness across the full corpus and report the per-command
   byte-parity pass rate from its actual output.

3. Summarize `divergence-ledger.md`: open vs justified divergences.

4. State the next gate and what blocks it.

Output a compact status table: phase, corpus pass rate, open divergences, next
gate. If the harness or oracle can't run, say so plainly rather than reporting a
stale or assumed status.
