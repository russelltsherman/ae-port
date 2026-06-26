# Slice EPIC-002-S6 — 200 OK success response with Item JSON

**Capability:** When an item with the requested id exists, the `GET /items/{id}` handler responds with HTTP 200 and the matched `Item` serialized as JSON.

- Build order: this is slice **#8**.
- Slice id: `EPIC-002-S6`
- Epic: `EPIC-002`
- dependsOn: `["EPIC-002-S3", "SHARED-response-helpers"]`

---

## 1. End-to-end behavior thread

This slice is the **final fall-through line** of the `GET /items/{id}` handler. It is reached only after two earlier guard clauses (an integer-parse guard and a not-found guard) have each declined to return. The components, in execution order, with their source locations:

| Order | Component | Source `path:line` |
|-------|-----------|--------------------|
| 1 | Not-found guard returns early when the store reports `ErrNotFound` (so success branch is only reached otherwise) | `src/examples/seeds-go/server.go:53` |
| 2 | Success response: `writeJSON(w, http.StatusOK, it)` | `src/examples/seeds-go/server.go:57` |
| 3 | `Item` struct whose JSON field tags define the serialized body | `src/examples/seeds-go/item.go:13` (fields at `item.go:14-17`) |
| 4 | `writeJSON` helper that sets Content-Type, writes status, and JSON-encodes the value | `src/examples/seeds-go/server.go:76-80` |
| 5 | Acceptance test asserting `GET /items/1` returns 200 | `src/examples/seeds-go/server_test.go:79-82` |

### Exact source context (for grounding only — rebuild from the contract below, not by copying)

The `GET /items/{id}` handler body, as it exists at `server.go:46-58`:

```go
mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))   // server.go:47  (parse — slice EPIC-002-S2/S3 territory)
	if err != nil {                               // server.go:48
		writeError(w, http.StatusBadRequest, "id must be an integer") // server.go:49
		return                                    // server.go:50
	}
	it, err := store.Get(id)                      // server.go:52
	if errors.Is(err, ErrNotFound) {              // server.go:53  (not-found guard)
		writeError(w, http.StatusNotFound, err.Error()) // server.go:54
		return                                    // server.go:55
	}
	writeJSON(w, http.StatusOK, it)               // server.go:57  <-- THIS SLICE
})
```

**The single line this slice owns is `server.go:57`.** Everything else in the handler belongs to prerequisite slices (see §4); you rely on their behavior but do not re-implement it.

---

## 2. Interface / contract

This slice has no standalone function signature of its own — it is the success fall-through of the `GET /items/{id}` HTTP handler. The contract below describes the **observable HTTP behavior** that this line produces, plus the exact control-flow it depends on.

### Inputs (the state in which this line executes)

By the time control reaches `server.go:57`:

1. `id` is a valid `int` parsed from the `{id}` path segment via `strconv.Atoi(r.PathValue("id"))` (`server.go:47`). The parse succeeded — `err` from the parse was `nil` (otherwise the handler returned at `server.go:49-50`).
2. `it, err := store.Get(id)` (`server.go:52`) was called. The success branch is reached **only when `errors.Is(err, ErrNotFound)` is `false`** (`server.go:53`). In the current codebase that means `store.Get` returned a found item with `err == nil`.
3. `it` is the `Item` value returned by `store.Get(id)`.

### Output (the HTTP response)

Produced by `writeJSON(w, http.StatusOK, it)` (`server.go:57`), whose helper is defined at `server.go:76-80`:

- **Status code:** `200` (Go constant `http.StatusOK`). Set via `w.WriteHeader(status)` at `server.go:78`.
- **Header:** `Content-Type: application/json`. Set via `w.Header().Set("Content-Type", "application/json")` at `server.go:77`, **before** `WriteHeader` (ordering matters — see §3).
- **Body:** `it` (the `Item`) encoded by `json.NewEncoder(w).Encode(v)` at `server.go:79`.

### Exact body shape

The body is the JSON object produced by encoding the `Item` struct. The struct and its JSON tags are at `item.go:13-18`:

```go
type Item struct {
	ID        int       `json:"id"`         // item.go:14
	Name      string    `json:"name"`       // item.go:15
	Done      bool      `json:"done"`       // item.go:16
	CreatedAt time.Time `json:"createdAt"`  // item.go:17
}
```

Therefore the body is a single JSON object with exactly these four keys, in this order (Go's `encoding/json` emits struct fields in declaration order):

| JSON key | Go type | Encoding notes |
|----------|---------|----------------|
| `id` | `int` | bare integer, e.g. `1` |
| `name` | `string` | JSON string |
| `done` | `bool` | `true` or `false` |
| `createdAt` | `time.Time` | RFC3339 timestamp string (Go's default `time.Time` JSON marshaling), e.g. `"2026-06-26T13:31:00Z"` |

Example body for item id 1 named `"findme"`:

```json
{"id":1,"name":"findme","done":false,"createdAt":"2026-06-26T13:31:00Z"}
```

**Trailing newline:** `json.NewEncoder(...).Encode(v)` appends a single `\n` after the JSON object (`server.go:79`). The full body bytes therefore end with `}\n`.

### Exact behavior, errors, edge cases, ordering

- **No `else`/`return` guards line 57.** It is plain fall-through. It is reached *only* because the not-found branch (`server.go:53-55`) returned early. **Preserve this control-flow exactly:** a found item must reach `server.go:57` and must NOT also have produced an error response. Do not add an `else` or wrap line 57 in a conditional — the early `return` at `server.go:55` is what guarantees mutual exclusivity.
- **Encode errors are ignored.** `writeJSON` discards the encode error: `_ = json.NewEncoder(w).Encode(v)` (`server.go:79`). Do not add error handling here — match the source exactly.
- **Header-before-status ordering is mandatory.** `Content-Type` is set at `server.go:77` *before* `w.WriteHeader(200)` at `server.go:78`. In Go's `net/http`, headers set after `WriteHeader` are silently dropped. The helper already does this correctly; preserve the order.
- **Status-before-body ordering.** `WriteHeader(200)` (`server.go:78`) precedes the encode write (`server.go:79`). The first byte written by `Encode` would otherwise implicitly trigger a `200`, so an explicit `WriteHeader` must come first to guarantee the status.
- **This slice does NOT handle:** invalid (non-integer) id — that is the `server.go:48-50` guard (a different slice); missing item / `ErrNotFound` — that is the `server.go:53-55` guard (a different slice). Do not duplicate those responses in this line.

---

## 3. Prerequisite slices (build order)

This is slice **#8**. It depends on:

- **`EPIC-002-S3`** — provides the upstream parts of the `GET /items/{id}` handler that must already exist for line 57 to be reachable: the route registration `mux.HandleFunc("GET /items/{id}", ...)` (`server.go:46`), the integer parse `strconv.Atoi(r.PathValue("id"))` (`server.go:47`), the parse guard (`server.go:48-50`), the `store.Get(id)` call (`server.go:52`), and the not-found guard (`server.go:53-55`). **Do not re-implement these.** This slice only adds/owns the success fall-through at `server.go:57`.
- **`SHARED-response-helpers`** — provides the `writeJSON(w http.ResponseWriter, status int, v any)` helper (`server.go:76-80`). This slice **calls** it; it does not define it. Treat its contract as: sets `Content-Type: application/json`, writes the given status, JSON-encodes `v` with a trailing newline. **Do not redefine `writeJSON`.**

> Cross-slice rule: do not depend on any internal detail of these slices beyond the stated contracts above. You rely on the *handler reaching line 57 with a valid `it`* and on *`writeJSON` doing what its contract says* — nothing more.

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec. Concrete and runnable in spirit. They live in the same `package seeds` test file pattern as `server_test.go`, using the existing `newTestServer()` (`server_test.go:11-13`) and `do(...)` (`server_test.go:15-26`) helpers.

### AT-1 (VERIFIED in source) — GET an existing item returns 200

Grounded in `server_test.go:75-82`. After creating one item via `POST /items`, `GET /items/1` must return status `200`.

```go
func TestGetItemSuccessStatus(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"findme"}`) // creates item id 1

	w := do(t, srv, "GET", "/items/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items/1 code = %d, want 200", w.Code)
	}
}
```

### AT-2 (SOURCE-GROUNDED, recommended new test) — 200 body is the matched Item with correct fields

Grounded in `server.go:57` (passes `it` to `writeJSON`) and the JSON tags at `item.go:14-17`. The existing test does **not** assert the body (see coverage flag below); this test closes that gap. The downstream model SHOULD add it.

```go
func TestGetItemSuccessBody(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"findme"}`) // creates item id 1

	w := do(t, srv, "GET", "/items/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items/1 code = %d, want 200", w.Code)
	}
	var it Item
	if err := json.Unmarshal(w.Body.Bytes(), &it); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if it.ID != 1 {
		t.Errorf("id = %d, want 1", it.ID)
	}
	if it.Name != "findme" {
		t.Errorf("name = %q, want %q", it.Name, "findme")
	}
	if it.Done {
		t.Errorf("done = true, want false for a freshly created item")
	}
	if it.CreatedAt.IsZero() {
		t.Errorf("createdAt is zero, want a populated timestamp")
	}
}
```

### AT-3 (SOURCE-GROUNDED, recommended new test) — 200 response Content-Type is application/json

Grounded in `server.go:77` (the `writeJSON` helper sets the header). Not asserted by any existing test.

```go
func TestGetItemSuccessContentType(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"findme"}`)

	w := do(t, srv, "GET", "/items/1", "")
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
```

### Coverage flag — THIN (carried from the behavioral spec, do not lose this)

> **LOUD COVERAGE FLAG (THIN):** The ONLY pre-existing test exercising this slice is `server_test.go:79-82`, and it asserts **only the status code 200** for `GET /items/1`. It does **NOT** decode or assert the returned Item's body (no check of `id`/`name`/`done`/`createdAt`) and does **NOT** assert `Content-Type`. The body fidelity of the GET-by-id 200 response is **[SOURCE-GROUNDED but UNVERIFIED BY ANY EXISTING TEST]** — though `writeJSON`'s correctness for an `Item` is *indirectly* evidenced by the POST test, which decodes an `Item` from a `writeJSON`-produced body at `server_test.go:35-41`. AT-2 and AT-3 above are recommended to remove this gap.

---

## 5. Function/unit-sized build steps (each individually checkable)

> Precondition: the `GET /items/{id}` handler from `EPIC-002-S3` already exists with the parse guard and not-found guard, and `writeJSON` from `SHARED-response-helpers` already exists. If either is missing, those slices must be built first.

1. **Locate the handler.** Find `mux.HandleFunc("GET /items/{id}", ...)` (`server.go:46`). Confirm the function body already contains, in order: the `strconv.Atoi(r.PathValue("id"))` parse (`server.go:47`), the parse guard with early `return` (`server.go:48-50`), the `it, err := store.Get(id)` call (`server.go:52`), and the `errors.Is(err, ErrNotFound)` guard with early `return` (`server.go:53-55`).
   - *Check:* the two guard blocks each end in `return`.

2. **Add the success fall-through as the LAST statement of the handler body**, after the not-found guard block and before the closing `})`:
   ```go
   writeJSON(w, http.StatusOK, it)
   ```
   - *Check:* it is a bare statement with no surrounding `if`/`else`; it uses `http.StatusOK` (not a literal `200`) and the variable `it` returned by `store.Get`.

3. **Confirm imports.** The handler relies on `net/http` (for `http.StatusOK` and `http.ResponseWriter`) and `errors` (for the guard above). These are already imported at `server.go:3-8`. This line adds no new imports.
   - *Check:* `go build ./...` compiles with no unused/missing import errors.

4. **Confirm `writeJSON` is in scope.** It is defined at `server.go:76-80` in the same package. No change needed.
   - *Check:* the call resolves; no "undefined: writeJSON".

5. **Verify control-flow mutual exclusivity.** Ensure a found item reaches line 57 and a not-found id never does (the `return` at `server.go:55` guarantees this).
   - *Check:* AT-1 passes (200 for an existing id); the prerequisite slices' tests for 404 (`server_test.go:84-86`) and 400 (`server_test.go:87-89`) still pass — confirming the success line did not break the guards.

6. **Run the slice tests.** `go test ./...` from `src/examples/seeds-go`. AT-1 must pass; add and pass AT-2 and AT-3 to close the thin-coverage gap.
   - *Check:* all tests green.

---

## 6. Kernel references

This slice relies on the following kernel-level names, types, and conventions. **Reference the kernel for their definitions; this doc does not restate them, and this slice does not depend on any other slice's internals beyond the §3 contracts.**

- `net/http` standard library: `http.StatusOK` (constant `200`), `http.ResponseWriter`, `http.Request`, `http.HandleFunc` registration via `*http.ServeMux`, and the `Header().Set` / `WriteHeader` ordering rules.
- `encoding/json` standard library: `json.NewEncoder(w).Encode(v)` semantics, including struct-field-order output, the `json:"..."` field-tag convention, and the trailing-newline behavior of `Encode`.
- `time.Time` standard library: its default JSON marshaling to an RFC3339 string.
- Package convention: all source is in `package seeds` (`server.go:1`, `item.go:1`, `server_test.go:1`).
- Project type `Item` (`item.go:13`) — its field set and JSON tags are the contract for the response body (referenced, defined in the kernel/shared model, not redefined here).
