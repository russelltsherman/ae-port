export const meta = {
  name: 'port-fanout',
  description: 'Port each command to byte-parity, TDD against the differential harness, then verify.',
  phases: [
    { title: 'Port', detail: 'one port-engineer per command in its own worktree, commit-on-green' },
  ],
  // NOTE: byte-parity is verified by the INTEGRATOR's post-merge full-harness run
  // AFTER this workflow returns (LEARNINGS.md A4), not by a base-tree verify stage.
}

// Config: { commands: [{ name, contractPath }], harnessCmd, worktreeBase }
//   harnessCmd is invoked as `${harnessCmd} <command>` and must exit non-zero on any byte divergence.
//   worktreeBase (optional): dir holding one pre-created git worktree per command (<worktreeBase>/<name>).
//
// ⚠️ KNOWN LIMITATION (LEARNINGS.md A1): `args` does NOT forward when this
// workflow is launched via {scriptPath}, and workflow scripts have NO filesystem
// access (can't read a config file). So the driving /cli-port:run command MUST
// BAKE the config into BAKED_CONFIG below before launching (overwrite this const),
// OR launch by {name} with args if the platform forwards args that way. `args`
// is honored first when present.
const BAKED_CONFIG = { commands: [], harnessCmd: '', worktreeBase: '' } // <- /cli-port:run overwrites this
const project = (args && Array.isArray(args.commands)) ? args : BAKED_CONFIG
const commands = Array.isArray(project.commands) ? project.commands : []
const harnessCmd = project.harnessCmd || 'echo "NO harnessCmd configured" && false'
const worktreeBase = project.worktreeBase || ''

if (commands.length === 0) {
  log('No commands in args.commands or BAKED_CONFIG — nothing to port. /cli-port:run must bake the mapped command list into this script (see LEARNINGS.md A1).')
  return { ported: [], note: 'empty command list — config not baked (see LEARNINGS.md A1)' }
}

const VERDICT = {
  type: 'object',
  required: ['command', 'byteIdentical', 'branch'],
  properties: {
    command: { type: 'string' },
    byteIdentical: { type: 'boolean', description: 'true ONLY if the engineer saw the harness exit clean IN ITS WORKTREE' },
    branch: { type: 'string', description: 'git branch holding the committed port, for the integrator to merge (empty if not committed)' },
    commit: { type: 'string' },
    summary: { type: 'string', description: 'verbatim harness summary line' },
    divergence: { type: 'string', description: 'first byte diff if any, else empty' },
    filesTouched: { type: 'string', description: 'paths changed beyond the command file (for the integrator to anticipate merge collisions)' },
  },
}

// One port-engineer per command, each in its OWN git worktree, committing on a
// branch. There is NO separate base-tree "verify" stage: worktree code is not
// visible in the base tree until merged, so a base-tree verify would false-red
// every command (LEARNINGS.md A4). Authoritative verification is the INTEGRATOR's
// post-merge full-harness run, done after this workflow returns. parallel() is a
// barrier; that's fine here — the integrator needs all branches together.
//
// Worktree isolation: `isolation: 'worktree'` requires the platform auto-worktree
// feature (a git repo at session start + WorktreeCreate support). If unavailable
// (LEARNINGS.md A3), set worktreeBase and pre-create one git worktree per command;
// each engineer is bound to its absolute path (distinct path → distinct harness
// scratch → safe parallel) and we DON'T pass isolation:'worktree'.
const useAutoWorktree = !worktreeBase
const results = await parallel(
  commands.map((cmd) => () => {
    const wt = worktreeBase ? `${worktreeBase}/${cmd.name}` : ''
    const opts = { label: `port:${cmd.name}`, phase: 'Port', agentType: 'cli-port:port-engineer', schema: VERDICT }
    if (useAutoWorktree) opts.isolation = 'worktree'
    return agent(
      (wt ? `Work ONLY in your dedicated git worktree \`${wt}\` (cd there first; use absolute paths under it; never touch the base tree or other worktrees). ` : ``) +
        `Port the "${cmd.name}" command so the differential harness is byte-identical across the corpus. ` +
        `Read its contract at ${cmd.contractPath || `contracts/${cmd.name}.md`} and the relevant rules in determinism.md and port/ADDING_A_COMMAND.md (reuse substrate helpers; name any new package-level helper command-specifically to avoid cross-engineer collisions). ` +
        `Work strictly TDD: run \`${harnessCmd} ${cmd.name}\` to see the divergence (red), then make it green with idiomatic target code. Build output with insertion-ordered structures (never rely on Go map iteration for output). ` +
        `Never weaken the harness, a mask, or a format assertion — the contract is fixed; your code conforms to it. ` +
        `When \`${harnessCmd} ${cmd.name}\` exits clean, commit on your branch: \`git add -A && git commit -m "port(${cmd.name}): byte-green vs oracle"\`. ` +
        `Report byteIdentical (true only if you saw the harness exit clean), the verbatim summary line, branch=\`git rev-parse --abbrev-ref HEAD\`, commit, and filesTouched beyond the command file.`,
      opts,
    )
  }),
)

const verdicts = results.filter(Boolean)
const green = verdicts.filter((v) => v.byteIdentical && v.branch)
const notGreen = verdicts.filter((v) => !(v.byteIdentical && v.branch))

log(`Port fan-out complete: ${green.length}/${commands.length} self-reported byte-green & committed. The INTEGRATOR must now merge every branch and re-run the FULL harness — that post-merge run is authoritative, NOT these self-reports (LEARNINGS.md C-KEEP).`)

return {
  green: green.map((v) => ({ command: v.command, branch: v.branch, commit: v.commit, summary: v.summary, filesTouched: v.filesTouched })),
  notGreen: notGreen.map((v) => ({ command: v.command, byteIdentical: v.byteIdentical, branch: v.branch, divergence: v.divergence })),
  note: 'self-reports only — integrator merge + full-gate run is authoritative',
}
