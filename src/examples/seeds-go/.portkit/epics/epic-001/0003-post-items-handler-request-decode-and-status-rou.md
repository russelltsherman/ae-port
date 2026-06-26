# EPIC-001-S5 — POST /items handler: request decode and status routing

**Capability:** Handle `POST /items` — decode the JSON request body, call `Store.Create`, and map each outcome to the correct HTTP status (201 / 400 / 409 / 500).

This is **build #3** in epic EPIC-001.

---

## Scope

This slice builds exactly ONE thing: the `POST /items` route handler registered inside
`NewServer`. It does NOT build the other routes (`GET /items`, `GET /items/{id}`,
`DELETE /items/{id}`), the `Store`, the `Item` type, the sentinel errors, or the response
helpers — those belong to the kernel and to other slices (see Prerequisites). Build only the
handler closure and its registration on the mux.

---

## Prerequisite slices (build order)

This slice **dependsOn**:

1. **`SHARED-response-helpers`** — provides `writeJSON(w http.ResponseWriter, status int, v any)`
   and `writeError(w http.ResponseWriter, status int, msg string)`. This slice CALLS them but
   does NOT define them. (Defined at `server.go:76-84`; see Kernel references.)
2. **`EPIC-001-S4`** — provides `Store.Create(name string) (Item, error)` and its error
   contract (returns `ErrNameRequired`, `ErrNameTooLong`, or `ErrDuplicate`). This slice CALLS
   `store.Create` and switches on its returned error, but does NOT implement `Create`.

Also required from the **kernel** (build kernel first): the `Item` type, the `Store` type +
`NewStore`, and the four sentinel errors (`ErrNameRequired`, `ErrNameTooLong`, `ErrDuplicate`,
`ErrNotFound`). See "Kernel references" below. Do NOT reach into any other slice's internals;
interact only through the public signatures named here.

---

## End-to-end behavior thread

Each step cites the source it is rebuilt from (`path:line`). Paths are relative to
`src/examples/seeds-go`.

1. **Route registration** — the handler is registered on an `*http.ServeMux` (created by
   `NewServer` at `server.go:19`) using Go 1.22+ method+path routing with the pattern string
   `"POST /items"` (`server.go:21`). The mux only dispatches `POST` requests whose path is
   exactly `/items` to this handler.
2. **Anonymous body struct** — inside the handler, declare a local variable whose type is an
   anonymous struct with a single field `Name string` tagged ``json:"name"``
   (`server.go:22-24`). Only the `name` JSON key is read; any other JSON fields in the body are
   ignored by the decoder.
3. **JSON decode** — `json.NewDecoder(r.Body).Decode(&body)` (`server.go:25`). If `Decode`
   returns ANY non-nil error (malformed JSON, a JSON value of the wrong type for `name`, or an
   empty body which yields `io.EOF`), respond `writeError(w, http.StatusBadRequest, "invalid JSON body")`
   and `return` immediately (`server.go:26-27`). An empty request body therefore yields **400**
   with message `"invalid JSON body"` (EOF), NOT a missing-name 400.
4. **Call the store** — on successful decode, call `it, err := store.Create(body.Name)`
   (`server.go:29`). `body.Name` is passed exactly as decoded (the store trims/validates it).
5. **Status routing via `switch`** (`server.go:30-39`) — a tag-less `switch` evaluated
   top-to-bottom; the FIRST matching case wins:
   - **Case A (400 validation):** `errors.Is(err, ErrNameRequired)` OR
     `errors.Is(err, ErrNameTooLong)` → `writeError(w, http.StatusBadRequest, err.Error())`
     (`server.go:31-32`). The body carries the validation message, e.g. `{"error":"name is required"}`.
   - **Case B (409 duplicate):** `errors.Is(err, ErrDuplicate)` →
     `writeError(w, http.StatusConflict, err.Error())` (`server.go:33-34`), i.e.
     `{"error":"an item with that name already exists"}`.
   - **Case C (500 defensive):** `err != nil` (any other non-nil error) →
     `writeError(w, http.StatusInternalServerError, "internal error")` (`server.go:35-36`).
     This is a defensive catch-all. With the kernel's in-memory `Store`, `Create` only ever
     returns the three sentinels above, so this branch is **unreachable in practice**
     `[UNVERIFIED at runtime — no test exercises it]`, but it MUST exist per the source.
   - **Case D (201 success):** `default` (i.e. `err == nil`) →
     `writeJSON(w, http.StatusCreated, it)` (`server.go:37-38`), returning the created `Item`
     as JSON with status 201 Created.

**Ordering guarantee:** decode-failure is checked BEFORE `Create` is ever called. The two
distinct 400 outcomes differ only in message: a decode failure says `"invalid JSON body"`; a
validation failure says the validation error text (e.g. `"name is required"`). Both are HTTP 400.

---

## Interface / contract (exact behavior)

**Registration:** `mux.HandleFunc("POST /items", handler)` where `mux` is the `*http.ServeMux`
NewServer returns wrapped as `http.Handler` (`server.go:21`).

**Handler signature:** `func(w http.ResponseWriter, r *http.Request)` (`server.go:21`).

**Input:** the request body, expected to be a JSON object with a string field `name`. Example:
`{"name":"buy milk"}`. Extra fields are ignored (`server.go:22-24`).

**Outputs — exhaustive outcome table** (first match wins, in this order):

| # | Condition | HTTP status | Response body | Source |
|---|-----------|-------------|---------------|--------|
| 1 | `Decode` returns any error (malformed JSON, wrong type for `name`, or empty body → EOF) | **400** | `{"error":"invalid JSON body"}` | `server.go:25-27` |
| 2 | `Create` returns `ErrNameRequired` or `ErrNameTooLong` (matched via `errors.Is`) | **400** | `{"error":"<err.Error()>"}` — e.g. `{"error":"name is required"}` or `{"error":"name must be at most 100 characters"}` | `server.go:31-32` |
| 3 | `Create` returns `ErrDuplicate` (matched via `errors.Is`) | **409** | `{"error":"an item with that name already exists"}` | `server.go:33-34` |
| 4 | `Create` returns any other non-nil error | **500** | `{"error":"internal error"}` | `server.go:35-36` |
| 5 | `Create` returns `nil` error | **201** | the created `Item` as JSON (e.g. `{"id":1,"name":"buy milk","done":false,"createdAt":"..."}`) | `server.go:37-38` |

**Exact behavior notes (do not deviate):**

- Error matching MUST use `errors.Is`, NOT `==` (`server.go:31,33`), so a wrapped sentinel
  still matches. (Kernel convention; see Kernel references.)
- On a decode error the handler returns IMMEDIATELY without calling `Create` (`server.go:27`).
- An empty request body maps to outcome #1 (`"invalid JSON body"`), NOT outcome #2 — because
  `Decode` on an empty reader returns `io.EOF` before any name validation runs (`server.go:25-27`).
- An empty name (`{"name":""}`) and an all-whitespace name (`{"name":"   "}`) BOTH reach
  `Create`, which returns `ErrNameRequired`, mapping to outcome #2 (400). The store trims the
  name before validating, so whitespace-only is treated as empty (kernel/S4 behavior).
- A name longer than 100 runes maps to outcome #2 (400) via `ErrNameTooLong`. Test-backed at the
  HTTP layer by `TestCreateItemHTTPOverLongName` (`server_test.go:163-177`), which posts a
  `MaxNameLen+1`-rune name and asserts status 400 with body `{"error":"name must be at most 100
  characters"}`. (Also verified at the store layer in S4.)
- Response bodies for ALL error outcomes are the JSON envelope `{"error":<msg>}` produced by
  `writeError` (`server.go:32,34,36,82-83`). The success body is the bare encoded `Item`
  (no envelope) produced by `writeJSON` (`server.go:38,76-79`). The error envelope shape is
  test-backed by `TestErrorResponseBodyShape` (`server_test.go:181-265`), which decodes the body
  into a `map[string]string` and asserts it has exactly one key `"error"` with the expected
  message across the 400 (invalid JSON), 400 (empty name), and 409 (duplicate) POST outcomes
  (plus the GET/DELETE error outcomes from epics 002/004).
- The success status is exactly `http.StatusCreated` (201), not 200 (`server.go:38`).

---

## Acceptance tests for THIS slice

These mirror the behavioral spec and the existing HTTP tests (`server_test.go`). They are
concrete and runnable-in-spirit: build the server with the kernel `NewStore()`, send an HTTP
request via `httptest`, and assert the status (and where noted, the body). VERIFIED references
point at the existing test that already proves the behavior.

1. **Route + happy path (201).** `POST /items` with body `{"name":"buy milk"}` returns status
   **201**, and the response body decodes to an `Item` with `ID == 1` and `Name == "buy milk"`.
   *(VERIFIED `server_test.go:31-41`; grounded `server.go:38`.)*
2. **Malformed JSON → 400.** `POST /items` with body `{not json` returns status **400**.
   (Decode-failure path; message `"invalid JSON body"`.) *(VERIFIED `server_test.go:50`;
   grounded `server.go:25-27`.)*
3. **Empty name → 400.** `POST /items` with body `{"name":""}` returns status **400**.
   (Validation path: `ErrNameRequired`.) *(VERIFIED `server_test.go:51`; grounded `server.go:31-32`.)*
4. **Whitespace-only name → 400.** `POST /items` with body `{"name":"   "}` returns status
   **400**. (Validation path: `ErrNameRequired`, because the store trims before validating.)
   *(VERIFIED `server_test.go:52`; grounded `server.go:31-32`.)*
5. **Duplicate name (case-insensitive) → 409.** First `POST /items` `{"name":"dup"}`, then a
   second `POST /items` `{"name":"DUP"}` returns status **409**. *(VERIFIED `server_test.go:65-72`;
   grounded `server.go:33-34`.)*
6. **Over-length name → 400.** A name of more than 100 runes maps to 400 via `ErrNameTooLong`,
   with body `{"error":"name must be at most 100 characters"}`.
   *(VERIFIED `TestCreateItemHTTPOverLongName` at `server_test.go:163-177`; grounded
   `server.go:31-32`.)*
7. **Unexpected store error → 500.** Any non-validation, non-duplicate error from `Create` maps
   to status **500** with body `{"error":"internal error"}`. *(Grounded `server.go:35-36`.
   `[UNVERIFIED / unreachable with the default in-memory store — keep the branch present per the
   source contract; it cannot be triggered without an alternate store.]`)*
8. **Status mapping is exhaustive and ordered.** validation (required/too-long) → 400;
   duplicate → 409; any other error → 500; success → 201; and decode failure → 400 BEFORE
   `Create` is called. *(Grounded `server.go:25-39`.)*
9. **Error body shape.** Every reachable 4xx response body is `{"error":<msg>}` from `writeError`
   — a JSON object with exactly one key `"error"`.
   *(VERIFIED `TestErrorResponseBodyShape` at `server_test.go:181-265`, which asserts the
   single-key `{"error":...}` envelope and exact message for the 400/409 POST outcomes and the
   GET/DELETE 400/404 outcomes; grounded `server.go:32,34,36,82-83`. NOTE: the 500 branch body
   `{"error":"internal error"}` is NOT covered — that branch is unreachable with the in-memory
   store; see AT-7.)*

Test harness pattern (matches `server_test.go:11-26`): construct with
`NewServer(NewStore())`, build the request with `httptest.NewRequest("POST", "/items", body)`
(use a `nil` body for the empty-body case, `strings.NewReader(...)` otherwise), serve into an
`httptest.NewRecorder()`, then read `w.Code` and `w.Body`.

---

## Build steps (each individually checkable)

Assume the kernel and prerequisite slices already exist (`Item`, `Store`, `NewStore`,
`Store.Create`, the four sentinels, `writeJSON`, `writeError`). All code goes in package
`seeds` (`server.go:1`).

1. **Register the route.** Inside `NewServer`, on the `*http.ServeMux`, call
   `mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) { ... })`
   (`server.go:21`). *Check:* the server routes a POST to `/items` to this closure and rejects
   other methods/paths.
2. **Declare the body struct + decode.** Inside the closure, declare
   `var body struct { Name string \`json:"name"\` }`, then
   `if err := json.NewDecoder(r.Body).Decode(&body); err != nil { writeError(w, http.StatusBadRequest, "invalid JSON body"); return }`
   (`server.go:22-28`). Add `"encoding/json"` and `"net/http"` to imports. *Check:* malformed
   JSON and empty body both yield 400 `{"error":"invalid JSON body"}`.
3. **Call the store.** `it, err := store.Create(body.Name)` (`server.go:29`). *Check:* a valid
   name reaches `Create`.
4. **Map validation errors → 400.** First `switch` case:
   `case errors.Is(err, ErrNameRequired), errors.Is(err, ErrNameTooLong): writeError(w, http.StatusBadRequest, err.Error())`
   (`server.go:30-32`). Add `"errors"` to imports. *Check:* `{"name":""}` → 400 with
   `{"error":"name is required"}`.
5. **Map duplicate → 409.** Next case:
   `case errors.Is(err, ErrDuplicate): writeError(w, http.StatusConflict, err.Error())`
   (`server.go:33-34`). *Check:* a second create of a case-insensitive duplicate → 409.
6. **Map any other error → 500 (defensive).** Next case:
   `case err != nil: writeError(w, http.StatusInternalServerError, "internal error")`
   (`server.go:35-36`). *Check:* compiles and is ordered AFTER the two sentinel cases.
7. **Success → 201.** `default: writeJSON(w, http.StatusCreated, it)` (`server.go:37-38`).
   *Check:* a valid create returns 201 with the `Item` JSON.
8. **Wire acceptance tests.** Add HTTP tests covering acceptance criteria 1-6 and 9
   (`TestCreateItemHTTP*` and `TestErrorResponseBodyShape` in `server_test.go`). Criterion 7 (the
   500 branch) stays unreachable with the in-memory store and is intentionally untested; criterion
   8 is an ordering property covered indirectly by 1-6. *Check:* `go test -race ./...` passes.

---

## Kernel references (do not restate — these come from KERNEL.md)

This slice relies on the following kernel-defined names/types/conventions. Reference them; do
not redefine them here.

- **Package** `seeds` at the module root (`KERNEL.md` "Module / package facts"; `server.go:1`).
- **`Item`** struct — the success response payload encoded as JSON with keys
  `id`, `name`, `done`, `createdAt` (`KERNEL.md` "Type glossary"; `item.go:13-18`).
- **`Store`** + **`NewStore() *Store`** — built by the caller/binary and passed into
  `NewServer`; this slice only calls `store.Create` on it (`KERNEL.md` "Type glossary";
  `store.go:19-29`).
- **`Store.Create(name string) (Item, error)`** — provided by EPIC-001-S4; returns one of the
  validation/duplicate sentinels or `nil`. This slice consumes its error contract.
- **Sentinel errors** `ErrNameRequired` (`name is required`), `ErrNameTooLong`
  (`name must be at most 100 characters`), `ErrDuplicate`
  (`an item with that name already exists`) — package-level `errors.New(...)` values matched
  with `errors.Is` (`KERNEL.md` "Sentinel error glossary"; `item.go:22-23`, `store.go:14`).
  The exact message strings are part of the HTTP contract (echoed via `err.Error()`).
- **Response helpers** `writeJSON` and `writeError` — provided by `SHARED-response-helpers`;
  `writeError` produces the `{"error":<message>}` envelope (`KERNEL.md` "response envelope";
  `server.go:82-83`). This slice calls them; it does not define them.
- **Convention:** match sentinels with `errors.Is`, never `==` (`KERNEL.md` "Sentinel error
  glossary"; `server.go:31,33`).
