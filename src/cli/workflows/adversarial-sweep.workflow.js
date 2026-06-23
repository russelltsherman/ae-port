export const meta = {
  name: 'adversarial-sweep',
  description: 'Generate hostile inputs and hunt byte divergences against the harness until rounds come back dry.',
  phases: [
    { title: 'Sweep', detail: 'adversarial-differ rounds (read-only, parallel), loop until dry' },
    { title: 'Fix', detail: 'route each divergence to its owner — SERIAL in base tree, or worktree-isolated' },
  ],
}

// Config: { harnessCmd, dryRoundsToStop, maxRounds, categories }
//   harnessCmd: shell cmd that runs a SINGLE ad-hoc case through the differential
//     harness in an ISOLATED corpus dir, e.g.
//     `<harness> port <command> -corpus <scratch>/corpus -scratch <scratch>/sb`.
//     (Differs build their own one-case corpus dir; they never touch the real corpus.)
//
// ⚠️ KNOWN LIMITATION (LEARNINGS.md A1): `args` does NOT forward when launched via
// {scriptPath}, and workflow scripts have NO filesystem access. The driving
// /cli-port:verify command must BAKE config into BAKED_CONFIG below before launch
// (overwrite this const), OR launch by {name} with args. `args` is honored first.
const BAKED_CONFIG = { harnessCmd: '', dryRoundsToStop: 2, maxRounds: 12, categories: null } // <- /cli-port:verify overwrites
const cfg = (args && typeof args === 'object' && args.harnessCmd) ? args : BAKED_CONFIG
const harnessCmd = cfg.harnessCmd || 'echo "NO harnessCmd configured" && false'
const dryRoundsToStop = cfg.dryRoundsToStop || 2
const maxRounds = cfg.maxRounds || 12

// Categories. The first six were the original set; `cli_surface` and `file_effects`
// were added because they were the single most productive categories in practice
// and were previously uncovered (LEARNINGS.md B8). See templates/js-to-go-parity-hazards.md.
const categories = cfg.categories || [
  'unicode (combining, RTL, emoji, ZWJ, NUL, BOM U+FEFF, NEL U+0085, astral, NFC/NFD) in titles/labels/descriptions/queries — on BOTH read and WRITE/normalize paths',
  'quoting/escaping (embedded quotes, backslashes, literal newlines/tabs/CR, <>&, control chars, JSON-significant chars)',
  'structural extremes (empty/missing files, malformed/non-object JSONL lines, numeric/duplicate ids, huge strings, deeply nested config/plan)',
  'ordering (insertion vs sorted, UTF-16 code-unit vs byte string sort, ICU localeCompare which ignores LC_ALL, integer-index key ordering, multi-record sort stability)',
  'io_concurrency (broken pipe | head, SIGPIPE exit code, concurrent lock contention, partial writes, stdin handling)',
  'numbers (boundary ints, -0, 1e-7 exponent form, huge/overflow integers, fractional, high precision in config/extensions)',
  'cli_surface (bare invocation, -h/--help at top-level/per-command/NESTED-subcommand, required-vs-unknown error ordering, --opt=val on booleans, combined short flags -qx/-vx, -- terminator, suggestSimilar "Did you mean", empty --format=)',
  'file_effects (compare the BYTES WRITTEN to .seeds/*.jsonl and config.yaml after mutate commands, not just stdout — serialization/escaping/key-order/trim differences surface on disk)',
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
        required: ['command', 'category', 'title', 'minimizedCaseJSON', 'byteDiff', 'owner', 'reproducible'],
        properties: {
          command: { type: 'string' },
          category: { type: 'string' },
          title: { type: 'string', description: 'one-line description of the divergence' },
          minimizedCaseJSON: { type: 'string', description: 'minimal corpus-case JSON (corpus/*.json schema) that reproduces it — ready to drop in as a fixture' },
          byteDiff: { type: 'string', description: 'verbatim harness byte diff (A=oracle, B=port), incl which stream' },
          owner: { type: 'string', description: 'port-engineer (logic) | parity-specialist (serialization/format)' },
          reproducible: { type: 'boolean', description: 'true ONLY if it reproduced ≥2x AND is TS-vs-TS stable (the source itself is byte-stable run-to-run — NOT entropy)' },
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
  // SWEEP: one differ per category, concurrently, each READ-ONLY (explores in its
  // own scratch dir; does NOT edit corpus/ or port/ in parallel — see LEARNINGS.md A4).
  const roundResults = await parallel(
    categories.map((cat) => () =>
      agent(
        `Adversarial-differ round ${round}, category: ${cat}. ` +
          `Find byte divergences between the port and the live oracle that the current corpus does NOT already catch, using \`${harnessCmd}\` on ad-hoc cases you build in your OWN scratch corpus dir (never touch the real corpus/ or port/). ` +
          `For every divergence: MINIMIZE to the smallest case, and capture the verbatim byte diff. ` +
          `ENTROPY FILTER (mandatory, LEARNINGS.md B8): before reporting, confirm the finding is REPRODUCIBLE (≥2 runs) AND TS-vs-TS stable — run the case source-vs-source; if THAT diverges, the input has unmasked entropy (e.g. --timing wall-clock) and is NOT a port divergence — set reproducible=false and do not chase it. ` +
          `Note the JSONPARSE-tail trap (B8): a runtime parse error that echoes an input token in quotes survives a quote-bounded mask — prefer minimized inputs whose runtime error has no double-quote. ` +
          `Never resolve anything by weakening the harness, a mask, or a format assertion. Report only genuine, reproducible divergences (reproducible=true).`,
        { label: `sweep:r${round}:${cat.slice(0, 16)}`, phase: 'Sweep', schema: FINDINGS, agentType: 'cli-port:adversarial-differ' },
      ),
    ),
  )

  const fresh = roundResults.filter(Boolean).flatMap((r) => (r.findings || []).filter((f) => f.reproducible))
  if (fresh.length === 0) {
    consecutiveDry++
    log(`Round ${round}: dry (${consecutiveDry}/${dryRoundsToStop} consecutive).`)
    continue
  }
  consecutiveDry = 0
  allFindings.push(...fresh)
  log(`Round ${round}: ${fresh.length} reproducible divergence(s). Adding as fixtures, then fixing SERIALLY (LEARNINGS.md A4: parallel fixers in a shared base tree collide).`)

  // FIX: one fixer at a time. Parallel fixers in a shared base tree collide on
  // shared files (cli/serial/main) with no isolation/commit (LEARNINGS.md A4).
  // For large/cross-cutting fixes, prefer the worktree+integrator pattern OUTSIDE
  // this workflow (find-only sweep → cluster fixes in worktrees → integrator merge).
  for (const f of fresh) {
    await agent(
      `Add the minimized case for the "${f.command}" divergence (${f.category}) to corpus/ as a PERMANENT regression fixture (verify it is TS-vs-TS stable first), then fix the port so \`${harnessCmd}\` is byte-identical for it AND the full corpus. ` +
        `Title: ${f.title}. Byte diff: ${f.byteDiff}. Minimized case: ${f.minimizedCaseJSON}. ` +
        `Idiomatic target code; reuse substrate helpers; never weaken the harness, a mask, or a format assertion. Report the verbatim harness result.`,
      { label: `fix:${f.command}`, phase: 'Fix', agentType: f.owner === 'parity-specialist' ? 'cli-port:parity-specialist' : 'cli-port:port-engineer' },
    )
  }
}

log(
  `Adversarial sweep done: ${round} round(s), ${allFindings.length} total reproducible divergence(s), ` +
    `stopped after ${consecutiveDry} consecutive dry round(s)${round >= maxRounds ? ' (HIT maxRounds backstop — coverage may be incomplete; do NOT read this as fully dry)' : ''}. ` +
    `The INTEGRATOR must now confirm the FULL corpus is byte-identical and stable before sign-off (LEARNINGS.md B5: run the gate repeatedly under load).`,
)

return { rounds: round, findings: allFindings, hitMaxRounds: round >= maxRounds, dryReached: consecutiveDry >= dryRoundsToStop }
