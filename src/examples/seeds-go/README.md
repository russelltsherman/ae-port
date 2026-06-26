# seeds-go

A tiny, fully-tested Go HTTP JSON service used as a **source fixture** for the `portkit` plugin.
It is intentionally small but real: clear vertical capabilities, validation rules, and error/edge
paths, with table tests that act as the behavioral contract.

## Capabilities (vertical threads)

| Capability | Route | Outcomes |
|---|---|---|
| Create item | `POST /items` | 201 created · 400 invalid name/JSON · 409 duplicate name |
| Get item | `GET /items/{id}` | 200 · 400 non-integer id · 404 not found |
| List items | `GET /items` | 200 (sorted by id / creation order) |
| Delete item | `DELETE /items/{id}` | 204 · 400 non-integer id · 404 not found |

Rules: names are trimmed, required, ≤ 100 runes, and unique case-insensitively.

## Layout

```
item.go         # model + name validation
store.go        # thread-safe in-memory store, business rules
server.go       # HTTP layer mapping store errors -> status codes
cmd/server/     # main entrypoint
*_test.go       # behavioral spec (store + HTTP)
```

## Run

```bash
go test ./...        # run the behavioral spec
go vet ./...         # static checks
go run ./cmd/server  # serve on :8080
```

## Use as a portkit fixture

```
/portkit go src/examples/seeds-go      # or any target language
```

Then validate the generated `src/examples/seeds-go/.portkit/` against the gates in the plugin's
`VERIFICATION.md`.
