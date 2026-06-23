---
description: Inspect a source CLI, interview for the rest, generate port.config.json into the output dir, and scaffold the working tree.
argument-hint: <inputdir> <outputdir>
---

Initialize a cli-port project by **generating** its `port.config.json` from the
source repo plus a short interview, then scaffolding the working tree. This sets
up everything the rest of the workflow operates on; it does not port any code yet.

Arguments: `$ARGUMENTS`

- **inputdir** (first arg, required): path to the source CLI repo being ported.
  Must exist. init inspects it to infer the source fields — do not ask the user
  for anything that can be read from here.
- **outputdir** (second arg, required): where the generated `port.config.json`
  and the working tree are written. May name a directory that does not exist yet
  — create it. This is how the port's output is directed somewhere of the user's
  choosing, without editing any file by hand.

If either argument is missing, STOP and state the required form
(`/cli-port:init <inputdir> <outputdir>`) — do not guess paths.

Steps:

1. **Inspect `inputdir`** and infer the source fields, reporting what was detected
   and from which evidence (do not assume — read it):
   - `source.repo` = the absolute path to `inputdir`.
   - `source.lang` / `source.runtime` — from the manifest and lockfiles
     (e.g. `package.json` + `bun.lock` → typescript/bun; `go.mod` → go;
     `pyproject.toml` → python).
   - `source.entrypoint` — the main/`bin` target (e.g. `package.json` `bin`).
   - `source.binary` — the invoked command name (the `bin` key).
   If any source field cannot be inferred, add it to the interview rather than
   inventing it.

2. **Interview the user** for only what cannot be read from `inputdir`. Propose an
   inferred default for each and let the user confirm or override; ask concisely
   (batch the questions). Required answers:
   - `target.lang` (e.g. go) — no default.
   - `target.module` (e.g. `github.com/you/seeds-go`) — propose from the source
     repo name + target lang convention.
   - `target.binary` — default to `source.binary`.
   - `oracle.command` — propose from the detected runtime + entrypoint
     (e.g. `bun src/index.ts`); it is executed with the working directory set to
     `source.repo`. `oracle.mode` defaults to `live`.
   - `corpus` — default `corpus`.

3. **Determinism contract — sensible defaults; Phase 1 finalizes.** Do NOT make
   the user author regexes here. Write:
   - `envPins`: the standard set `NO_COLOR=1`, `FORCE_COLOR=0`, `TZ=UTC`,
     `LC_ALL=C` (offer the user a chance to add to these, don't require it).
   - `sandboxPathVar`: `<ROOT>`.
   - `maskRules`: scan the source for obvious entropy and emit **candidate** rules
     only for what is actually observed — random IDs, ISO-8601 timestamps — each
     with `mode: ordinal` and `assertFormat: true`. If none are observed, write an
     empty array. These are PROVISIONAL: state explicitly, in your report and as a
     comment in `determinism.md`, that Phase 1's determinism-auditor verifies and
     completes them. Never fabricate a mask rule for entropy you did not observe.

4. **Validate** the assembled config against `templates/port.config.schema.json`.
   If it does not validate, fix the assembly and report what was wrong — do not
   write an invalid config.

5. **Write** the config to `<outputdir>/port.config.json`. If that file already
   exists, show it and ask whether to overwrite or reuse it — never silently
   clobber an existing config.

6. **Scaffold** the working tree under `outputdir`:
   - `contracts/`        (Cartographer output)
   - `determinism.md`    (Determinism Auditor output — placeholder header noting
                          the provisional mask rules from step 3)
   - `harness/`          (Harness Engineer output)
   - `corpus/`           (seed + adversarial cases)
   - `port/`             (target-language source)
   - `divergence-ledger.md` (empty, owned by Integrator)

7. **Verify the oracle actually runs:** invoke `oracle.command` (with cwd =
   `source.repo`) on a trivial command in a throwaway sandbox and confirm it
   produces output and a clean exit. Report the captured output verbatim. If it
   fails, STOP — the live oracle is a precondition for every later phase.

8. Report the absolute `outputdir` path and direct the user to run the rest of the
   workflow from there. Every later command (`/cli-port:map`, `:harness`, `:run`,
   `:verify`, `:status`) operates on the current directory, so they must be run
   from the working tree. Print the next step as
   `cd <outputdir> && /cli-port:map` and a one-line readiness summary.

Be honest about readiness: if the oracle isn't runnable, a required field could
not be resolved, or the config did not validate, the project is NOT initialized —
say so plainly.
