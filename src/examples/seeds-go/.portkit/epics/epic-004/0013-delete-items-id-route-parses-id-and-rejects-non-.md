# Slice 0013 — DELETE /items/{id} route parses id and rejects non-integer with 400

**Capability:** A `DELETE` request to `/items/{id}` is routed to a handler that parses the `{id}` path segment as a base-10 integer; a non-integer id returns HTTP 400 with a JSON error body `{"error":"id must be an integer"}` **before the store is ever touched**.

- Slice id: `EPIC-004-S3`
- Epic: `EPIC-004`
- Build number: 13 (build order: this is #13)
- Depends on: `SHARED-response-helpers`

---

## 1. End-to-end behavior thread

The flow for a `DELETE /items/{id}` request, in execution order, with the source location of each component:

1. **Route registration** — `NewServer(store *Store) http.Handler` creates a mux with `http.NewServeMux()` (`server.go:19`) and registers the route with `mux.HandleFunc("DELETE /items/{id}", func(w http.ResponseWriter, r *http.Request) {...})` (`server.go:60`). This uses Go 1.22+ method+wildcard pattern matching: the literal method `DELETE`, the literal path prefix `/items/`, and a named wildcard `{id}` that captures the final path segment.
2. **id parse** — Inside the handler, the wildcard is read with `r.PathValue("id")` and parsed with `strconv.Atoi(...)`: `id, err := strconv.Atoi(r.PathValue("id"))` (`server.go:61`).
3. **400 branch on parse error** — If `err != nil`, the handler calls `writeError(w, http.StatusBadRequest, "id must be an integer")` and `return`s immediately (`server.go:62-65`). The `return` at `server.go:64` is **before** the store-delete call at `server.go:66`.
4. **writeError helper** — `writeError(w, status, msg)` wraps the message as a JSON object: it calls `writeJSON(w, status, map[string]string{"error": msg})` (`server.go:82-84`).
5. **writeJSON helper** — `writeJSON(w, status, v)` sets `Content-Type: application/json` (`server.go:77`), writes the status code with `w.WriteHeader(status)` (`server.go:78`), then encodes the body with `json.NewEncoder(w).Encode(v)` (`server.go:79`).

> This slice is **only** responsible for routing and the non-integer 400 rejection. The 404-not-found and 204-success branches (`server.go:66-70`) belong to other slices and are out of scope here, but the handler function must still compile with them present.

Imports relied on by this thread: `encoding/json` (`server.go:4`), `errors` (`server.go:5`), `net/http` (`server.go:6`), `strconv` (`server.go:7`).

---

## 2. Interface / contract

### Inputs
- An HTTP request with method `DELETE` and a path matching `/items/{id}`, where `{id}` is the single final path segment (e.g. `/items/1`, `/items/xyz`).

### Function signature (the slice surface)
- `func NewServer(store *Store) http.Handler` (`server.go:18`) — returns the mux (an `http.Handler`) with the `DELETE /items/{id}` route registered.

### Outputs and EXACT behavior

For the non-integer case (the scope of this slice):

- **Parse rule:** `{id}` is parsed via `strconv.Atoi`. `strconv.Atoi` succeeds only for an optional leading `+`/`-` sign followed by base-10 digits, with no surrounding whitespace and within the platform `int` range. It returns a non-nil error for, e.g.:
  - non-numeric strings such as `"abc"`, `"xyz"`
  - decimals such as `"1.5"`
  - the empty string `""`
  - values out of `int` range (numeric-overflow error)
- **WHEN `strconv.Atoi` returns a non-nil error:**
  - Response status code: **400 Bad Request** (`http.StatusBadRequest`) — `server.go:63`.
  - Response header: `Content-Type: application/json` — set by `writeJSON` at `server.go:77`.
  - Response body: the JSON serialization of `map[string]string{"error": "id must be an integer"}`, i.e. exactly `{"error":"id must be an integer"}`. Because the body is produced by `json.NewEncoder(...).Encode(...)` (`server.go:79`), the encoder appends a trailing newline (`\n`) to the body. The literal message string `"id must be an integer"` is at `server.go:63`.
  - The handler **returns immediately** (`server.go:64`); no further code in the handler runs.

### Ordering guarantee (CRITICAL)
- The id-validation branch executes **before** any store interaction. The early `return` at `server.go:64` precedes the `store.Delete(...)` call at `server.go:66`. A malformed id therefore **never** reaches the store. Do not reorder: parse → on-error-return → only-then store.

### Error-message exactness
- The 400 message is the literal `id must be an integer` (no trailing period, lowercase) — `server.go:63`. The JSON key is exactly `error` — `server.go:83`.

---

## 3. Prerequisite slices (build order)

- `SHARED-response-helpers` — provides `writeJSON` (`server.go:76-80`) and `writeError` (`server.go:82-84`). This slice **calls** `writeError`; it must exist first. Treat these helpers as a fixed dependency surface:
  - `writeJSON(w http.ResponseWriter, status int, v any)` — sets `Content-Type: application/json`, writes `status`, JSON-encodes `v`.
  - `writeError(w http.ResponseWriter, status int, msg string)` — emits `{"error": msg}` with the given status via `writeJSON`.
- Do **not** reimplement these helpers in this slice; depend only on the documented contract above, not on any other slice's internals.

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec. The first is the verbatim source test; the rest are derived and concrete.

**AT-1 (source-verified) — non-integer id returns 400.**
Source test: `server_test.go:120-122`. Build a server over a fresh store, send `DELETE /items/xyz` with an empty body, assert the recorder's status code is `400`.

```go
func TestDeleteItem_NonIntegerID_Returns400(t *testing.T) {
    srv := NewServer(NewStore())
    r := httptest.NewRequest("DELETE", "/items/xyz", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, r)
    if w.Code != http.StatusBadRequest {
        t.Fatalf("DELETE non-int code = %d, want 400", w.Code)
    }
}
```

**AT-2 (derived, grounded in source code) — 400 body and Content-Type are exact.**
[UNVERIFIED by the source test: `server_test.go:120-122` asserts ONLY the status code, not the body or Content-Type. The body shape and message below are grounded in source code (`server.go:63`, `server.go:77`, `server.go:82-84`) only.]
Send `DELETE /items/abc`; assert status 400, header `Content-Type: application/json`, and body equal to `{"error":"id must be an integer"}` (allowing a trailing newline from `json.Encoder`).

```go
func TestDeleteItem_NonIntegerID_ErrorBody(t *testing.T) {
    srv := NewServer(NewStore())
    r := httptest.NewRequest("DELETE", "/items/abc", nil)
    w := httptest.NewRecorder()
    srv.ServeHTTP(w, r)
    if w.Code != http.StatusBadRequest {
        t.Fatalf("code = %d, want 400", w.Code)
    }
    if ct := w.Header().Get("Content-Type"); ct != "application/json" {
        t.Errorf("Content-Type = %q, want application/json", ct)
    }
    if got := strings.TrimSpace(w.Body.String()); got != `{"error":"id must be an integer"}` {
        t.Errorf("body = %q, want %q", got, `{"error":"id must be an integer"}`)
    }
}
```

**AT-3 (derived, grounded in source code) — store is not touched on a malformed id.**
[PARTIALLY UNVERIFIED: the source test does not separately assert the store was untouched; grounded in source code only via the early `return` at `server.go:64` preceding `store.Delete` at `server.go:66`.]
With a store containing exactly one item (id 1), send `DELETE /items/xyz` and assert status 400, then assert the item with id 1 still exists (e.g. via the store's read path). The malformed-id request must not have deleted anything.

**AT-4 (derived) — other non-integer forms also yield 400.**
Repeat AT-1 for paths `/items/1.5` and `/items/` (empty segment); each must yield status 400. (Grounded in the `strconv.Atoi` semantics at `server.go:61` and the unconditional 400 branch at `server.go:62-64`.)

---

## 5. Build steps (each individually checkable)

1. **Ensure imports.** In `server.go`, the `import` block must include `encoding/json`, `errors`, `net/http`, `strconv` (`server.go:4-7`). Check: file compiles with these present.
2. **Create the mux.** Inside `NewServer(store *Store) http.Handler`, create `mux := http.NewServeMux()` (`server.go:19`). Check: `mux` is an `*http.ServeMux`.
3. **Register the DELETE route.** Call `mux.HandleFunc("DELETE /items/{id}", func(w http.ResponseWriter, r *http.Request) { ... })` (`server.go:60`). The pattern string must be exactly `DELETE /items/{id}` (method, single space, path with `{id}` wildcard). Check: a `DELETE /items/1` request reaches this handler rather than 404-ing at the mux.
4. **Parse the id.** As the first statement in the handler body, write `id, err := strconv.Atoi(r.PathValue("id"))` (`server.go:61`). The wildcard name in `PathValue` (`"id"`) must match the route wildcard `{id}`. Check: a numeric segment parses without error.
5. **Reject non-integer with 400 and return.** Immediately after the parse, add `if err != nil { writeError(w, http.StatusBadRequest, "id must be an integer"); return }` (`server.go:62-65`). The message must be the literal `id must be an integer`. Check: `DELETE /items/xyz` yields status 400 (AT-1).
6. **Preserve ordering.** Confirm the `return` (step 5) appears textually before any `store.Delete(...)` call. No store call may precede the parse-error return. Check: AT-3 (store untouched on malformed id).
7. **Return the mux.** Ensure `NewServer` ends with `return mux` (`server.go:73`) so the registered route is exposed. Check: `NewServer(NewStore())` returns a non-nil `http.Handler` that routes `DELETE /items/{id}`.
8. **Rely on the response helpers (do not redefine).** Confirm `writeError`/`writeJSON` from `SHARED-response-helpers` are in scope and unchanged (`server.go:76-84`). Check: the 400 body is `{"error":"id must be an integer"}` with `Content-Type: application/json` (AT-2).

---

## 6. Kernel references

This slice relies on the following standard-library kernel names/types/conventions. Reference only — do not restate or reimplement them:

- `net/http` — `http.Handler`, `http.ResponseWriter`, `*http.Request`, `http.NewServeMux()`, `(*http.ServeMux).HandleFunc`, `(*http.Request).PathValue`, `http.StatusBadRequest`.
- Go 1.22+ `http.ServeMux` **method + wildcard pattern** convention: `"DELETE /items/{id}"` matches method `DELETE` and binds the final segment to the `{id}` wildcard, retrievable via `r.PathValue("id")`.
- `strconv` — `strconv.Atoi(string) (int, error)`: base-10 integer parse; returns a non-nil error for non-integer or out-of-range input.
- `encoding/json` — `json.NewEncoder(w).Encode(v)` (used inside the shared `writeJSON` helper; appends a trailing newline to the stream).

Internal dependency surface (from prerequisite slice `SHARED-response-helpers`, not the kernel): `writeError(w, status, msg)` and `writeJSON(w, status, v)` as documented in section 3.
