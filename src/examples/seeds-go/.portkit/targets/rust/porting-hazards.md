# Porting Hazards: Go → Rust (seeds-go)

This document lists every place where a Go-language assumption in `seeds-go` will break, behave
differently, or silently diverge when reimplemented in Rust, and the EXACT fix. The downstream
consumer is a weak model rebuilding from these notes WITHOUT reading the Go source. Follow each
fix precisely. Every claim is grounded to `path:line` in the source.

---

## H1. Garbage collection vs ownership: `Item` is a VALUE type, copied freely

**Go assumption.** `Item` is a struct (`item.go:13-18`) with value semantics. The store holds
`map[int]Item` (`store.go:21`) — VALUES, not pointers. Every read copies the struct out:
- `Get` returns `it, ok := s.items[id]` then returns `it` by value (`store.go:56-60`).
- `List` appends copies into a new slice (`store.go:67-72`).
- `Create` builds an `Item` value, stores a copy, returns a copy (`store.go:46-49`).
Go's GC makes all this aliasing-free and effortless.

**Why it breaks in Rust.** Rust has no GC and enforces ownership/borrowing. You cannot return a
reference into a `Mutex`-guarded `HashMap` after the guard drops, and you cannot move a value out
of a map you only have `&` access to.

**Fix (exact).**
- Make `Item` derive `Clone` (and `Serialize`, plus `Deserialize` only if you deserialize into
  it — the tests deserialize the response into `Item`, see server_test.go:36).
- In `get`/`list`/`create`, `.clone()` the `Item` out of the `HashMap` WHILE holding the mutex
  guard, then return the owned clone. This reproduces Go's copy-out value semantics exactly.
- Store as `HashMap<i64, Item>` (key type per dependency-map id-width decision).

---

## H2. `sync.Mutex` + `defer Unlock` vs RAII guard

**Go assumption.** Every store method locks at the top and unlocks via `defer`
(`store.go:39-40`, `54-55`, `65-66`, `77-78`). The mutex is a struct field (`store.go:20`) and is
zero-value-usable (no init needed). Go mutexes are NOT re-entrant and protect `items`+`nextID`.

**Why it breaks / differs in Rust.**
- Rust `std::sync::Mutex<T>` wraps the DATA, not sits beside it. You cannot have a bare `Mutex`
  field next to an unprotected `HashMap` field and expect the compiler to enforce the pairing —
  you must put the protected state INSIDE the mutex.
- The lock guard releases at end of scope (RAII), which is the analog of `defer Unlock`, but only
  if you do not accidentally hold it longer than the Go critical section.
- Rust `Mutex` lock returns a `Result` (poisoned-lock case) — Go has no equivalent.

**Fix (exact).**
- Define inner state and wrap it:
  ```rust
  struct StoreInner { items: HashMap<i64, Item>, next_id: i64 }
  struct Store { inner: Mutex<StoreInner>, now: Box<dyn Fn() -> DateTime<Utc> + Send + Sync> }
  ```
  Note `next_id` starts at **1** (`store.go:28`), not 0.
- Each method: `let mut g = self.inner.lock().unwrap();` then operate on `g.items`/`g.next_id`.
  Using `.unwrap()` on the lock matches Go's behavior of not handling a "poisoned" state (Go has
  no poisoning; a panic mid-critical-section is its own problem). Acceptable for this port.
- Do NOT use `tokio::sync::Mutex`. The critical sections (`store.go:41-49`, `56-60`, `67-72`,
  `79-83`) contain NO `.await`. A std `Mutex` is correct and faster. Holding a `tokio::Mutex`
  guard is only needed across await points, which never happens here.
- Share the store as `Arc<Store>` injected into axum handlers via router state (Go shares one
  `*Store` pointer across all requests — `main.go:12-13`, `server.go:18` takes `*Store`).

---

## H3. Concurrency model: goroutines-per-request vs async tasks

**Go assumption.** `net/http` serves each request on its own goroutine; the single shared
`*Store` (one instance, `main.go:12`) is made safe purely by its internal `sync.Mutex`. There is
no other shared mutable state.

**Why it differs in Rust.** axum/tokio run handlers as async tasks across a thread pool. Shared
state must be `Send + Sync` and wrapped in `Arc`. The clock closure stored in `Store.now`
(`store.go:23`) must therefore be `Send + Sync` (hence the `+ Send + Sync` bound in H2).

**Fix.** `Arc<Store>` as axum state; ensure `Item`, `StoreInner`, and the `now` closure are all
`Send + Sync`. No additional locking is needed beyond the one `Mutex` (mirrors Go).

---

## H4. Duck typing / `any` vs Rust generics & traits

**Go assumption.** `writeJSON(w, status, v any)` (`server.go:76`) takes `any` and JSON-encodes
whatever it is — an `Item`, a `[]Item`, or a `map[string]string` for errors
(`server.go:83`: `map[string]string{"error": msg}`). One function, runtime polymorphism.

**Why it breaks in Rust.** Rust has no `any`-style runtime JSON of arbitrary types without a
trait bound. You cannot pass heterogeneous types to one monomorphic function.

**Fix (exact).**
- Make the writer generic over `serde::Serialize`:
  `fn write_json<T: Serialize>(status: StatusCode, v: T) -> Response`.
- For the error body, Go uses `map[string]string{"error": msg}` (`server.go:83`) which serializes
  to `{"error":"<msg>"}`. Reproduce with either a `serde_json::json!({ "error": msg })` value or a
  small `#[derive(Serialize)] struct ErrorBody { error: String }`. The output JSON MUST be
  exactly `{"error":"..."}` — this is asserted indirectly by tests checking status codes and by
  the contract in `server.go:11-17`.

---

## H5. JSON field naming: Go struct tags vs serde must match BYTE-for-BYTE

**Go assumption.** `Item` fields use struct tags (`item.go:14-17`):
- `ID int` → `"id"`
- `Name string` → `"name"`
- `Done bool` → `"done"`
- `CreatedAt time.Time` → `"createdAt"` (camelCase, NOT snake_case)

The request body decode target uses tag `"name"` (`server.go:22-24`).

**Why it breaks in Rust.** Rust field naming convention is snake_case (`created_at`), and serde by
default serializes the Rust field name verbatim → would emit `created_at`, breaking the contract.

**Fix (exact).** Add explicit serde renames:
```rust
#[derive(Clone, Serialize, Deserialize)]
struct Item {
    id: i64,
    name: String,
    done: bool,
    #[serde(rename = "createdAt")]
    created_at: DateTime<Utc>,
}
```
`id`, `name`, `done` already match lowercase; only `created_at` needs the rename. For the request
body, deserialize into `struct CreateBody { name: String }` (field `name` matches tag).
DO NOT apply `#[serde(rename_all = "camelCase")]` blindly — it would turn nothing here (single-
word fields) but is a foot-gun if fields change; explicit per-field rename is safer and exact.

---

## H6. `time.Now()` is LOCAL time; chrono `Utc::now()` is UTC

**Go assumption.** Default clock is `time.Now` (`store.go:28`), which returns LOCAL wall-clock
time with the machine's timezone offset. `CreatedAt` is set from `s.now()` (`store.go:46`) and
marshaled via the `createdAt` tag (`item.go:17`) using Go's default `time.Time` JSON format:
RFC 3339 with nanosecond precision AND the local numeric offset (e.g. `-07:00`), not necessarily
`Z`.

**Why it diverges in Rust.** `chrono::Utc::now()` is UTC and serializes with `Z`. Output strings
will differ from Go's local-offset output.

**Mitigating fact (grounded).** NO test asserts on the timestamp. Searched the two test files:
neither `store_test.go` nor `server_test.go` references `CreatedAt`, `createdAt`, or any time
value (confirmed against `store_test.go:1-123` and `server_test.go:1-124`). So a format change is
test-invisible.

**Fix.** Default to `DateTime<Utc>` + `Utc::now`. This is a deliberate, documented divergence
from Go's local-time output. If exact parity with Go's local-offset RFC 3339 is required, that is
HUMAN-DECISION-REQUIRED (see dependency-map.md). Do NOT silently assume the formats are identical.

---

## H7. Injectable clock (`now func() time.Time`) — preserve testability

**Go assumption.** `Store.now` is a field of function type (`store.go:23`) so tests can swap in a
fixed clock. (The current tests use the default `time.Now`, but the seam exists by design — see
the doc comment `store.go:26-27` and the `now` field.)

**Why it matters in Rust.** If you hardcode `Utc::now()` inside `create`, you lose the seam.

**Fix.** Keep the seam: store `now: Box<dyn Fn() -> DateTime<Utc> + Send + Sync>`; `new()`
defaults it to `Box::new(Utc::now)`; provide a test constructor that injects a fixed-time closure.
Call it at item creation (`create`) exactly where Go does (`store.go:46`).

---

## H8. Numeric type: Go `int` is 64-bit here; `strconv.Atoi` & `nextID`

**Go assumption.** `Item.ID` is `int` (`item.go:14`); `Store.nextID` is `int` (`store.go:22`),
starts at 1 (`store.go:28`), increments by 1 per create (`store.go:48`). Path id is parsed with
`strconv.Atoi` → `int` (`server.go:47,61`). On the Go targets in scope, `int` is 64-bit.

**Why it can diverge in Rust.** Choosing `i32` narrows the range vs Go `int`; choosing `usize`
ties to pointer width. Mismatch could change overflow behavior (though unreachable here).

**Fix.** Use `i64` for `id`, `next_id`, and the parse target to mirror Go `int`. Parse with
`s.parse::<i64>()`. See H9 for parse-error edge cases.

---

## H9. `strconv.Atoi` vs Rust `parse::<i64>()` — parse edge cases & exact error

**Go assumption.** `strconv.Atoi(r.PathValue("id"))` (`server.go:47,61`). On ANY parse error →
HTTP 400 with body EXACTLY `{"error":"id must be an integer"}` (`server.go:49,63`). Tests assert
400 for `"abc"` (`server_test.go:87`) and `"xyz"` (`server_test.go:120`), and 200/404 for valid
ints `"1"`/`"999"` (`server_test.go:79,84,114,117`).

**Differences to know.**
- `strconv.Atoi` accepts a single optional leading `+` or `-`, no underscores, no whitespace,
  no `0x`/`0b` prefixes (it is base-10). Rust `i64::from_str` accepts optional leading `+`/`-`,
  NO underscores, no radix prefixes, no whitespace — effectively the same grammar for base-10.
- Both REJECT empty string, `"abc"`, `"1.0"`, `" 1"`, `"1 "`. They AGREE on all test inputs.
- Edge nuance: Go `Atoi("+")` errors; Rust `"+".parse::<i64>()` errors too. Aligned.

**Fix (exact).** Do the parse YOURSELF so you control the error body (do NOT lean on axum's
`Path<i64>` auto-extraction, which produces a different 400 body — see H11):
```rust
let id: i64 = match raw_id.parse() {
    Ok(v) => v,
    Err(_) => return write_json(StatusCode::BAD_REQUEST,
                                json!({ "error": "id must be an integer" })),
};
```
The message string MUST be exactly `id must be an integer`.

---

## H10. Evaluation order & duplicate detection: validate BEFORE trim BEFORE dup-check

**Go assumption (ordering is observable).** `Store.Create` (`store.go:33-49`):
1. `ValidateName(name)` first (`store.go:34`) — on error returns immediately, BEFORE locking.
   `ValidateName` (`item.go:28-37`): trims, then `trimmed == ""` → `ErrNameRequired`
   (`item.go:30-32`); then `len([]rune(trimmed)) > 100` → `ErrNameTooLong`
   (`item.go:33-35`). Length is measured in RUNES (Unicode code points), not bytes — see H12.
2. Compute `trimmed = strings.TrimSpace(name)` AGAIN (`store.go:37`).
3. Lock, then linear scan for a case-insensitive match via `strings.EqualFold(it.Name, trimmed)`
   (`store.go:41-45`) → `ErrDuplicate` if found.
4. Insert with `Name: trimmed` (the TRIMMED form is stored, `store.go:46`).

So the stored name is trimmed (proven by `store_test.go:41-43`: input `"  Write tests  "` stored
as `"Write tests"`), and duplicate check is BOTH case-insensitive AND trim-insensitive (proven by
`store_test.go:69`: `"Unique"` then `"  unique "` → `ErrDuplicate`).

**Why it matters in Rust.** If you reorder (e.g. dup-check before validate, or store the untrimmed
name), you change observable behavior and break tests. Rust has no special evaluation-order
hazard here, but the LOGICAL order must be preserved exactly.

**Fix (exact, in this order):**
1. `validate_name(name)` → returns `Err(NameRequired)` / `Err(NameTooLong)` first.
2. `let trimmed = name.trim();`
3. lock; scan existing items; if any `existing.name.eq...(trimmed)` (case-insensitive, see H13)
   → `Err(Duplicate)`.
4. insert `Item { id: next_id, name: trimmed.to_string(), done: false, created_at: (now)() }`;
   then `next_id += 1`. NOTE `done` defaults to `false` — Go leaves it as the zero value
   (`store.go:46` never sets `Done`, so it is `false`).

---

## H11. axum `Json<T>` / `Path<T>` extractor rejections produce WRONG error bodies

**Go assumption.** Malformed JSON request body → 400 `{"error":"invalid JSON body"}`
(`server.go:25-27`), proven by `server_test.go:50` (`{not json` → 400). The handler decodes
manually and writes its own error.

**Why it breaks in Rust.** If you use axum's `Json<CreateBody>` extractor as a handler argument,
a malformed body is rejected by axum BEFORE your handler runs, returning axum's DEFAULT plain-text
400 (e.g. `Failed to parse the request body as JSON: ...`) — NOT `{"error":"invalid JSON body"}`.
Same problem with `Path<i64>` (see H9): wrong 400 body.

**Fix (exact).** Take the RAW body and parse inside the handler so you control every error:
- For POST /items: accept `body: Bytes` (or `String`) and call
  `serde_json::from_slice::<CreateBody>(&body)`; on `Err` →
  `write_json(BAD_REQUEST, json!({"error":"invalid JSON body"}))`.
- For path id: accept `Path<String>` and parse per H9.
This reproduces Go's exact 400 bodies. Alternatively, implement a custom rejection handler, but
manual parsing is simpler and exact for a weak model to get right.

---

## H12. String length is RUNES in Go, not bytes — `MaxNameLen` check

**Go assumption.** `ValidateName` measures `len([]rune(trimmed))` (`item.go:33`) — the count of
Unicode code points, compared against `MaxNameLen = 100` (`item.go:10`). A 100-emoji name passes;
a 101-rune name fails with `ErrNameTooLong` (`item.go:34`). Tests use ASCII `"x"*101` (>100, fail)
and `"x"*100` (==100, ok) (`store_test.go:19-20`).

**Why it breaks in Rust.** Rust `str::len()` returns BYTE length, not char count. A 100-char
multi-byte name would wrongly exceed 100 if you use `.len()`.

**Fix (exact).** Use `trimmed.chars().count() > 100` → `Err(NameTooLong)`.
- IMPORTANT subtlety: Go's `[]rune` counts Unicode code points (`rune` = code point). Rust
  `chars()` ALSO iterates Unicode scalar values (code points, excluding surrogates which can't
  appear in valid UTF-8). For all real inputs these counts MATCH. Do NOT use `.len()`,
  `.bytes().count()`, or grapheme clusters. Use `.chars().count()`.

---

## H13. `strings.EqualFold` vs Rust case comparison — Unicode fold divergence

**Go assumption.** Duplicate detection uses `strings.EqualFold(it.Name, trimmed)`
(`store.go:42`) — Unicode SIMPLE case-folding equality. Test proves case-insensitivity for ASCII:
`"Unique"` vs `"unique"` (`store_test.go:69`), and HTTP `"dup"` vs `"DUP"` (`server_test.go:67-70`
→ 409).

**Why it diverges in Rust.** There is no exact `EqualFold` equivalent in std:
- `a.eq_ignore_ascii_case(b)` — folds ONLY ASCII A–Z. Matches Go for ASCII; differs for non-ASCII
  (e.g. `'K'` Kelvin sign, German `ß`, Turkish dotless i).
- `a.to_lowercase() == b.to_lowercase()` — full Unicode lowercase mapping, NOT identical to Go's
  simple fold (e.g. `ß` → `ss` in Rust full lowering; Go EqualFold treats `ß`/`ẞ` via simple fold).

**Fix.**
- If ASCII-only behavior is acceptable (all tests are ASCII): use
  `existing.name.eq_ignore_ascii_case(trimmed)`. This passes every existing test
  (`store_test.go:69`, `server_test.go:67-70`).
- If full Unicode parity with Go matters: HUMAN-DECISION-REQUIRED (see dependency-map.md). Do NOT
  silently pick `to_lowercase` and claim parity — it is NOT byte-identical to `EqualFold`.
- Recommended default for this port: `eq_ignore_ascii_case` (documented as ASCII-scoped).

---

## H14. `strings.TrimSpace` vs `str::trim` — whitespace set

**Go assumption.** `strings.TrimSpace` (`item.go:29`, `store.go:37`) trims leading/trailing runes
where `unicode.IsSpace` is true. Tests: `"  hi  "` → ok/trimmed (`store_test.go:18` via
ValidateName, `store_test.go:34` stored as `"Write tests"`), `"   "` (all spaces) → empty →
`ErrNameRequired` (`store_test.go:18`? that's whitespace-only at `store_test.go:18`:
`{"whitespace only", "   ", ErrNameRequired}`).

**Why it can diverge.** Go's `unicode.IsSpace` and Rust's `char::is_whitespace` use slightly
different definitions of whitespace at the margins (e.g. some control chars, the `U+0085 NEL`,
zero-width chars). They AGREE on ASCII space/tab/newline/CR — the only whitespace in the tests.

**Fix.** Use `str::trim()`. For ASCII inputs this is exact. If the source must trim an unusual
Unicode space identically to Go, that is an edge case not covered by tests — note it but do not
over-engineer. `str::trim()` is the correct default.

---

## H15. `done` zero value & `Item{}` zero value returns

**Go assumption.** On error paths, store methods return `Item{}` (the zero value:
`ID:0, Name:"", Done:false, CreatedAt:` zero time) — e.g. `store.go:34,43,53? actually 58,76? no`
(`store.go:35` returns `Item{}, err`; `store.go:43` `Item{}, ErrDuplicate`; `store.go:58`
`Item{}, ErrNotFound`). The caller never uses the zero Item on the error path (it checks the
error first — `server.go:30-39,52-57`), so the zero value is never serialized.

**Why it matters in Rust.** Rust has no implicit zero value. Returning `Result<Item, AppError>`
is the correct model — on `Err`, there is NO Item at all, which is STRICTER and SAFER than Go and
produces identical observable behavior (the error branch is taken, the Item is never read).

**Fix.** All store methods return `Result<Item, AppError>` (or `Result<(), AppError>` for
`delete`, `Result<Vec<Item>, _>`/just `Vec<Item>` for `list` which never errors — `store.go:64-73`
has no error path, so `list` returns `Vec<Item>` with no Result). `done` is set to `false` on
create (mirrors the never-set zero value, see H10 step 4).

---

## H16. Routing: method+path matching & trailing-segment behavior

**Go assumption.** `http.ServeMux` with method-prefixed patterns (Go 1.22+): `POST /items`,
`GET /items`, `GET /items/{id}`, `DELETE /items/{id}` (`server.go:21,42,46,60`). `{id}` is a
single path segment. An unmatched method on a matched path yields Go's default 405; an unmatched
path yields 404. (No test exercises 405/unknown-path; behavior is Go-mux default.)

**Why it differs in Rust.** axum routing must replicate method+path. axum returns 405 for a known
path with an unsupported method automatically when methods are registered on the same route, and
404 for unknown paths — close to Go's behavior, but EXACT 405/404 bodies differ (both default to
empty/standard bodies; no test asserts them).

**Fix.**
```rust
Router::new()
    .route("/items", post(create_item).get(list_items))
    .route("/items/{id}", get(get_item).delete(delete_item))
    .with_state(store)
```
Confirm `{id}` path syntax against your pinned axum version (`{id}` in axum 0.7+, `:id` earlier) —
version-dependent, verify at build (`[UNVERIFIED]`). No test asserts 405/unknown-path bodies, so
axum defaults are acceptable.

---

## H17. Response Content-Type & header/status ORDERING

**Go assumption.** `writeJSON` sets `Content-Type: application/json` BEFORE `WriteHeader(status)`
BEFORE encoding the body (`server.go:77-79`). In Go you MUST set headers before `WriteHeader`, and
`WriteHeader` before writing the body — ordering is enforced by the API contract.
The 204 No Content path (`server.go:70`: `w.WriteHeader(http.StatusNoContent)`) writes NO body and
does NOT go through `writeJSON`, so it has NO `Content-Type` header.

**Why it matters in Rust.** axum responses are built declaratively, so explicit ordering is less
error-prone, BUT you must replicate: JSON responses carry `Content-Type: application/json`; the
204 DELETE-success response carries NO body (`server_test.go:114` asserts 204).

**Fix.**
- JSON helper returns a response with status + `application/json` body (axum `Json<T>` sets the
  content-type automatically when used as a responder; if you build manually, set the header).
- DELETE success → return `StatusCode::NO_CONTENT` with an EMPTY body (do NOT attach a JSON body).
  In axum, returning `StatusCode::NO_CONTENT` alone yields an empty-body 204.

---

## H18. `log.Fatal` semantics (exit code) — main entrypoint

**Go assumption.** `main` does `log.Fatal(http.ListenAndServe(":8080", srv))` (`main.go:15`):
`ListenAndServe` only returns on error; `log.Fatal` prints it to stderr and calls `os.Exit(1)`.
Startup log line `"seeds-go listening on :8080"` goes to stderr first (`main.go:14`).

**Why it differs in Rust.** Rust `main` returns `()` or `Result`; a panic gives exit code 101,
not 1. To match Go's exit code 1 on serve failure, handle the error explicitly.

**Fix.**
```rust
#[tokio::main]
async fn main() {
    let store = Arc::new(Store::new());
    let app = build_router(store);
    eprintln!("seeds-go listening on :8080");
    let listener = match tokio::net::TcpListener::bind("0.0.0.0:8080").await {
        Ok(l) => l,
        Err(e) => { eprintln!("{e}"); std::process::exit(1); }
    };
    if let Err(e) = axum::serve(listener, app).await {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
```
Bind `0.0.0.0:8080` (Go `:8080` = all interfaces). Port stays hardcoded. Use `std::process::exit(1)`
to mirror `log.Fatal`'s exit code (NOT a panic, which would be 101).

---

## H19. Panics vs Go's error-return discipline

**Go assumption.** This code NEVER panics on the request path. All failures are values: validation
errors, not-found, duplicates, parse errors — all returned as `error` and mapped to status codes
(`server.go:30-39,53-57,66-69`). The only `os.Exit`-style termination is startup (`main.go:15`).

**Why it matters in Rust.** Idiomatic Rust must NOT `.unwrap()`/`.expect()`/panic on request-path
fallible operations (JSON parse, id parse) — a panic in an axum handler aborts that request (and
can poison a `std::sync::Mutex`). Go's discipline of returning errors must be preserved with
`Result` + explicit status mapping.

**Fix.**
- Request-path: NO `.unwrap()` on `parse`/`from_slice`/etc. Use `match`/`?` into the error→status
  mapping (H4, H9, H11).
- Acceptable `.unwrap()`: only on `Mutex::lock()` (poisoning is unreachable in correct code and
  matches Go's no-poisoning model — H2) and in test code.

---

## H20. Error→status mapping must be exhaustive and exact

**Go assumption (the full mapping, grounded).**
- POST /items (`server.go:21-40`):
  - body decode error → 400 `{"error":"invalid JSON body"}` (`server.go:25-27`).
  - `ErrNameRequired` or `ErrNameTooLong` → 400 with `err.Error()` as message
    (`server.go:31-32`).
  - `ErrDuplicate` → 409 with `err.Error()` (`server.go:33-34`).
  - any other non-nil error → 500 `{"error":"internal error"}` (`server.go:35-36`).
  - success → 201 + the created item as JSON (`server.go:37-38`).
- GET /items → 200 + JSON array of items (`server.go:42-44`).
- GET /items/{id} (`server.go:46-58`):
  - non-int id → 400 `{"error":"id must be an integer"}` (`server.go:48-50`).
  - `ErrNotFound` → 404 with `err.Error()` (`server.go:53-55`).
  - success → 200 + item (`server.go:57`).
- DELETE /items/{id} (`server.go:60-71`):
  - non-int id → 400 `{"error":"id must be an integer"}` (`server.go:62-64`).
  - `ErrNotFound` → 404 with `ErrNotFound.Error()` (`server.go:66-67`).
  - success → 204, no body (`server.go:70`).

**Fix.** Implement this mapping branch-for-branch. The message strings come from the error enum
`Display` (H, and dependency-map.md item 4 sentinel strings). The 500/`"internal error"` branch
(`server.go:35-36`) is unreachable in current code (Create only returns the three known errors)
but MUST be kept for parity — map any unexpected `AppError`/other error to 500
`{"error":"internal error"}`.

---

## H21. `List` ordering determinism — Go map iteration is RANDOM, Rust HashMap too

**Go assumption.** `List` iterates the map (random order in Go) then SORTS by ID ascending
(`store.go:67-72`, sort at `store.go:71`). The sort is what makes output deterministic;
map-iteration order is explicitly NOT relied upon. Proven by `store_test.go:91-107` asserting
ascending-by-ID order.

**Why it matters in Rust.** Rust `HashMap` iteration is ALSO unordered (randomized per-process).
If you forget the sort, output order is nondeterministic and `store_test.go`'s ordering assertion
fails.

**Fix.** After collecting items from the `HashMap`, `out.sort_by_key(|it| it.id);` BEFORE
returning (mirrors `store.go:71`). IDs are unique so stable vs unstable sort is irrelevant.
Capacity hint `Vec::with_capacity(map.len())` mirrors `make([]Item, 0, len(s.items))`
(`store.go:67`) — optional, perf only.

---

## H22. Empty list serialization: `[]` not `null`

**Go assumption.** `List` returns `make([]Item, 0, ...)` (`store.go:67`) — a non-nil empty slice,
which JSON-marshals to `[]`. (A nil slice would also marshal to `[]` in Go.) So GET /items on an
empty store returns `[]`.

**Why it matters in Rust.** A Rust `Vec<Item>` that is empty serializes via serde_json to `[]`
(NOT `null`) — this matches Go. An `Option<Vec<_>>` of `None` would serialize to `null` — do NOT
use that. (No test asserts the empty case, but the contract is `[]`.)

**Fix.** `list` returns `Vec<Item>`; empty → `[]`. Correct by default with serde_json.

---

## Quick checklist for the rebuilder

1. `Item` derives `Clone, Serialize, Deserialize`; `createdAt` renamed (H5).
2. Length check uses `.chars().count() > 100` (H12).
3. Validate → trim → lock → case-insensitive dup scan → insert, IN THAT ORDER (H10).
4. Store stores the TRIMMED name; `done = false` on create (H10).
5. State inside `Mutex`, shared as `Arc`, `std::sync::Mutex` not tokio (H2, H3).
6. Manual JSON-body parse → 400 `{"error":"invalid JSON body"}`; manual id parse → 400
   `{"error":"id must be an integer"}` (H9, H11).
7. Full error→status mapping incl. unreachable-but-required 500 (H20).
8. `List` sorts by id ascending (H21); empty → `[]` (H22).
9. DELETE success → 204, empty body (H17).
10. `main`: bind `0.0.0.0:8080`, `eprintln!` startup, `std::process::exit(1)` on serve error (H18).
11. No `.unwrap()` on request-path fallibles (H19).
12. Timestamp = `DateTime<Utc>` (documented divergence from Go local time) unless human decides
    otherwise (H6).
