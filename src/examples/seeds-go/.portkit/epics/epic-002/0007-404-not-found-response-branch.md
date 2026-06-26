# Slice 0007 — 404 not-found response branch (EPIC-002-S4)

**Capability:** When the store has no item with the requested id, the `GET /items/{id}` handler responds with HTTP `404 Not Found` and JSON body `{"error":"item not found"}`.

---

## 1. End-to-end behavior thread

This is the path a request takes when fetching a single item that does not exist. Each row cites the exact source location it is grounded in.

| # | Component | What it does | Source `path:line` |
|---|-----------|--------------|--------------------|
| 1 | `errors` import | Makes `errors.Is` available in the server file. | `server.go:5` |
| 2 | `id, err := strconv.Atoi(...)` then `if err != nil` | Integer-parse the path id; on parse failure this branch returns 400 and we never reach the 404 branch. (Ordering precondition — owned by slice EPIC-002-S2, included here only to fix ordering.) | `server.go:47-50` |
| 3 | `it, err := store.Get(id)` | Looks up the item; returns `ErrNotFound` when the id is absent. | `server.go:52` |
| 4 | `if errors.Is(err, ErrNotFound)` | Detects the not-found sentinel (matches it directly or any error wrapping it). | `server.go:53` |
| 5 | `writeError(w, http.StatusNotFound, err.Error())` + `return` | Writes the 404 JSON envelope and returns immediately, skipping success serialization. | `server.go:54-55` |
| 6 | `ErrNotFound = errors.New("item not found")` | The sentinel error whose message text becomes the response body's error string. | `store.go:13` |
| 7 | `func (s *Store) Get(id int) (Item, error)` | Returns `Item{}, ErrNotFound` when `id` is not in the map. | `store.go:53-60` (specifically `store.go:57-58`) |
| 8 | `writeError` / `writeJSON` | Wrap the message into `{"error": msg}`, set `Content-Type: application/json`, write the status. (Shared helpers — owned by SHARED-response-helpers, referenced not reimplemented here.) | `server.go:82-83`, `server.go:76-80` |
| 9 | Test: `GET /items/999 -> 404` | Asserts the status code only. | `server_test.go:84-86` |

---

## 2. Interface / contract (EXACT behavior)

This slice is the **not-found branch inside the `GET /items/{id}` handler**. It is not a standalone function; it is a conditional block in the handler registered at `server.go:46`.

### Inputs
- An incoming HTTP request `GET /items/{id}` where `{id}` is a path segment.
- Precondition (must already hold before this branch runs): `{id}` parsed successfully as a base-10 integer via `strconv.Atoi` (`server.go:47`). If parsing failed, control already returned with 400 at `server.go:48-50` and this branch is never reached.
- The result of `store.Get(id)` (`server.go:52`): `(Item, error)`. When no item with `id` exists, the error is `ErrNotFound` (`store.go:57-58`).

### Outputs (when the item is NOT found)
- HTTP status: `404` (`http.StatusNotFound`) — grounded `server.go:54`.
- Response body: exactly the JSON object `{"error":"item not found"}`.
  - The message string passed is `err.Error()` (`server.go:54`). `err` here is `ErrNotFound`, whose `.Error()` is the literal `item not found` (`store.go:13`).
  - `writeError` wraps it as `map[string]string{"error": msg}` (`server.go:83`), which JSON-encodes to `{"error":"item not found"}`.
  - Because `json.NewEncoder(...).Encode(...)` (`server.go:79`) is used, the encoded body ends with a trailing newline (`\n`). Treat the body as `{"error":"item not found"}\n`.
- Response header: `Content-Type: application/json` (set in `writeJSON` at `server.go:77`).
- The handler `return`s immediately (`server.go:55`); it MUST NOT fall through to the success path `writeJSON(w, http.StatusOK, it)` at `server.go:57`.

### Outputs (when the item IS found — for contrast; owned by EPIC-002-S3)
- The `errors.Is(err, ErrNotFound)` check (`server.go:53`) is false, so this branch is skipped and the handler proceeds to `writeJSON(w, http.StatusOK, it)` (`server.go:57`). This slice MUST NOT alter that path.

### Exact behavioral rules, errors, edge cases, ordering
1. **Matching uses `errors.Is`, not `==`** (`server.go:53`). Any error that wraps `ErrNotFound` also triggers the 404 branch. In practice `Store.Get` returns the sentinel directly (`store.go:58`), but the matcher MUST be `errors.Is` to preserve wrapping semantics.
2. **Status MUST be 404** (`http.StatusNotFound`), grounded `server.go:54`. Do not use any other not-found-ish code.
3. **Body error text MUST be the sentinel's message string** `item not found` (`store.go:13`). The code passes `err.Error()` (`server.go:54`); since the only error reaching this branch is `ErrNotFound`, the rendered text is `item not found`. Do not hardcode a different string and do not change the sentinel's message.
4. **Immediate return** (`server.go:55`). After writing the 404 the handler MUST return; it MUST NOT continue to serialize a (zero-value) item as 200.
5. **Ordering guarantee:** the integer-parse check (`server.go:48`) precedes the not-found check (`server.go:53`). Therefore a non-integer id (e.g. `/items/abc`) is a **400**, never a 404 (`server_test.go:87-89` confirms `/items/abc` -> 400). This 404 branch applies ONLY after a successful integer parse.
6. **Single write:** exactly one response is written. `writeError` calls `writeJSON` once (`server.go:83`), which sets the header, writes the status, and encodes the body once (`server.go:77-79`). Do not write the header or status more than once.

---

## 3. Prerequisite slices (build order)

This is build **#7** in EPIC-002.

`dependsOn: ["EPIC-002-S3", "SHARED-response-helpers"]`

- **EPIC-002-S3** — the `GET /items/{id}` success path (handler registered at `server.go:46`, the `store.Get(id)` call at `server.go:52`, and the `writeJSON(w, http.StatusOK, it)` success serialization at `server.go:57`). This slice inserts the not-found branch BETWEEN the `store.Get` call and the success write. S3 must exist first so there is a handler to add the branch to.
- **SHARED-response-helpers** — provides `writeError(w, status, msg)` (`server.go:82-84`) and `writeJSON(w, status, v)` (`server.go:76-80`). This slice CALLS `writeError`; it does not define it. Do not reimplement these helpers here.

Also implicitly required (already present from earlier EPIC-002 slices): the `ErrNotFound` sentinel (`store.go:13`) and `Store.Get` returning it (`store.go:57-58`). Do not redefine the sentinel or change its message — reference it.

---

## 4. Acceptance tests for THIS slice

Concrete, runnable-in-spirit. The handler is exercised via `httptest` (pattern at `server_test.go:11-26`). Build a server with `NewServer(NewStore())`, create an item if needed, then issue requests.

### AT-1 — Missing id returns 404 (THE grounded test, `server_test.go:84-86`)
```
srv := NewServer(NewStore())
// store is empty, so id 999 does not exist
w := do(GET, "/items/999", "")
assert w.Code == 404            // http.StatusNotFound
```
This is the ONLY existing test for this slice (`server_test.go:84-86`). It asserts status only.

### AT-2 — Missing id returns the exact JSON body `{"error":"item not found"}`  [SOURCE-GROUNDED, NOT COVERED BY ANY EXISTING TEST — add it]
```
srv := NewServer(NewStore())
w := do(GET, "/items/999", "")
assert w.Code == 404
assert strings.TrimSpace(w.Body.String()) == `{"error":"item not found"}`
// or decode into map[string]string and assert m["error"] == "item not found"
```
Grounded in `server.go:54` (passes `err.Error()`), `store.go:13` (`ErrNotFound` message), `server.go:83` (`{"error": msg}` wrapping). See the THIN coverage flag in §6.

### AT-3 — Missing id sets `Content-Type: application/json`  [SOURCE-GROUNDED, NOT COVERED BY ANY EXISTING TEST — add it]
```
w := do(GET, "/items/999", "")
assert w.Header().Get("Content-Type") == "application/json"
```
Grounded in `server.go:77`.

### AT-4 — Existing id is NOT a 404 (branch does not fire on success)
```
srv := NewServer(NewStore())
do(POST, "/items", `{"name":"findme"}`)   // creates id 1
w := do(GET, "/items/1", "")
assert w.Code == 200                        // not 404
```
Grounded in `server.go:53` (check is false for found items) and the existing success test `server_test.go:79-82`.

### AT-5 — Non-integer id is 400, not 404 (ordering)
```
w := do(GET, "/items/abc", "")
assert w.Code == 400                         // http.StatusBadRequest, not 404
```
Grounded in `server.go:48` preceding `server.go:53`, confirmed by `server_test.go:87-89`.

---

## 5. Function/unit-sized build steps (each individually checkable)

Each step is small and independently verifiable. You are editing the `GET /items/{id}` handler created in EPIC-002-S3.

1. **Ensure the `errors` package is imported** in the server file.
   - Add `"errors"` to the import block.
   - Check: import block contains `"errors"` (matches `server.go:5`). `go build ./...` succeeds.

2. **Locate the call site.** Inside the `GET /items/{id}` handler, after the id has been parsed (the `strconv.Atoi` + `if err != nil { writeError(400); return }` block), call `store.Get(id)`:
   - `it, err := store.Get(id)` (matches `server.go:52`).
   - Check: the handler has both `it` and `err` from `store.Get` in scope.

3. **Add the not-found branch** immediately after the `store.Get` call and BEFORE the success `writeJSON`:
   ```go
   if errors.Is(err, ErrNotFound) {
       writeError(w, http.StatusNotFound, err.Error())
       return
   }
   ```
   This matches `server.go:53-56` exactly.
   - Check: uses `errors.Is` (not `==`); status is `http.StatusNotFound`; message is `err.Error()`; the block ends with `return`.

4. **Confirm the success path still follows** the branch: `writeJSON(w, http.StatusOK, it)` (matches `server.go:57`) runs only when the not-found branch did not fire.
   - Check: success serialization is reachable only when `errors.Is(err, ErrNotFound)` is false.

5. **Do NOT modify** `ErrNotFound` (`store.go:13`), `Store.Get` (`store.go:53-61`), `writeError` (`server.go:82-84`), or `writeJSON` (`server.go:76-80`). They are provided by prerequisite slices.
   - Check: no diff in `store.go` helpers or the shared response helpers.

6. **Run the acceptance tests** from §4.
   - Check: AT-1 passes (`server_test.go:84-86`). Add and pass AT-2 and AT-3 (the currently-uncovered body and content-type assertions). AT-4 and AT-5 pass.
   - Verify: `go test ./...` is green.

---

## 6. Coverage flag

**LOUD COVERAGE FLAG (THIN):** The ONLY existing test exercising this slice is `server_test.go:84-86`, which asserts ONLY that `GET /items/999` returns status `404`. There is **NO existing test** asserting the exact JSON body `{"error":"item not found"}` nor the `Content-Type` for the 404 case. The body content and content-type are **[SOURCE-GROUNDED but UNVERIFIED BY ANY EXISTING TEST]** — grounded in `server.go:54`, `server.go:77`, `server.go:83`, and `store.go:13`. AT-2 and AT-3 in §4 are added to close this gap and MUST be made to pass during rebuild.

---

## 7. Kernel references (relied upon, not restated)

This slice relies on the following kernel/standard-library names, types, and conventions. Reference the kernel for their definitions; do not restate or reimplement them, and do not depend on any other slice's internals beyond the prerequisites in §3.

- `net/http`: `http.StatusNotFound` (404), `http.ResponseWriter`, `http.Request`, `(*http.Request).PathValue("id")`, `http.NewServeMux`, method+path route patterns like `"GET /items/{id}"`.
- `errors`: `errors.Is(err, target)` for sentinel/wrapped-error matching; `errors.New` (used to define the sentinel in `store.go`, not here).
- `strconv`: `strconv.Atoi` (parse precondition, owned by EPIC-002-S2).
- `encoding/json`: `json.NewEncoder(w).Encode(v)` (used inside `writeJSON`; trailing newline behavior).
- Project sentinel: `ErrNotFound` (defined `store.go:13`, message `item not found`).
- Project store API: `Store.Get(id int) (Item, error)` (defined `store.go:53-61`).
- Shared helpers (SHARED-response-helpers): `writeError(w, status, msg)` and `writeJSON(w, status, v)` — JSON envelope shape `{"error": msg}` and `Content-Type: application/json`.
- Package name: `seeds` (all files declare `package seeds`).
