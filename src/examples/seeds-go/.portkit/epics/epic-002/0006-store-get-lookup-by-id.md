# Slice 0006 — Store.Get lookup by id

**Capability:** Given a valid integer id, look up the matching `Item` in the in-memory store, returning the `Item` (with `nil` error) or the sentinel `ErrNotFound`.

- Slice id: `EPIC-002-S3`
- Epic: `EPIC-002`
- Build number: 6
- Depends on: `EPIC-002-S2` (see "Prerequisite slices" below)

---

## 1. End-to-end behavior thread

This slice implements the store-level `Get(id)` method and the two package-level
facts it relies on (the `ErrNotFound` sentinel and the `Item` struct). It also
documents the single HTTP call site that consumes it, but the HTTP handler
itself is NOT part of this slice — it is shown only so you understand the
contract `Get` must satisfy.

Components, in execution order, each with its source location:

1. **HTTP call site (context only, not built here)** — `src/examples/seeds-go/server.go:52`
   The `GET /items/{id}` handler parses the path segment into an `int` and calls
   `it, err := store.Get(id)`. On `errors.Is(err, ErrNotFound)` it writes HTTP
   404; otherwise it writes the item as JSON with HTTP 200
   (`src/examples/seeds-go/server.go:52-57`).

2. **`Store.Get` definition** — `src/examples/seeds-go/store.go:53`
   Method signature: `func (s *Store) Get(id int) (Item, error)`.

3. **Mutex lock/unlock around the read** — `src/examples/seeds-go/store.go:54-55`
   `s.mu.Lock()` followed immediately by `defer s.mu.Unlock()`. The mutex is the
   `sync.Mutex` field `mu` declared at `src/examples/seeds-go/store.go:20`. This
   makes the read thread-safe and serializes it against concurrent
   `Create`/`Delete`/`List` calls that share the same `s.mu`.

4. **Map lookup** — `src/examples/seeds-go/store.go:56`
   `it, ok := s.items[id]` — a single exact-key fetch against the field
   `items map[int]Item` (declared `src/examples/seeds-go/store.go:21`,
   initialized to an empty map by `NewStore` at `src/examples/seeds-go/store.go:28`).

5. **Not-found branch** — `src/examples/seeds-go/store.go:57-58`
   `if !ok { return Item{}, ErrNotFound }` — returns the zero-value `Item` and
   the sentinel error.

6. **Found branch** — `src/examples/seeds-go/store.go:60`
   `return it, nil` — returns the stored item value and a `nil` error.

7. **`ErrNotFound` sentinel definition** — `src/examples/seeds-go/store.go:13`
   `ErrNotFound = errors.New("item not found")` (inside the `var (...)` block at
   `src/examples/seeds-go/store.go:12-15`).

8. **`Item` struct + JSON tags** — `src/examples/seeds-go/item.go:13-18`
   The struct value that `Get` returns on success.

---

## 2. Interface / contract (EXACT behavior)

### `Store.Get`

```go
func (s *Store) Get(id int) (Item, error)
```

**Inputs**
- `id int` — the exact integer key to look up. Any `int` is accepted; there is
  no range validation inside `Get`. Negative, zero, and out-of-range positive
  ids are all valid inputs and simply will not match any key (because ids are
  assigned starting at 1 — see "Kernel/store facts" below).

**Outputs (exactly two cases — there are no others):**

1. **Key present** — when `s.items` contains an entry for `id`:
   - Returns the stored `Item` value (a copy of the struct held in the map).
   - Returns `nil` error.
   - Grounded: `src/examples/seeds-go/store.go:56,60`.

2. **Key absent** — when `s.items` has no entry for `id`:
   - Returns the zero-value `Item{}` (all fields zero: `ID==0`, `Name==""`,
     `Done==false`, `CreatedAt` is the zero `time.Time`).
   - Returns the sentinel error `ErrNotFound`. It MUST be this exact sentinel
     value, NOT a freshly constructed generic error, so that callers using
     `errors.Is(err, ErrNotFound)` succeed.
   - Grounded: `src/examples/seeds-go/store.go:57-58`.

**Exactness / edge cases (all mandatory):**
- Lookup is **exact-match on the integer key only**. There is no fuzzy,
  prefix, or name-based matching. Grounded: `src/examples/seeds-go/store.go:56`.
- The method takes `s.mu` for the entire duration via `Lock()` + `defer
  Unlock()`. It MUST be safe to call concurrently with other `Store` methods.
  Grounded: `src/examples/seeds-go/store.go:54-55`.
- `Get` performs no mutation: it never writes to `s.items`, `s.nextID`, or any
  field. It is a pure read. Grounded: `src/examples/seeds-go/store.go:53-61`.
- There is exactly one return-success path and exactly one return-not-found
  path; no panics, no other error values are ever returned from `Get`.

### `ErrNotFound` sentinel

```go
var ErrNotFound = errors.New("item not found")
```

- Package-level exported variable. Message text MUST be the literal string
  `item not found`. Grounded: `src/examples/seeds-go/store.go:13`.
- Note: `store_test.go` checks identity via `errors.Is`, not the literal string;
  the message text is SOURCE-GROUNDED at `store.go:13` but not asserted by any
  test. The HTTP layer (out of scope here) does surface `err.Error()` as the
  404 body (`src/examples/seeds-go/server.go:54`), so the text is observable.

### `Item` struct

```go
type Item struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Done      bool      `json:"done"`
    CreatedAt time.Time `json:"createdAt"`
}
```

- Four fields with JSON tags `id`, `name`, `done`, `createdAt` respectively.
  Grounded: `src/examples/seeds-go/item.go:13-18`.

---

## 3. Prerequisite slices (build order)

This is build #6. It `dependsOn ["EPIC-002-S2"]`.

- `Get` cannot be exercised meaningfully without items in the store, and items
  are created by `Store.Create` / `NewStore`. Slice `EPIC-002-S2` is assumed to
  have already established the `Store` type, its fields (`mu sync.Mutex`,
  `items map[int]Item`, `nextID int`, `now func() time.Time`), the
  `NewStore()` constructor (`src/examples/seeds-go/store.go:26-29`), and
  `Store.Create` (`src/examples/seeds-go/store.go:33-50`).
- **Do NOT reach into or re-derive the internals of that slice.** Treat
  `NewStore()` and `Create(name string) (Item, error)` as already-existing
  callable APIs: `NewStore()` returns an empty `*Store`; `Create` inserts an
  item and returns it with its assigned `ID`. This slice only ADDS the `Get`
  method (and relies on `ErrNotFound` and `Item` already existing in the
  package).

---

## 4. Acceptance tests for THIS slice

These mirror `TestStoreGet` in `src/examples/seeds-go/store_test.go:74-89` and
the behavioral spec. They are package-internal Go tests (`package seeds`),
runnable with `go test ./...` from `src/examples/seeds-go`.

**AT-1 — found returns the item, no error**
(Grounded: `store.go:53-60`; verified `store_test.go:76-84`.)
```go
s := NewStore()
created, _ := s.Create("findme")
got, err := s.Get(created.ID)
// expect: err == nil
// expect: got.Name == "findme"
// expect: got.ID == created.ID
```

**AT-2 — missing id returns ErrNotFound**
(Grounded: `store.go:57-58`; verified `store_test.go:86-88`.)
```go
s := NewStore()
_, err := s.Get(999)
// expect: errors.Is(err, ErrNotFound) == true
```

**AT-3 — missing id returns the zero-value Item**
(Grounded: `store.go:58`.)
```go
s := NewStore()
got, _ := s.Get(999)
// expect: got == Item{}  (ID==0, Name=="", Done==false, CreatedAt zero)
```

**AT-4 — exact-key match only (no fuzzy / off-by-one)**
(Grounded: `store.go:56`; ids start at 1 per `store.go:28,46-48`.)
```go
s := NewStore()
first, _ := s.Create("a") // first.ID == 1
_, err0 := s.Get(0)       // expect errors.Is(err0, ErrNotFound)
_, err2 := s.Get(2)       // expect errors.Is(err2, ErrNotFound)
got, err1 := s.Get(1)     // expect err1 == nil and got.ID == 1
```

**AT-5 — ErrNotFound message text**
(Grounded: `store.go:13`. Not asserted by upstream tests; assert as a guard.)
```go
// expect: ErrNotFound.Error() == "item not found"
```

---

## 5. Build steps (function/unit-sized, each individually checkable)

Assume the `seeds` package already has the `Store` type, `NewStore`, and
`Create` from the prerequisite slice. Add the following.

**Step 1 — Ensure the `ErrNotFound` sentinel exists.**
In `store.go`, inside the package-level error `var (...)` block, define:
`ErrNotFound = errors.New("item not found")`. Requires `import "errors"`.
- Check: `ErrNotFound.Error() == "item not found"` and it is a single shared
  package variable (so `errors.Is` works by identity).
- Grounded: `src/examples/seeds-go/store.go:13`.

**Step 2 — Ensure the `Item` struct exists with correct JSON tags.**
In `item.go`: struct with `ID int json:"id"`, `Name string json:"name"`,
`Done bool json:"done"`, `CreatedAt time.Time json:"createdAt"`. Requires
`import "time"`.
- Check: `json.Marshal(Item{ID:1,Name:"x"})` yields keys `id,name,done,createdAt`.
- Grounded: `src/examples/seeds-go/item.go:13-18`.

**Step 3 — Implement `Store.Get`.**
Add the method to `store.go`:
```go
// Get returns the item with the given id, or ErrNotFound.
func (s *Store) Get(id int) (Item, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    it, ok := s.items[id]
    if !ok {
        return Item{}, ErrNotFound
    }
    return it, nil
}
```
- Check: compiles; method set on `*Store`.
- Grounded: `src/examples/seeds-go/store.go:52-61`.

**Step 4 — Verify locking.**
Confirm the first two statements are `s.mu.Lock()` then `defer s.mu.Unlock()`,
and that no other lock is taken. The read is fully serialized.
- Check: `go vet ./...` is clean; no double-lock.
- Grounded: `src/examples/seeds-go/store.go:54-55`.

**Step 5 — Verify the two branches.**
Exactly one not-found return (`Item{}, ErrNotFound`) and one found return
(`it, nil`). No other return paths, no panics.
- Check: AT-1 and AT-2 pass.
- Grounded: `src/examples/seeds-go/store.go:56-60`.

**Step 6 — Run the acceptance tests.**
Add tests covering AT-1 through AT-5 (see section 4). From
`src/examples/seeds-go`, run `go test ./...`.
- Check: all pass; in particular `errors.Is(err, ErrNotFound)` for a missing id.

---

## 6. Kernel references (rely on; do NOT restate)

This slice relies on the following standard-library and package conventions.
Reference them; do not redefine their semantics here.

- `sync.Mutex` (`Lock`/`Unlock`) — used as the `Store.mu` field for thread
  safety. Convention: `Lock()` then `defer Unlock()`.
- `errors.New` — constructs the `ErrNotFound` sentinel.
- `errors.Is` — the canonical way callers (and tests) compare against
  `ErrNotFound`; `Get` must return the sentinel by identity for this to work.
- `time.Time` — type of the `Item.CreatedAt` field.
- Go map two-value index form `v, ok := m[k]` — the `ok` boolean drives the
  found/not-found branch.
- Go zero values — `Item{}` is the documented zero return on not-found.
- Package layout convention: this code lives in `package seeds`; `Get`,
  `ErrNotFound`, and `Item` are all in the same package, so no import is needed
  between them.

**Store facts this slice assumes from the prerequisite slice (do not re-derive):**
- `s.items` is `map[int]Item`, initialized empty by `NewStore`
  (`store.go:21,28`).
- IDs are assigned sequentially starting at `1` by `Create`
  (`store.go:28` `nextID:1`, `store.go:46-48` assign + increment). Therefore
  valid stored keys are positive ints; `Get(0)` and `Get(<negative>)` never
  match.
