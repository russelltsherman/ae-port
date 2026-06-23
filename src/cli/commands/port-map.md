---
description: "Phase 0 — fan out the cartographer to map every source command into a precise IO contract."
---

Run Phase 0 (Map) of the faithful port. Run this from the working tree
`/cli-port:init` created (the directory that contains `port.config.json`); every
path below is relative to it.

1. Identify the command registry in the source entrypoint (from
   `./port.config.json`) and enumerate every command the CLI dispatches.

2. Fan out **cartographer** subagents — one per command (or batched in small
   groups for a large surface) — to produce `contracts/<command>.md` covering the
   seven required sections: synopsis, stdout, stderr, exit codes, file effects,
   error paths, and flagged nondeterministic fields. Launch independent
   cartographers concurrently.

3. Collect results and write `contracts/INDEX.md`: every command, a one-line
   summary, and a checkbox confirming all seven sections are present.

**Gate:** every command in the registry has a complete contract. If any command
is missing or any section is unverifiable, list exactly which — do not proceed to
Phase 1 until the map is complete.

Report the command count, the contracts produced, and any gaps. Next step:
`/cli-port:harness`.
