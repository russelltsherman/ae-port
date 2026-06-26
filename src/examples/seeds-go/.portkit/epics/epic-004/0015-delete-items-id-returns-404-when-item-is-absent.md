# EPIC-004-S5 — DELETE /items/{id} returns 404 when item is absent

**Build #15** · Epic EPIC-004 · dependsOn: `EPIC-004-S2`, `EPIC-004-S3`

## Capability (one line)

A `DELETE /items/{id}` request whose `{id}` parses to a valid integer but does NOT
exist in the store returns HTTP **404 Not Found** with the JSON body
`{"error":"item not found"}` and `Content-Type: application/json`.

---

## End-to-end behavior thread

Components in execution order, each with its source `path:line`. Paths are relative to
`src/examples/seeds-go`.

1. **Route registration** — `DELETE /items/{id}` is registered on the
   `http.ServeMux` returned by `NewServer` (`server.go:60`). Go 1.22+ method+wildcard
   pattern routing; `{id}` is a path wildcard read via `r.PathValue("id")`.
2. **id parse (prerequisite, from S3)** — `id, err := strconv.Atoi(r.PathValue("id"))`
   (`server.go:61`). On parse failure the handler returns 400 and never reaches the
   delete branch (`server.go:62-64`). For THIS slice the id is always a valid integer,
   so parsing succeeds and execution falls through to the delete guard.
3. **Delete-with-guard** — `if errors.Is(store.Delete(id), ErrNotFound) { ... }`
   (`server.go:66`). `store.Delete(id)` is called exactly once; its returned `error`
   is fed directly into `errors.Is`.
4. **Store.Delete, absent path** — `store.Delete` locks the mutex, looks up the id in
   the map, and because the key is absent (`if _, ok := s.items[id]; !ok`) returns
   `ErrNotFound` without mutating the map (`store.go:79-80`). The map is left
   unchanged; `nextID` is untouched.
5. **404 branch** — because `errors.Is(...) == true`, the handler calls
   `writeError(w, http.StatusNotFound, ErrNotFound.Error())` and `return`s
   immediately (`server.go:67-68`). The early `return` prevents the
   `w.WriteHeader(http.StatusNoContent)` 204 fall-through at `server.go:70`.
6. **ErrNotFound message** — `ErrNotFound = errors.New("item not found")`
   (`store.go:13`). `ErrNotFound.Error()` is therefore the literal string
   `item not found`.
7. **writeError → writeJSON serialization** — `writeError` wraps the message in a
   single-key map `map[string]string{"error": msg}` and delegates to `writeJSON`
   (`server.go:82-83`). `writeJSON` sets `Content-Type: application/json`
   (`server.go:77`), writes the status code 404 (`server.go:78`), then JSON-encodes
   the map (`server.go:79`), producing body `{"error":"item not found"}`.

---

## Interface / contract — EXACT behavior

### Inputs

- HTTP method: `DELETE`.
- Path: `/items/{id}` where `{id}` is a path segment that parses as a base-10 integer
  via `strconv.Atoi` (`server.go:61`) AND is NOT a key present in the store's `items`
  map.
- Request body: ignored (none is read on the DELETE path; `server.go:60-71` never
  touches `r.Body`).

### Output (the success case for THIS slice = the 404)

| Aspect | Exact value | Source |
|---|---|---|
| Status code | `404` (`http.StatusNotFound`) | `server.go:67`, written at `server.go:78` |
| `Content-Type` header | `application/json` | `server.go:77` (set before `WriteHeader`) |
| Body | `{"error":"item not found"}` | `server.go:67,82-83`; message `store.go:13` |

- The body is the JSON encoding of `map[string]string{"error":"item not found"}`. Go's
  `encoding/json` emits the object as `{"error":"item not found"}`. The encoder used is
  `json.NewEncoder(w).Encode(v)` (`server.go:79`), which appends a trailing newline
  (`\n`) to the body. The error return of `Encode` is intentionally discarded
  (`_ =`, `server.go:79`).
- Header ordering: `Content-Type` MUST be set BEFORE `WriteHeader` is called, otherwise
  Go silently drops the header (`server.go:77` precedes `server.go:78`). Preserve this
  order.

### Behavioral rules, errors, edge cases, ordering guarantees

1. **Single Delete call.** `store.Delete(id)` is invoked exactly once and its result is
   the sole input to `errors.Is` (`server.go:66`). Do NOT call `Delete` a second time
   to re-check; that would be a behavior change.
2. **`errors.Is`, not `==`.** Matching uses `errors.Is(err, ErrNotFound)`
   (`server.go:66`), so a wrapped error still matches. Reproduce with `errors.Is`.
3. **The handler passes `ErrNotFound.Error()`, NOT the returned err.** At
   `server.go:67` the message argument is `ErrNotFound.Error()` (the package-level
   sentinel's message), NOT the `err` value returned by `store.Delete`. The resulting
   string is identical (`item not found`) because `Delete` returns that same sentinel
   (`store.go:80`), but reproduce the source exactly: pass `ErrNotFound.Error()`.
4. **Early return prevents 204.** The `return` at `server.go:68` is mandatory. Without
   it, control would fall to `w.WriteHeader(http.StatusNoContent)` (`server.go:70`)
   after the 404 was already written — a bug. Keep the early return.
5. **Mutual exclusivity with S4 (the 204 case).** For any single valid-integer id, the
   request yields EITHER 204 (item existed and was deleted — S4) OR 404 (item absent —
   this slice). Never both. The branch at `server.go:66` is the sole discriminator.
6. **No store mutation on the 404 path.** When the id is absent, `store.Delete` returns
   before reaching `delete(s.items, id)` (`store.go:79-82`); the store is unchanged.
7. **Non-integer id is out of scope here.** A non-parseable `{id}` (e.g. `xyz`) is
   handled by the 400 branch (`server.go:62-64`) owned by S3, not this slice.

---

## Prerequisite slices (build order)

This is build **#15**. It depends on:

- **`EPIC-004-S2`** — `Store.Delete` semantics: returns `ErrNotFound` when the id is
  absent and `nil` after removing a present id (`store.go:76-84`). This slice relies on
  the absent-path return value (`store.go:79-80`) but MUST NOT re-implement or restate
  `Store.Delete`'s internals beyond consuming its documented contract: `Delete(id int)
  error` returns `ErrNotFound` for an absent id.
- **`EPIC-004-S3`** — the DELETE route's id-parse step and 400-on-non-integer branch
  (`server.go:60-64`). This slice assumes id parsing has already succeeded.

Kernel artifacts (`Item`, `Store` struct + `NewStore`, sentinel errors, `writeJSON`/
`writeError`) are assumed already built per the KERNEL doc — see Kernel references.

---

## Acceptance tests for THIS slice

Concrete and runnable-in-spirit. The package is `seeds`; tests use
`net/http/httptest`. Helper pattern mirrors the existing suite: build a server with
`NewServer(NewStore())`, then drive requests with an `httptest.NewRecorder`
(`server_test.go:11-26`).

### AT-1 (test-verified) — 404 status for an absent id

Grounded in `server_test.go:117-118`. The source test creates one item (id 1),
DELETEs id 1 successfully (204), then DELETEs id 1 again. The second delete targets a
now-absent id and MUST return 404.

```
POST   /items   {"name":"temp"}     -> 201   (setup, id == 1)
DELETE /items/1                      -> 204   (setup: first delete succeeds; S4)
DELETE /items/1                      -> 404   <-- THIS slice asserts code == 404
```

Assertion: response status code == `http.StatusNotFound` (404).

A simpler equivalent (also satisfies the capability): DELETE an id that was never
created, e.g. `DELETE /items/999` against a fresh store -> 404.

### AT-2 (grounded in source only, NOT test-verified) — exact 404 body

**FLAG — THIN COVERAGE.** The source HTTP test (`server_test.go:117-118`) asserts
ONLY the status code. It does NOT assert the body or `Content-Type`. The body shape is
grounded in source code (`server.go:67,82-83`; `store.go:13`) but is NOT pinned by any
existing test.

Assertion (add this test to honor the capability): after the 404 response,
`response.Body == {"error":"item not found"}` (a trailing `\n` from `json.Encoder` is
acceptable). JSON-decode the body into `map[string]string` and assert
`m["error"] == "item not found"`.

### AT-3 (grounded in source only, NOT test-verified) — Content-Type

Assertion: the 404 response's `Content-Type` header == `application/json`
(`server.go:77`, reached via `writeError`→`writeJSON` at `server.go:83`).

### Rebuild self-check (mandatory)

Because NO existing source test pins the 404 body or Content-Type, a rebuild that
returns 404 with an empty or different body would still pass the existing test
(`server_test.go:117-118`). To honor the slice capability, the rebuild MUST emit body
`{"error":"item not found"}` with `Content-Type: application/json`, and SHOULD add
AT-2 and AT-3 to lock the contract.

---

## Build steps (function/unit-sized, each individually checkable)

Each step assumes the kernel is already built (see Kernel references). The work of
this slice is the 404 branch inside the `DELETE /items/{id}` handler.

1. **Register the route.** Inside `NewServer`, register a handler for the pattern
   `"DELETE /items/{id}"` on the mux (`server.go:60`).
   *Check:* a `DELETE /items/1` request reaches the handler (not a 405/404-from-mux).

2. **Reuse the parsed id (from S3).** Within the handler, obtain `id` via
   `strconv.Atoi(r.PathValue("id"))`, returning 400 on parse error
   (`server.go:61-64`). This is S3's responsibility; do not duplicate logic if S3
   already produced it.
   *Check:* `DELETE /items/xyz` -> 400; `DELETE /items/1` proceeds past the parse.

3. **Call Delete inside the guard.** Write `if errors.Is(store.Delete(id),
   ErrNotFound) {` (`server.go:66`). Call `Delete` exactly once; pass its result
   straight into `errors.Is` against the `ErrNotFound` sentinel.
   *Check:* for an absent id, the branch body executes; for a present id it does not.

4. **Emit the 404 inside the branch.** Inside the `if` body, call
   `writeError(w, http.StatusNotFound, ErrNotFound.Error())` then `return`
   (`server.go:67-68`). Use `ErrNotFound.Error()` as the message argument (NOT the
   `err` returned by `Delete`), matching the source exactly.
   *Check (AT-1):* second `DELETE /items/1` after a successful delete -> status 404.
   *Check (AT-2/AT-3):* body == `{"error":"item not found"}`, Content-Type
   `application/json`.

5. **Confirm the early return guards the 204 fall-through.** Ensure the `return` at
   step 4 precedes `w.WriteHeader(http.StatusNoContent)` (`server.go:70`, owned by S4)
   so the absent-id path never writes 204.
   *Check (mutual exclusivity):* a single absent-id request produces exactly one status
   write (404), never 204.

6. **Verify body/Content-Type via writeError → writeJSON (kernel).** No new code —
   rely on the kernel helpers `writeError` (`server.go:82-83`) and `writeJSON`
   (`server.go:76-80`). Confirm `writeJSON` sets `Content-Type` before `WriteHeader`.
   *Check:* AT-2 and AT-3 pass.

---

## Kernel references (reference only — do NOT restate; see `.portkit/KERNEL.md`)

This slice relies on these kernel-owned names/types/conventions. Build them per the
KERNEL doc, not here.

- **`ErrNotFound`** — sentinel `error`, message `item not found`
  (`store.go:13`; KERNEL "Sentinel error glossary"). Maps to HTTP 404. Compared with
  `errors.Is`, never `==`.
- **`Store` + `NewStore`** — the in-memory store; `Store.Delete(id int) error` is the
  method consumed here, owned by S2 per the KERNEL note that store methods belong to
  their slices (`store.go:19-29`, `store.go:76-84`; KERNEL "Type glossary").
- **`writeError(w, status, msg)`** — wraps `msg` as `{"error": msg}` and delegates to
  `writeJSON` (`server.go:82-83`; KERNEL "response envelope").
- **`writeJSON(w, status, v)`** — sets `Content-Type: application/json`, writes the
  status, then JSON-encodes `v` (`server.go:76-80`).
- **`NewServer(store *Store) http.Handler`** — the public constructor that owns the mux
  and all four routes (`server.go:18`; KERNEL "Public surface").
- **Package** — all library code is package `seeds` at the module root
  (`server.go:1`, `store.go:1`; KERNEL "Module / package facts").
- **Response-envelope convention** — error responses are the single-key JSON object
  `{"error":"<message>"}`; exact message strings are part of the HTTP contract and must
  be reproduced character-for-character (KERNEL "Sentinel error glossary", "Domain
  vocabulary").
