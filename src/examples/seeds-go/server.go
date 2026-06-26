package seeds

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// NewServer wires the item routes to an http.Handler backed by store.
//
// Routes:
//
//	POST   /items       create an item        201 | 400 | 409
//	GET    /items       list items            200
//	GET    /items/{id}  fetch one item        200 | 400 | 404
//	DELETE /items/{id}  delete one item       204 | 400 | 404
func NewServer(store *Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		it, err := store.Create(body.Name)
		switch {
		case errors.Is(err, ErrNameRequired), errors.Is(err, ErrNameTooLong):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrDuplicate):
			writeError(w, http.StatusConflict, err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal error")
		default:
			writeJSON(w, http.StatusCreated, it)
		}
	})

	mux.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	})

	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		it, err := store.Get(id)
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, it)
	})

	mux.HandleFunc("DELETE /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		if errors.Is(store.Delete(id), ErrNotFound) {
			writeError(w, http.StatusNotFound, ErrNotFound.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
