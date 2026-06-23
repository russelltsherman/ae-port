export const meta = {
  name: 'adversarial-sweep',
  description: 'Generate hostile inputs and hunt byte divergences against the harness until rounds come back dry.',
  phases: [
    { title: 'Sweep', detail: 'adversarial-differ rounds, loop until dry' },
    { title: 'Fix', detail: 'route each divergence to its owner' },
  ],
}

// args: {
//   harnessCmd: "<shell cmd; runs a single adversarial case through the differential harness>",
//   dryRoundsToStop: 2,        // consecutive empty rounds before stopping
//   maxRounds: 12,             // hard backstop
//   categories: [ ... ]        // adversarial categories to exercise
// }
const cfg = args || {}
const harnessCmd = cfg.harnessCmd || 'echo "NO harnessCmd configured" && false'
const dryRoundsToStop = cfg.dryRoundsToStop || 2
const maxRounds = cfg.maxRounds || 12
const categories = cfg.categories || [
  'unicode (combining, RTL, emoji, NUL, BOM, astral, normalization)',
  'quoting/escaping (quotes, backslashes, newlines, <>&)',
  'structural extremes (empty, missing files, huge, deeply nested, duplicate IDs)',
  'ordering (insertion vs sorted, locale-sensitive comparison)',
  'IO/concurrency (broken pipe | head, concurrent lock contention, partial writes)',
  'numbers (boundary ints, -0, high precision)',
]

const FINDINGS = {
  type: 'object',
  required: ['divergencesFound', 'findings'],
  properties: {
    divergencesFound: { type: 'integer' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        required: ['command', 'category', 'minimizedInput', 'byteDiff', 'owner'],
        properties: {
          command: { type: 'string' },
          category: { type: 'string' },
          minimizedInput: { type: 'string' },
          byteDiff: { type: 'string' },
          owner: { type: 'string', description: 'port-engineer | parity-specialist' },
          fixtureAdded: { type: 'boolean' },
        },
      },
    },
  },
}

const allFindings = []
let consecutiveDry = 0
let round = 0

while (consecutiveDry < dryRoundsToStop && round < maxRounds) {
  round++
  // One differ per category, concurrently, each blind to the others this round.
  const roundResults = await parallel(
    categories.map((cat) => () =>
      agent(
        `Adversarial-differ round ${round}, category: ${cat}. ` +
          `Generate hostile inputs in this category and run each through the differential harness via \`${harnessCmd}\`. ` +
          `For every byte divergence: minimize to the smallest diverging input, add it to corpus/ as a permanent regression fixture, ` +
          `and record the owner (port-engineer for logic, parity-specialist for serialization). ` +
          `Never resolve a divergence by weakening the harness, a mask, or a format assertion. ` +
          `Report only genuine, harness-confirmed divergences.`,
        { label: `sweep:r${round}:${cat.slice(0, 16)}`, phase: 'Sweep', schema: FINDINGS, agentType: 'adversarial-differ' },
      ),
    ),
  )

  const fresh = roundResults.filter(Boolean).flatMap((r) => r.findings || [])
  if (fresh.length === 0) {
    consecutiveDry++
    log(`Round ${round}: dry (${consecutiveDry}/${dryRoundsToStop} consecutive).`)
    continue
  }
  consecutiveDry = 0
  allFindings.push(...fresh)
  log(`Round ${round}: ${fresh.length} divergence(s) found and added as fixtures.`)

  // Fix stage: route each fresh divergence to its owner to fix to byte-parity.
  await parallel(
    fresh.map((f) => () =>
      agent(
        `Fix the byte divergence in the "${f.command}" command (${f.category}). Minimized input: ${f.minimizedInput}. ` +
          `Byte diff: ${f.byteDiff}. Make the harness byte-identical for this fixture and the full corpus. ` +
          `Idiomatic target code; never weaken the harness.`,
        { label: `fix:${f.command}`, phase: 'Fix', agentType: f.owner === 'parity-specialist' ? 'parity-specialist' : 'port-engineer' },
      ),
    ),
  )
}

log(
  `Adversarial sweep done: ${round} round(s), ${allFindings.length} total divergence(s) found and fixed, ` +
    `stopped after ${consecutiveDry} consecutive dry round(s)${round >= maxRounds ? ' (HIT maxRounds backstop — coverage may be incomplete)' : ''}.`,
)

return { rounds: round, findings: allFindings, hitMaxRounds: round >= maxRounds }
