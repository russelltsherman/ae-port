# Slice 0001 — Name validation (`ValidateName`)

**Capability:** Validate a proposed item name — trim surrounding whitespace, reject empty/all-whitespace as required, and reject names longer than 100 runes as too long.

- **Slice ID:** `EPIC-001-S3`
- **Epic:** `EPIC-001`
- **Build order:** #1 (the first slice to build in this epic)
- **Depends on:** `["KERNEL-item-model"]` — see the kernel doc for the package name, the `Item` model, and `MaxNameLen`.

---

## 1. End-to-end behavior thread

`ValidateName` is a single pure function. There is exactly one call path; it has no dependencies on any other slice. Each component below cites its source location.

1. **`ValidateName` function entry** — `item.go:28`
   Signature: `func ValidateName(name string) error`. Takes one candidate name `string`, returns a single `error` (`nil` on success).

2. **Trim surrounding whitespace before validation** — `item.go:29`
   Computes `trimmed := strings.TrimSpace(name)`. ALL subsequent checks operate on `trimmed`, never on the raw `name`. (`strings.TrimSpace` removes leading and trailing Unicode whitespace.)

3. **Empty check → `ErrNameRequired`** — `item.go:30-32`
   `if trimmed == "" { return ErrNameRequired }`. Because the check is on the trimmed value, a name that is empty OR consists only of whitespace both hit this branch.

4. **Length check → `ErrNameTooLong`** — `item.go:33-35`
   `if len([]rune(trimmed)) > MaxNameLen { return ErrNameTooLong }`. The length is the RUNE count of the trimmed string, compared strictly-greater-than against `MaxNameLen` (= 100, `item.go:10`).

5. **Success** — `item.go:36`
   `return nil`.

6. **Error sentinels** — `item.go:21-24`
   `var ( ErrNameRequired = errors.New("name is required"); ErrNameTooLong = errors.New("name must be at most 100 characters") )`. Package-level vars, created with `errors.New`, so each is a stable identifiable value matchable with `errors.Is`.

---

## 2. Interface / contract (exact behavior)

### Signature
```go
func ValidateName(name string) error
```

### Inputs
- `name string` — the raw, untrimmed candidate name. May contain leading/trailing whitespace, may be empty, may contain multi-byte UTF-8 characters.

### Outputs
Exactly one of:
- `nil` — the name is valid.
- `ErrNameRequired` — the trimmed name is empty.
- `ErrNameTooLong` — the trimmed name exceeds the rune limit.

The returned error values are the package-level sentinels themselves (returned directly, not wrapped), so callers can compare with both `==` and `errors.Is`.

### Exact algorithm and ordering (must be implemented in this order)
1. `trimmed := strings.TrimSpace(name)` — `item.go:29`.
2. If `trimmed == ""` → return `ErrNameRequired` — `item.go:30-31`.
3. Else if `len([]rune(trimmed)) > MaxNameLen` → return `ErrNameTooLong` — `item.go:33-34`.
4. Else → return `nil` — `item.go:36`.

### Edge cases and guarantees (all must hold)
- **Ordering / precedence:** the empty/whitespace check runs BEFORE the length check (`item.go:30` precedes `item.go:33`). The two error conditions are mutually exclusive in practice (an empty string cannot be too long), but the empty check is unconditionally first.
- **Whitespace trimming:** validation is on the trimmed string. Input `"  hi  "` is valid because it trims to `"hi"` (`store_test.go:16`).
- **Length counts runes, not bytes:** `len([]rune(trimmed))` (`item.go:33`) — a multi-byte UTF-8 character counts as 1, not as its byte length. Do NOT use `len(trimmed)` (that is a byte count).
- **Length is measured on the trimmed string:** leading/trailing whitespace does not count toward the 100 limit (`item.go:33` uses `trimmed`).
- **Boundary is strictly greater-than:** exactly `MaxNameLen` (100) runes is VALID; only `> MaxNameLen` (101+) triggers `ErrNameTooLong` (`item.go:33`, `store_test.go:19-20`).
- **All-whitespace == empty:** `"   "` trims to `""` and returns `ErrNameRequired` (`item.go:30`, `store_test.go:18`).
- **No mutation / no side effects:** the function only reads `name` and returns an error. It does not modify global state.

### Error messages (exact strings — `item.go:22-23`)
- `ErrNameRequired` message: `name is required`
- `ErrNameTooLong` message: `name must be at most 100 characters`

---

## 3. Prerequisite slices (build order)

- This is slice **#1** in the build order for `EPIC-001`.
- `dependsOn: ["KERNEL-item-model"]`.
- From the KERNEL it relies on (do NOT restate the kernel here — reference it):
  - The package name (the file declares `package seeds`, `item.go:1`).
  - `const MaxNameLen = 100` (`item.go:10`) — the rune limit.
- It depends on NO other slice's internals. `ValidateName` does not import or call any store/handler code.

---

## 4. Acceptance tests for THIS slice

These mirror the behavioral spec and the existing table-driven test (`store_test.go:9-29`). All cases match the returned error with `errors.Is` (`store_test.go:24`).

| Case | Input | Expected result | Grounded |
|------|-------|-----------------|----------|
| Normal name OK | `"buy milk"` | `nil` (no error) | `store_test.go:15` |
| Trims to OK | `"  hi  "` | `nil` (no error) | `store_test.go:16` |
| Empty string | `""` | `ErrNameRequired` | `store_test.go:17`, `item.go:30-31` |
| Whitespace only | `"   "` | `ErrNameRequired` | `store_test.go:18`, `item.go:30` |
| Too long | 101 `x` chars (`strings.Repeat("x", MaxNameLen+1)`) | `ErrNameTooLong` | `store_test.go:19`, `item.go:33-34` |
| Max length OK | 100 `x` chars (`strings.Repeat("x", MaxNameLen)`) | `nil` (no error) | `store_test.go:20`, `item.go:33` |

Additional cases implied by the spec (runnable-in-spirit, NOT in the current source — add them to harden the slice):
- A trimmed name of exactly 100 multi-byte runes (e.g. 100 `"é"` characters, which is 200 bytes) must return `nil` — proves rune counting, not byte counting (`item.go:33`).
- A trimmed name of 101 multi-byte runes must return `ErrNameTooLong`.
- Errors must be matchable via `errors.Is` against the named sentinels (`store_test.go:24`).

Reference test skeleton (Go, table-driven, matching the source style):
```go
func TestValidateName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"ok", "buy milk", nil},
		{"trims to ok", "  hi  ", nil},
		{"empty", "", ErrNameRequired},
		{"whitespace only", "   ", ErrNameRequired},
		{"too long", strings.Repeat("x", MaxNameLen+1), ErrNameTooLong},
		{"max length ok", strings.Repeat("x", MaxNameLen), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidateName(c.in); !errors.Is(got, c.want) {
				t.Fatalf("ValidateName(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
```

---

## 5. Build steps (function/unit-sized, each individually checkable)

1. **Declare the rune limit constant** (if not already provided by the kernel): `const MaxNameLen = 100` (`item.go:10`).
   - Check: `MaxNameLen` is an untyped integer constant equal to 100.

2. **Declare the two error sentinels** (`item.go:21-24`):
   ```go
   var (
       ErrNameRequired = errors.New("name is required")
       ErrNameTooLong  = errors.New("name must be at most 100 characters")
   )
   ```
   - Check: both are package-level `error` values; messages match the exact strings above.

3. **Implement `ValidateName`** (`item.go:28-37`):
   ```go
   func ValidateName(name string) error {
       trimmed := strings.TrimSpace(name)
       if trimmed == "" {
           return ErrNameRequired
       }
       if len([]rune(trimmed)) > MaxNameLen {
           return ErrNameTooLong
       }
       return nil
   }
   ```
   - Check (trim): `ValidateName("  hi  ")` returns `nil`.
   - Check (empty): `ValidateName("")` returns `ErrNameRequired`.
   - Check (whitespace): `ValidateName("   ")` returns `ErrNameRequired`.
   - Check (boundary OK): `ValidateName(strings.Repeat("x", 100))` returns `nil`.
   - Check (boundary fail): `ValidateName(strings.Repeat("x", 101))` returns `ErrNameTooLong`.
   - Check (rune vs byte): `ValidateName(strings.Repeat("é", 100))` returns `nil`.

4. **Add the required imports**: `errors` and `strings` (`item.go:3-7`).
   - Check: package compiles with `go build ./...`.

5. **Write the acceptance tests** from Section 4.
   - Check: `go test ./...` passes (the `TestValidateName` table above).

---

## 6. Kernel references (reference only — do NOT restate)

This slice relies on the following from `KERNEL-item-model`; consult the kernel doc for their definitions:
- **Package** — the package these symbols live in (source declares `package seeds`, `item.go:1`).
- **`MaxNameLen`** — `const MaxNameLen = 100` (`item.go:10`), the rune limit used by the length check.

Standard-library conventions this slice relies on (Go stdlib, not the kernel):
- `strings.TrimSpace` — removes leading/trailing Unicode whitespace (`item.go:5`, `item.go:29`).
- `errors.New` — constructs sentinel error values (`item.go:4`, `item.go:22-23`).
- `errors.Is` — used by tests to match returned errors against sentinels (`store_test.go:4`, `store_test.go:24`).
- `[]rune(s)` conversion + `len(...)` — counts Unicode code points (`item.go:33`).

This slice does NOT depend on any other slice's internals (store, handlers, persistence). `ValidateName` is a leaf function.
