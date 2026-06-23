export const meta = {
  name: 'port-fanout',
  description: 'Port each command to byte-parity, TDD against the differential harness, then verify.',
  phases: [
    { title: 'Port', detail: 'one port-engineer per command, isolated worktrees' },
    { title: 'Verify', detail: 're-run the harness to confirm byte-parity' },
  ],
}

// `args` is the project context the driving command passes in:
//   { commands: [{ name, contractPath }], harnessCmd: "<shell cmd that runs the harness for one command>" }
// harnessCmd is invoked as `${harnessCmd} <command>` and must exit non-zero on any byte divergence.
const project = args || {}
const commands = Array.isArray(project.commands) ? project.commands : []
const harnessCmd = project.harnessCmd || 'echo "NO harnessCmd configured" && false'

if (commands.length === 0) {
  log('No commands provided in args.commands — nothing to port. Run /cli-port:map first.')
  return { ported: [], note: 'empty command list' }
}

const VERDICT = {
  type: 'object',
  required: ['command', 'byteIdentical'],
  properties: {
    command: { type: 'string' },
    byteIdentical: { type: 'boolean' },
    divergence: { type: 'string', description: 'first byte diff if any, else empty' },
  },
}

// Pipeline: each command flows port -> verify independently, no barrier between
// stages, so a fast command verifies while a slow one is still being ported.
const results = await pipeline(
  commands,
  (cmd) =>
    agent(
      `Port the "${cmd.name}" command to the target language so the differential harness is byte-identical across the corpus. ` +
        `Read its contract at ${cmd.contractPath || `contracts/${cmd.name}.md`} and the relevant rules in determinism.md. ` +
        `Work strictly TDD: run \`${harnessCmd} ${cmd.name}\` to see the current divergence (red), then make it green with idiomatic target code. ` +
        `Honor every serialization and ordering rule. Never weaken the harness, a mask, or a format assertion. ` +
        `Return when \`${harnessCmd} ${cmd.name}\` exits clean.`,
      { label: `port:${cmd.name}`, phase: 'Port', isolation: 'worktree', agentType: 'port-engineer' },
    ),
  (_portResult, cmd) =>
    agent(
      `Verify byte-parity for the "${cmd.name}" command. Run \`${harnessCmd} ${cmd.name}\` and report the result strictly from its output. ` +
        `Set byteIdentical=true only if it exits clean with zero diff; otherwise capture the first divergence verbatim.`,
      { label: `verify:${cmd.name}`, phase: 'Verify', schema: VERDICT },
    ),
)

const verdicts = results.filter(Boolean)
const passing = verdicts.filter((v) => v.byteIdentical)
const failing = verdicts.filter((v) => !v.byteIdentical)

log(`Port fan-out complete: ${passing.length}/${commands.length} byte-identical, ${failing.length} diverging.`)

return { passing: passing.map((v) => v.command), failing }
