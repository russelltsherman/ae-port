# RISKS AND GAPS — seeds-go PortKit

Audit log of gaps that could prevent a LESS CAPABLE local model from rebuilding the project
from this kit ALONE. Each entry: severity, what, where (`path:line`), why it matters, and
whether an agent can fix it without a human.

---

## Round 1 — CRITIC audit (Go source ground-truth verified)

Overall: the kit is unusually high quality. The kernel, system map, slices, and Rust target
docs are densely grounded with `path:line`, mark `[UNVERIFIED]` claims, and correctly hoist
HUMAN-DECISION-REQUIRED items (timestamp format, Unicode case-fold, id width, axum path
syntax). Sampled citations were verified against `src/examples/seeds-go` and held. The gaps
below are the exceptions.

### GAP-1 (HIGH, fixable) — Internal contradiction + factual error: method-mismatch status is 405, NOT 404 — RESOLVED

**RESOLVED**: `00-system-map.md` and `kernel/cross-cutting.md` §4 were corrected to state
**405 Method Not Allowed** for a method mismatch on a registered path and **404 Not Found**
for an unknown path, matching the verified Go behavior and the already-correct
`porting-hazards.md` H16. Original audit finding preserved below for the record.

Two kit docs asserted that an HTTP method mismatch on a registered path (e.g. `PUT /items`)
falls through to a **404** from `ServeMux`:

- `00-system-map.md:73` — "method mismatches (e.g. `PUT /items`) fall through to a default
  404 from `ServeMux`."
- `kernel/cross-cutting.md:87-88` — "Method mismatches (e.g. `PUT /items`) fall through to
  ServeMux's default 404 — no handler exists for them."

This is wrong, and it contradicts the porting doc, which correctly says 405:

- `targets/rust/porting-hazards.md:341-342` — "An unmatched method on a matched path yields
  Go's default 405; an unmatched path yields 404."

VERIFIED EMPIRICALLY against `go 1.26.4`: a `ServeMux` registered with Go 1.22+
method-prefixed patterns (`POST /items`, `GET /items/{id}`) returns **405 Method Not Allowed**
for `PUT /items` (registered path, wrong method) and **404 Not Found** for an unknown path
(`GET /nope`). The relevant registrations are `server.go:21,42,46,60`.

Why it matters: the downstream model rebuilds WITHOUT reading the Go source. The system map
and cross-cutting docs are the authoritative behavior contract; a weak model will faithfully
reproduce a 404-on-method-mismatch, diverging from the real service. The two docs also
disagree with each other, so the model has no way to resolve it from the kit alone. Neither
404 claim is `[UNVERIFIED]`-flagged — both are stated as fact.

Fix (agent, no human): change `00-system-map.md:73` and `cross-cutting.md:87-88` to state
405 for method-mismatch on a registered path and 404 for an unknown path, matching the
verified Go behavior and the already-correct `porting-hazards.md:341`. (No test exercises
405/404, so add `[UNVERIFIED by test; verified by Go ServeMux semantics]` if desired, but the
status code itself is a documented/verified Go fact.)

### GAP-2 (MEDIUM, fixable) — Missing Rust slice-hint 0013.md for EPIC-004-S3

The Rust target ships prescriptive slice-hints numbered to match the epic slice files
(`epics/.../NNNN-*.md` ↔ `targets/rust/slice-hints/NNNN.md`). Hints 0001–0015 should exist;
**0013.md is absent** (`targets/rust/slice-hints/` contains 0001,0002,...,0012,0014,0015).

Slice 0013 = **EPIC-004-S3** "DELETE /items/{id} route: parse id, 400 on non-integer"
(`epics/epic-004/0013-delete-items-id-route-parses-id-and-rejects-non-.md`, grounded
`server.go:60-65`). Its GET-side twin, EPIC-002-S2 (the GET id-parse/400 slice, slice 0005),
DOES have a dedicated hint (`targets/rust/slice-hints/0005.md`), so the omission is an
inconsistency, not a deliberate merge. EPIC-004-S3 appears only as a *dependency line* inside
0014.md (`targets/rust/slice-hints/0014.md:3,28,72`) and 0015.md, never as its own
prescriptive build doc.

Why it matters: the neutral slice doc exists and is complete, and `porting-hazards.md` H9/H11
plus the parallel 0005.md cover the same id-parse-to-400 pattern, so a strong model could
recover it. But a LESS CAPABLE model following the per-slice hint sequence will hit a missing
file at build step #15 and has no Rust-specific prescription for the DELETE id-parse branch.

Fix (agent, no human): author `targets/rust/slice-hints/0013.md` for EPIC-004-S3 mirroring
0005.md (GET id-parse) but for the DELETE route — extract `{id}` via axum `Path<String>`,
`parse::<i64>()`, on error return `400 {"error":"id must be an integer"}` BEFORE the store is
touched (`server.go:61-64`), per porting-hazards H9 (`porting-hazards.md:181-204`) and H11.

### GAP-3 (LOW, not a defect — recorded for completeness) — Untested behaviors are flagged but remain assertion-free in the rebuilt suite

The kit already and correctly flags every untested behavior with `[UNVERIFIED]`:

- Empty-store `List()` → non-nil `[]` is a source fact, not test-backed
  (`epics/epic-003/0009-*.md:66`, grounded `store.go:67`). No HTTP empty-list test exists
  either (`server_test.go:92-108` creates 2 items before listing).
- Concurrency / mutex safety is never exercised by any test (flagged across the store slices,
  e.g. `epics/epic-001/0002-*.md:259`); grounded only by source (`store.go:20,39-40`).
- Over-length name at the HTTP layer (400 via `ErrNameTooLong`) is verified only at the store
  layer, not via HTTP (`epics/epic-001/0003-*.md:514-515,544-546`).
- The 500 "internal error" branch is unreachable with the in-memory store and untested
  (`epics/epic-001/0003-*.md:547-550`, grounded `server.go:35-36`).
- Error-response *body* shape (`{"error":...}`) is asserted by no existing test — tests check
  only status codes (`epics/epic-001/0003-*.md:554-556`).

This is correct handling, NOT a gap in the kit's honesty. Recorded only so a rebuilder knows
these are spec-derived, not test-derived. Each slice's "Acceptance tests" section already
suggests adding the missing assertions; a rebuilder SHOULD add them to harden parity.
Fixable=true in the sense that an agent can add the suggested tests, but no kit document needs
correcting.

---

### Verification notes (what the critic checked against source)

- Citations sampled and confirmed: `item.go:14-17` (JSON tags incl. camelCase `createdAt`),
  `store.go:56-60` (Get returns by value), `store_test.go:18` (whitespace-only case),
  `server_test.go:67-70` (case-insensitive duplicate → 409), `server.go:11-17` (route doc
  comment). All matched the kit's claims.
- Confirmed NO test references `CreatedAt`/`createdAt` (`grep` over both `_test.go` files) —
  the kit's repeated claim that the timestamp is test-invisible is accurate
  (`porting-hazards.md:142-145`, `dependency-map.md:726-730`).
- Confirmed every epic slice file (15) has an "Acceptance tests" section and a `dependsOn`.
- Confirmed the build order (`epics/INDEX.md:69-89`) is a valid topological order with no
  dangling `dependsOn`: every referenced slice (`KERNEL-item-model`,
  `SHARED-response-helpers`, and EPIC-00x-Sy) is defined.
- No capability lacks slices: all four epics (create/get/list/delete) plus the two kernel
  slices are present and accounted for.

---

## Round 2 — CRITIC audit (re-verification after fix agents)

Overall: all three Round-1 gaps are VERIFIED RESOLVED against `src/examples/seeds-go`
(ground truth re-read). The kit is now in very good shape for a less-capable rebuilder. One
new minor defect found — in the audit log itself, not the rebuild-facing kit.

### Re-verification of Round-1 gaps (all RESOLVED)

- **GAP-1 (405 vs 404) — RESOLVED.** No incorrect "method-mismatch → 404 / falls through"
  claim remains anywhere in the rebuild-facing kit. All three docs now agree on the verified
  Go semantics: `00-system-map.md:72-79` (405 for method mismatch on a registered path, 404
  for an unknown path), `kernel/cross-cutting.md:87-93` (same), and `porting-hazards.md` H16
  (`porting-hazards.md:341-342`). `grep` for `fall through|default 404|mismatch.*404` over
  `.portkit/` returns hits ONLY inside this RISKS log (documenting the resolved gap) and in
  unrelated handler-control-flow "fall through to 204/200" narratives (correct usage).
  Registrations re-confirmed at `server.go:21,42,46,60`.
- **GAP-2 (missing slice-hint 0013) — RESOLVED.** `targets/rust/slice-hints/0013.md` now
  exists (15 hints total, 0001–0015 contiguous). Content is the correct EPIC-004-S3 DELETE
  id-parse/400 prescription, explicitly mirroring 0005.md and grounded in `server.go:60-65`,
  with accurate cross-refs to `porting-hazards.md:166-176` (H8) and H9.
- **GAP-3 (thin coverage) — RESOLVED in the live suite.** The previously assertion-free
  behaviors now have concrete, passing tests: `TestStoreListEmptyIsNonNil`
  (`store_test.go:111-120`) and `TestListItemsHTTPEmpty` (`server_test.go:113-123`) lock the
  non-nil `[]` guarantee at both layers; `TestStoreConcurrentAccess` (`store_test.go:125-148`)
  exercises the mutex; `TestCreateItemHTTPOverLongName` (`server_test.go:163-177`) covers the
  over-length-name 400 at the HTTP layer; `TestErrorResponseBodyShape`
  (`server_test.go:181-265`) asserts the exact `{"error":<msg>}` envelope (single key) across
  six error outcomes; `TestListItemsHTTPAscendingOrder` and `TestListItemsHTTPContentType`
  add HTTP-layer ordering/Content-Type assertions. `go test ./...` passes; `go vet ./...`
  clean. The 500 "internal error" branch remains unreachable/untested (correctly, and still
  flagged) — this is inherent to the in-memory store, not a kit defect.

### GAP-4 (LOW, fixable) — Stale/uncheckable line citation INSIDE this RISKS log

`RISKS-AND-GAPS.md:110` cites `dependency-map.md:726-730` for the "timestamp is
test-invisible" claim, but `dependency-map.md` is only **272 lines** long (verified
`wc -l`), so that citation cannot be resolved. The actual supporting content is at
`dependency-map.md:218-221` (inside the §9 `time` HUMAN-DECISION note: "The existing tests …
NEVER assert on `CreatedAt`/`createdAt` … verified: no test references …"). The companion
citation in the same line, `porting-hazards.md:142-145`, IS correct (H6 confirms the same
test-invisibility). Impact: NONE on a rebuilder — this is in the critic's own audit log, not
in any kernel/slice/target doc the downstream model builds from. Recorded for hygiene only.
Fix (agent, no human): change `dependency-map.md:726-730` → `dependency-map.md:218-221` at
`RISKS-AND-GAPS.md:110`.

### Round-2 verification notes (what the critic re-checked against source)

- Re-grounded sampled citations: `go.mod:1-3` (3-line file, no `require`), `store.go:67`
  (`make([]Item, 0, ...)` non-nil slice), `store.go:71` (`sort.Slice` by ID asc),
  `server.go:79` (`json.NewEncoder(w).Encode(v)` — trailing newline), `item.go:22-23`
  (sentinel strings), `server.go:31,33,53,66` (`errors.Is` branches). All matched the kit.
- Re-confirmed the slice-file ↔ slice-id ↔ build-order mapping: files 0001–0015 map to
  EPIC-001-S3..EPIC-004-S5; build-order steps 3–17 in `epics/INDEX.md:69-89` are offset by
  the two kernel items (steps 1–2). No off-by-one defect; the numbering is internally
  consistent.
- Re-confirmed no dangling `dependsOn`: every referenced id (`KERNEL-item-model`,
  `SHARED-response-helpers`, all `EPIC-00x-Sy`) is defined in `INDEX.md`. The merged/promoted
  ids (`EPIC-001-S1/-S2`, `EPIC-002-S5`, `EPIC-003-S2`) are documented in the normalization
  notes (`INDEX.md:7-22`), explaining the non-contiguous slice-number suffixes.
- Re-confirmed every one of the 15 epic slice files has an "Acceptance tests" section; the
  Rust hint cross-references to `porting-hazards.md` section line ranges (e.g. H8 at 166–176,
  H12 at 259–273, H14 at 300–314) resolve correctly.
- HUMAN-DECISION-REQUIRED items remain correctly hoisted and not silently guessed
  (`dependency-map.md:262-272`): timestamp format/timezone, Unicode case-fold parity, id
  width, axum path-param syntax + `Json` extractor rejection body.
