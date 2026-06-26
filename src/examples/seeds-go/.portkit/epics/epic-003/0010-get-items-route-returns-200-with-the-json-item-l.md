# Slice EPIC-003-S3 — GET /items route returns 200 with the JSON item list

**Capability (one line):** An HTTP `GET /items` request returns `200 OK` with a JSON array of all items sorted by `id` ascending; an empty store returns `[]` (a JSON empty array), never `null`.

- Build number: **#10**
- Epic: EPIC-003
- dependsOn: `["EPIC-003-S1", "SHARED-response-helpers"]`

---

## 1. End-to-end behavior thread

The request flows through these components, in order. Each is cited to its exact source location (source root: `src/examples/seeds-go`).

1. **`NewServer(store *Store) http.Handler`** — constructs the router and returns it as an `http.Handler`.
   - `server.go:18` — signature `func NewServer(store *Store) http.Handler`.
   - `server.go:19` — `mux := http.NewServeMux()`.
   - `server.go:73` — `return mux`.

2. **Route registration `GET /items`** — registers the list handler on the mux using a Go 1.22+ method-prefixed pattern.
   - `server.go:42` — `mux.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {`.

3. **Handler body** — unconditionally serializes the full item list at 200.
   - `server.go:43` — `writeJSON(w, http.StatusOK, store.List())`.
   - `server.go:44` — closing `})` of the handler func.

4. **`Store.List()`** — provides the sorted slice that becomes the response body.
   - `store.go:64` — `func (s *Store) List() []Item`.
   - `store.go:67` — `out := make([]Item, 0, len(s.items))` (non-nil empty slice when the store is empty).
   - `store.go:71` — `sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })` (ascending by ID).
   - `store.go:72` — `return out`.

5. **`writeJSON(w, status, v)`** — sets the header, writes the status, and JSON-encodes the value.
   - `server.go:76` — `func writeJSON(w http.ResponseWriter, status int, v any)`.
   - `server.go:77` — `w.Header().Set("Content-Type", "application/json")`.
   - `server.go:78` — `w.WriteHeader(status)`.
   - `server.go:79` — `_ = json.NewEncoder(w).Encode(v)`.

---

## 2. Interface / contract — EXACT behavior

### Request
- **Method + path:** `GET /items`. The route pattern string is **exactly** `"GET /items"` (`server.go:42`), relying on Go 1.22+ `http.ServeMux` method-prefixed routing.
- **Inputs consumed by the handler:** NONE. The handler reads no request body, no query parameters, and no path parameters (`server.go:43` is the entire body — it ignores `r` completely). A reimplementation MUST NOT validate, parse, or branch on anything in the request.

### Response — success (the only outcome)
- **Status:** `200 OK` (`http.StatusOK`) — `server.go:43`. This route has **no error branches**; it always returns 200.
- **Header:** `Content-Type: application/json` — set by `writeJSON` at `server.go:77`. Test-backed by `TestListItemsHTTPContentType` (`server_test.go:153-159`).
- **Body:** A JSON **array** (`[...]`), not an object, not a single item. Each element is a serialized `Item` object.
- **Element shape** (from struct tags at `item.go:13-18`):
  - `id` — integer (`item.go:14`, tag `json:"id"`).
  - `name` — string (`item.go:15`, tag `json:"name"`).
  - `done` — boolean (`item.go:16`, tag `json:"done"`).
  - `createdAt` — RFC3339 timestamp string, Go's default `time.Time` JSON encoding (`item.go:17`, tag `json:"createdAt"`).

### Ordering guarantee
- Array elements MUST be ordered by `id` **ascending** (numerically increasing). This comes from `Store.List()` sorting with `out[i].ID < out[j].ID` (`store.go:71`).
- Because IDs are assigned sequentially starting at 1 in creation order (`store.go:28` sets `nextID: 1`; `store.go:46-48` assigns and increments), ascending-by-ID equals creation order.

### Empty-store guarantee
- When the store has no items, the body MUST be `[]` (an empty JSON array), **never** `null`.
- Mechanism: `Store.List()` builds the slice with `make([]Item, 0, len(s.items))` (`store.go:67`), producing a **non-nil** empty slice. Go's `encoding/json` encodes a non-nil empty slice as `[]` and a nil slice as `null`. A reimplementation that returns a `nil` slice (e.g. declaring `var out []Item`) would emit `null` and VIOLATE this contract.

### Error behavior
- There are NO error responses for this route. No 400, 404, 409, or 500 path exists in the handler (`server.go:42-44`). Any reimplementation that adds an error branch is wrong.

---

## 3. Prerequisite slices (build order)

This is slice **#10**. It depends on:

- **EPIC-003-S1** — provides `Store.List() []Item`, the data source. This slice consumes `List()`'s output and relies on its two guarantees: ascending-by-ID sort and non-nil-empty-slice-when-empty. Do NOT re-derive or reimplement sorting here; call `store.List()`.
- **SHARED-response-helpers** — provides `writeJSON(w http.ResponseWriter, status int, v any)` (`server.go:76-80`). This slice calls it to serialize the 200 response. Do NOT inline JSON encoding here; call `writeJSON`.

Treat both as already-built black boxes with the contracts described in Section 2. Do not depend on their internal implementation beyond the stated guarantees.

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec. The existing HTTP test is `TestListItemsHTTP` at `server_test.go:92-108`. Tests use `net/http/httptest`: build the handler, drive it with `ServeHTTP` against an `httptest.ResponseRecorder` (helper `do` at `server_test.go:15-26`; `newTestServer` at `server_test.go:11-13`).

### AT-1 — Two items, status 200 (test-backed)
GIVEN a fresh server wired to a new empty store, WHEN two items are created via `POST /items` with bodies `{"name":"a"}` then `{"name":"b"}` (`server_test.go:94-95`), THEN `GET /items` MUST return HTTP `200`.
- Assertion: `w.Code == 200`, else fail (`server_test.go:98-99`).

### AT-2 — Body is a JSON array of length 2 (test-backed)
GIVEN the two items from AT-1, WHEN `GET /items` is issued, THEN the response body MUST decode into a Go `[]Item` (a JSON array, not an object or single item) whose length is exactly `2`.
- Assertion: `json.Unmarshal(body, &items)` succeeds and `len(items) == 2` (`server_test.go:101-107`).

### AT-3 — Empty store returns `[]`, not `null` (test-backed)
GIVEN a fresh server with an empty store (no items created), WHEN `GET /items` is issued, THEN status MUST be `200` AND the raw response body MUST be the two characters `[]` (a JSON empty array, ignoring a trailing newline), and MUST NOT be `null`.
- Assertion: `w.Code == 200` and `strings.TrimSpace(w.Body.String()) == "[]"`.
- Test-backed by `TestListItemsHTTPEmpty` (`server_test.go:113-123`). This locks the SOURCE guarantee (`store.go:67`); a reimplementation returning `null` would pass the original two-item test but FAILS this one.

### AT-4 — Ascending-by-ID order at the HTTP layer (test-backed)
GIVEN three items created in order `a`, `b`, `c` (IDs 1, 2, 3), WHEN `GET /items` is issued, THEN the decoded array's element IDs MUST be strictly increasing: `items[0].id < items[1].id < items[2].id`.
- Test-backed by `TestListItemsHTTPAscendingOrder` (`server_test.go:127-149`): decodes into `[]Item`, asserts `len == 3`, then asserts `items[i-1].ID < items[i].ID` for every adjacent pair. (Store-level sorting is also verified at `store_test.go:93-107`.)

### AT-5 — Content-Type is application/json (test-backed)
GIVEN any successful `GET /items`, THEN the response header `Content-Type` MUST equal `application/json`.
- Assertion: `w.Header().Get("Content-Type") == "application/json"`.
- Test-backed by `TestListItemsHTTPContentType` (`server_test.go:153-159`). This is the SOURCE fact via `writeJSON` (`server.go:77`).

---

## 5. Build steps (function/unit-sized, each individually checkable)

> Assumes EPIC-003-S1 (`Store.List`) and SHARED-response-helpers (`writeJSON`) already exist with the contracts in Section 2. Package is `seeds` (`server.go:1`).

1. **Ensure the mux exists in `NewServer`.** Confirm `NewServer(store *Store) http.Handler` creates `mux := http.NewServeMux()` (`server.go:18-19`). Checkable: function compiles and returns an `http.Handler`.

2. **Register the list route.** Add `mux.HandleFunc("GET /items", <handler>)` with the pattern string EXACTLY `"GET /items"` (`server.go:42`). Checkable: a `GET /items` request reaches the handler (returns 200, not 404/405).

3. **Implement the handler body — one line.** Inside the handler, call `writeJSON(w, http.StatusOK, store.List())` and nothing else (`server.go:43`). Do NOT read `r`, parse params, validate, or add branches. Checkable: handler has no conditional logic.

4. **Return the mux.** Ensure `NewServer` ends with `return mux` (`server.go:73`). Checkable: routes are reachable through the returned handler.

5. **Verify the data-source guarantees are honored (no new code).** Confirm the handler uses `store.List()` (so sorting at `store.go:71` and non-nil-empty-slice at `store.go:67` apply). Do not sort or post-process the result. Checkable: AT-3 and AT-4 pass.

6. **Verify serialization (no new code).** Confirm `writeJSON` sets `Content-Type: application/json` and writes status before encoding (`server.go:77-79`). Checkable: AT-5 passes.

7. **Write the acceptance tests** AT-1..AT-5 (Section 4) using `httptest` per `server_test.go:11-26`. Checkable: `go test ./...` passes for this package.

---

## 6. Kernel references (relied upon, NOT restated)

This slice relies on these standard-library names, types, and conventions. Reference the kernel/stdlib for their semantics; do not reimplement them.

- `net/http` — `http.Handler`, `http.ResponseWriter`, `*http.Request`, `http.NewServeMux()`, `(*ServeMux).HandleFunc`, `http.StatusOK` (= 200), `(http.Header).Set`, `(ResponseWriter).WriteHeader`. Imported at `server.go:6`.
- **Go 1.22+ `ServeMux` method-prefixed routing** — the pattern `"METHOD /path"` (here `"GET /items"`) matches both method and path. This is a kernel routing convention, required for the pattern at `server.go:42` to work.
- `encoding/json` — `json.NewEncoder(w).Encode(v)` for response encoding (`server.go:79`); `json.Unmarshal` in tests (`server_test.go:102`). Convention: non-nil empty slice → `[]`, nil slice → `null`. Imported at `server.go:4`.
- `net/http/httptest` — `httptest.NewRequest`, `httptest.NewRecorder`, `(*ResponseRecorder).Code`, `.Body`, `.Header()`; drive via `srv.ServeHTTP(w, r)` (`server_test.go:6`, `server_test.go:15-26`).
- Project types/funcs from prerequisite slices (treat as kernel-level black boxes here): `Item` (`item.go:13-18`), `Store` and `Store.List` (`store.go:19`, `store.go:64`), `writeJSON` (`server.go:76`).
