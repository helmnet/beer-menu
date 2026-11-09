package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/kegs"
)

// listKegTypes handles GET /api/keg-types (session or any token): 200 with the fixed keg type
// table as a JSON array of {type, label, size_l, shape}, so every client shares one table.
func listKegTypes(Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, kegs.Types)
	})
}

// listKegs handles GET /api/kegs (session or any token): 200 with the kegs as a JSON array.
// state is current (default: on a tap and waiting, oldest first), waiting or kicked (history,
// newest kicked first); beer_id and tap_id, positive integers, filter further. Any other value
// gives 400 invalid_request.
func listKegs(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := kegs.KegFilter{State: q.Get("state")}
		switch f.State {
		case "", kegs.StateCurrent, kegs.StateWaiting, kegs.StateKicked:
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "state must be current, waiting or kicked.")
			return
		}
		for name, dst := range map[string]*int64{"beer_id": &f.BeerID, "tap_id": &f.TapID} {
			if !q.Has(name) {
				continue
			}
			id, err := strconv.ParseInt(q.Get(name), 10, 64)
			if err != nil || id < 1 {
				writeError(w, http.StatusBadRequest, "invalid_request", name+" must be a positive integer.")
				return
			}
			*dst = id
		}
		list, err := kegs.ListKegs(deps.DB, f)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// getKeg handles GET /api/kegs/{id} (session or any token): 200 with the keg in any state. An
// unknown or non-positive id gives 404 not_found.
func getKeg(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		k, err := kegs.GetKeg(deps.DB, id)
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, k)
	})
}

// createKeg handles POST /api/kegs (write): it creates a keg from the KegCreate body, on a tap
// or waiting, and returns 201 with the Keg. Invalid fields, a missing or archived beer and a
// missing tap give 400 with a fields map; unknown fields 400 invalid_request; a tap that has a
// keg 409 tap_occupied.
func createKeg(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in kegs.KegInput
		if !decodeJSON(w, r, &in, maxBody) {
			return
		}
		k, err := kegs.CreateKeg(deps.DB, in, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "keg created", "keg_id", k.ID, "beer_id", k.BeerID, "state", k.State)
		writeJSON(w, http.StatusCreated, k)
	})
}

// updateKeg handles PATCH /api/kegs/{id} (write): it applies the KegPatch body with merge-patch
// semantics (correct beer, type or size; set remaining_l; move with tap_id, off with null; kick
// with kicked true) and returns 200 with the Keg. An empty body object or invalid fields give
// 400; an unknown or non-positive id 404 not_found; a tap with another keg 409 tap_occupied; any
// change but a repeat kick on a kicked keg 409 keg_kicked.
func updateKeg(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var p kegs.KegPatch
		if !decodeJSON(w, r, &p, maxBody) {
			return
		}
		k, err := kegs.UpdateKeg(deps.DB, id, p, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "keg updated", "keg_id", k.ID, "beer_id", k.BeerID, "state", k.State)
		writeJSON(w, http.StatusOK, k)
	})
}

// deleteKeg handles DELETE /api/kegs/{id} (write): it removes the keg record permanently, in
// any state, and returns 204. An unknown or non-positive id gives 404 not_found.
func deleteKeg(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := kegs.DeleteKeg(deps.DB, id); err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "keg deleted", "keg_id", id)
		w.WriteHeader(http.StatusNoContent)
	})
}

// pourKeg handles POST /api/kegs/{id}/pours (write): it subtracts the Pour body's amount_l
// (required, litres, over 0 and at most 100) from the keg's level, stopping at 0, and returns
// 200 with the Keg. Concurrent pours all count. This is the stable contract Home Assistant and
// keg scales call with a write token; it stays backward compatible within the major version.
// A missing or bad amount gives 400 with a fields map; unknown fields 400 invalid_request; an
// unknown or non-positive id 404 not_found; a kicked keg 409 keg_kicked.
func pourKeg(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body struct {
			AmountL *float64 `json:"amount_l"`
		}
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		if body.AmountL == nil {
			writeKegError(w, r, beers.FieldErrors{"amount_l": "Amount is required."})
			return
		}
		k, err := kegs.Pour(deps.DB, id, *body.AmountL, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "keg poured", "keg_id", k.ID, "amount_l", *body.AmountL, "remaining_l", k.RemainingL)
		writeJSON(w, http.StatusOK, k)
	})
}

// writeKegError answers err from the kegs package: 400 with a fields map for FieldErrors, 400
// for an empty patch, 404 for kegs.ErrNotFound, 409 with the conflict's code for
// ErrTapOccupied (tap_occupied), ErrTapHasKeg (tap_has_keg), ErrLastTap (last_tap) and
// ErrKegKicked (keg_kicked), and 500 otherwise.
func writeKegError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, kegs.ErrNotFound):
		writeNotFound(w)
	case errors.Is(err, kegs.ErrTapOccupied):
		writeError(w, http.StatusConflict, "tap_occupied", "That tap already has a keg. Take it off or kick it first.")
	case errors.Is(err, kegs.ErrTapHasKeg):
		writeError(w, http.StatusConflict, "tap_has_keg", "This tap has a keg. Take it off or kick it first.")
	case errors.Is(err, kegs.ErrLastTap):
		writeError(w, http.StatusConflict, "last_tap", "The last tap cannot be removed.")
	case errors.Is(err, kegs.ErrKegKicked):
		writeError(w, http.StatusConflict, "keg_kicked", "This keg is kicked and cannot change.")
	default:
		writeBeerError(w, r, err)
	}
}
