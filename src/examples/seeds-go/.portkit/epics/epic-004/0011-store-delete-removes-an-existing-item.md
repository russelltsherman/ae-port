# Slice 0011 — Store.Delete removes an existing item

**Capability:** Given an `id` that exists in the store, the item is removed from the
in-memory map and a `nil` error is returned; a subsequent `Get`/`List` no longer sees it.

- Slice id: `EPIC-004-S1`
- Epic: `EPIC-004`
- Build number: 11
- dependsOn: `["KERNEL-item-model"]`

> Source root for every `path:line` citation below: `src/examples/seeds-go`.
> The library package is `seeds` (`store.go:1`). All code in this slice lives in
> package `seeds` at the module root.

---

## 1. End-to-end behavior thread

This slice adds exactly one method to the existing kernel `Store` type. The thread is:

1. **`Store.Delete(id int) error` method** — `store.go:76-84`.
   Acquires the mutex, looks up the id, deletes the map entry when present, returns `nil`.
   This is the ONLY code this slice creates.
2. **`Store` struct + mutex/map fields** — `store.go:19-24` (KERNEL, do not re-create).
   Provides `mu sync.Mutex`, `items map[int]Item`, `nextID int`, `now func() time.Time`.
   `Delete` reads/writes `s.mu` and `s.items`.
3. **`NewStore` initializer** — `store.go:27-29` (KERNEL, do not re-create).
   Returns a `*Store` with `items` an empty non-nil map, `nextID` = 1, `now` = `time.Now`.

The acceptance tests for this slice also EXERCISE (but do not re-implement) two other
methods that belong to OTHER slices and to the kernel:

- `Store.Create(name string) (Item, error)` — `store.go:33-50` — used only to set up an
  item to delete. It assigns `ID: s.nextID` (`store.go:46`), starting at 1 (`store.go:28`).
- `Store.Get(id int) (Item, error)` — `store.go:53-61` — used only to verify the item is
  gone. Returns `ErrNotFound` when the id is absent from the map (`store.go:57-58`).

> IMPORTANT for the rebuilder: do NOT depend on the internal logic of `Create`/`Get`/`List`.
> Treat them as already-built siblings. If you are rebuilding the whole file, implement
> them per their own slice docs; this doc only specifies `Delete`.

---

## 2. Interface / contract — `Store.Delete`

### Signature

```go
// Delete removes the item with the given id, or returns ErrNotFound.
func (s *Store) Delete(id int) error
```

Citation: `store.go:75-76`.

### Inputs

- `id int` — the integer key of the item to remove. IDs are integer map keys
  (`items map[int]Item`, `store.go:21`).

### Output

- A single `error` value. It is either:
  - `nil` — the id existed and was removed, OR
  - `ErrNotFound` — the id did not exist (no item removed).

### Exact behavior, in order (the rebuilt body MUST do exactly this)

1. `s.mu.Lock()` — acquire the store mutex (`store.go:77`).
2. `defer s.mu.Unlock()` — defer the unlock so it runs on every return path (`store.go:78`).
   The lock is held for the entire body; this is what makes `Delete` thread-safe.
3. Look up the id with the comma-ok form: `if _, ok := s.items[id]; !ok { ... }`
   (`store.go:79`).
4. **Not-found path:** when `ok` is `false`, return `ErrNotFound` immediately — do NOT call
   `delete` (`store.go:80`). `ErrNotFound` is the shared kernel sentinel (see §6).
5. **Found path:** call Go's builtin `delete(s.items, id)` to remove the entry
   (`store.go:82`).
6. Return `nil` (`store.go:83`).

### Edge cases and guarantees (exhaustive)

- **Idempotency / repeat delete:** Deleting an id that is already absent returns
  `ErrNotFound` and mutates nothing. Verified: the acceptance test deletes the same id a
  second time and expects `ErrNotFound` (`store_test.go:119-121`).
- **No ID reuse:** `Delete` does NOT decrement or otherwise touch `s.nextID`. Freed IDs are
  never reissued by `Create`. There is no code path in `Delete` that writes `nextID`
  (`store.go:76-84` contains no `nextID` reference).
- **No timestamp / `now` interaction:** `Delete` never reads `s.now`. The body touches only
  `s.mu` and `s.items` (`store.go:76-84`).
- **Single-key effect:** `delete(s.items, id)` removes exactly the one keyed entry; other
  ids remain present (Go builtin map-delete semantics, `store.go:82`). [PARTIALLY
  UNVERIFIED: no source test deletes one of several items and re-checks the survivors; this
  follows from Go's `delete` semantics, not from a dedicated test.]
- **Return value is the bare sentinel, not a wrapped error.** The not-found path returns
  `ErrNotFound` directly (`store.go:80`), so callers can match it with `errors.Is`.
- **Thread safety:** because the whole body runs under `s.mu` (`store.go:77-78`), concurrent
  `Delete`/`Create`/`Get`/`List` calls cannot observe a partially-mutated map.

---

## 3. Prerequisite slices (build order)

- This is slice **#11**.
- **dependsOn:** `KERNEL-item-model`.
  - The kernel must already provide the `Store` struct (`store.go:19-24`), `NewStore`
    (`store.go:27-29`), the `Item` type (`item.go:13-18`), and the `ErrNotFound` sentinel
    (`store.go:13`). Do NOT re-declare any of these inside this slice.
- No dependency on any other `Store` method slice's *internals*. The acceptance tests call
  `Create` and `Get`, so for the tests to run those methods must exist in the package — but
  this slice does not rely on HOW they are implemented, only on their documented contracts
  (see §1).

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec at `store_test.go:109-122` (`func TestStoreDelete`). They
are concrete and runnable in spirit. The rebuilt code must make ALL of them pass.

> Test file is package `seeds` and imports `errors` and `testing` (`store_test.go:1-7`).
> Use `errors.Is(err, ErrNotFound)` for not-found comparisons (identity-based, see §6).

```go
func TestStoreDelete(t *testing.T) {
    s := NewStore()              // store.go:27-29 — empty map, nextID=1
    it, _ := s.Create("temp")    // store.go:33-50 — it.ID == 1 (first id)

    // (a) Deleting an existing id returns nil.
    if err := s.Delete(it.ID); err != nil {
        t.Fatalf("Delete existing: %v", err)
    }

    // (b) After delete, Get for the same id returns ErrNotFound (item is gone).
    if _, err := s.Get(it.ID); !errors.Is(err, ErrNotFound) {
        t.Errorf("after Delete, Get err = %v, want ErrNotFound", err)
    }

    // (c) Deleting the same (now absent) id again returns ErrNotFound.
    if err := s.Delete(it.ID); !errors.Is(err, ErrNotFound) {
        t.Errorf("Delete missing err = %v, want ErrNotFound", err)
    }
}
```

Mapping to acceptance criteria / source:

- (a) success on existing id — `store_test.go:113-115`; produced by `store.go:82-83`.
- (b) gone afterward — `store_test.go:116-118`; `Get` returns `ErrNotFound` via
  `store.go:57-58` because the key was removed by `store.go:82`.
- (c) repeat delete is `ErrNotFound` — `store_test.go:119-121`; produced by `store.go:79-80`.
- Sentinel message — `ErrNotFound` carries exactly `"item not found"` (`store.go:13`). The
  rebuild MUST compare by identity (`errors.Is`), NOT by string and NOT by allocating a new
  error per call.

Run: `go test ./...` from `src/examples/seeds-go` (module `example.com/seeds-go`,
`go.mod:1`).

---

## 5. Build steps (function/unit-sized, each individually checkable)

1. **Confirm prerequisites exist.** In `store.go`, verify the `Store` struct
   (`store.go:19-24`), `NewStore` (`store.go:27-29`), and the `var ErrNotFound = ...`
   sentinel (`store.go:13`) are present. If rebuilding from scratch, create them per the
   kernel doc first. Checkable: file compiles with those declarations.

2. **Add the method skeleton.** Append to `store.go`:
   ```go
   // Delete removes the item with the given id, or returns ErrNotFound.
   func (s *Store) Delete(id int) error {
       return nil
   }
   ```
   Checkable: `go build ./...` succeeds.

3. **Add locking.** Insert as the first two lines of the body:
   ```go
   s.mu.Lock()
   defer s.mu.Unlock()
   ```
   Checkable: still compiles; `go vet` clean.

4. **Add the presence check + not-found return.** Before the final return:
   ```go
   if _, ok := s.items[id]; !ok {
       return ErrNotFound
   }
   ```
   Checkable: deleting a never-created id returns `ErrNotFound`.

5. **Add the deletion + success return.** Replace the trailing `return nil` so the body ends:
   ```go
   delete(s.items, id)
   return nil
   ```
   Final body matches `store.go:76-84` exactly. Checkable: the §4 test passes.

6. **Verify.** Run `go test ./...` from the module root. All of §4 (a)/(b)/(c) pass.

---

## 6. Kernel references (do NOT restate the kernel; rely on it)

This slice consumes the following kernel artifacts. Reference them; do not re-declare them.

- **`Store` struct** — fields `mu sync.Mutex`, `items map[int]Item`, `nextID int`,
  `now func() time.Time` (`store.go:19-24`). `Delete` uses `s.mu` and `s.items` only.
- **`NewStore() *Store`** — empty non-nil map, `nextID` 1, `now` = `time.Now`
  (`store.go:27-29`). Used in tests to construct the store.
- **`Item` type** — `item.go:13-18`; relevant only as the map value type `map[int]Item`.
- **`ErrNotFound` sentinel** — `var ErrNotFound = errors.New("item not found")`
  (`store.go:13`). A single package-level value; compare with `errors.Is`. `Delete` returns
  this exact value (never a new allocation) on the not-found path.
- **Package / module conventions** — package `seeds` (`store.go:1`); module
  `example.com/seeds-go`, Go 1.26 (`go.mod:1,3`); standard library only (no `require`
  block). Builtin `delete` and `sync.Mutex` need no imports beyond `sync` (already imported
  at `store.go:7`).
