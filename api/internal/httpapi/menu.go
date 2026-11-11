package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/kegs"
	"github.com/helmnet/beer-menu/api/internal/settings"
)

// menuResponse is the GET /api/menu body: the menu plus the public display settings. The join
// lives here so kegs stays free of settings.
type menuResponse struct {
	kegs.Menu
	Display settings.Public `json:"display"`
}

// getMenu handles GET /api/menu. Public: no credential, so the TV display needs no secret
// (constitution V). It returns 200 with the menu (enabled taps and the on-deck list) holding
// display fields only, never ids, sizes, volumes or timestamps; clients that need those use the
// credentialed tap and keg reads. display holds every TV setting (settings.Public); units and
// every other setting are not public. A settings read error is logged and serves the
// default display, so it never fails the TV's only read; a menu read error still gives 500. The
// body is a compatibility contract: fields may be added but not removed or renamed within a
// major version. withSecurity sends Cache-Control: no-store.
func getMenu(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, err := kegs.GetMenu(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		d, err := settings.GetDisplay(deps.DB)
		if err != nil {
			slog.ErrorContext(r.Context(), "read display settings", "err", err)
		}
		writeJSON(w, http.StatusOK, menuResponse{Menu: m, Display: d.Public()})
	})
}
