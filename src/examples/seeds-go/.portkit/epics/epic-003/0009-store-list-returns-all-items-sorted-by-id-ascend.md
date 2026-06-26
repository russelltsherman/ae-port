# Slice 0009 — `Store.List` returns all items sorted by ID ascending

**Capability:** Calling `Store.List()` returns a slice containing every stored `Item`, ordered strictly ascending by `ID` (which equals creation order, since IDs are assigned sequentially starting at 1).

---

## Build order

- This is build **#9** in epic `EPIC-003` (slice id `EPIC-003-S1`).
- **dependsOn:** `["KERNEL-item-model"]`
- Prerequisite slices: the kernel must already exist (the `Item` type, the `Store` struct, and `NewStore`). See `.portkit/KERNEL.md`. This slice depends on **no other slice's internals** — only on the kernel.

> NOTE: `Store.List` reads the same `s.items` map that `Store.Create` populates. This slice does **not** depend on the Create slice's code; it only relies on the kernel-defined `Store` struct shape and on the invariant (documented in the kernel) that IDs are sequential starting at 1. The acceptance tests below use `Store.Create` to set up state, so to *run* the tests the Create slice must also be present — but the *implementation* of `List` is self-contained.

---

## Kernel references (do NOT restate the kernel — reference it)

This slice relies on the following kernel-defined artifacts. Build them per `.portkit/KERNEL.md`, not here:

- **Package:** `seeds` (library package at module root; `item.go:1`, `store.go:1`).
- **`Item` struct** — kernel `item.go:13-18`. Fields: `ID int`, `Name string`, `Done bool`, `CreatedAt time.Time`. Only `ID` is load-bearing for this slice (it is the sort key). `List` returns whole `Item` values, so all four fields ride along unchanged.
- **`Store` struct** — kernel `store.go:19-24`. Fields: `mu sync.Mutex`, `items map[int]Item`, `nextID int`, `now func() time.Time`. This slice reads `s.items` and `s.mu`.
- **`NewStore() *Store`** — kernel `store.go:27-29`. Initializes `items` to an empty non-nil map and `nextID` to `1`. Used by the tests to get a fresh store.
- **ID invariant** — kernel domain vocabulary ("**id**", `KERNEL.md`): sequential integer key, starting at 1, never reused. This is why ascending-by-ID equals creation order.

Standard library imports this slice adds to `store.go`: `sort` (`store.go:5`) and `sync` (`store.go:7`, already present for the mutex).

---

## End-to-end behavior thread (each component with source `path:line`)

1. **Method entry** — `func (s *Store) List() []Item` declared on the `Store` struct — `store.go:64`.
2. **Mutex guard (read under lock)** — `s.mu.Lock()` then `defer s.mu.Unlock()` — `store.go:65-66`. The whole read happens under the lock; the unlock is deferred so it runs on return.
3. **Allocate non-nil result slice** — `out := make([]Item, 0, len(s.items))` — `store.go:67`. Zero length, capacity `len(s.items)`. This guarantees a non-nil empty slice when the store is empty.
4. **Collect map values** — `for _, it := range s.items { out = append(out, it) }` — `store.go:68-70`. Ranges over the `map[int]Item`; Go map iteration order is **random/unspecified**, so `out` is unordered at this point.
5. **Sort ascending by ID** — `sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })` — `store.go:71`. Strict `<` comparator orders by the `ID` field ascending.
6. **Return** — `return out` — `store.go:72`.
7. **Why ascending == creation order** — IDs start at 1 (`store.go:28`, kernel) and only increment (`store.go:48` `nextID++` in Create). So a slice sorted ascending by ID is the same as the order items were created.

---

## Interface / contract — EXACT behavior

### Signature

```go
func (s *Store) List() []Item
```

(`store.go:64`)

### Inputs

- The receiver `s *Store` only. No parameters.

### Outputs

- A `[]Item` containing **every** item currently in `s.items` — one element per stored item, each a full `Item` value copy (`ID`, `Name`, `Done`, `CreatedAt`).

### Exact behavior, ordering, and edge cases

1. **Length.** `len(List())` equals the number of items currently stored (`len(s.items)`). One element per stored item, no duplicates, no omissions. (`store.go:67-70`)
2. **Ordering guarantee (total + strictly ascending by ID).** For every adjacent pair in the result, `result[i-1].ID < result[i].ID`. Because no two stored items share an ID (IDs are unique map keys, `store.go:21` + sequential assignment `store.go:46,48`), the ordering is a *total* strict ascending order — there are never ties. (`store.go:71`)
3. **Equivalence to creation order.** Since IDs are assigned 1, 2, 3, … in creation order (`store.go:28,46,48`), the ascending-ID result is identical to the order items were created.
4. **Empty store → non-nil empty slice.** With zero items, `List()` returns a slice of length 0 that is **non-nil** (because it is built with `make`, not left as a nil slice). It is never `nil` and never a "null/absent" value. Grounded at `store.go:67`; **test-backed** by `TestStoreListEmptyIsNonNil` (`store_test.go:111-120`), which asserts both `list != nil` and `len(list) == 0`.
5. **Snapshot / copy semantics.** The returned slice is a fresh slice (`store.go:67`) holding copies of the `Item` *values* (Go ranges and appends values, `store.go:68-69`). Mutating the returned slice does not affect the store's internal map. The slice contents reflect the store state at the moment the lock was held.
6. **Thread safety.** The entire collect-and-sort runs while holding `s.mu` (`store.go:65-66`), so `List` is safe to call concurrently with other `Store` methods that also lock `s.mu`. **Test-backed** by `TestStoreConcurrentAccess` (`store_test.go:125-148`), which drives `Create`/`List`/`Get` from 50 goroutines and is run under `go test -race`.
7. **No errors.** `List` has no error return and cannot fail; it never panics for an empty or populated store.
8. **ID reuse after deletion.** `[UNVERIFIED BY TEST]` — IDs are never reused (`nextID` only increments, `store.go:48`; delete does not decrement, kernel `store.go:82`). So after deletions and re-creation the result is still strictly ascending by ID, but there may be gaps in the ID sequence. Not asserted by any List test; grounded only in source.

---

## Acceptance tests for THIS slice

These mirror the behavioral spec. The canonical test is `TestStoreListIsSortedByID` at `store_test.go:91-107`.

### Test 1 — populated store: count and ascending order (test-backed)

Source of truth: `store_test.go:91-107`.

- **GIVEN** a fresh store: `s := NewStore()`.
- **WHEN** you create three items in this order: `"a"`, then `"b"`, then `"c"` (via `s.Create(n)` for each; fail the test if any `Create` returns an error — `store_test.go:93-96`).
- **AND** you call `list := s.List()`.
- **THEN** `len(list) == 3` (`store_test.go:99-101`).
- **AND** for every `i` from 1 to `len(list)-1`, `list[i-1].ID < list[i].ID` — i.e. strictly ascending; the test fails if `list[i-1].ID >= list[i].ID` (`store_test.go:102-106`).

Concrete expectation (grounded): IDs are `[1, 2, 3]` because the first create yields ID 1 (`store_test.go:38`) and the second yields ID 2 (`store_test.go:50-51`), so the third is 3. Names are `["a","b","c"]` in that order — but **the List test does NOT assert names**, only count and ID ordering. The name→order mapping is **inferred** from create semantics, not directly verified by `store_test.go:91`.

### Test 2 — empty store: non-nil, length 0 (test-backed)

Source of truth: `TestStoreListEmptyIsNonNil` at `store_test.go:111-120`. This locks in the `store.go:67` guarantee:

- **GIVEN** a fresh store with no items: `s := NewStore()`.
- **WHEN** you call `list := s.List()`.
- **THEN** `len(list) == 0` (`store_test.go:117-119`).
- **AND** `list != nil` (the slice is non-nil empty, never nil) (`store_test.go:114-116`).

Verbatim test (`store_test.go:111-120`):

```go
func TestStoreListEmptyIsNonNil(t *testing.T) {
    s := NewStore()
    list := s.List()
    if list == nil {
        t.Fatalf("List() on empty store = nil, want non-nil empty slice")
    }
    if len(list) != 0 {
        t.Fatalf("List() on empty store len = %d, want 0", len(list))
    }
}
```

### Test 3 — concurrent access is race-free (test-backed)

Source of truth: `TestStoreConcurrentAccess` at `store_test.go:125-148`. This locks in the thread-safety guarantee (`store.go:20,65-66`):

- **GIVEN** a fresh store `s := NewStore()`.
- **WHEN** 50 goroutines each call `s.Create("item-"+strconv.Itoa(n))` (unique name per goroutine), then `s.List()` and `s.Get(n)`, and the test waits on a `sync.WaitGroup` (`store_test.go:131-144`).
- **THEN** `len(s.List()) == 50` (all distinct-named creates persisted) (`store_test.go:146-148`).
- **AND** run under `go test -race` the test reports no data race.

---

## Function-sized build steps (each individually checkable)

Each step is a small, verifiable edit to `store.go` (package `seeds`).

1. **Ensure imports.** Confirm `store.go` imports `sort` (`store.go:5`) and `sync` (`store.go:7`). Add `sort` if absent. *Check:* `go build ./...` does not complain about an unused/missing `sort` import after step 5.

2. **Declare the method.** Add `func (s *Store) List() []Item {` with a doc comment `// List returns all items sorted by ID ascending (creation order).` (`store.go:63-64`). *Check:* compiles with an empty/`return nil` body.

3. **Lock for the duration.** As the first two lines of the body: `s.mu.Lock()` then `defer s.mu.Unlock()` (`store.go:65-66`). *Check:* `go vet ./...` clean; running with `-race` (step 8) shows no data race.

4. **Allocate the non-nil result slice.** `out := make([]Item, 0, len(s.items))` (`store.go:67`). Use `make` with length 0 and capacity `len(s.items)` — do NOT use `var out []Item` (that would be nil when empty and violates Test 2). *Check:* on an empty store the eventual return is non-nil (Test 2).

5. **Collect map values.** `for _, it := range s.items { out = append(out, it) }` (`store.go:68-70`). Append the value (`it`), not a pointer. *Check:* on a 3-item store, `len(out) == 3` before sorting.

6. **Sort ascending by ID.** `sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })` (`store.go:71`). Use strict `<` (ascending). *Check:* Test 1's ascending assertion passes.

7. **Return.** `return out` (`store.go:72`). *Check:* both acceptance tests pass.

8. **Run the slice tests.** `go test -race ./...` from `src/examples/seeds-go`. *Check:* `TestStoreListIsSortedByID` (`store_test.go:93`), `TestStoreListEmptyIsNonNil` (`store_test.go:111`), and `TestStoreConcurrentAccess` (`store_test.go:125`) all pass; no race detected.

---

## Self-containment note

A less capable model can rebuild `Store.List` from this doc plus `.portkit/KERNEL.md` alone: the full method body, exact ordering/empty/concurrency semantics, imports, and acceptance tests are spelled out, with the only external dependency being kernel-defined types (`Item`, `Store`, `NewStore`).
