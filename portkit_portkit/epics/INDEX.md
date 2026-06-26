# Epics → Slices Index + Build Order

This is the navigable map of the `ae-port` build kit. It lists every epic, its
normalized vertical slices, and the single deterministic **build order** (dependencies
first) a downstream model follows. Build each slice from its own doc plus `KERNEL.md`
and `kernel/cross-cutting.md` — nothing else.

- **Total epics:** 9
- **Total slices (after dedup):** 113
- Slice docs live at `epics/<slug(epicId)>/<NNNN>-<slug(name)>.md`, where `NNNN` is the
  4-digit build number from the order below (`pad(n)`, see KERNEL §4).

## Normalization / dedup

Two slices discovered under `ep-cli-run-all` were the same thread as `ep-cli-init`
leaves and are merged (their behavior is identical; same primary citations):

- `ep-cli-init-s9` (Scaffold the working tree) — `mergedFrom: [ep-cli-run-all-s07]` (both `src/cli/commands/port-init.md:76-83`).
- `ep-cli-init-s10` (Verify the oracle runs) — `mergedFrom: [ep-cli-run-all-s08]` (both `src/cli/commands/port-init.md:85-88`).

All other `ep-cli-run-all` slices are kept: they are orchestration wrappers (resume
detection, gate enforcement, `--auto` policy, STOP/ask protocols) that add behavior
beyond the wrapped phase, not exact duplicates.

---

## Epic tree (`#NN` = build number)

### ep-portkit-analyze — `/portkit` codebase-to-buildkit analysis
`src/portkit/commands/portkit.md:1`, `src/portkit/workflows/portkit.js`

| # | slice | depends on |
|---|-------|-----------|
| 1 | s01 slug() identifier normalizer | — |
| 2 | s02 pad() zero-padded sequence number | — |
| 3 | s03 cap() fan-out truncation ledger | — |
| 4 | s05 input normalization (args→cfg) | — |
| 5 | s06 SOURCE input-dir resolution | s05 |
| 6 | s07 TARGET language resolution | s05 |
| 7 | s08 OUT output-dir derivation | s06, s07, s01 |
| 8 | s09 scale-guard & concurrency knobs | s05 |
| 9 | s04 pooled() bounded-concurrency fan-out | s09 |
| 10 | s10 output schemas, GROUND_RULE, meta | — |
| 11 | s11 preflight probe + loud abort | s06, s10 |
| 12 | s12 Map phase — survey + epic inventory | s11, s03, s08 |
| 13 | s13 Discover slices + behavior spec | s12, s04, s10 |
| 14 | s14 flatten slices + attach behavior | s13, s03 |
| 15 | s15 Synthesize — dedup/kernel/build order (barrier) | s14, s10 |
| 16 | s16 behavioral-spec doc writer | s15, s14 |
| 17 | s17 Write slices — one self-contained doc/unit | s15, s04, s01, s02, s10 |
| 18 | s18 Target mapping — deps/hazards/hints | s17, s07, s03, s04, s01, s02 |
| 19 | s19 Critic + bounded gap-fill loop | s17, s16, s04, s09, s10 |
| 20 | s20 final result assembly | s19, s18 |
| 21 | s21 slash command — Workflow env gate | — |
| 22 | s22 slash command — argument parsing | s21 |
| 23 | s23 slash command — invoke workflow + report | s22, s20 |

### ep-cli-init — `/cli-port:init` config gen + scaffold
`src/cli/commands/port-init.md:1`

| # | slice | depends on |
|---|-------|-----------|
| 24 | s1 parse args + guard required inputdir | — |
| 25 | s2 inspect source → infer source.* | s1 |
| 26 | s3 interview for target/oracle/corpus | s2 |
| 27 | s4 infer outputdir when omitted | s1, s3 |
| 28 | s5 determinism defaults (envPins + sandboxPathVar) | s1 |
| 29 | s6 scan source for entropy → candidate maskRules | s2 |
| 30 | s7 validate config against schema | s2, s3, s5, s6 |
| 31 | s8 write config with overwrite protection | s4, s7 |
| 32 | s9 scaffold the working tree  *(mergedFrom run-all-s07)* | s4, s6 |
| 33 | s10 verify the live oracle runs  *(mergedFrom run-all-s08)* | s2, s3 |
| 34 | s11 report readiness + next-step handoff | s8, s9, s10 |

### ep-cli-map — `/cli-port:map` Phase 0 cartography
`src/cli/commands/port-map.md:1`, `src/cli/agents/cartographer.md:1`

| # | slice | depends on |
|---|-------|-----------|
| 35 | 01 locate working tree + load config | — |
| 36 | 02 identify registry + enumerate commands | 01 |
| 37 | 03 fan out cartographer subagents | 02 |
| 38 | 04 read source AND tests as ground truth | 03 |
| 39 | 05 write seven required contract sections | 04 |
| 40 | 06 map framework surface as IO contracts | 04 |
| 41 | 07 collect results + write contracts/INDEX.md | 05, 06 |
| 42 | 08 completeness gate before Phase 1 | 07 |
| 43 | 09 final report + Phase 1 handoff | 08 |

### ep-cli-harness — `/cli-port:harness` Phase 1 determinism + harness
`src/cli/commands/port-harness.md:1`

| # | slice | depends on |
|---|-------|-----------|
| 44 | s01 config schema + loader/validator | — |
| 45 | s15 determinism audit → determinism.md | — |
| 46 | s02 corpus case model + loader | s01 |
| 47 | s03 sandbox creation + env pin application | s01 |
| 48 | s04 step execution + capture object | s03 |
| 49 | s05 cross-step output reference resolution | s04 |
| 50 | s06 dual execution (oracle vs target) | s04, s05, s01 |
| 51 | s07 canonicalize 1 — sandbox path rewrite | s01, s04 |
| 52 | s08 canonicalize 2 — format assertions | s07 |
| 53 | s09 canonicalize 3 — ordinal masking | s08 |
| 54 | s10 canonicalize 4 — ordering assertions | s09 |
| 55 | s11 comparator + divergence reporter | s06, s10 |
| 56 | s12 golden cache with --refresh | s06, s10 |
| 57 | s13 harness CLI: single-command + ad-hoc modes | s02, s06, s11 |
| 58 | s14 TS-vs-TS stability gate (under load) | s06, s10, s11, s02 |
| 59 | s16 seed corpus generation | s02 |
| 60 | s17 Phase-1 orchestration command | s14, s15, s16 |

### ep-cli-run — `/cli-port:run` Phase 2 port fan-out
`src/cli/commands/port-run.md:1`, `src/cli/workflows/port-fanout.workflow.js`

| # | slice | depends on |
|---|-------|-----------|
| 61 | 10 dependency-ordered work sequencing | — |
| 62 | 11 bake config + launch fan-out workflow | 10 |
| 63 | 01 resolve effective config (args-first, BAKED_CONFIG) | 11 |
| 64 | 02 empty-command-list guard + early return | 01 |
| 65 | 04 engineer VERDICT report schema | — |
| 66 | 03 per-command isolation mode decision | 01 |
| 67 | 05 port-engineer prompt construction (TDD) | 03, 04 |
| 68 | 06 parallel fan-out barrier over commands | 05 |
| 69 | 07 verdict aggregation + green/not-green | 06 |
| 70 | 08 completion log (integrator authority) | 07 |
| 71 | 09 workflow return: green/notGreen handoff | 07 |
| 72 | 12 integrator post-merge reconciliation + gate | 09 |

### ep-cli-verify — `/cli-port:verify` Phase 3 adversarial sweep
`src/cli/commands/port-verify.md:1`, `src/cli/workflows/adversarial-sweep.workflow.js`

| # | slice | depends on |
|---|-------|-----------|
| 73 | S1 config resolution + defaults | — |
| 74 | S2 adversarial category set construction | S1 |
| 75 | S3 findings output schema | — |
| 76 | S5 per-category parallel differ fan-out | S1, S2, S3 |
| 77 | S6 fresh reproducible-divergence extraction | S5 |
| 78 | S7 dry-round accounting | S6 |
| 79 | S4 sweep loop control + termination | S1, S7 |
| 80 | S8 serial fix routing + fixture addition | S7 |
| 81 | S9 final summary log + return value | S4, S7 |
| 82 | S10 command entry + config baking | S1 |
| 83 | S11 Phase 4 integrator sign-off | S9 |

### ep-cli-run-all — `/cli-port:run-all` full pipeline orchestration
`src/cli/commands/port-run-all.md:1`

| # | slice | depends on |
|---|-------|-----------|
| 84 | s01 parse run-all arguments | — |
| 85 | s02 guard missing inputdir | s01 |
| 86 | s03 infer outputdir from inputdir + target.lang | s01 |
| 87 | s04 Step 0 config existence check | s03 |
| 88 | s05 mandatory target.lang interview gate | s04 |
| 89 | s06 Step 0 init — inspect source & generate config | s05 |
| 90 | s09 Step 1 resume-point detection | s04 |
| 91 | s10 Phase 0 — Map + gate | s09 |
| 92 | s11 Phase 1 — Harness + TS-vs-TS gate | s10 |
| 93 | s12 Phase 2 — Port fan-out + byte-parity gate | s11 |
| 94 | s13 Phase 3 — Adversarial sweep + sign-off | s12 |
| 95 | s14 Step 6 final report | s13 |
| 96 | s15 --auto automation behavior | s01 |
| 97 | s16 gate-failure protocol | s10, s11, s12, s13 |

*(merged out: s07 → init s9; s08 → init s10)*

### ep-cli-status — `/cli-port:status` evidence-based status
`src/cli/commands/port-status.md:1`

| # | slice | depends on |
|---|-------|-----------|
| 98 | s1 locate + validate the working tree | — |
| 99 | s2 detect current phase from artifacts | s1 |
| 100 | s7 honest harness/oracle-unrunnable reporting | s1 |
| 101 | s3 run harness + per-command pass rate | s1, s2, s7 |
| 102 | s4 summarize divergence ledger | s1 |
| 103 | s5 determine next gate + blocker | s2, s3, s4 |
| 104 | s6 render compact status table | s2, s3, s4, s5, s7 |

### ep-marketplace-install — marketplace install / plugin discovery
`.claude-plugin/marketplace.json:1`

| # | slice | depends on |
|---|-------|-----------|
| 105 | 01 root marketplace manifest | — |
| 106 | 02 catalog entry: cli plugin | 01 |
| 107 | 03 catalog entry: portkit plugin | 01 |
| 108 | 04 cli plugin manifest | 02 |
| 109 | 05 portkit plugin manifest | 03 |
| 110 | 06 cli slash-command auto-discovery | 04 |
| 111 | 07 cli subagent auto-discovery | 04 |
| 112 | 08 cli workflow auto-discovery | 04 |
| 113 | 09 portkit command + workflow auto-discovery | 05 |

---

## Build order (deterministic; dependencies first)

Kahn topological sort, ties broken by discovery order. Verified: every `dependsOn`
precedes its slice.

1. ep-portkit-analyze-s01-slug
2. ep-portkit-analyze-s02-pad
3. ep-portkit-analyze-s03-cap
4. ep-portkit-analyze-s05-args-normalize
5. ep-portkit-analyze-s06-source-resolve
6. ep-portkit-analyze-s07-target-resolve
7. ep-portkit-analyze-s08-out-derive
8. ep-portkit-analyze-s09-scale-guards
9. ep-portkit-analyze-s04-pooled
10. ep-portkit-analyze-s10-schemas-constants
11. ep-portkit-analyze-s11-preflight
12. ep-portkit-analyze-s12-map
13. ep-portkit-analyze-s13-discover
14. ep-portkit-analyze-s14-flatten
15. ep-portkit-analyze-s15-synthesize
16. ep-portkit-analyze-s16-behavioral-spec-doc
17. ep-portkit-analyze-s17-write-slices
18. ep-portkit-analyze-s18-target-mapping
19. ep-portkit-analyze-s19-critic
20. ep-portkit-analyze-s20-result
21. ep-portkit-analyze-s21-cmd-env-gate
22. ep-portkit-analyze-s22-cmd-argparse
23. ep-portkit-analyze-s23-cmd-invoke-report
24. ep-cli-init-s1
25. ep-cli-init-s2
26. ep-cli-init-s3
27. ep-cli-init-s4
28. ep-cli-init-s5
29. ep-cli-init-s6
30. ep-cli-init-s7
31. ep-cli-init-s8
32. ep-cli-init-s9
33. ep-cli-init-s10
34. ep-cli-init-s11
35. ep-cli-map-01
36. ep-cli-map-02
37. ep-cli-map-03
38. ep-cli-map-04
39. ep-cli-map-05
40. ep-cli-map-06
41. ep-cli-map-07
42. ep-cli-map-08
43. ep-cli-map-09
44. ep-cli-harness-s01
45. ep-cli-harness-s15
46. ep-cli-harness-s02
47. ep-cli-harness-s03
48. ep-cli-harness-s04
49. ep-cli-harness-s05
50. ep-cli-harness-s06
51. ep-cli-harness-s07
52. ep-cli-harness-s08
53. ep-cli-harness-s09
54. ep-cli-harness-s10
55. ep-cli-harness-s11
56. ep-cli-harness-s12
57. ep-cli-harness-s13
58. ep-cli-harness-s14
59. ep-cli-harness-s16
60. ep-cli-harness-s17
61. ep-cli-run-10
62. ep-cli-run-11
63. ep-cli-run-01
64. ep-cli-run-02
65. ep-cli-run-04
66. ep-cli-run-03
67. ep-cli-run-05
68. ep-cli-run-06
69. ep-cli-run-07
70. ep-cli-run-08
71. ep-cli-run-09
72. ep-cli-run-12
73. ep-cli-verify-S1
74. ep-cli-verify-S2
75. ep-cli-verify-S3
76. ep-cli-verify-S5
77. ep-cli-verify-S6
78. ep-cli-verify-S7
79. ep-cli-verify-S4
80. ep-cli-verify-S8
81. ep-cli-verify-S9
82. ep-cli-verify-S10
83. ep-cli-verify-S11
84. ep-cli-run-all-s01
85. ep-cli-run-all-s02
86. ep-cli-run-all-s03
87. ep-cli-run-all-s04
88. ep-cli-run-all-s05
89. ep-cli-run-all-s06
90. ep-cli-run-all-s09
91. ep-cli-run-all-s10
92. ep-cli-run-all-s11
93. ep-cli-run-all-s12
94. ep-cli-run-all-s13
95. ep-cli-run-all-s14
96. ep-cli-run-all-s15
97. ep-cli-run-all-s16
98. ep-cli-status-s1
99. ep-cli-status-s2
100. ep-cli-status-s7
101. ep-cli-status-s3
102. ep-cli-status-s4
103. ep-cli-status-s5
104. ep-cli-status-s6
105. ep-marketplace-install-01
106. ep-marketplace-install-02
107. ep-marketplace-install-03
108. ep-marketplace-install-04
109. ep-marketplace-install-05
110. ep-marketplace-install-06
111. ep-marketplace-install-07
112. ep-marketplace-install-08
113. ep-marketplace-install-09
