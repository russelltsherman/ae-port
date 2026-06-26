package seeds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() http.Handler {
	return NewServer(NewStore())
}

func do(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func TestCreateItemHTTP(t *testing.T) {
	srv := newTestServer()

	w := do(t, srv, "POST", "/items", `{"name":"buy milk"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /items code = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var it Item
	if err := json.Unmarshal(w.Body.Bytes(), &it); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if it.ID != 1 || it.Name != "buy milk" {
		t.Errorf("created item = %+v, want ID 1 name 'buy milk'", it)
	}
}

func TestCreateItemHTTPErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"invalid json", `{not json`, http.StatusBadRequest},
		{"empty name", `{"name":""}`, http.StatusBadRequest},
		{"whitespace name", `{"name":"   "}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newTestServer()
			w := do(t, srv, "POST", "/items", c.body)
			if w.Code != c.want {
				t.Errorf("code = %d, want %d; body=%s", w.Code, c.want, w.Body.String())
			}
		})
	}
}

func TestCreateItemHTTPDuplicate(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"dup"}`)

	w := do(t, srv, "POST", "/items", `{"name":"DUP"}`)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate POST code = %d, want 409", w.Code)
	}
}

func TestGetItemHTTP(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"findme"}`)

	w := do(t, srv, "GET", "/items/1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items/1 code = %d, want 200", w.Code)
	}

	if w := do(t, srv, "GET", "/items/999", ""); w.Code != http.StatusNotFound {
		t.Errorf("GET missing code = %d, want 404", w.Code)
	}
	if w := do(t, srv, "GET", "/items/abc", ""); w.Code != http.StatusBadRequest {
		t.Errorf("GET non-int code = %d, want 400", w.Code)
	}
}

func TestListItemsHTTP(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"a"}`)
	do(t, srv, "POST", "/items", `{"name":"b"}`)

	w := do(t, srv, "GET", "/items", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items code = %d, want 200", w.Code)
	}
	var items []Item
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("list len = %d, want 2", len(items))
	}
}

// TestListItemsHTTPEmpty locks the empty-store guarantee at the HTTP layer:
// the body MUST be a JSON empty array "[]", never "null" (server.go:43 ->
// store.go:67 builds a non-nil empty slice).
func TestListItemsHTTPEmpty(t *testing.T) {
	srv := newTestServer()

	w := do(t, srv, "GET", "/items", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items (empty) code = %d, want 200", w.Code)
	}
	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Errorf("GET /items (empty) body = %q, want %q (must not be null)", got, "[]")
	}
}

// TestListItemsHTTPAscendingOrder locks the ascending-by-id ordering guarantee
// at the HTTP layer (server.go:43 -> store.go:71).
func TestListItemsHTTPAscendingOrder(t *testing.T) {
	srv := newTestServer()
	for _, n := range []string{"a", "b", "c"} {
		do(t, srv, "POST", "/items", `{"name":"`+n+`"}`)
	}

	w := do(t, srv, "GET", "/items", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /items code = %d, want 200", w.Code)
	}
	var items []Item
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("list len = %d, want 3", len(items))
	}
	for i := 1; i < len(items); i++ {
		if items[i-1].ID >= items[i].ID {
			t.Errorf("HTTP list not ascending by id: %+v", items)
		}
	}
}

// TestListItemsHTTPContentType locks the Content-Type header set by writeJSON
// (server.go:77).
func TestListItemsHTTPContentType(t *testing.T) {
	srv := newTestServer()
	w := do(t, srv, "GET", "/items", "")
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// TestCreateItemHTTPOverLongName locks the over-length-name -> 400 mapping at
// the HTTP layer via ErrNameTooLong (server.go:31-32). MaxNameLen+1 runes.
func TestCreateItemHTTPOverLongName(t *testing.T) {
	srv := newTestServer()
	longName := strings.Repeat("x", MaxNameLen+1)
	w := do(t, srv, "POST", "/items", `{"name":"`+longName+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("over-length POST code = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	var env map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if env["error"] != ErrNameTooLong.Error() {
		t.Errorf("error body = %q, want %q", env["error"], ErrNameTooLong.Error())
	}
}

// TestErrorResponseBodyShape locks the {"error":<msg>} envelope produced by
// writeError (server.go:82-83) across representative error outcomes.
func TestErrorResponseBodyShape(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(srv http.Handler)
		method    string
		path      string
		body      string
		wantCode  int
		wantError string
	}{
		{
			name:      "invalid json 400",
			method:    "POST",
			path:      "/items",
			body:      `{not json`,
			wantCode:  http.StatusBadRequest,
			wantError: "invalid JSON body",
		},
		{
			name:      "empty name 400",
			method:    "POST",
			path:      "/items",
			body:      `{"name":""}`,
			wantCode:  http.StatusBadRequest,
			wantError: ErrNameRequired.Error(),
		},
		{
			name: "duplicate 409",
			setup: func(srv http.Handler) {
				do(t, srv, "POST", "/items", `{"name":"dup"}`)
			},
			method:    "POST",
			path:      "/items",
			body:      `{"name":"DUP"}`,
			wantCode:  http.StatusConflict,
			wantError: ErrDuplicate.Error(),
		},
		{
			name:      "get non-int id 400",
			method:    "GET",
			path:      "/items/abc",
			wantCode:  http.StatusBadRequest,
			wantError: "id must be an integer",
		},
		{
			name:      "get missing 404",
			method:    "GET",
			path:      "/items/999",
			wantCode:  http.StatusNotFound,
			wantError: ErrNotFound.Error(),
		},
		{
			name:      "delete missing 404",
			method:    "DELETE",
			path:      "/items/999",
			wantCode:  http.StatusNotFound,
			wantError: ErrNotFound.Error(),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newTestServer()
			if c.setup != nil {
				c.setup(srv)
			}
			w := do(t, srv, c.method, c.path, c.body)
			if w.Code != c.wantCode {
				t.Fatalf("code = %d, want %d; body=%s", w.Code, c.wantCode, w.Body.String())
			}
			var env map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("error body not a JSON object: %v; raw=%s", err, w.Body.String())
			}
			if _, ok := env["error"]; !ok {
				t.Fatalf("error body missing %q key; got %v", "error", env)
			}
			if len(env) != 1 {
				t.Errorf("error body has %d keys, want exactly 1 ({\"error\":...}); got %v", len(env), env)
			}
			if env["error"] != c.wantError {
				t.Errorf("error message = %q, want %q", env["error"], c.wantError)
			}
		})
	}
}

func TestDeleteItemHTTP(t *testing.T) {
	srv := newTestServer()
	do(t, srv, "POST", "/items", `{"name":"temp"}`)

	if w := do(t, srv, "DELETE", "/items/1", ""); w.Code != http.StatusNoContent {
		t.Fatalf("DELETE existing code = %d, want 204", w.Code)
	}
	if w := do(t, srv, "DELETE", "/items/1", ""); w.Code != http.StatusNotFound {
		t.Errorf("DELETE missing code = %d, want 404", w.Code)
	}
	if w := do(t, srv, "DELETE", "/items/xyz", ""); w.Code != http.StatusBadRequest {
		t.Errorf("DELETE non-int code = %d, want 400", w.Code)
	}
}
