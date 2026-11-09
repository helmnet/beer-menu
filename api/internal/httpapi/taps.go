package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/kegs"
)

// listTaps handles GET /api/taps (session or any token): 200 with every tap in position order,
// each with its current keg or null, as a JSON array.
func listTaps(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list, err := kegs.ListTaps(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// getTap handles GET /api/taps/{id} (session or any token): 200 with the tap and its current
// keg. An unknown or non-positive id gives 404 not_found.
func getTap(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		tp, err := kegs.GetTap(deps.DB, id)
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, tp)
	})
}

// createTap handles POST /api/taps (write): it adds a tap at the end from the TapCreate body and
// returns 201 with the Tap. Without a label the tap is named "Tap N". An invalid label gives 400
// with a fields map; unknown fields give 400 invalid_request.
func createTap(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in kegs.TapInput
		if !decodeJSON(w, r, &in, maxBody) {
			return
		}
		tp, err := kegs.CreateTap(deps.DB, in, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "tap created", "tap_id", tp.ID, "tap_label", tp.Label)
		writeJSON(w, http.StatusCreated, tp)
	})
}

// updateTap handles PATCH /api/taps/{id} (write): it renames, enables or disables the tap from
// the TapPatch body and returns 200 with the Tap. Disabling keeps its keg. An empty body object
// or invalid fields give 400; an unknown or non-positive id gives 404 not_found.
func updateTap(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var p kegs.TapPatch
		if !decodeJSON(w, r, &p, maxBody) {
			return
		}
		tp, err := kegs.UpdateTap(deps.DB, id, p, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "tap updated", "tap_id", tp.ID, "tap_label", tp.Label)
		writeJSON(w, http.StatusOK, tp)
	})
}

// reorderTaps handles PUT /api/taps/order (write): the TapOrder body {"ids": [...]} must list
// every tap exactly once, first to last. It sets positions 1..N in that order and returns 200
// with the taps in the new order. Any other list gives 400 with a fields map naming ids.
func reorderTaps(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		list, err := kegs.ReorderTaps(deps.DB, body.IDs, deps.Now())
		if err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "taps reordered", "taps", len(list))
		writeJSON(w, http.StatusOK, list)
	})
}

// deleteTap handles DELETE /api/taps/{id} (write): it removes the tap, closes the gap in
// positions and returns 204. A tap with a current keg gives 409 tap_has_keg, the only tap 409
// last_tap, and an unknown or non-positive id 404 not_found.
func deleteTap(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := kegs.DeleteTap(deps.DB, id); err != nil {
			writeKegError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "tap deleted", "tap_id", id)
		w.WriteHeader(http.StatusNoContent)
	})
}
