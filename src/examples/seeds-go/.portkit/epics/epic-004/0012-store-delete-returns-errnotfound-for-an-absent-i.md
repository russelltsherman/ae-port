# Slice EPIC-004-S2 (build #12): Store.Delete returns ErrNotFound for an absent id

**Capability:** Given an id that does NOT exist in the store, `Store.Delete` returns the sentinel `ErrNotFound` error and leaves the store unchanged.

---

## 1. End-to-end behavior thread

This slice covers the absent-id branch of `Store.Delete`. The full method body and its supporting sentinel are:

| # | Component | Source `path:line` |
|---|-----------|--------------------|
| 1 | `ErrNotFound` sentinel definition — `errors.New("item not found")` | `store.go:13` |
| 2 | `Delete` acquires the mutex and defers unlock | `store.go:77-78` |
| 3 | Comma-ok map lookup `if _, ok := s.items[id]; !ok` | `store.go:79` |
| 4 | Absent-id branch: `return ErrNotFound` (no mutation) | `store.go:80` |
| 5 | Present-id branch (NOT this slice; counterpart S1): `delete(...)` + `return nil` | `store.go:82-83` |

Behavior trace for an absent id:
1. Caller invokes `s.Delete(id)` where `id` is not a key in `s.items`.
2. `Delete` locks `s.mu` and defers the unlock (`store.go:77-78`).
3. It performs a comma-ok lookup on `s.items[id]`; the value is discarded with `_` and only the presence flag `ok` is tested (`store.go:79`).
4. Because the key is absent, `ok == false`, so the negated condition `!ok` is true and the method returns the package-level sentinel `ErrNotFound` (`store.go:80`).
5. The return at `store.go:80` precedes any mutating call. `delete(s.items, id)` at `store.go:82` is never reached, so the map is unchanged.

---

## 2. Interface / contract

### Signature (this slice constrains the absent-id path only)
```go
func (s *Store) Delete(id int) error
```
Source: `store.go:76`.

### Inputs
- `id int` — the id to delete. For THIS slice, `id` is any int that is **not** currently a key in `s.items` (the absent case). This includes:
  - an id that was never created,
  - an id that was created and already deleted,
  - the zero value `0` or negative ids (which are never assigned, since ids start at 1 per `store.go:18` / `store.go:28`).

### Output
- Returns a single `error` value.
- **Absent id (this slice):** returns exactly the package-level sentinel `ErrNotFound` (the same `error` value defined at `store.go:13`). Source: `store.go:79-80`.

### EXACT behavior, errors, edge cases, ordering
1. **Returned error is the sentinel itself.** `Delete` returns the package-level `ErrNotFound` variable directly (`store.go:80`); it does not wrap it. Callers MUST compare with `errors.Is(err, ErrNotFound)`. Because it is the identical value, `err == ErrNotFound` would also be true, but `errors.Is` is the required, future-proof comparison (matching the test at `store_test.go:119`).
2. **Error message is exactly `item not found`.** Source: `store.go:13` — `ErrNotFound = errors.New("item not found")`. No prefix, suffix, or punctuation.
3. **No mutation on absent id.** The early `return ErrNotFound` at `store.go:80` executes before `delete(s.items, id)` at `store.go:82`. Therefore on an absent-id call: no item is added, no item is removed, and no id counter (`s.nextID`) is touched (`s.nextID` is only modified in `Create` at `store.go:48`, never in `Delete`).
   - [PARTIALLY UNVERIFIED] The existing source test does not re-list or re-count the store after a failed `Delete` to empirically prove non-mutation; this non-mutation guarantee is grounded in source code (`store.go:79-82`) only, not in an existing assertion.
4. **Mutual exclusivity with S1.** Exactly one of `{nil, ErrNotFound}` is returned per `Delete` call: `nil` for a present id (S1, `store.go:82-83`), `ErrNotFound` for an absent id (this slice, `store.go:79-80`). There is no other return path.
5. **Thread safety / ordering.** The lookup is performed while holding `s.mu` (locked at `store.go:77`, unlocked via `defer` at `store.go:78`). The lock is held for the entire decision so the presence check and (in S1) the delete are atomic with respect to other `Store` methods.
6. **No panic.** A comma-ok read of a missing key on a non-nil map is safe in Go and never panics; the map is always non-nil because `NewStore` initializes it (`store.go:28`).

---

## 3. Prerequisite slices (build order)

- This is build **#12**.
- `dependsOn: ["EPIC-004-S1"]`.
- **EPIC-004-S1** implements the present-id (success) path of `Store.Delete`: `delete(s.items, id)` then `return nil` (`store.go:82-83`), and establishes the `Delete` method signature, the mutex lock/defer scaffolding (`store.go:77-78`), and the `Store` type with its `items map[int]Item` field (`store.go:19-24`) plus `NewStore` (`store.go:27-29`).
- This slice (S2) only ADDS the absent-id branch (`store.go:79-80`) in front of the S1 delete. Do NOT re-specify or change S1's success path. Do NOT depend on the internals of any slice other than S1, and only depend on S1's public surface (the `Delete` method existing and the `Store`/`NewStore` types).

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec and the existing source test `TestStoreDelete` (`store_test.go:109-122`). They are runnable-in-spirit Go test fragments in package `seeds`.

**AT-1 — Deleting an already-deleted id returns ErrNotFound (identity-comparable).**
Grounded in `store_test.go:113` (first delete succeeds) and `store_test.go:119` (second delete returns `ErrNotFound`); behavior at `store.go:79-80`.
```go
s := NewStore()
it, _ := s.Create("temp")
if err := s.Delete(it.ID); err != nil {        // first delete succeeds (S1 path)
    t.Fatalf("Delete existing: %v", err)
}
if err := s.Delete(it.ID); !errors.Is(err, ErrNotFound) {  // second: now absent
    t.Errorf("Delete missing err = %v, want ErrNotFound", err)
}
```

**AT-2 — Deleting a never-created id returns ErrNotFound.**
Grounded in `store.go:79-80` (any absent key). (Analogous to `store_test.go:86` which exercises the same absent-key/`ErrNotFound` shape for `Get` with id `999`.)
```go
s := NewStore()
if err := s.Delete(999); !errors.Is(err, ErrNotFound) {
    t.Errorf("Delete never-created err = %v, want ErrNotFound", err)
}
```

**AT-3 — The not-found error message is exactly `item not found`.**
Grounded in `store.go:13`.
```go
if ErrNotFound.Error() != "item not found" {
    t.Errorf("ErrNotFound message = %q, want %q", ErrNotFound.Error(), "item not found")
}
```

**AT-4 — Absent-id delete leaves the store unchanged (non-mutation).**
[PARTIALLY UNVERIFIED — not asserted by the existing source test; grounded in `store.go:79-82` only. Include as a strengthening test for this slice.]
```go
s := NewStore()
a, _ := s.Create("a")
b, _ := s.Create("b")
before := len(s.List())                 // S1/List prerequisite; expect 2
if err := s.Delete(999); !errors.Is(err, ErrNotFound) {
    t.Fatalf("Delete absent err = %v, want ErrNotFound", err)
}
if after := len(s.List()); after != before {
    t.Errorf("store size changed after failed Delete: before=%d after=%d", before, after)
}
// Both originally-created ids still resolve.
if _, err := s.Get(a.ID); err != nil { t.Errorf("a missing after failed delete: %v", err) }
if _, err := s.Get(b.ID); err != nil { t.Errorf("b missing after failed delete: %v", err) }
```

**AT-5 — Mutual exclusivity: exactly one of {nil, ErrNotFound}.**
Grounded in `store.go:79-83`. A present id yields `nil` (S1); the same id afterward yields `ErrNotFound` (this slice). Covered structurally by AT-1.

---

## 5. Build steps (function/unit-sized, individually checkable)

> Assumes S1 is already built: the `Store` type, `items map[int]Item` field, `NewStore`, and the `Delete` success path all exist. This slice only adds the absent-id guard.

1. **Ensure the sentinel exists.** Confirm a package-level `var ErrNotFound = errors.New("item not found")` in the `var ( ... )` block (`store.go:13`). Import `"errors"` (`store.go:4`). *Check:* `ErrNotFound.Error()` equals `"item not found"` (AT-3).

2. **Confirm the locked scaffolding from S1.** In `func (s *Store) Delete(id int) error`, the first two statements must be `s.mu.Lock()` and `defer s.mu.Unlock()` (`store.go:77-78`). The absent-id check must run while the lock is held. *Check:* method compiles; lock precedes the lookup.

3. **Add the comma-ok presence check guard.** Immediately after the deferred unlock and before any `delete`, insert:
   ```go
   if _, ok := s.items[id]; !ok {
       return ErrNotFound
   }
   ```
   Discard the value with `_`; test only `!ok` (`store.go:79-80`). This MUST appear before the S1 `delete(s.items, id)` line (`store.go:82`). *Check:* AT-1 and AT-2 pass (absent id returns `ErrNotFound`); AT-4 passes (store unchanged).

4. **Verify return is the bare sentinel.** The guard returns `ErrNotFound` directly — not `fmt.Errorf("...%w", ErrNotFound)` and not a new error. *Check:* `errors.Is(err, ErrNotFound)` is true (AT-1).

5. **Verify mutual exclusivity / no regression of S1.** A present id still returns `nil` and removes the item (S1, `store.go:82-83`); deleting it again returns `ErrNotFound`. *Check:* AT-1 (both halves) and AT-5 pass.

6. **Run the package tests.** `go test ./...` from `src/examples/seeds-go`. *Check:* `TestStoreDelete` (`store_test.go:109-122`) and the slice ATs pass.

---

## 6. Kernel references (Go stdlib / language conventions relied on — do NOT restate)

- **`errors` package** (stdlib): `errors.New` to define the sentinel (`store.go:13`); `errors.Is` for identity comparison by callers/tests (`store_test.go:119`). Reference the standard `errors` semantics; do not re-implement.
- **Go sentinel-error convention:** a package-level `var Err... = errors.New(...)` compared via `errors.Is`. This is the established convention across this file (`ErrNotFound`, `ErrDuplicate` at `store.go:13-14`).
- **Go comma-ok map access:** `v, ok := m[k]` returns the zero value and `ok == false` for an absent key; safe (never panics) on a non-nil map. The map is initialized non-nil in `NewStore` (`store.go:28`).
- **`sync.Mutex`** (stdlib): `Lock` / `defer Unlock` for the critical section (`store.go:77-78`). Reference standard mutex semantics.
- **Testing convention:** standard `testing` package, package `seeds`, table/inline assertions with `t.Errorf` / `t.Fatalf` as in `store_test.go`.

Do NOT depend on the internals of any slice other than S1, and do NOT restate kernel/stdlib behavior beyond the references above.
