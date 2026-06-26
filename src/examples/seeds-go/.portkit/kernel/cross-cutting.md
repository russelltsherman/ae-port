# seeds-go — Cross-Cutting Conventions

Rules every slice OBEYS. This is not a layer to build feature behavior into; it states the
conventions (response shaping, error handling, concurrency, config, logging) that slices
must follow so a rebuild stays consistent. Each rule cites `path:line`.

## 1. HTTP response shaping (the shared response helpers)

Two package-level helpers produce EVERY HTTP response in the service. They are concrete
shared code — build them once (slice `SHARED-response-helpers`) and have all route slices
call them.

```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json") // server.go:77
    w.WriteHeader(status)                              // server.go:78
    _ = json.NewEncoder(w).Encode(v)                   // server.go:79
}
func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg}) // server.go:83
}
```

RULES (all load-bearing):

- **Header ordering is mandatory.** Set `Content-Type: application/json` BEFORE
  `WriteHeader(status)`, and call `WriteHeader` BEFORE writing the body
  (`server.go:77→78→79`). Setting a header after `WriteHeader` has no effect in Go; the
  header would be silently dropped.
- **Encode error is intentionally discarded** with `_ =` (`server.go:79`). Do not add error
  handling here.
- **`json.NewEncoder(...).Encode` appends a trailing newline** to the body (standard
  library behavior). Expect the byte after the closing `}`/`]` to be `\n`.
- **Error envelope shape is exactly** `{"error":"<msg>"}` — a `map[string]string` with the
  single key `"error"` (`server.go:83`).
- **Success bodies** are the value serialized directly: a single `Item` object for create
  (201) and get (200), a JSON array of `Item` for list (200). The list of an empty store
  encodes to `[]` (not `null`) because `Store.List` returns a non-nil slice
  (`store.go:67`).
- **204 No Content is the ONE exception**: the delete success path calls
  `w.WriteHeader(http.StatusNoContent)` directly with NO body and does NOT call
  `writeJSON`, so it sets NO `Content-Type` (`server.go:70`). Every other response goes
  through the helpers.

## 2. Error handling

- Store methods return **sentinel errors** (see KERNEL.md glossary), never ad-hoc/wrapped
  strings. Construction validation errors propagate unchanged from `ValidateName` through
  `Store.Create` (`store.go:34-36`).
- Handlers branch on errors with **`errors.Is(err, Sentinel)`**, never `==`
  (`server.go:31,33,53,66`).
- Error → status mapping is centralized in the handler switch/if-chains. The canonical map:
  - `ErrNameRequired` | `ErrNameTooLong` → **400**, body = `err.Error()` (`server.go:31-32`)
  - `ErrDuplicate` → **409**, body = `err.Error()` (`server.go:33-34`)
  - `ErrNotFound` → **404**, body = `err.Error()` / `ErrNotFound.Error()` (`server.go:53-54`, `server.go:66-67`)
  - any other non-nil error → **500** `{"error":"internal error"}` — a defensive catch-all
    in the POST handler that is UNREACHABLE with the current store (`server.go:35-36`).
  - decode failure (before the store is touched) → **400** `{"error":"invalid JSON body"}`
    (`server.go:25-27`).
  - non-integer `{id}` (before the store is touched) → **400**
    `{"error":"id must be an integer"}` (`server.go:48-50`, `server.go:62-64`).
- **Two distinct 400s exist** and must keep their distinct messages: malformed/empty body
  → `"invalid JSON body"`; failed name validation → the validation sentinel's message.
- **Validate/parse before touching the store.** Both `{id}` routes parse with
  `strconv.Atoi` and 400-return BEFORE any store call (`server.go:47-50`, `server.go:61-65`).

## 3. Concurrency

- The store is thread-safe via a single `sync.Mutex` field `mu` (`store.go:20`).
- EVERY store method acquires the lock with `s.mu.Lock(); defer s.mu.Unlock()` at entry
  (`store.go:39-40`, `store.go:54-55`, `store.go:65-66`, `store.go:77-78`).
- `Create` validates and trims BEFORE taking the lock (`store.go:34-37`), then does the
  duplicate scan + insert under the lock (`store.go:39-49`).
- `nextID` increments ONLY on a successful insert (`store.go:48`); rejected creates do not
  consume an ID. Deletes never decrement it — IDs are never reused (`store.go:82`).

## 4. Routing convention

- All routes registered on a single `http.ServeMux` from `http.NewServeMux()`
  (`server.go:19`) inside `NewServer`, which returns the mux as `http.Handler`
  (`server.go:73`).
- Use **Go 1.22+ method+pattern strings** exactly: `"POST /items"` (`server.go:21`),
  `"GET /items"` (`server.go:42`), `"GET /items/{id}"` (`server.go:46`),
  `"DELETE /items/{id}"` (`server.go:60`).
- Path wildcards are read with `r.PathValue("id")` (`server.go:47,61`). Do NOT lowercase,
  trim, or otherwise alter the captured string before `strconv.Atoi`.
- Method mismatches on a registered path (e.g. `PUT /items`, where only `POST /items` and
  `GET /items` are registered — `server.go:21,42`) yield **405 Method Not Allowed** from
  ServeMux, NOT 404. This is Go 1.22+ method-routing behavior: when the path matches a
  registered pattern but no pattern with that method exists, `ServeMux` responds 405. A path
  that matches no registered pattern at all (e.g. `GET /unknown`) yields 404. The 405/404
  response bodies are the `net/http` defaults (plain text), not the JSON `{"error":...}`
  shape; no test in this tree exercises these paths.

## 5. Configuration

- None to speak of. The listen address `:8080` is hardcoded (`cmd/server/main.go:15`); no
  flags, no env vars, no config files are read anywhere.
- Timestamps are the only injected dependency: `Store.now func() time.Time`, defaulting to
  `time.Now` (`store.go:23,28`).

## 6. Logging

- Exactly one log line, at startup: `log.Println("seeds-go listening on :8080")`
  (`cmd/server/main.go:14`). Request handling produces NO logs. `log.Fatal` wraps
  `http.ListenAndServe` (`cmd/server/main.go:15`).

## 7. Testing convention (rebuild parity)

- Standard library `testing` only; no third-party deps (`store_test.go:6`,
  `server_test.go:8`).
- Tests live in package `seeds` alongside source. Store-level behavior is unit-tested in
  `store_test.go`; HTTP-level behavior via `net/http/httptest` against the
  `NewServer(...)` handler in `server_test.go`.
- Style is table-driven subtests. Each slice's behavior has corresponding tests; treat the
  tests as the behavioral contract.
