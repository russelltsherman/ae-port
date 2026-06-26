# seeds-go — System Map

A small in-memory todo HTTP JSON service. Four user-observable capabilities (create / get
/ list / delete an item) over an `http.ServeMux` backed by a thread-safe in-memory store.

All claims below cite `path:line` into the source. Paths are relative to
`src/examples/seeds-go`.

## Languages

- Go — only language present. `go.mod` declares `go 1.26` (`go.mod:3`). Module path is
  `example.com/seeds-go` (`go.mod:1`).
- The library package is named `seeds` (`item.go:1`, `store.go:1`, `server.go:1`); the
  binary package is `main` (`cmd/server/main.go:2`).

## Build system

- Standard Go toolchain (`go build` / `go run`). No Makefile, Dockerfile, or CI config
  present in the tree.
- Single binary entrypoint: `cmd/server/main.go`. `main` builds a store, wires the server,
  and calls `http.ListenAndServe(":8080", srv)` (`cmd/server/main.go:11-16`). Listen
  address `:8080` is hardcoded (`cmd/server/main.go:15`); no flags or env vars are read.
- README documents the commands (`README.md:30-34`): `go run ./cmd/server` serves on
  `:8080`, `go vet ./...` for static checks.

## Test framework(s) and locations

- Standard library `testing` package only — no third-party test deps
  (`store_test.go:6`, `server_test.go:8`).
- Tests live alongside source in the package root, in package `seeds`:
  - `store_test.go` — unit tests for `ValidateName` and `Store` (Create/Get/List/Delete),
    including validation, case/trim-insensitive duplicate detection, ID increment, and
    sort order (`store_test.go:9-122`).
  - `server_test.go` — HTTP-level tests via `net/http/httptest`, covering each route and
    its status-code outcomes (`server_test.go:28-123`).
- Run with `go test ./...` (`README.md:32`).
- Style: table-driven subtests (e.g. `store_test.go:10-28`, `server_test.go:44-63`). The
  tests are explicitly the behavioral contract (`README.md:4-5`).

## Dependency manifest file(s)

- `go.mod` (`go.mod:1-3`). It declares only the module path and Go version; there is **no
  `require` block** — the code uses standard library only. There is no `go.sum` in the tree.

## Behavioral rules (shared across capabilities)

- **Name validation** (`item.go:28-37`, exported as `ValidateName`):
  - Input is trimmed of surrounding whitespace before checks (`item.go:29`).
  - Empty after trim (incl. all-whitespace) → `ErrNameRequired` ("name is required")
    (`item.go:22`, `item.go:30-32`).
  - Length is measured in **runes**, not bytes; > `MaxNameLen` (= 100, `item.go:10`) →
    `ErrNameTooLong` ("name must be at most 100 characters") (`item.go:23`, `item.go:33-35`).
    Exactly 100 runes is allowed (`store_test.go:20`).
- **Store** (`store.go:19-29`): in-memory `map[int]Item`, guarded by a single
  `sync.Mutex` (`store.go:20-23`). IDs are sequential starting at 1 and never reused
  (`store.go:28`, `store.go:46-48`). Timestamps come from an injectable `now func()`,
  defaulting to `time.Now` (`store.go:23`, `store.go:28`).
- **Duplicate detection** on create: the trimmed name is compared against every existing
  item's name with `strings.EqualFold` → case-insensitive (and trim-insensitive, since the
  new name is trimmed first); a match returns `ErrDuplicate` ("an item with that name
  already exists") (`store.go:14`, `store.go:41-45`). Stored name is the **trimmed** form
  (`store.go:37`, `store.go:46`).
- **Item JSON shape** (`item.go:13-18`): `id` (int), `name` (string), `done` (bool),
  `createdAt` (RFC3339 time). `Done` is always `false` on create (never set true by any
  code path in this tree) (`store.go:46`).
- **HTTP error responses** are JSON `{"error": "<message>"}` with `Content-Type:
  application/json` (`server.go:76-84`). Success responses are the JSON-encoded value.

## Capabilities (draft epic inventory)

Vertical, externally-observable behaviors. Each is an HTTP route registered in
`NewServer` (`server.go:18-74`). The Go 1.22+ method-prefixed mux patterns
(`server.go:21,42,46,60`) mean a request whose path matches a registered pattern but whose
method does not (e.g. `PUT /items` against `POST /items` + `GET /items`) yields **405 Method
Not Allowed** from `ServeMux`, not 404. Go's `ServeMux` only returns 404 when the path
matches no registered pattern at all (e.g. `GET /unknown`). This is standard Go 1.22+
method-routing behavior; no test in this tree exercises the 405/404 paths, so the exact
response body is the `net/http` default (a plain-text body, not the JSON `{"error":...}`
shape used by the handlers). See `kernel/cross-cutting.md` §4 and `targets/rust/porting-hazards.md` H16.

### EPIC-001 — Create item (`POST /items`)
- Entry: `server.go:21` (handler), delegates to `Store.Create` (`store.go:33`).
- Request body: JSON object `{"name": "<string>"}` (`server.go:22-24`).
- Outcomes:
  - **201 Created** with the created `Item` JSON on success (`server.go:38`).
  - **400 Bad Request** `{"error":"invalid JSON body"}` if the body fails to decode
    (`server.go:25-28`). Note: an empty/absent body also fails decode (EOF) → 400.
  - **400 Bad Request** with the validation error message on `ErrNameRequired` /
    `ErrNameTooLong` (`server.go:31-32`).
  - **409 Conflict** with `ErrDuplicate` message on a case-insensitive duplicate name
    (`server.go:33-34`).
  - **500 Internal Server Error** `{"error":"internal error"}` for any other non-nil
    error (`server.go:35-36`) — unreachable with the current store, defensive only.
- Verified by `server_test.go:28-73` (success, invalid JSON, empty name, whitespace name,
  duplicate `DUP`/`dup`) and `store_test.go:31-72`.

### EPIC-002 — Get item by id (`GET /items/{id}`)
- Entry: `server.go:46` (handler), delegates to `Store.Get` (`store.go:53`).
- `{id}` path segment parsed via `strconv.Atoi` (`server.go:47`).
- Outcomes:
  - **200 OK** with the `Item` JSON when found (`server.go:57`).
  - **400 Bad Request** `{"error":"id must be an integer"}` if `{id}` is not an integer
    (`server.go:48-50`).
  - **404 Not Found** with `ErrNotFound` message ("item not found") when no item has that
    id (`server.go:53-56`, `store.go:13`).
- Verified by `server_test.go:75-90` and `store_test.go:74-89`.

### EPIC-003 — List items (`GET /items`)
- Entry: `server.go:42` (handler), delegates to `Store.List` (`store.go:64`).
- Outcomes:
  - **200 OK** with a JSON array of `Item`, sorted by `id` ascending = creation order
    (`server.go:43`, `store.go:71`).
  - An empty store returns `[]` (a zero-length, non-nil slice is allocated at
    `store.go:67`).
- Verified by `server_test.go:92-108` and `store_test.go:91-107`.

### EPIC-004 — Delete item by id (`DELETE /items/{id}`)
- Entry: `server.go:60` (handler), delegates to `Store.Delete` (`store.go:76`).
- `{id}` path segment parsed via `strconv.Atoi` (`server.go:61`).
- Outcomes:
  - **204 No Content** (empty body) on successful delete (`server.go:70`).
  - **400 Bad Request** `{"error":"id must be an integer"}` if `{id}` is not an integer
    (`server.go:62-64`).
  - **404 Not Found** with `ErrNotFound` message when no item has that id
    (`server.go:66-68`, `store.go:79-80`).
- Verified by `server_test.go:110-123` and `store_test.go:109-122`.

## Non-capabilities / gaps (for the rebuilder)

- No update/PATCH route and no way to set `done` to true anywhere in this tree
  (`server.go:18-74`).
- No persistence — state is lost on restart (in-memory map, `store.go:19-29`).
- No authentication, pagination, configuration, logging beyond a startup line
  (`cmd/server/main.go:14`), or graceful shutdown.
