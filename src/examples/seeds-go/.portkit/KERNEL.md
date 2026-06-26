# seeds-go — KERNEL

The thin shared layer every slice references. This is the naming/type glossary and domain
vocabulary. It is **not** a layer to build feature logic into — it is the contract the
slices share so they stay consistent. Build the kernel's concrete code artifacts first
(the `Item` type, the sentinel errors, and the response helpers); everything else lives
inside its own slice.

Paths are relative to `src/examples/seeds-go`. Every claim cites `path:line`.

## Module / package facts

- Module path: `example.com/seeds-go` (`go.mod:1`). Go version `1.26` (`go.mod:3`).
- Library package name: `seeds` (`item.go:1`, `store.go:1`, `server.go:1`). All library
  code (`Item`, `Store`, `NewServer`, helpers, sentinels) lives in package `seeds` at the
  module root.
- Binary package: `main` at `cmd/server/main.go:2`; it imports the library as
  `seeds "example.com/seeds-go"` (`cmd/server/main.go:8`) and serves on hardcoded `:8080`
  (`cmd/server/main.go:15`).
- Standard library only — `go.mod` has no `require` block (`go.mod:1-3`); no `go.sum`.

## Type glossary (build these as shared kernel code)

### `Item` (the domain record) — `item.go:13-18`

```go
type Item struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Done      bool      `json:"done"`
    CreatedAt time.Time `json:"createdAt"`
}
```

- JSON keys are EXACTLY `id`, `name`, `done`, `createdAt` — note camelCase `createdAt`,
  not `created_at` (`item.go:14-17`).
- No field has `omitempty`. `done` therefore ALWAYS appears in output; for a newly created
  item it is `false` (zero value) because no code path ever sets `Done = true`
  (`store.go:46` sets only ID/Name/CreatedAt).
- `CreatedAt` is a `time.Time`; Go's default JSON encoding serializes it as an RFC3339
  string (e.g. `"2026-06-26T..."`).

### `Store` (in-memory persistence) — `store.go:19-29`

```go
type Store struct {
    mu     sync.Mutex
    items  map[int]Item
    nextID int
    now    func() time.Time
}
func NewStore() *Store { return &Store{items: map[int]Item{}, nextID: 1, now: time.Now} }
```

- Built once and shared by all four route handlers (`cmd/server/main.go:12-13`).
- `NewStore` initializes `items` to an empty non-nil map, `nextID` to `1`, and `now` to
  `time.Now` (`store.go:28`). `now` is injectable for deterministic tests.
- The `Store` methods (`Create`/`Get`/`List`/`Delete`) are each owned by their slice; only
  the struct + `NewStore` are kernel.

## Sentinel error glossary (build these as shared kernel code)

All defined as package-level `errors.New(...)` values. Handlers match them with
`errors.Is`, NOT `==`, so a wrapping error would still match (`server.go:31,33,53,66`).

| Sentinel          | Message string                              | Defined at      | HTTP status it maps to |
|-------------------|---------------------------------------------|-----------------|------------------------|
| `ErrNameRequired` | `name is required`                          | `item.go:22`    | 400 (`server.go:31-32`)|
| `ErrNameTooLong`  | `name must be at most 100 characters`       | `item.go:23`    | 400 (`server.go:31-32`)|
| `ErrNotFound`     | `item not found`                            | `store.go:13`   | 404 (`server.go:53-54`, `server.go:66-67`)|
| `ErrDuplicate`    | `an item with that name already exists`     | `store.go:14`   | 409 (`server.go:33-34`)|

- The exact message strings are part of the HTTP contract: error responses echo
  `err.Error()` (or, for delete, `ErrNotFound.Error()` — same string, `server.go:67`).
  Reproduce them character-for-character.

## Naming constants

- `MaxNameLen = 100` (`item.go:10`) — the rune limit for item names; used by
  `ValidateName` (`item.go:33`).

## Domain vocabulary

- **item** — a single todo entry; the `Item` struct (`item.go:13`).
- **store** — the thread-safe in-memory collection of items keyed by integer ID
  (`store.go:19-24`).
- **name** — the user-supplied item label. Always stored in its **trimmed** form
  (`store.go:37,46`); compared for uniqueness **case-insensitively** via
  `strings.EqualFold` (`store.go:42`).
- **id** — sequential integer key, starting at 1, never reused (deletes do not decrement
  `nextID`) (`store.go:28,48,82`).
- **sentinel error** — a package-level `error` value compared with `errors.Is`; the unit
  of error identity across the store→handler boundary.
- **response envelope** — successful responses are the JSON-encoded value; error responses
  are a single-key JSON object `{"error":"<message>"}` (`server.go:82-83`).

## Public surface the binary depends on

- `NewStore() *Store` (`store.go:27`)
- `NewServer(store *Store) http.Handler` (`server.go:18`)
- These two are the only symbols `cmd/server/main.go` references (`cmd/server/main.go:12-13`).
