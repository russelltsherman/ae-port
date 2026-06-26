# Rust Dependency Map — seeds-go → Rust

Source: `example.com/seeds-go` (`src/examples/seeds-go/go.mod:1`), `go 1.26` (`go.mod:3`).

## CRITICAL FACT: there are ZERO third-party dependencies

`go.mod` declares the module and the Go version and **nothing else** — there is no
`require` block (`src/examples/seeds-go/go.mod:1-3`, the entire file is 3 lines). Every
import in the codebase is from the Go **standard library**. So this map is about mapping
**Go stdlib packages** to Rust crates / Rust std, not about external libraries.

Each entry below names every Go stdlib import found in the source, the exact file:line where
it is imported, what it is used for in this code, and the **concrete** Rust strategy.

---

## Recommended Rust crate baseline (pin these in `Cargo.toml`)

The downstream model MUST use this exact set. Do not substitute. These are the standard,
widely-used choices that cover everything this code does.

| Purpose | Rust crate | Why this one |
|---|---|---|
| Async runtime | `tokio` (features `["full"]`) | Required by `axum`; provides the async executor + TCP listener. |
| HTTP server + routing | `axum` | Path-param routing (`/items/{id}`), method routing (`POST`/`GET`/`DELETE`), JSON extractors, typed status codes — a 1:1 fit for `net/http.ServeMux` method+path patterns used here. |
| JSON encode/decode | `serde` (feature `["derive"]`) + `serde_json` | Replaces `encoding/json`. `#[derive(Serialize, Deserialize)]` replaces struct tags. |
| Time / timestamps | `chrono` (feature `["serde"]`) | Replaces `time.Time` with RFC 3339 serialization matching Go's default `time.Time` JSON format. See HUMAN-DECISION note below about exact format. |

Suggested versions at time of writing (the model SHOULD pick the latest compatible patch,
but these are known-good major lines): `tokio = "1"`, `axum = "0.7"`, `serde = "1"`,
`serde_json = "1"`, `chrono = "0.4"`. `[UNVERIFIED]` — these versions are not pinned by any
source file; they are the porter's recommendation, not a fact extracted from the repo.

---

## Per-import mapping

### 1. `encoding/json`
- **Imported at:** `src/examples/seeds-go/server.go:4`; `src/examples/seeds-go/server_test.go:4`.
- **Used for:**
  - Decoding the request body into an anonymous struct `{ Name string }`
    (`server.go:22-28`): `json.NewDecoder(r.Body).Decode(&body)`.
  - Encoding any value to the response writer (`server.go:76-80`):
    `json.NewEncoder(w).Encode(v)`.
  - In tests, `json.Unmarshal` of the response body into `Item` / `[]Item`
    (`server_test.go:36`, `server_test.go:102`).
- **Strategy:** TARGET-EQUIVALENT → `serde` + `serde_json`.
  - Replace the Go struct tags on `Item` (`item.go:13-18`) with serde rename attributes
    (see field-name table in porting-hazards.md). `axum`'s `Json<T>` extractor/responder
    wraps `serde_json` and handles both directions.
  - Decode-failure behavior: Go returns HTTP 400 `{"error":"invalid JSON body"}` when
    `Decode` errors (`server.go:25-27`); in Rust this is the `Json<T>` extractor rejection —
    you MUST convert that rejection into the same 400 + body shape (see hazards doc).

### 2. `net/http`
- **Imported at:** `src/examples/seeds-go/cmd/server/main.go:6`; `src/examples/seeds-go/server.go:6`; `src/examples/seeds-go/server_test.go:5`.
- **Used for:**
  - `http.NewServeMux()` and method+path route registration
    (`server.go:19`, `server.go:21,42,46,60`). Routes (from `server.go:11-17` doc comment,
    confirmed by the handler registrations):
    - `POST /items` → 201 | 400 | 409
    - `GET /items` → 200
    - `GET /items/{id}` → 200 | 400 | 404
    - `DELETE /items/{id}` → 204 | 400 | 404
  - `r.PathValue("id")` to read the `{id}` path segment (`server.go:47`, `server.go:61`).
  - `http.StatusXxx` constants (e.g. `server.go:26,32,34,36,38,49,54,57,63,67,70`).
  - `w.Header().Set("Content-Type", "application/json")` and `w.WriteHeader(status)`
    (`server.go:77-78`).
  - `http.ListenAndServe(":8080", srv)` (`main.go:15`).
  - `http.Handler` as the server type returned by `NewServer` (`server.go:18`).
- **Strategy:** TARGET-EQUIVALENT → `axum` (on `tokio`).
  - `http.NewServeMux` → `axum::Router`. Register routes with
    `.route("/items", post(create).get(list))` and
    `.route("/items/{id}", get(get_one).delete(delete_one))`.
    NOTE: axum 0.7 path syntax uses `{id}` (matching Go's `{id}`). Verify against the axum
    version you pin; older axum used `:id`. `[UNVERIFIED]` — exact axum path syntax depends
    on the pinned version; confirm at build time.
  - `r.PathValue("id")` (a **string**) → axum `Path<String>` then parse, OR `Path<i32>`.
    IMPORTANT ORDERING HAZARD: Go reads the raw string and calls `strconv.Atoi` itself
    (`server.go:47-50`), returning **400 `{"error":"id must be an integer"}`** on parse
    failure. If you use axum `Path<i32>`, axum produces its OWN 400 with a DIFFERENT body.
    To preserve the exact body, extract `Path<String>` and parse manually so you control the
    error response. See hazards doc.
  - `http.ListenAndServe(":8080", srv)` → bind a `tokio::net::TcpListener` to `0.0.0.0:8080`
    and `axum::serve(listener, app)`. NOTE: Go's `:8080` binds all interfaces; use
    `0.0.0.0:8080` (not `127.0.0.1`) to match. The port `8080` is hardcoded (`main.go:15`) —
    keep it hardcoded unless told otherwise.
  - `http.Handler` return type → return an `axum::Router` from the constructor function.

### 3. `log`
- **Imported at:** `src/examples/seeds-go/cmd/server/main.go:5`.
- **Used for:** `log.Println("seeds-go listening on :8080")` (`main.go:14`) and
  `log.Fatal(http.ListenAndServe(...))` (`main.go:15`). `log.Fatal` prints the error and
  calls `os.Exit(1)`.
- **Strategy:** REIMPLEMENT with Rust std — do NOT pull in a logging crate for this.
  - `log.Println(...)` → `eprintln!("seeds-go listening on :8080");` (Go's `log` writes to
    stderr by default and prepends a timestamp; the message content is what matters here).
    `[UNVERIFIED]` — whether the downstream cares about the leading timestamp Go's `log`
    adds. If exact stderr formatting must match, that is HUMAN-DECISION-REQUIRED; otherwise
    a plain `eprintln!`/`println!` is the pragmatic equivalent.
  - `log.Fatal(err)` → on serve error, `eprintln!("{err}"); std::process::exit(1);`
    (or have `main` return `Result` and let a non-zero exit happen). The semantic is:
    print the error and exit non-zero.

### 4. `errors`
- **Imported at:** `src/examples/seeds-go/server.go:5`; `src/examples/seeds-go/item.go:4`;
  `src/examples/seeds-go/store.go:4`; `src/examples/seeds-go/store_test.go:4`;
  `src/examples/seeds-go/server_test.go` (NOT imported here — tests use `json` only).
- **Used for:**
  - Sentinel error construction: `errors.New(...)` for `ErrNameRequired`, `ErrNameTooLong`
    (`item.go:22-23`), `ErrNotFound`, `ErrDuplicate` (`store.go:13-14`).
  - Sentinel comparison: `errors.Is(err, ErrXxx)` throughout (`server.go:31,33,53,66`;
    `store_test.go:24,58,61,69,86,116,119`).
- **Strategy:** REIMPLEMENT idiomatically with a Rust `enum` error type. Do NOT reach for
  `anyhow`/`thiserror` unless the model wants `thiserror` purely for `Display` derivation.
  - Model the four sentinels as one enum, e.g.:
    ```rust
    enum AppError { NameRequired, NameTooLong, NotFound, Duplicate }
    ```
  - `errors.Is(err, ErrXxx)` → Rust `match` / `matches!(err, AppError::Xxx)`.
  - Each variant MUST carry the EXACT message string (these strings are returned to the HTTP
    client verbatim via `err.Error()` at `server.go:32`):
    - `ErrNameRequired` = `"name is required"` (`item.go:22`).
    - `ErrNameTooLong`  = `"name must be at most 100 characters"` (`item.go:23`).
    - `ErrNotFound`     = `"item not found"` (`store.go:13`).
    - `ErrDuplicate`    = `"an item with that name already exists"` (`store.go:14`).
  - Implement `Display` to emit those exact strings.

### 5. `strings`
- **Imported at:** `src/examples/seeds-go/item.go:5`; `src/examples/seeds-go/store.go:6`;
  `src/examples/seeds-go/store_test.go:5`; `src/examples/seeds-go/server_test.go:7`.
- **Used for:**
  - `strings.TrimSpace(name)` (`item.go:29`, `store.go:37`) — trims leading/trailing
    Unicode whitespace.
  - `strings.EqualFold(a, b)` (`store.go:42`) — case-insensitive string equality, Unicode
    case-folding.
  - `strings.Repeat("x", n)` in tests (`store_test.go:19,20,61`).
  - `strings.NewReader(body)` in tests (`server_test.go:20`) — wraps a string as an
    `io.Reader` for the test request body.
- **Strategy:** REIMPLEMENT with Rust std + careful attention to semantics (NOT a crate):
  - `strings.TrimSpace` → `str::trim()`. BOTH trim Unicode whitespace; this is a close match.
    HAZARD: the exact set of "whitespace" code points differs between Go's `unicode.IsSpace`
    and Rust's `char::is_whitespace`. See hazards doc — for ASCII inputs (the only ones in
    the tests) they agree.
  - `strings.EqualFold` → NOT exactly `a.to_lowercase() == b.to_lowercase()`. Go's
    `EqualFold` is Unicode simple case-folding; Rust `to_lowercase` is full case mapping.
    For ASCII they agree; for general Unicode they can differ. See hazards doc — flag as a
    behavioral-equivalence hazard, not a drop. Pragmatic port:
    `a.eq_ignore_ascii_case(b)` if you accept ASCII-only semantics, OR
    `a.to_lowercase() == b.to_lowercase()` for a broader (but not identical) fold.
    **HUMAN-DECISION-REQUIRED** if exact Unicode case-fold parity with Go matters.
  - `strings.Repeat` → `"x".repeat(n)` (test code).
  - `strings.NewReader` → in Rust/axum tests, build the request body from a `String`/`&str`
    directly (e.g. `Body::from(...)`); no separate reader type needed.

### 6. `strconv`
- **Imported at:** `src/examples/seeds-go/server.go:7`.
- **Used for:** `strconv.Atoi(r.PathValue("id"))` to parse the path id (`server.go:47,61`).
  On error → 400 `"id must be an integer"`.
- **Strategy:** REIMPLEMENT with Rust std → `s.parse::<i32>()`.
  - HAZARD: `strconv.Atoi` parses into Go's `int` (platform-width, 64-bit on all targets the
    Go toolchain supports here). `Item.ID` is Go `int` (`item.go:14`). Use Rust `i64` to
    match Go `int` width safely, OR `i32` if you accept narrower ids. The id is only used as
    a map key and is auto-assigned from 1 (`store.go:28,46,48`), so overflow is not reachable
    in practice, but the **type choice** is a decision. Recommend `i64` to mirror Go `int`.
  - `strconv.Atoi` accepts an optional leading `+`/`-` and rejects whitespace/empty;
    Rust `i64::from_str` is similar but NOT identical (e.g. Go's `Atoi` rejects `"+"` alone,
    so does Rust). For the test inputs (`"abc"`, `"xyz"`, valid `"1"`, `"999"`:
    `server_test.go:84,87,88,114,117,120`) both behave identically (valid ints parse, garbage
    fails → 400). See hazards doc for the leading-`+`/`-` and underscore edge cases.

### 7. `sort`
- **Imported at:** `src/examples/seeds-go/store.go:5`.
- **Used for:** `sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })`
  (`store.go:71`) — sorts the listed items by ID ascending.
- **Strategy:** REIMPLEMENT with Rust std → `out.sort_by_key(|it| it.id)` or
  `out.sort_by(|a, b| a.id.cmp(&b.id))`.
  - NOTE: Go's `sort.Slice` is NOT stable; `sort.SliceStable` is. Here IDs are unique
    (auto-incremented, `store.go:46-48`) so stability is irrelevant — any sort by ID gives a
    deterministic order. Rust `sort_by_key` (stable) is a safe superset.

### 8. `sync`
- **Imported at:** `src/examples/seeds-go/store.go:7`.
- **Used for:** `sync.Mutex` field on `Store` (`store.go:20`); `s.mu.Lock()` /
  `defer s.mu.Unlock()` in every store method (`store.go:39-40,54-55,65-66,77-78`).
  Protects the `items map` + `nextID`.
- **Strategy:** REIMPLEMENT with Rust std → `std::sync::Mutex<StoreInner>` (NOT `tokio::Mutex`
  unless you hold the lock across `.await`; this code does not — all critical sections are
  synchronous map operations). The `Store` is shared across async handlers via
  `axum`'s state, so wrap as `Arc<Store>` where `Store` contains a `Mutex`.
  - CRITICAL DESIGN HAZARD — Go's `defer Unlock` + Rust borrow rules differ: in Rust the lock
    guard releases on scope end; the whole-method critical sections map cleanly to taking the
    guard at the top of each method. See hazards doc.
  - `Item` is a value type copied out of the map in Go (`store.go:46-49,56-60`). In Rust,
    `Item` must be `Clone` and you `.clone()` it out of the map while holding the guard, to
    avoid returning a borrow of locked data.

### 9. `time`
- **Imported at:** `src/examples/seeds-go/item.go:6`; `src/examples/seeds-go/store.go:8`.
- **Used for:**
  - `Item.CreatedAt time.Time` field, JSON tag `createdAt` (`item.go:17`).
  - `Store.now func() time.Time` injectable clock (`store.go:23`), defaulting to `time.Now`
    (`store.go:28`), called at item creation (`store.go:46`: `CreatedAt: s.now()`).
- **Strategy:** TARGET-EQUIVALENT → `chrono` (`DateTime<Utc>`), with an injectable clock.
  - `time.Time` field → `chrono::DateTime<chrono::Utc>` field named `created_at` and
    serde-renamed to `createdAt` (matching `item.go:17`).
  - `Store.now func() time.Time` → store a clock as a boxed closure
    (`Box<dyn Fn() -> DateTime<Utc> + Send + Sync>`) or a generic/trait clock, so tests can
    inject a fixed time exactly like Go does. Default = `Utc::now`.
  - **HUMAN-DECISION-REQUIRED — JSON timestamp format & timezone.** Go's `time.Time` marshals
    to RFC 3339 with nanosecond precision in the time's location (e.g.
    `"2026-06-26T12:00:00.123456789Z"` for UTC, or with a numeric offset for local time).
    `time.Now()` returns **local** time, so Go's default output includes the machine's offset,
    not necessarily `Z`. `chrono::DateTime<Utc>` serializes as RFC 3339 with `Z`. These are
    NOT byte-identical in the general case. Decisions required:
    1. Should the port emit UTC (`Z`) always, or mirror Go's local-offset behavior?
    2. Must nanosecond precision and trailing-zero handling match Go exactly?
    The existing tests (`server_test.go`, `store_test.go`) NEVER assert on `CreatedAt`/
    `createdAt` content — verified: no test references `CreatedAt`, `createdAt`, or any time
    value. So no test will catch a format divergence. Flag this for a human; default to
    `DateTime<Utc>` (UTC `Z`) as the pragmatic choice and document the divergence.

---

## Test-framework dependencies

- **Go testing:** `testing` (`store_test.go:6`, `server_test.go:8`), table-driven subtests
  via `t.Run` (`store_test.go:23,55? actually 22-23`, `server_test.go:55`).
- **HTTP test harness:** `net/http/httptest` (`server_test.go:6`) — `httptest.NewRequest`
  (`server_test.go:19,21`), `httptest.NewRecorder` (`server_test.go:23`),
  `srv.ServeHTTP(w, r)` (`server_test.go:24`).
- **Strategy:** TARGET-EQUIVALENT for the runner; REIMPLEMENT the HTTP harness:
  - `testing` → Rust's built-in `#[test]` (and `#[tokio::test]` for async handler tests).
    Use module `#[cfg(test)] mod tests`. Table-driven subtests → a `for` loop over an array
    of cases inside one `#[test]` fn (Rust has no native subtests; the loop + descriptive
    assert messages preserve the intent of `store_test.go:22-28` and `server_test.go:54-62`).
  - `net/http/httptest` → exercise the `axum::Router` in-process with `tower::ServiceExt`'s
    `.oneshot(request)` (the `tower` crate, dev-dependency) — this is the idiomatic axum
    equivalent of `ServeHTTP(recorder, req)`: build an `http::Request`, call
    `app.clone().oneshot(req).await`, inspect `response.status()` and the collected body
    bytes. Add `tower = { version = "0.5", features = ["util"] }` and `http-body-util` as
    **dev-dependencies**. `[UNVERIFIED]` versions — pick latest compatible.

---

## Summary table

| Go import | Where | Rust strategy | Concrete target |
|---|---|---|---|
| `encoding/json` | server.go:4 | TARGET-EQUIVALENT | `serde` + `serde_json` (via axum `Json`) |
| `net/http` | main.go:6, server.go:6 | TARGET-EQUIVALENT | `axum` + `tokio` |
| `log` | main.go:5 | REIMPLEMENT | `eprintln!` + `std::process::exit(1)` |
| `errors` | item.go:4, store.go:4, server.go:5 | REIMPLEMENT | Rust `enum` error + `Display` |
| `strings` | item.go:5, store.go:6 | REIMPLEMENT | `str::trim`, case-fold (see hazard) |
| `strconv` | server.go:7 | REIMPLEMENT | `str::parse::<i64>()` |
| `sort` | store.go:5 | REIMPLEMENT | `Vec::sort_by_key` |
| `sync` | store.go:7 | REIMPLEMENT | `std::sync::Mutex` + `Arc` |
| `time` | item.go:6, store.go:8 | TARGET-EQUIVALENT | `chrono` (see timestamp HUMAN-DECISION) |
| `testing` | *_test.go | TARGET-EQUIVALENT | built-in `#[test]`/`#[tokio::test]` |
| `net/http/httptest` | server_test.go:6 | REIMPLEMENT | `tower::ServiceExt::oneshot` |

## Items explicitly flagged for a human (do not silently guess)

1. **JSON timestamp format/timezone** for `CreatedAt`/`createdAt` (Go local-time RFC 3339 w/
   nanos vs chrono UTC `Z`). No test covers it. → HUMAN-DECISION-REQUIRED.
2. **Unicode case-fold parity** for duplicate detection (`strings.EqualFold`, store.go:42) —
   ASCII-equivalent, Unicode-divergent. → HUMAN-DECISION-REQUIRED only if non-ASCII names
   must fold identically to Go.
3. **`int` width for ids** (Go `int` = 64-bit here; pick Rust `i64` vs `i32`). Recommend
   `i64`; flag if a narrower type is intended.
4. **axum path-param syntax** (`{id}` vs `:id`) and **`Json` extractor rejection body** must
   be wired to reproduce Go's exact 400 bodies — version-dependent; verify at build.
