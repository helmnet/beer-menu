package httpapi

import "net/http"

// withSecurity wraps the API (research R5, R11). Every response gets nosniff, DENY framing,
// no-referrer and a CSP allowing nothing, and every response except /api/health gets
// Cache-Control: no-store. Inside that, http.CrossOriginProtection refuses non-safe requests
// from another site (Sec-Fetch-Site, or Origin against Host) with 403 forbidden before any
// guard or handler runs; requests with neither header (scripts, curl) pass.
func withSecurity(h http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusForbidden, "forbidden", "Cross-site request refused.")
	}))
	inner := cop.Handler(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "DENY")
		hdr.Set("Referrer-Policy", "no-referrer")
		hdr.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.URL.Path != "/api/health" {
			hdr.Set("Cache-Control", "no-store")
		}
		inner.ServeHTTP(w, r)
	})
}
