# Slice EPIC-002-S1 — Route registration and path id extraction

**Capability:** A GET request to `/items/{id}` is routed to a dedicated handler, and the `{id}` path segment is extracted as a raw string (no transformation) so a downstream slice can parse it.

Build order: this is slice **#4**. `dependsOn: []` (no prerequisite slices — it depends only on the kernel).

---

## 1. Scope boundary (read first)

This slice owns EXACTLY two things:

1. Creating the `net/http` `ServeMux` inside `NewServer` and registering the literal route pattern `"GET /items/{id}"` on it (`server.go:18`, `server.go:19`, `server.go:46`).
2. Extracting the `{id}` wildcard inside that handler as a raw string via `r.PathValue("id")` (`server.go:47`).

This slice does **NOT** own: the numeric parse of the id, the store lookup, the success/error response bodies, or the other three routes' handler logic. In the real source, `server.go:47` parses the extracted string with `strconv.Atoi` on the same line; the parse + lookup + status mapping belong to other EPIC-002 slices. For THIS slice, the deliverable is: the route is registered with the exact method+pattern, and the matched segment is read with `PathValue("id")` and handed off **unaltered** (not lowercased, trimmed, or otherwise modified) to the parsing step. Grounded: `server.go:47` calls `r.PathValue("id")` directly inside `strconv.Atoi(...)` with no intervening transformation.

> The acceptance tests for this repo exercise the route through the full handler (status codes 200/404/400). To make those tests pass you will necessarily also wire the downstream parse + lookup. The build steps below describe the full handler body as it exists in source, but flag which lines are THIS slice's responsibility vs. handed off.

---

## 2. End-to-end behavior thread

Each component with its source `path:line`:

1. **`NewServer` constructor** — `server.go:18`. Signature `func NewServer(store *Store) http.Handler`. Entry point; everything below happens inside it.
2. **`ServeMux` instantiation** — `server.go:19`. `mux := http.NewServeMux()`. Creates the router.
3. **`GET /items/{id}` route registration** — `server.go:46`. `mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) { ... })`. Registers the method-scoped pattern.
4. **Path wildcard extraction** — `server.go:47`. `r.PathValue("id")` returns the string captured by the `{id}` wildcard. This is the slice's core capability.
5. **Handler returned as `http.Handler`** — `server.go:73`. `return mux`. The `*http.ServeMux` satisfies `http.Handler`, so the function returns it directly.

Package is named `seeds` (`server.go:1`).

---

## 3. Interface / contract — EXACT behavior

### 3.1 `NewServer`
- Signature: `func NewServer(store *Store) http.Handler` (`server.go:18`).
- Returns the configured `*http.ServeMux` (which implements `http.Handler`) (`server.go:73`).
- Takes a `*Store` (kernel type) and closes over it inside the handler closures. For THIS slice the handler reads from `store` via `store.Get(id)` after the id is parsed (`server.go:52`), but the store interaction is a downstream concern — see Scope boundary.

### 3.2 The route pattern string — must be EXACT
- The literal pattern registered is the string `"GET /items/{id}"` (`server.go:46`). Reproduce it character-for-character: HTTP method `GET`, one space, then `/items/`, then the wildcard `{id}`.
- This uses Go 1.22+ `net/http.ServeMux` enhanced pattern syntax (method + path with `{name}` wildcards). The kernel records Go version `1.26` (`KERNEL.md`, `go.mod:3`), so this syntax is available.
- `{id}` is a **named single-segment wildcard**: it matches exactly ONE non-empty path segment and binds it to the name `id`. It does NOT match across `/` (a multi-segment match would require `{id...}`, which is NOT used here).

### 3.3 Method specificity / dispatch ordering (guarantees)
- Routing is method-specific: only HTTP **GET** requests reach this handler. A request with a different method on the same path shape is a SEPARATE route. The `DELETE /items/{id}` route is registered independently (`server.go:60`); GET and DELETE do not share a handler. Grounded: `server.go:46` (GET) vs `server.go:60` (DELETE).
- This pattern coexists with `"GET /items"` (the list route, `server.go:42`) and `"DELETE /items/{id}"` (`server.go:60`). `ServeMux` dispatches by method and by the most specific matching pattern:
  - `GET /items` (no trailing segment) → list handler (`server.go:42`), NOT this handler.
  - `GET /items/1` (one trailing segment) → this handler (`server.go:46`).
- Registration order in source is: `POST /items` (`server.go:21`), `GET /items` (`server.go:42`), `GET /items/{id}` (`server.go:46`), `DELETE /items/{id}` (`server.go:60`). `ServeMux` precedence is by pattern specificity, not registration order, so the order is not load-bearing for correctness — but reproduce it as written for fidelity.

### 3.4 Path value extraction — EXACT behavior
- Inside the handler, obtain the id segment with `r.PathValue("id")` (`server.go:47`). The argument string `"id"` MUST match the wildcard name in the pattern (`{id}`) exactly; a mismatch returns the empty string.
- `PathValue("id")` returns the matched segment as a `string`, URL-path-decoded by the mux per standard `net/http` behavior. For inputs like `/items/1`, `/items/999`, `/items/abc`, it returns `"1"`, `"999"`, `"abc"` respectively.
- **Do NOT transform the returned string** before handing it to the parser: do not lowercase, uppercase, trim whitespace, or strip characters. The source passes it directly into `strconv.Atoi` (`server.go:47`). Any transformation would change behavior and is out of contract.

### 3.5 Downstream behavior reached through this route (verified by tests, but owned by other slices)
The acceptance tests prove the route is reached by asserting the downstream status codes. For reference (these statuses are produced by parse/lookup logic, not by this slice):
- Non-integer id (e.g. `abc`) → `strconv.Atoi` errors → 400 with body `{"error":"id must be an integer"}` (`server.go:48-50`).
- Integer id not present in store → `ErrNotFound` → 404 (`server.go:53-55`).
- Integer id present → 200 with the JSON item (`server.go:57`).

If you are building ONLY this slice in isolation, the minimal observable contract is: "a `GET /items/<segment>` request invokes a handler that can read `<segment>` verbatim via `PathValue("id")`."

---

## 4. Kernel references (do not restate the kernel)

This slice relies on these kernel-defined names/types/conventions (see `KERNEL.md`):
- **Package name `seeds`** — all library code lives in package `seeds` at the module root (`server.go:1`).
- **`*Store`** — the kernel persistence type; `NewServer` takes one and the handler closure reads from it. Built via `NewStore()`. (Kernel: Type glossary / Public surface.)
- **`http.Handler`** — the return type of `NewServer` (standard library; kernel records it as the public surface the binary depends on).
- The id-parse downstream behavior uses the kernel sentinel **`ErrNotFound`** (`store.go:13`) — but matching/mapping it is a downstream slice's job, not this one.

Do NOT depend on any other slice's internals. This slice needs only: the `seeds` package, the `*Store` type/constructor (kernel), and the standard library (`net/http`, `strconv`).

---

## 5. Acceptance tests for THIS slice

Derived from the behavioral spec and `server_test.go`. Concrete and runnable-in-spirit. The repo's test driver is:

```go
// from server_test.go:11-26 (helpers you can rely on)
func newTestServer() http.Handler { return NewServer(NewStore()) }
func do(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
    // builds an httptest request, records the response, returns the recorder
}
```

### AT-1 — GET /items/{id} is routed and the id reaches the handler (verified, status 200)
Grounded by `server_test.go:75-82`.
```
srv := NewServer(NewStore())
// create one item so id 1 exists
do(t, srv, "POST", "/items", `{"name":"findme"}`)   // server_test.go:77
w := do(t, srv, "GET", "/items/1", "")               // server_test.go:79
assert w.Code == 200                                 // server_test.go:80-82
```
Proves: `GET /items/1` matches the `GET /items/{id}` pattern (`server.go:46`) and the handler reads `id="1"` via `PathValue` (`server.go:47`).

### AT-2 — non-numeric id segment still reaches the handler (verified, status 400)
Grounded by `server_test.go:87-89`.
```
w := do(t, srv, "GET", "/items/abc", "")
assert w.Code == 400
```
Proves: the wildcard captures the non-numeric segment `"abc"` (route is reached); the downstream parse rejects it. If the route were NOT registered, the mux would return 404, not 400 — so a 400 specifically confirms this slice's route + extraction worked.

### AT-3 — numeric-but-missing id reaches the handler (verified, status 404)
Grounded by `server_test.go:84-86`.
```
w := do(t, srv, "GET", "/items/999", "")
assert w.Code == 404
```
Proves: `999` is captured and parsed (route reached), then the store lookup misses.

### AT-4 — method specificity (derived from spec; not a direct repo test for GET)
The `{id}` route is GET-only. A `DELETE /items/{id}` is a separate route (`server.go:60`). Confirm GET and DELETE dispatch independently:
```
// DELETE on the same path shape is handled by a DIFFERENT route
do(t, srv, "POST", "/items", `{"name":"temp"}`)
w := do(t, srv, "DELETE", "/items/1", "")
assert w.Code == 204   // server_test.go:114-116 — proves DELETE route is separate from GET route
```

### AT-5 — list route vs single-item route do not collide (derived)
```
do(t, srv, "POST", "/items", `{"name":"a"}`)
wList := do(t, srv, "GET", "/items", "")
assert wList.Code == 200   // GET /items -> list handler (server.go:42), not the {id} handler
```
Grounded by `server_test.go:97-100`. Confirms `GET /items` (no segment) does NOT fall into the `{id}` handler.

> NOTE (from spec): No test directly inspects the extracted STRING value of `{id}`. The captured value is verified only indirectly via downstream status codes (400 for `abc`, 404 for `999`, 200 for `1`). If you want a direct unit assertion of extraction, you may add one (not present in source): register a throwaway mux with `GET /items/{id}` whose handler writes `r.PathValue("id")` to the response body, then assert the body equals the segment verbatim including case — e.g. request `/items/AbC` returns body `AbC`. This directly proves no transformation occurs.

---

## 6. Build steps (function/unit-sized, each individually checkable)

1. **Create the package + file.** In package `seeds` (`server.go:1`), add `server.go` importing `net/http` (and `encoding/json`, `errors`, `strconv` for the full handler). Checkable: `go build` compiles.

2. **Declare `NewServer`.** `func NewServer(store *Store) http.Handler {` (`server.go:18`). It receives the kernel `*Store`. Checkable: signature matches the kernel public surface exactly.

3. **Create the mux.** `mux := http.NewServeMux()` (`server.go:19`). Checkable: `mux` is an `*http.ServeMux`.

4. **Register the GET-single route with the EXACT pattern.** `mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) { ... })` (`server.go:46`). Checkable: a `GET /items/1` request reaches this handler (not 404 from the mux). Verify the pattern string is `"GET /items/{id}"` byte-for-byte.

5. **Extract the wildcard.** As the first action in the handler, read the segment: `r.PathValue("id")` (`server.go:47`). The name argument is `"id"`, matching `{id}`. Checkable (isolated): a handler that echoes `PathValue("id")` returns the request's last segment verbatim, including original case (e.g. `/items/AbC` → `AbC`).

6. **Hand the raw string to the downstream parser unaltered (boundary).** Pass the value directly into `strconv.Atoi` with no intervening transformation: `id, err := strconv.Atoi(r.PathValue("id"))` (`server.go:47`). Checkable: the captured string is not lowercased/trimmed/modified before parsing — `/items/ABC` and `/items/abc` both reach the parser as-is (both fail Atoi → 400, confirming the route was reached for non-numeric input).

7. **Return the mux.** `return mux` (`server.go:73`). The `*http.ServeMux` satisfies `http.Handler`. Checkable: `NewServer(NewStore())` returns a non-nil `http.Handler` that serves the routes above.

8. **Do NOT register `GET /items/{id}` in a way that shadows `GET /items`.** Keep `GET /items` (list, `server.go:42`) as a distinct registration. Checkable: AT-5 — `GET /items` returns 200 from the list handler, never the single-item handler.

9. **Keep the route GET-only.** Do not collapse it with the `DELETE /items/{id}` route (`server.go:60`); register them as two separate patterns. Checkable: AT-4 — DELETE on `/items/1` is handled by the delete route (204), independent of the GET route.

10. **Run the acceptance tests.** `go test ./...` — `TestGetItemHTTP` (`server_test.go:75-90`) must pass (200/404/400), and `TestListItemsHTTP` / `TestDeleteItemHTTP` must still pass, proving no route collision. Checkable: all green.
