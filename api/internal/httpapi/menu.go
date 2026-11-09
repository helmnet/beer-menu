package httpapi

import (
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/kegs"
)

// getMenu handles GET /api/menu. Public: no credential, so the TV display needs no secret
// (constitution V). It returns 200 with the menu (enabled taps and the on-deck list) holding
// display fields only, never ids, sizes, volumes or timestamps; clients that need those use the
// credentialed tap and keg reads. The body is a compatibility contract: fields may be added but
// not removed or renamed within a major version. withSecurity sends Cache-Control: no-store.
func getMenu(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, err := kegs.GetMenu(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})
}
