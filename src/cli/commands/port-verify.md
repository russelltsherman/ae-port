---
description: "Phase 3 — adversarial sweep. Loop the differ until rounds come back dry, then sign off."
---

Run Phase 3 (Adversarial sweep) and the Phase 4 sign-off. Run this from the
working tree `/cli-port:init` created (the directory that contains
`port.config.json`); every path below is relative to it.

1. Drive the sweep with the Workflow tool using `workflows/adversarial-sweep.workflow.js`
   (invoke via its `scriptPath`). It runs **adversarial-differ** rounds that
   generate hostile inputs — unicode, quoting/escaping, structural extremes,
   ordering, broken pipes, concurrent locks, number boundaries — against the
   differential harness. Each divergence is minimized, added to the corpus as a
   permanent regression fixture, and routed to the owning **port-engineer** or
   the **parity-specialist** to fix.

2. **Loop until dry:** keep running rounds until the configured number of
   consecutive rounds (default 2) find nothing new. Report the round count and
   the categories exercised; never let a silent cap read as full coverage.

3. **Phase 4 sign-off (integrator):** confirm the full corpus AND the adversarial
   corpus are byte-identical, target-language test coverage is at parity, and
   `divergence-ledger.md` is empty or every entry is justified. Produce a final
   report stating exactly what was verified and any residual divergences.

**Gate:** consecutive dry rounds reached and the sign-off conditions met. If a
genuine, irreducible divergence remains, surface it to the user with the byte
diff — do not paper over it. Report honestly: pass rate, rounds, open items.
