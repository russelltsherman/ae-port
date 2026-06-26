# Slice EPIC-001-S4 — Store.Create: persist with validation, duplicate check, sequential ID

**Capability:** Validate a proposed name, reject case-insensitive duplicates, and insert a new item with a trimmed name and a sequential integer ID into the thread-safe in-memory store.

- **Epic:** EPIC-001
- **Build number:** 2 (build this second in EPIC-001)
- **Source root:** `src/examples/seeds-go` (all `path:line` citations are relative to this root)
- **Target symbol:** `func (s *Store) Create(name string) (Item, error)` in `store.go`

---

## 1. End-to-end behavior thread

Each component below is part of the single `Store.Create` code path, in execution order, with its source location.

1. **`Store` struct** — `store.go:19-24`. Fields: `mu sync.Mutex`, `items map[int]Item`, `nextID int`, `now func() time.Time`. (Kernel type — see §7.)
2. **`NewStore` constructor** — `store.go:27-29`. Returns `&Store{items: map[int]Item{}, nextID: 1, now: time.Now}`. Sets the starting ID to `1` and injects `time.Now` as the timestamp source. (Kernel symbol — see §7.)
3. **Validate first** — `store.go:34-36`. `Create` calls `ValidateName(name)`; if it returns a non-nil error, `Create` returns `(Item{}, err)` immediately, propagating the error value unchanged.
4. **Trim the input** — `store.go:37`. `trimmed := strings.TrimSpace(name)`. The trimmed value (not the raw input) is what gets stored and what is compared for duplicates.
5. **Lock for thread safety** — `store.go:39-40`. `s.mu.Lock()` followed by `defer s.mu.Unlock()`. The entire scan-and-insert section runs under the mutex.
6. **Case-insensitive duplicate check** — `store.go:41-44`. Iterate every existing item; if any stored `it.Name` matches `trimmed` under `strings.EqualFold`, return `(Item{}, ErrDuplicate)`. `strings.EqualFold` is Unicode-aware case folding.
7. **Construct the item** — `store.go:46`. `it := Item{ID: s.nextID, Name: trimmed, CreatedAt: s.now()}`. `Done` is left at its zero value `false`.
8. **Insert and advance the counter** — `store.go:47-48`. `s.items[it.ID] = it` then `s.nextID++`. The counter is incremented **only** on a successful insert.
9. **Return** — `store.go:49`. `return it, nil`.
10. **`ErrDuplicate` sentinel** — `store.go:14`. `ErrDuplicate = errors.New("an item with that name already exists")`. (Kernel sentinel — see §7.)

---

## 2. Interface / contract

### Signature

```go
func (s *Store) Create(name string) (Item, error)
```

`name` is the raw, user-supplied name (may contain surrounding whitespace and any casing). Returns the inserted `Item` and `nil` on success, or a zero-value `Item{}` and a non-nil error on failure.

### Exact behavior, in order

1. **Validation precedes everything.** Call `ValidateName(name)` first (`store.go:34`).
   - If `name` trimmed of surrounding whitespace is empty (`""` or all-whitespace) → return `(Item{}, ErrNameRequired)`.
   - If the trimmed name's **rune** length exceeds `MaxNameLen` (100) → return `(Item{}, ErrNameTooLong)`.
   - The error is returned **unchanged** (same sentinel value), with no wrapping (`store.go:35`).
   - On a validation error, the mutex is never taken, no scan occurs, no item is inserted, and `nextID` is NOT advanced.
2. **Trim** the input: `trimmed = strings.TrimSpace(name)` (`store.go:37`). This is the canonical form used for both storage and comparison.
3. **Lock** the mutex for the remainder of the operation (`store.go:39-40`).
4. **Duplicate check** (`store.go:41-44`): for each already-stored item, compare its `Name` (already trimmed, see invariant below) against `trimmed` using `strings.EqualFold`. On the first match, return `(Item{}, ErrDuplicate)`. The duplicate check is therefore both **case-insensitive** (via `EqualFold`) and **trim-insensitive** for the candidate (because the candidate was trimmed in step 2). On a duplicate, no item is inserted and `nextID` is NOT advanced.
5. **Construct** `Item{ID: s.nextID, Name: trimmed, CreatedAt: s.now()}` (`store.go:46`). `Done` is the zero value `false`; no code path sets it to `true` here.
6. **Insert** into `s.items` keyed by the new ID, then **increment** `s.nextID` (`store.go:47-48`).
7. **Return** the newly constructed item and `nil` (`store.go:49`).

### Inputs

- `name string` — raw name. Casing and surrounding whitespace are caller-controlled.

### Outputs

- Success: `(Item{ID: <assigned>, Name: <trimmed>, Done: false, CreatedAt: <now()>}, nil)`.
- Validation failure: `(Item{}, ErrNameRequired)` or `(Item{}, ErrNameTooLong)`.
- Duplicate failure: `(Item{}, ErrDuplicate)`.

### Ordering guarantees

- **Validation happens before the lock and before the duplicate scan** (`store.go:34-45`). A validation error is returned before any state is read or written.
- **IDs are sequential and strictly increasing by 1 per successful create**, starting at `1` (`store.go:28`, `store.go:46-48`). First success → ID `1`, second → ID `2`, etc.
- **`nextID` is consumed only on success.** A rejected create (validation or duplicate) does NOT consume an ID; the next successful create reuses the would-be ID (`store.go:35`, `store.go:43`, `store.go:46-48`).

### Invariants relied upon

- Every stored `Name` is already trimmed, because every insert path passes `trimmed` (`store.go:37,46`). This is why comparing the trimmed candidate against stored names with `EqualFold` is sufficient for trim-insensitive duplicate detection.
- The store is thread-safe: all reads and writes of `items`/`nextID` occur under the single `s.mu` mutex (`store.go:20,39-40`). Test-backed by `TestStoreConcurrentAccess` (`store_test.go:125-148`), which drives `Create`/`List`/`Get` from 50 concurrent goroutines and is run under `go test -race`; it asserts no race and that all 50 distinct-named creates persist.

---

## 3. Prerequisite slices (build order)

- **Build order:** this is slice **#2** in EPIC-001.
- **dependsOn:** `["KERNEL-item-model", "EPIC-001-S3"]`
  - `KERNEL-item-model` — provides the `Item` struct, `MaxNameLen`, the `Store` struct + `NewStore`, and the sentinel errors (`ErrDuplicate`, `ErrNameRequired`, `ErrNameTooLong`). See §7.
  - `EPIC-001-S3` — provides `ValidateName(name string) error`. `Create` calls it directly (`store.go:34`). This slice depends only on that function's **public contract** (described in §7), not on its internals.

Do not depend on any other slice's internals.

---

## 4. Acceptance tests for THIS slice

These mirror the existing behavioral spec in `store_test.go`. They are concrete and runnable-in-spirit; a rebuild must satisfy all of them. Tests live in package `seeds`.

> Test grounding: `store_test.go:31-53` (`TestStoreCreate`) and `store_test.go:55-72` (`TestStoreCreateValidationAndDuplicates`).

1. **Sequential IDs starting at 1.** On a fresh `NewStore()`, `Create("  Write tests  ")` returns an item with `ID == 1`; a subsequent `Create("Second")` returns `ID == 2`. IDs increment by exactly 1 per success. (VERIFIED `store_test.go:38,50`; grounded `store.go:28,46-48`.)
2. **Stored name is trimmed.** `Create("  Write tests  ")` returns an item whose `Name == "Write tests"` (surrounding whitespace removed). (VERIFIED `store_test.go:34,41-42`; grounded `store.go:37,46`.)
3. **Empty name rejected.** `Create("")` returns an error matching `errors.Is(err, ErrNameRequired)` and creates nothing. (VERIFIED `store_test.go:58`; grounded `store.go:34-36`.)
4. **Over-length name rejected.** `Create(strings.Repeat("y", MaxNameLen+1))` (101 runes) returns an error matching `errors.Is(err, ErrNameTooLong)` and creates nothing. (VERIFIED `store_test.go:61`; grounded `store.go:34-36`.)
5. **Duplicate is case- and trim-insensitive.** After `Create("Unique")` succeeds, `Create("  unique ")` (different case + surrounding whitespace) returns an error matching `errors.Is(err, ErrDuplicate)` and creates nothing. The error message is exactly `"an item with that name already exists"`. (VERIFIED `store_test.go:65,69`; grounded `store.go:41-44`.)
6. **Success returns the inserted item with nil error.** A successful `Create` returns the populated item (assigned ID, trimmed name) and `err == nil`. (VERIFIED `store_test.go:34-43`; grounded `store.go:46-49`.)
7. **Validation precedes duplicate check.** An invalid name returns its validation error before any duplicate scan. (Grounded `store.go:34-45`; ordering, not separately asserted by a dedicated test.)
8. **Rejected create consumes no ID.** After one or more rejected creates, the next successful create gets the lowest unused sequential ID (no gap introduced by the rejections). (Grounded `store.go:35,43,46-48`; ordering, not separately asserted by a dedicated test.)

### Suggested test skeleton (runnable-in-spirit)

```go
func TestStoreCreate(t *testing.T) {
    s := NewStore()
    it, err := s.Create("  Write tests  ")
    if err != nil { t.Fatalf("Create: unexpected error %v", err) }
    if it.ID != 1 { t.Errorf("first item ID = %d, want 1", it.ID) }
    if it.Name != "Write tests" { t.Errorf("Name = %q, want %q", it.Name, "Write tests") }

    it2, err := s.Create("Second")
    if err != nil { t.Fatalf("Create second: %v", err) }
    if it2.ID != 2 { t.Errorf("second item ID = %d, want 2", it2.ID) }
}

func TestStoreCreateValidationAndDuplicates(t *testing.T) {
    s := NewStore()
    if _, err := s.Create(""); !errors.Is(err, ErrNameRequired) {
        t.Errorf("empty name err = %v, want ErrNameRequired", err)
    }
    if _, err := s.Create(strings.Repeat("y", MaxNameLen+1)); !errors.Is(err, ErrNameTooLong) {
        t.Errorf("long name err = %v, want ErrNameTooLong", err)
    }
    if _, err := s.Create("Unique"); err != nil {
        t.Fatalf("Create: %v", err)
    }
    if _, err := s.Create("  unique "); !errors.Is(err, ErrDuplicate) {
        t.Errorf("duplicate err = %v, want ErrDuplicate", err)
    }
}
```

---

## 5. Build steps (function/unit-sized, each individually checkable)

Build these in order. Each step is independently verifiable.

1. **Confirm kernel prerequisites exist** (from `KERNEL-item-model`): the `Item` struct, `MaxNameLen`, the `Store` struct, `NewStore`, and the `ErrDuplicate` sentinel. Confirm `ValidateName` exists (from `EPIC-001-S3`). See §7 for exact contracts. *Checkable: the package compiles with these symbols referenced.*
2. **Declare the method**: `func (s *Store) Create(name string) (Item, error)` on the `Store` type in `store.go` (`store.go:33`). *Checkable: signature compiles.*
3. **Add validation guard** (`store.go:34-36`):
   ```go
   if err := ValidateName(name); err != nil {
       return Item{}, err
   }
   ```
   *Checkable: `Create("")` returns `ErrNameRequired`; `Create(101 runes)` returns `ErrNameTooLong`.*
4. **Trim the input** (`store.go:37`): `trimmed := strings.TrimSpace(name)`. *Checkable: stored name has no surrounding whitespace.*
5. **Acquire the lock** (`store.go:39-40`): `s.mu.Lock()` then `defer s.mu.Unlock()`. *Checkable: compiles; no double-lock.*
6. **Duplicate scan** (`store.go:41-45`):
   ```go
   for _, it := range s.items {
       if strings.EqualFold(it.Name, trimmed) {
           return Item{}, ErrDuplicate
       }
   }
   ```
   *Checkable: after creating `"Unique"`, `Create("  unique ")` returns `ErrDuplicate`.*
7. **Construct, insert, increment** (`store.go:46-48`):
   ```go
   it := Item{ID: s.nextID, Name: trimmed, CreatedAt: s.now()}
   s.items[it.ID] = it
   s.nextID++
   ```
   *Checkable: first success → ID 1, second → ID 2; `Done == false`; `CreatedAt` set from `s.now()`.*
8. **Return success** (`store.go:49`): `return it, nil`. *Checkable: success returns the populated item and nil.*
9. **Add the required imports** to `store.go`: `errors`, `strings`, `sync`, `time` (and `sort` is used by sibling methods if present, but `Create` itself uses `strings`, `sync` indirectly via the struct, and `time` via the struct/`now`). *Checkable: `go build` succeeds.*
10. **Write and run the acceptance tests** from §4. *Checkable: `go test ./...` passes.*

---

## 6. Exact reference implementation (for parity)

This is the verbatim contract of the method as it exists at `store.go:33-50`. A rebuild should match this behavior exactly.

```go
// Create validates name, rejects a case-insensitive duplicate, and inserts a
// new item. The stored name is the trimmed form of the input.
func (s *Store) Create(name string) (Item, error) {
    if err := ValidateName(name); err != nil {
        return Item{}, err
    }
    trimmed := strings.TrimSpace(name)

    s.mu.Lock()
    defer s.mu.Unlock()
    for _, it := range s.items {
        if strings.EqualFold(it.Name, trimmed) {
            return Item{}, ErrDuplicate
        }
    }
    it := Item{ID: s.nextID, Name: trimmed, CreatedAt: s.now()}
    s.items[it.ID] = it
    s.nextID++
    return it, nil
}
```

---

## 7. Kernel references (do NOT restate the kernel; reference it)

This slice relies on the following kernel-provided names/types/conventions. Their definitions live in the kernel (`.portkit/KERNEL.md`) and the `KERNEL-item-model` slice. Do **not** redefine them here; reference them.

- **`Item` struct** — kernel type (`item.go:13-18`). Fields `ID int`, `Name string`, `Done bool`, `CreatedAt time.Time`. `Create` sets `ID`, `Name`, `CreatedAt` and leaves `Done` at zero (`false`).
- **`Store` struct + `NewStore`** — kernel type/constructor (`store.go:19-29`). `NewStore` sets `nextID = 1` and `now = time.Now`. `Create` is one of the methods on this struct; only the struct and `NewStore` are kernel, the method bodies belong to their slices.
- **`MaxNameLen` constant** — kernel constant `= 100` (`item.go:10`). Used transitively via `ValidateName`.
- **`ErrDuplicate` sentinel** — kernel error `errors.New("an item with that name already exists")` (`store.go:14`). Matched by callers with `errors.Is`, not `==`.
- **`ErrNameRequired` / `ErrNameTooLong` sentinels** — kernel errors (`item.go:22-23`). Propagated unchanged from `ValidateName`.
- **`ValidateName(name string) error`** — provided by prerequisite slice `EPIC-001-S3`. Contract: trims surrounding whitespace; empty/all-whitespace → `ErrNameRequired`; trimmed **rune** length > `MaxNameLen` → `ErrNameTooLong`; otherwise `nil`. `Create` depends only on this contract, not the function's internals.
- **Convention — package** — all library code is in package `seeds` (`store.go:1`).
- **Convention — sentinel errors via `errors.Is`** — error identity across the store→handler boundary is by sentinel value compared with `errors.Is` (kernel: "Sentinel error glossary").
- **Convention — trimmed + case-insensitive name uniqueness** — names are stored trimmed and compared case-insensitively via `strings.EqualFold` (kernel: "Domain vocabulary → name").
- **Standard library only** — uses `strings.TrimSpace`, `strings.EqualFold` (`strings`), `sync.Mutex` (via the struct), and `time.Time`/`time.Now` (via the struct). No third-party deps (`go.mod` has no `require` block).
