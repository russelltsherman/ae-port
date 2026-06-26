# EPIC-004-S4 — DELETE /items/{id} returns 204 No Content on successful delete

**Capability:** A `DELETE /items/{id}` request whose `{id}` is a valid integer that exists in the store removes that item and returns HTTP `204 No Content` with an empty response body.

This is build **#14**. It completes the success (happy) path of the DELETE handler. The 400 (non-integer id) and 404 (missing id) branches of the same handler are owned by other slices; this slice owns only the `nil`-from-`Delete` fall-through to `204`.

---

## 1. End-to-end behavior thread

All paths are relative to `src/examples/seeds-go`. Each component below cites its source location.

| # | Component | Source | Role in this slice |
|---|-----------|--------|--------------------|
| 1 | `DELETE /items/{id}` route handler registered on the mux | `server.go:60` | Entry point. Registered with Go 1.22+ method+pattern routing (`http.NewServeMux`, `server.go:19`). The `{id}` wildcard is read via `r.PathValue("id")`. |
| 2 | id parse: `strconv.Atoi(r.PathValue("id"))` | `server.go:61` | Converts the path segment to `int`. On this slice's path it SUCCEEDS (id is a valid integer), so `err == nil` and the early `400` return at `server.go:62-64` is NOT taken. (The 400 branch belongs to a different slice.) |
| 3 | `store.Delete(id)` call inside the `errors.Is(...)` condition | `server.go:66` | Calls `Store.Delete(id)` exactly once and tests its returned error. |
| 4 | `Store.Delete` success path returning `nil` | `store.go:76-84`, specifically the success return at `store.go:82-83` | When the id exists, deletes the map entry and returns `nil` (provided by prerequisite slice EPIC-004-S1). |
| 5 | `errors.Is(nil, ErrNotFound)` evaluates to `false` | `server.go:66` | Because `Delete` returned `nil`, the condition is false; the `404` branch (`server.go:67-68`) is SKIPPED. |
| 6 | `204 No Content` fall-through: `w.WriteHeader(http.StatusNoContent)` | `server.go:70` | Writes ONLY the status line. No body is written; `writeJSON` is NOT called. The handler then returns normally (end of function, `server.go:71`). |

**Narrative (grounded):** After the id parses successfully (`server.go:61`, no early return), the handler calls `store.Delete(id)` and tests its result with `errors.Is(store.Delete(id), ErrNotFound)` (`server.go:66`). When `Delete` returns `nil` — the item existed and was removed (`store.go:79-83`) — the `errors.Is` check is `false`, the `404` branch (`server.go:67-68`) is skipped, and execution falls through to `w.WriteHeader(http.StatusNoContent)` (`server.go:70`), sending status `204`. NO body is written and `writeJSON` is NOT called, so the response has an empty body and the handler sets NO `Content-Type` (unlike the `400`/`404` paths, which route through `writeError` → `writeJSON`, `server.go:82-83` / `server.go:76-79`, and DO set `Content-Type: application/json`). The handler then returns normally. Net effect: the item is gone from the store and the client receives `204 No Content` with an empty body.

---

## 2. Interface / contract — EXACT behavior

### HTTP route

- **Method + pattern:** `DELETE /items/{id}` (`server.go:60`). Registered on an `http.ServeMux` (`server.go:19`); the `{id}` is a path wildcard read with `r.PathValue("id")` (`server.go:61`).
- **Request body:** ignored. No decoding occurs on the DELETE path.

### Inputs

- `{id}` path segment — a string captured by the mux wildcard.

### Decision logic (exact order — reproduce precisely)

1. Parse `id`: `id, err := strconv.Atoi(r.PathValue("id"))` (`server.go:61`).
   - If `err != nil` (id is NOT a valid integer): `writeError(w, http.StatusBadRequest, "id must be an integer")` then `return` (`server.go:62-64`). **[Owned by another slice — out of scope here, but you MUST keep this branch present and ordered first so this slice's success path is reachable only for integer ids.]**
2. Call delete and test: `if errors.Is(store.Delete(id), ErrNotFound) { ... }` (`server.go:66`).
   - If `Delete` returns an error that **is** `ErrNotFound` (id not present): `writeError(w, http.StatusNotFound, ErrNotFound.Error())` then `return` (`server.go:67-68`). **[404 path — owned by EPIC-004-S5; keep present.]** Note it passes `ErrNotFound.Error()` (the literal sentinel message), not `err.Error()` (`server.go:67`).
   - If `Delete` returns `nil` (id present, item removed): condition is `false`; fall through.
3. **THIS SLICE'S OUTPUT:** `w.WriteHeader(http.StatusNoContent)` (`server.go:70`). Handler returns.

### Output for the success path (this slice)

- **Status code:** `204` (`http.StatusNoContent`) — `server.go:70`.
- **Body:** EMPTY. Nothing is written after the status header. `writeJSON` is NOT invoked on this path. (`server.go:70-71`.)
- **`Content-Type` header:** NOT set by the handler. The success path does not call `writeJSON`/`writeError`, which are the only places `Content-Type: application/json` is set (`server.go:77`). Go's `net/http` may or may not infer a default; the handler itself sets none. *(Grounded: handler sets no header on this path; the absence of any `w.Header().Set` call before `WriteHeader` at `server.go:70`.)*
- **Side effect:** the item with that id is removed from the store (`store.go:82`). A subsequent operation on the same id behaves as "not found".

### Edge cases & guarantees

- **Exactly one `Delete` call:** `store.Delete(id)` appears once, inside the `errors.Is` condition (`server.go:66`). Do NOT call it twice (e.g., once to check, once to delete). The single call both deletes and reports.
- **Idempotency boundary:** deleting an existing id returns `204`; deleting the same id again returns `404` (the second call finds nothing — `store.go:79-80`). Confirmed transitively by test (`server_test.go:117-118`).
- **id is sequential, starts at 1, never reused:** deletes do NOT decrement `nextID` (`store.go:28,48,82`; kernel "id" vocabulary). So after deleting id 1, a future create gets the next id, not 1.
- **Ordering:** the integer-parse branch (`server.go:61-64`) MUST come before the delete (`server.go:66`); otherwise a non-integer id would reach `Delete`. The `404` test (`server.go:66-68`) MUST come before the `204` fall-through (`server.go:70`); otherwise a missing id would return `204`.

### `Store.Delete` contract relied upon (provided by EPIC-004-S1 — do not reimplement)

- Signature: `func (s *Store) Delete(id int) error` (`store.go:76`).
- Returns `nil` when the id existed and was removed (`store.go:82-83`).
- Returns `ErrNotFound` when the id was absent (`store.go:79-80`).
- Thread-safe: acquires `s.mu` for the duration (`store.go:77-78`).

---

## 3. Prerequisite slices (build order)

This slice is **#14** and `dependsOn: ["EPIC-004-S1", "EPIC-004-S3"]`.

- **EPIC-004-S1** — provides `Store.Delete(id) error` with the success path returning `nil` (`store.go:76-84`). Without it the `nil` return that this slice's fall-through depends on does not exist.
- **EPIC-004-S3** — provides the earlier part of the DELETE handler: route registration and the integer-parse / `400` branch (`server.go:60-64`). This slice extends that handler's body with the `Delete`-call test and the `204` fall-through (`server.go:66-70`).
- **Kernel** (build first, before any slice): the `Item` type, `Store` struct + `NewStore`, the `ErrNotFound` sentinel, and the `writeJSON`/`writeError` helpers. See `.portkit/KERNEL.md`.

> Do NOT depend on the internals of any slice other than via the contracts above. Treat `Store.Delete` as an opaque function with the documented return behavior.

---

## 4. Acceptance tests for THIS slice

From the behavioral spec; concrete and runnable-in-spirit. The canonical test is `TestDeleteItemHTTP` (`server_test.go:110-123`); the assertion specific to this slice is at `server_test.go:114-116`.

**Test harness available** (`server_test.go:11-26`):
- `newTestServer()` → `NewServer(NewStore())` (`server_test.go:11-13`).
- `do(t, srv, method, path, body)` builds an `httptest` request (nil body when `body == ""`), records the response, returns `*httptest.ResponseRecorder` (`server_test.go:15-26`).

**AC-1 — successful delete returns 204 (PRIMARY, verified by test):**
- Setup: `POST /items` with body `{"name":"temp"}` → creates an item with integer id `1` (ids start at 1: `store.go:28,46`; create is exercised at `server_test.go:112`).
- Action: `DELETE /items/1`.
- Assert: response `Code == 204` (`http.StatusNoContent`).
- Source: `server_test.go:114` (`do(..., "DELETE", "/items/1", "")` and asserts `204`); handler `server.go:70`.

**AC-2 — empty body on success (UNVERIFIED by existing test):**
- After `DELETE /items/1`, the response body is empty (zero bytes); no JSON is written.
- Source: `server.go:70` writes only the status header and returns; no body write on the success path.
- `[UNVERIFIED by test: server_test.go:114-116 asserts only the status code, not that the body is empty. Empty-body behavior is grounded in source code only (server.go:70-71). A new assertion — e.g. require recorder.Body.Len() == 0 — would cover it.]`

**AC-3 — delete actually removes the item (verified transitively):**
- After `DELETE /items/1` returns `204`, a second `DELETE /items/1` returns `404` (the item is gone).
- Source: `server_test.go:117-118` (second delete asserts `404`); this transitively confirms the first delete removed the item from the store (`store.go:82`). The `404` branch itself is EPIC-004-S5.

**Out of scope for this slice (other slices' tests, keep handler branches intact so they pass):**
- `DELETE /items/xyz` → `400` (`server_test.go:120-121`).
- `DELETE /items/1` (when absent) → `404` (`server_test.go:117-118`).

**Run command:** `go test ./...` from `src/examples/seeds-go` (standard-library-only module, `KERNEL.md` "Module / package facts"). To run just this test: `go test -run TestDeleteItemHTTP ./...`.

---

## 5. Build steps (function/unit-sized, each individually checkable)

> Precondition: kernel and prerequisite slices exist. The DELETE handler already has its registration and integer-parse/`400` branch from EPIC-004-S3, and `Store.Delete` exists from EPIC-004-S1. You are adding the delete-call test and the `204` fall-through.

**Step 1 — Confirm `Store.Delete` is available and returns `nil` on success.**
- File: `store.go`. Expect `func (s *Store) Delete(id int) error` returning `nil` after `delete(s.items, id)` (`store.go:76-84`).
- Check: a unit call `NewStore()` → create an item (or directly insert) → `Delete(thatID)` returns `nil`; deleting again returns `ErrNotFound`.

**Step 2 — In the `DELETE /items/{id}` handler, after the integer-parse block, add the delete-and-test condition.**
- File: `server.go`, inside the handler body that begins at `server.go:60`, immediately after the `400` early-return block (`server.go:61-64`).
- Add exactly:
  ```go
  if errors.Is(store.Delete(id), ErrNotFound) {
      writeError(w, http.StatusNotFound, ErrNotFound.Error())
      return
  }
  ```
  (matches `server.go:66-68`). The `404` arm is EPIC-004-S5's concern, but the single `store.Delete(id)` call here is what performs the delete on the success path — it must be present and called exactly once.
- Check: imports include `errors` and `net/http` (`server.go:4-6`). The `ErrNotFound` sentinel resolves (kernel / `store.go:13`).

**Step 3 — Add the `204` fall-through as the final statement of the handler.**
- File: `server.go`, after the `if` block from Step 2.
- Add exactly:
  ```go
  w.WriteHeader(http.StatusNoContent)
  ```
  (matches `server.go:70`). Do NOT write a body. Do NOT call `writeJSON` or `writeError`. Do NOT set any header. The handler returns at the end of its function body (`server.go:71`).
- Check: the success path writes only the status header.

**Step 4 — Verify ordering.**
- Confirm statement order inside the handler: (a) parse id → `400` on error; (b) `if errors.Is(store.Delete(id), ErrNotFound)` → `404`; (c) `w.WriteHeader(http.StatusNoContent)`. This is the exact order at `server.go:61-70`. Wrong order breaks correctness (see "Ordering" in §2).

**Step 5 — Compile and run the acceptance test.**
- `go build ./...` then `go test -run TestDeleteItemHTTP ./...` from `src/examples/seeds-go`.
- Expect AC-1 (`204`) and AC-3 (`404` on second delete) to pass (`server_test.go:114-118`).

**Step 6 (optional, closes AC-2's gap) — add an empty-body assertion.**
- After the `204` assertion in the test, assert the recorder body length is `0` (e.g. `if w.Body.Len() != 0 { t.Errorf(...) }`). This makes AC-2 verified rather than source-only. `[This assertion does not exist in the current test (server_test.go:114-116); adding it is the recommended way to fully cover this slice's contract.]`

---

## 6. Kernel references (do NOT restate the kernel; rely on it)

See `.portkit/KERNEL.md`. This slice relies on these kernel names/types/conventions:

- **Package:** `seeds` (library package at module root; `KERNEL.md` "Module / package facts").
- **`Store` struct + `NewStore() *Store`** — the shared in-memory store (`store.go:19-29`; kernel "Type glossary"). This slice calls `Store.Delete` on it.
- **`ErrNotFound` sentinel** — `errors.New("item not found")` (`store.go:13`; kernel "Sentinel error glossary"). Matched with `errors.Is`, NOT `==` (kernel note; `server.go:66`). Maps to HTTP `404`.
- **`writeError(w, status, msg)` helper** — `server.go:82-83` (kernel "response helpers"). Used by the `404` arm only; the `204` success path does NOT use it.
- **`writeJSON(w, status, v)` helper** — `server.go:76-79` (kernel). Sets `Content-Type: application/json` then status then encodes. The `204` success path does NOT call it, which is exactly why the success response has no body and no `Content-Type` set by the handler.
- **Convention — `errors.Is` for sentinel matching** (`KERNEL.md` sentinel section; `server.go:66`).
- **Convention — id is sequential, starts at 1, never reused** (`store.go:28,48,82`; kernel "Domain vocabulary: id").
- **Standard library only** — no third-party deps (`KERNEL.md`); use `net/http`, `errors`, `strconv` (`server.go:3-8`).
