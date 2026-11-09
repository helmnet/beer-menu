package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// listBeers handles GET /api/beers (public menu read): 200 with the current beers sorted by
// name, or only the archived ones with ?archived=true, as a JSON array that is [] when empty.
// An archived value strconv.ParseBool refuses gives 400 invalid_request.
func listBeers(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		archived := false
		if q := r.URL.Query(); q.Has("archived") {
			var err error
			if archived, err = strconv.ParseBool(q.Get("archived")); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "archived must be true or false.")
				return
			}
		}
		list, err := beers.List(deps.DB, archived)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// getBeer handles GET /api/beers/{id} (public menu read): 200 with the beer, archived or not.
// An unknown or non-positive id gives 404 not_found.
func getBeer(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		b, err := beers.Get(deps.DB, id)
		if err != nil {
			writeBeerError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, b)
	})
}

// getSrmColors handles GET /api/srm-colors (public menu read): 200 with {"colors": [...]},
// the 40 display colors for SRM 1 to 40, so clients preview colors with the API's table.
func getSrmColors(Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Colors [40]string `json:"colors"`
		}{beers.SRMColors})
	})
}

// createBeer handles POST /api/beers (write): it creates a manual beer from the BeerCreate body,
// at most 16 KiB, and returns 201 with the Beer. Invalid fields give 400 with a fields map;
// unknown fields, including source and source_ref, give 400 invalid_request.
func createBeer(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in beers.Input
		if !decodeJSON(w, r, &in, maxBeerBody) {
			return
		}
		b, err := beers.Create(deps.DB, in, deps.Now())
		if err != nil {
			writeBeerError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "beer created", "beer_id", b.ID, "beer_name", b.Name)
		writeJSON(w, http.StatusCreated, b)
	})
}

// updateBeer handles PATCH /api/beers/{id} (write): it applies the BeerPatch body, at most
// 16 KiB, with merge-patch semantics and returns 200 with the Beer. An empty body object or
// invalid fields give 400; an unknown or non-positive id gives 404 not_found.
func updateBeer(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var p beers.Patch
		if !decodeJSON(w, r, &p, maxBeerBody) {
			return
		}
		b, err := beers.Update(deps.DB, id, p, deps.Now())
		if err != nil {
			writeBeerError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "beer updated", "beer_id", b.ID, "beer_name", b.Name)
		writeJSON(w, http.StatusOK, b)
	})
}

// deleteBeer handles DELETE /api/beers/{id} (write): it removes the beer permanently and
// returns 204. An unknown or non-positive id gives 404 not_found.
func deleteBeer(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := beers.Delete(deps.DB, id); err != nil {
			writeBeerError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "beer deleted", "beer_id", id)
		w.WriteHeader(http.StatusNoContent)
	})
}

// writeBeerError answers err from the beers package: 400 with a fields map for FieldErrors,
// 400 for an empty patch, 404 for ErrNotFound and 500 otherwise.
func writeBeerError(w http.ResponseWriter, r *http.Request, err error) {
	var fe beers.FieldErrors
	switch {
	case errors.As(err, &fe):
		writeJSON(w, http.StatusBadRequest, struct {
			errorBody
			Fields beers.FieldErrors `json:"fields"`
		}{errorBody{Error: "invalid_request", Message: "Some fields are invalid."}, fe})
	case errors.Is(err, beers.ErrEmptyPatch):
		writeError(w, http.StatusBadRequest, "invalid_request", "Send at least one field to change.")
	case errors.Is(err, beers.ErrNotFound):
		writeNotFound(w)
	default:
		writeInternal(w, r, err)
	}
}
