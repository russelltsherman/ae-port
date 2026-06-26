# Slice EPIC-002-S2 — Integer id parsing and 400 validation error

**Capability:** A non-integer `{id}` path segment is rejected with HTTP 400 and a JSON body `{"error":"id must be an integer"}`; a valid integer is parsed and used as the store lookup key.

---

## Build metadata

- **Slice id:** EPIC-002-S2
- **Epic:** EPIC-002
- **Build number:** 5 (this is the 5th slice to build overall)
- **Depends on (build order — these MUST exist first):**
  - `EPIC-002-S1` — provides the `GET /items/{id}` route registration on the mux and the handler that this slice's parsing logic lives inside. This slice adds the integer-parse guard at the top of that handler. Do NOT re-create the route; extend the handler that S1 established.
  - `SHARED-response-helpers` — provides the `writeError` and `writeJSON` package functions. This slice CALLS `writeError` but MUST NOT redefine it or depend on its internal implementation beyond the contract restated below.

> This slice depends only on the *public contracts* of the slices above. It does not reach into their internals.

---

## 1. End-to-end behavior thread

The flow, in execution order, with the exact source location of each component (source root `src/examples/seeds-go`):

1. **`strconv` import** — `server.go:7`. The `strconv` package must be in the file's import block (`server.go:3-8`).
2. **`strconv.Atoi` on the path value** — `server.go:47`. Inside the `GET /items/{id}` handler, the captured path segment is read via `r.PathValue("id")` and passed to `strconv.Atoi`, producing `id, err := strconv.Atoi(r.PathValue("id"))`.
3. **Atoi error branch → 400** — `server.go:48`. `if err != nil {` guards the failure case.
4. **`writeError` with literal message + `return`** — `server.go:49-50`. On error, call `writeError(w, http.StatusBadRequest, "id must be an integer")` then `return` immediately. No store lookup happens on this path.
5. **Success path** — `server.go:52`. When `err == nil`, the parsed `int` value `id` flows to `store.Get(id)` (the lookup; the lookup/404 handling itself is a separate slice — see "Scope boundary" below).
6. **Test: `GET /items/abc` → 400** — `server_test.go:87-89`. The single asserting test for this slice.

---

## 2. Interface / contract — EXACT behavior

This slice operates entirely inside the `GET /items/{id}` HTTP handler registered on the `*http.ServeMux` (route established by `EPIC-002-S1`).

### Input
- An HTTP request matched to the pattern `GET /items/{id}` (Go 1.22+ `net/http` method+wildcard routing).
- The wildcard segment is retrieved with `r.PathValue("id")`, which returns a `string` (the raw, un-decoded-further path segment).

### Parsing rule
- The string MUST be parsed with `strconv.Atoi` (`server.go:47`). `strconv.Atoi(s)` is equivalent to `strconv.ParseInt(s, 10, 0)` converted to `int`: it accepts an **optionally signed, base-10** integer.

### Behavior on parse FAILURE (`err != nil`)
Ordering is strict and MUST be preserved:
1. Call `writeError(w, http.StatusBadRequest, "id must be an integer")`.
2. `return` from the handler immediately.
3. As a consequence, `store.Get` is NEVER called for invalid input.

Resulting HTTP response:
- **Status:** `400` (`http.StatusBadRequest`). Grounded: `server.go:49`.
- **Body:** the exact JSON object `{"error":"id must be an integer"}`. The literal message string `"id must be an integer"` is at `server.go:49`; the single-key envelope `map[string]string{"error": msg}` is produced by `writeError` at `server.go:83`. `json.NewEncoder(...).Encode` appends a trailing newline, so the raw body bytes are `{"error":"id must be an integer"}\n`.
- **Content-Type header:** `application/json`. Set by `writeJSON` at `server.go:77` (reached via `writeError`).
- The message text MUST be EXACTLY `id must be an integer` — no capitalization change, no punctuation, no rephrasing.

### Behavior on parse SUCCESS (`err == nil`)
- The parsed `int` value (named `id`) is passed to `store.Get(id)` (`server.go:52`). Handling of the returned item / not-found case is OUT OF SCOPE for this slice (it belongs to the lookup slice). This slice's only success obligation is: produce a valid `int` named `id` and let control fall through to the lookup with no error response written.

### Edge cases — MUST be preserved exactly (driven by `strconv.Atoi` semantics)
These all make `strconv.Atoi` return a non-nil error and therefore MUST produce the 400 response above:
- `"abc"` (non-numeric) → 400. (This is the only test-covered case: `server_test.go:87`.)
- `""` (empty captured segment) → 400.
- `"1.5"` (float syntax) → 400.
- `"0x1"` (hex / non-base-10 prefix) → 400.
- `" 1"` or `"1 "` (leading/trailing whitespace) → 400. `Atoi` does NOT trim whitespace. Do NOT add any trimming.
- A value that overflows the platform `int` (e.g. a 30-digit number) → 400 (`Atoi` returns an `*strconv.NumError` with `ErrRange`).

These MUST be accepted (parse succeeds, no error response, control proceeds to `store.Get`):
- `"+5"` — leading `+` is accepted by `Atoi`.
- `"-5"` — leading `-` is accepted by `Atoi`.
- `"007"` — leading zeros are accepted; parses to `7`.
- `"0"` — parses to `0`.

> Do not special-case any of these in code. The single `strconv.Atoi` call plus the `err != nil` guard produces every behavior above automatically. Adding extra validation would diverge from source.

---

## 3. Kernel references (relied upon; referenced, not restated)

This slice relies on the following kernel/standard-library names and conventions. Reference the kernel for their definitions; do not re-implement them here.

- **`strconv.Atoi(s string) (int, error)`** — Go standard library. Parses an optionally-signed base-10 integer. Returns a non-nil error for any non-integer or out-of-range input. (Imported at `server.go:7`.)
- **`net/http` method+wildcard routing** — `r.PathValue("id") string` returns the `{id}` wildcard. `http.StatusBadRequest` is the integer `400`. The handler signature is `func(w http.ResponseWriter, r *http.Request)`.
- **`errors` package** — present in the import block (`server.go:5`) for sibling code; not directly required by this slice's added lines.

### Contract of `SHARED-response-helpers` consumed here (DO NOT redefine)
- **`writeError(w http.ResponseWriter, status int, msg string)`** — writes `status` as the HTTP status code and a JSON body `{"error": msg}` with `Content-Type: application/json`. This slice calls it as `writeError(w, http.StatusBadRequest, "id must be an integer")`. Treat this as a black box; only its observable behavior (status + `{"error":msg}` body + JSON content type) is depended upon.

---

## 4. Scope boundary (what this slice does and does NOT do)

- **DOES:** add the `strconv.Atoi` parse + `if err != nil { writeError(...400...); return }` guard at the top of the `GET /items/{id}` handler body (`server.go:47-51`).
- **DOES:** ensure the parsed `int id` is the value handed to `store.Get(id)` (`server.go:52`).
- **DOES NOT:** define the route (`EPIC-002-S1`).
- **DOES NOT:** implement `store.Get`, the 404/not-found branch, or the 200 success serialization (separate slices).
- **DOES NOT:** define `writeError` / `writeJSON` (`SHARED-response-helpers`).

---

## 5. Acceptance tests for THIS slice

Concrete and runnable-in-spirit. The server under test is built with `NewServer(NewStore())` and exercised via `httptest` (pattern: `server_test.go:11-26`).

### AT-1 — Non-integer id returns 400 (SOURCE-GROUNDED + TEST-VERIFIED)
The only existing test for this slice. Grounded: `server_test.go:87-89`.
```
GET /items/abc
=> response status code == 400
```

### AT-2 — Exact 400 JSON body [SOURCE-GROUNDED, UNVERIFIED BY ANY EXISTING TEST]
Grounded by source (`server.go:49` + `server.go:83`) but NOT covered by any existing test. A rebuild MUST still produce this exact body.
```
GET /items/abc
=> status == 400
=> body (parsed as JSON) deep-equals {"error":"id must be an integer"}
   (raw bytes: {"error":"id must be an integer"}\n)
```

### AT-3 — 400 Content-Type [SOURCE-GROUNDED, UNVERIFIED BY ANY EXISTING TEST]
Grounded: `server.go:77` via `writeError`.
```
GET /items/abc
=> response header Content-Type == "application/json"
```

### AT-4 — Empty / float / hex / whitespace / overflow all 400 [SOURCE-GROUNDED via Atoi, UNVERIFIED]
Each of these requests MUST return status 400 with body `{"error":"id must be an integer"}`:
```
GET /items/          (empty captured segment)   => 400   [see note]
GET /items/1.5                                   => 400
GET /items/0x1                                   => 400
GET /items/%201      ( " 1" — leading space)     => 400
GET /items/99999999999999999999999999999 (overflow) => 400
```
> Note on the empty case: `GET /items/` is matched by the `GET /items` (list) route in `net/http`, not by `GET /items/{id}`, so it will NOT reach this handler. To exercise an empty `{id}` against this handler you need a request whose `{id}` wildcard is genuinely empty while still matching `/items/{id}`; in practice the realistic non-integer cases to test are the non-empty ones above. Implement the guard so that *if* an empty string ever reaches `Atoi`, it yields 400 — which it does automatically, since `strconv.Atoi("")` returns an error. Do not add special handling.

### AT-5 — Valid integer is NOT rejected and reaches lookup (SOURCE-GROUNDED; success path)
Grounded: `server.go:52`. With an item created so id 1 exists:
```
POST /items {"name":"findme"}    => 201, item ID 1
GET  /items/1                    => status != 400  (parse succeeds; flows to store.Get)
```
(The exact 200/404 outcome is owned by the lookup slice; for THIS slice the only assertion is that a valid integer does NOT trigger the 400 parse-error branch.)

> **LOUD COVERAGE FLAG (THIN):** The ONLY test exercising this slice today is `server_test.go:87-89`, asserting solely that `GET /items/abc` returns status 400. There is NO test asserting the exact JSON error body, NO test asserting the `application/json` Content-Type, and NO test for empty/float/hex/whitespace/overflow inputs. AT-2 through AT-5 are SOURCE-GROUNDED but UNVERIFIED BY ANY EXISTING TEST. A correct rebuild MUST implement the exact body and header from source even though the current test suite would not catch a wrong message or missing header. When rebuilding, ADD tests for AT-2, AT-3, and at least one case from AT-4.

---

## 6. Build steps (function/unit-sized, each individually checkable)

> Precondition: `EPIC-002-S1` has already registered the `GET /items/{id}` handler on the mux, and `SHARED-response-helpers` has already defined `writeError`/`writeJSON`.

1. **Ensure the `strconv` import.** In `server.go`, confirm `"strconv"` is in the import block (`server.go:3-8`). If absent, add it.
   - *Check:* `go build ./...` compiles; no "imported and not used" error after step 2.

2. **Parse the path value at the top of the `GET /items/{id}` handler.** As the first statement of the handler body, add:
   ```go
   id, err := strconv.Atoi(r.PathValue("id"))
   ```
   (Matches `server.go:47`.)
   - *Check:* `id` is typed `int`, `err` is `error`.

3. **Add the parse-error guard returning 400 with the exact message.** Immediately after step 2:
   ```go
   if err != nil {
       writeError(w, http.StatusBadRequest, "id must be an integer")
       return
   }
   ```
   (Matches `server.go:48-51`.) The message MUST be exactly `id must be an integer`. The `return` MUST be present so no lookup runs on failure.
   - *Check:* AT-1 passes (`GET /items/abc` → 400). AT-2 passes (body is `{"error":"id must be an integer"}`). AT-3 passes (Content-Type `application/json`).

4. **Pass the parsed `id` to the store lookup on the success path.** After the guard, the existing/next code must use `id` (the `int`) as the lookup key, e.g. `it, err := store.Get(id)` (`server.go:52`). Ensure the `int` variable — not the raw string — is what flows downstream.
   - *Check:* AT-5 passes (`GET /items/1` does not return 400).

5. **Add the missing tests (TDD obligation, given the THIN coverage flag).** Add table-driven cases asserting status 400 AND the exact body `{"error":"id must be an integer"}` AND Content-Type `application/json` for at least `"abc"`, `"1.5"`, `"0x1"`. Pattern after `do(...)` / `httptest` usage in `server_test.go:15-26`.
   - *Check:* `go test ./...` passes with the new assertions green.

---

## 7. Reference snippet (target end-state of the added lines)

For the rebuild, the handler's parse guard must end up exactly as (`server.go:46-52`):

```go
mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "id must be an integer")
        return
    }
    it, err := store.Get(id)
    // ... lookup/404/200 handling owned by other slices ...
})
```
