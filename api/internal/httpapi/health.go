package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type healthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
}

// health handles GET /api/health. It is public and returns only the status, the app
// version and the database status: 200 {"status":"ok",...,"database":"ok"} when the
// database answers a query within 2 s, else 503 {"status":"error",...,"database":"unreachable"}.
func health(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		resp := healthResponse{Status: "ok", Version: deps.Version, Database: "ok"}
		code := http.StatusOK
		if err := pingDB(ctx, deps); err != nil {
			slog.WarnContext(ctx, "health check: database unreachable", "err", err)
			resp.Status, resp.Database, code = "error", "unreachable", http.StatusServiceUnavailable
		}
		body, _ := json.Marshal(resp) // three strings never fail to marshal
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(body)
	})
}

func pingDB(ctx context.Context, deps Deps) error {
	var one int
	return deps.DB.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}
