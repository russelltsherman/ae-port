---
description: "Phase 1 — author the determinism contract and build the differential harness. Gate: TS-vs-TS byte-stable."
---

Run Phase 1 (Determinism contract + harness). This is the phase that makes the
whole port test-driven; do not let it pass on optimism. Run this from the working
tree `/cli-port:init` created (the directory that contains `port.config.json`);
every path below is relative to it.

1. Spawn the **determinism-auditor** to produce `determinism.md`: the entropy
   inventory, env pins (verified, not assumed), ordinal canonicalization rules,
   format assertions, and ordering hazards. The auditor is read-only.

2. Spawn the **harness-engineer** to build `harness/`: the sandbox runner, dual
   execution (source via oracle vs target binary), the canonicalizer implementing
   the auditor's rules exactly, the comparator/reporter, the golden cache, and a
   seed corpus exercising every command's modes, error paths, and file effects.

3. **Run the gate — the TS-vs-TS stability test.** Execute the source against a
   second run of itself across the full corpus, canonicalize both, and require
   byte-identical output. Report the result from the harness, verbatim.

**Gate:** TS-vs-TS is byte-stable across the corpus. If it is not, the
canonicalization contract or the canonicalizer is wrong — loop the auditor and
harness-engineer until it is green. **No porting begins until this gate passes**,
because without it every later divergence is indistinguishable from entropy noise.

Report: the entropy sources found, the env pins, the corpus size, and the
TS-vs-TS result. Next step (only if green): `/cli-port:run`.
