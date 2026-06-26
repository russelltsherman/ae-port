# seeds-go — Epic / Slice Index + Build Order

Normalized vertical slices for the rebuild. Shared, cross-cutting code is hoisted into the
kernel (see `../KERNEL.md` and `../kernel/cross-cutting.md`); each slice below is otherwise
self-contained. Paths cite `src/examples/seeds-go`.

## Normalization notes

- **`SHARED-response-helpers`** is a new shared slice merged from three duplicate
  discoveries of the same `writeJSON` / `writeError` code (`server.go:76-84`):
  `EPIC-001-S1`, `EPIC-002-S5`, `EPIC-003-S2`. Every route slice depends on it. The
  conventions it must obey live in `../kernel/cross-cutting.md` §1.
- **`KERNEL-item-model`** (was `EPIC-001-S2`) is promoted to the kernel: the `Item` struct
  + JSON tags + `MaxNameLen` are shared by validation, both success-response paths, and
  store methods. It is built as kernel code; see `../KERNEL.md`.
- The `Item`-struct references inside `EPIC-002-S3`/`-S6`/`EPIC-003-S1`/`-S2` were just
  re-discoveries of the same type; they are now plain dependencies on `KERNEL-item-model`,
  not separate slices.
- Sentinel errors (`ErrNotFound`, `ErrDuplicate`, `ErrNameRequired`, `ErrNameTooLong`) are
  kernel vocabulary, not slices; their cross-store-method appearances are dependencies, not
  duplicates.
- No contradictions found between epics; the source confirms every cited claim.

## Epic → slice tree

### KERNEL (shared, built first)
- **KERNEL-item-model** — `Item` struct, JSON tags, `MaxNameLen` (`item.go:10,13-18`).
  Merged from `EPIC-001-S2`.
- **SHARED-response-helpers** — `writeJSON` / `writeError` (`server.go:76-84`).
  Merged from `EPIC-001-S1`, `EPIC-002-S5`, `EPIC-003-S2`.

### EPIC-001 — Create item (`POST /items`)
- **EPIC-001-S3** — Name validation `ValidateName` (`item.go:28-37`). deps: KERNEL-item-model.
- **EPIC-001-S4** — `Store.Create`: validate, trim, case-insensitive dup check, sequential
  ID (`store.go:33-50`). deps: KERNEL-item-model, EPIC-001-S3.
- **EPIC-001-S5** — `POST /items` handler: decode body, status routing 201/400/409/500
  (`server.go:21-40`). deps: SHARED-response-helpers, EPIC-001-S4.

### EPIC-002 — Get item by id (`GET /items/{id}`)
- **EPIC-002-S1** — Route registration + `r.PathValue("id")` extraction
  (`server.go:18-19,46-47,73`). deps: none.
- **EPIC-002-S2** — Integer id parse + 400 `"id must be an integer"`
  (`server.go:47-50`). deps: EPIC-002-S1, SHARED-response-helpers.
- **EPIC-002-S3** — `Store.Get` lookup by id → Item or `ErrNotFound`
  (`store.go:53-61`). deps: EPIC-002-S2.
- **EPIC-002-S4** — 404 branch `{"error":"item not found"}`
  (`server.go:53-55`). deps: EPIC-002-S3, SHARED-response-helpers.
- **EPIC-002-S6** — 200 OK success with Item JSON
  (`server.go:57`). deps: EPIC-002-S3, SHARED-response-helpers.

### EPIC-003 — List items (`GET /items`)
- **EPIC-003-S1** — `Store.List` → all items sorted by ID ascending, non-nil empty slice
  (`store.go:64-73`). deps: KERNEL-item-model.
- **EPIC-003-S3** — `GET /items` route → 200 with JSON array
  (`server.go:42-44`). deps: EPIC-003-S1, SHARED-response-helpers.

### EPIC-004 — Delete item by id (`DELETE /items/{id}`)
- **EPIC-004-S1** — `Store.Delete` removes existing item, returns nil
  (`store.go:76-84`). deps: KERNEL-item-model.
- **EPIC-004-S2** — `Store.Delete` returns `ErrNotFound` for absent id
  (`store.go:79-80`). deps: EPIC-004-S1.
- **EPIC-004-S3** — `DELETE /items/{id}` route: parse id, 400 on non-integer
  (`server.go:60-65`). deps: SHARED-response-helpers.
- **EPIC-004-S4** — 204 No Content on successful delete
  (`server.go:66,70`). deps: EPIC-004-S1, EPIC-004-S3.
- **EPIC-004-S5** — 404 when item absent
  (`server.go:66-68`). deps: EPIC-004-S2, EPIC-004-S3.

## Deterministic build order (dependencies first)

Topologically sorted; ties broken by epic order then slice id for determinism.

1. `KERNEL-item-model`
2. `SHARED-response-helpers`
3. `EPIC-001-S3`
4. `EPIC-001-S4`
5. `EPIC-001-S5`
6. `EPIC-002-S1`
7. `EPIC-002-S2`
8. `EPIC-002-S3`
9. `EPIC-002-S4`
10. `EPIC-002-S6`
11. `EPIC-003-S1`
12. `EPIC-003-S3`
13. `EPIC-004-S1`
14. `EPIC-004-S2`
15. `EPIC-004-S3`
16. `EPIC-004-S4`
17. `EPIC-004-S5`
