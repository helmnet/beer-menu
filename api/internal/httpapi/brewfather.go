package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/importer"
)

// providerDeadline bounds the provider work of a request someone waits on (FR-025a): the
// client's 15 s timeout per call, but a list of several calls must still answer in time. A
// variable so tests can shorten it.
var providerDeadline = 20 * time.Second

// maxImportBatches and maxBatchIDLen are the import body limits in api/openapi.yaml.
const (
	maxImportBatches = 50
	maxBatchIDLen    = 100
)

// getBrewfather handles GET /api/brewfather (session or any token): 200 with the connection
// and sync status. It never holds the user ID or any part of the key.
func getBrewfather(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, r, deps, http.StatusOK)
	})
}

// connectBrewfather handles PUT /api/brewfather/connection (admin session): it checks the
// {user_id, api_key} body, at most 8 KiB, with one provider call and stores the account,
// replacing any earlier one, then answers 200 with the status. The body is never logged.
// Failures keep the stored account and map as in writeImporterError.
func connectBrewfather(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			UserID string `json:"user_id"`
			APIKey string `json:"api_key"`
		}
		if !decodeJSON(w, r, &in, maxBody) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), providerDeadline)
		defer cancel()
		if err := deps.Importer.Connect(ctx, in.UserID, in.APIKey); err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		writeStatus(w, r, deps, http.StatusOK)
	})
}

// disconnectBrewfather handles DELETE /api/brewfather/connection (admin session): it forgets
// the account and answers 204. Imported beers stay. Idempotent.
func disconnectBrewfather(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Importer.Disconnect(r.Context()); err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// listBrewfatherBatches handles GET /api/brewfather/batches (session or write token, since a
// fresh list spends the owner's provider quota): 200 with {batches, fetched_at}, every batch
// not archived, newest brew date first, each with the beer_id imported from it or null. The
// list is cached for 5 minutes unless ?refresh=true; a refresh value strconv.ParseBool refuses
// gives 400 invalid_request.
func listBrewfatherBatches(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refresh := false
		if q := r.URL.Query(); q.Has("refresh") {
			var err error
			if refresh, err = strconv.ParseBool(q.Get("refresh")); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "refresh must be true or false.")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), providerDeadline)
		defer cancel()
		list, err := deps.Importer.Batches(ctx, refresh)
		if err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// importBrewfatherBatches handles POST /api/brewfather/imports (session or write token): the
// body {batch_ids} holds 1 to 50 distinct IDs of 1 to 100 characters. It answers 200 with one
// result per ID in request order, even when some fail; a deadline or an unreachable provider
// reports the untried batches as failed. A rate limit already in force gives 429 before any
// provider call.
func importBrewfatherBatches(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			BatchIDs []string `json:"batch_ids"`
		}
		if !decodeJSON(w, r, &in, maxBody) {
			return
		}
		if !validBatchIDs(in.BatchIDs) {
			writeError(w, http.StatusBadRequest, "invalid_request", "batch_ids must list 1 to 50 distinct batch IDs of 1 to 100 characters.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), providerDeadline)
		defer cancel()
		results, err := deps.Importer.Import(ctx, in.BatchIDs)
		if err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		imported := 0
		for _, res := range results {
			if res.Outcome == importer.OutcomeImported {
				imported++
			}
		}
		slog.InfoContext(r.Context(), "brewfather import", "provider", "brewfather", "requested", len(results), "imported", imported)
		writeJSON(w, http.StatusOK, struct {
			Results []importer.ImportResult `json:"results"`
		}{results})
	})
}

// validBatchIDs reports whether ids holds 1 to 50 distinct IDs of 1 to 100 characters.
func validBatchIDs(ids []string) bool {
	if len(ids) < 1 || len(ids) > maxImportBatches {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if n := utf8.RuneCountInString(id); n < 1 || n > maxBatchIDLen || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// resyncBeer handles POST /api/beers/{id}/sync (session or write token): it reads the beer's
// batch fresh and answers 200 with {beer, adjusted}, overwriting only ABV, IBU, SRM, brew date
// and status and possibly archiving (archived beers allowed). A missing batch marks the beer
// not found, still 200. An unknown or non-positive id gives 404, a manual beer 409
// not_imported, and provider failures map as in writeImporterError under the same 20 s
// deadline.
func resyncBeer(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), providerDeadline)
		defer cancel()
		b, adjusted, err := deps.Importer.Resync(ctx, id)
		if err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Beer     beers.Beer `json:"beer"`
			Adjusted []string   `json:"adjusted"`
		}{b, adjusted})
	})
}

// setBrewfatherSchedule handles PUT /api/brewfather/schedule (admin session): the body
// {interval} is off, 15m, 1h, 6h or 24h, else 400 invalid_request. It stores the interval and
// answers 200 with the status. No provider call.
func setBrewfatherSchedule(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A missing interval decodes as "", which SetInterval refuses.
		var in struct {
			Interval string `json:"interval"`
		}
		if !decodeJSON(w, r, &in, maxBody) {
			return
		}
		if err := deps.Importer.SetInterval(r.Context(), in.Interval); err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		writeStatus(w, r, deps, http.StatusOK)
	})
}

// syncBrewfather handles POST /api/brewfather/sync (session or write token): it starts a sync
// in the background on deps.Background and answers 202 with the status (sync_running true)
// without waiting, since a sync can outlast a proxy's timeout. 409 sync_running while one
// runs; not connected, rate limited and the other refusals map as in writeImporterError, all
// before any provider call.
func syncBrewfather(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Importer.StartSync(deps.Background); err != nil {
			writeImporterError(w, r, deps, err)
			return
		}
		writeStatus(w, r, deps, http.StatusAccepted)
	})
}

// writeStatus answers code with the import status.
func writeStatus(w http.ResponseWriter, r *http.Request, deps Deps, code int) {
	st, err := deps.Importer.Status(r.Context())
	if err != nil {
		writeImporterError(w, r, deps, err)
		return
	}
	writeJSON(w, code, st)
}

// writeImporterError answers err from the importer: 503 not_configured, 409 not_connected,
// reconnect_needed, not_imported or sync_running, 400 invalid_request for bad credentials
// input (without echoing it) or a bad interval, 400 credentials_refused or no_batch_access,
// 502 provider_unreachable (including a deadline), 429 provider_rate_limited with Retry-After
// (whole seconds, at least 1) and retry_at, and writeBeerError for anything else.
func writeImporterError(w http.ResponseWriter, r *http.Request, deps Deps, err error) {
	var rl *importer.RateLimitedError
	var fe beers.FieldErrors
	switch {
	case errors.Is(err, importer.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "not_configured", "Brewfather import is not set up on this server.")
	case errors.Is(err, importer.ErrNotConnected):
		writeError(w, http.StatusConflict, "not_connected", "Connect a Brewfather account first.")
	case errors.Is(err, importer.ErrReconnectNeeded):
		writeError(w, http.StatusConflict, "reconnect_needed", "The stored Brewfather key cannot be read; connect again.")
	case errors.Is(err, importer.ErrNotImported):
		writeError(w, http.StatusConflict, "not_imported", "This beer did not come from Brewfather.")
	case errors.Is(err, importer.ErrSyncRunning):
		writeError(w, http.StatusConflict, "sync_running", "A sync is already running.")
	case errors.Is(err, importer.ErrInvalidInterval):
		writeError(w, http.StatusBadRequest, "invalid_request", "interval must be off, 15m, 1h, 6h or 24h.")
	case errors.As(err, &fe):
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id must be 1 to 200 and api_key 1 to 500 characters, without control characters.")
	case errors.Is(err, importer.ErrCredentialsRefused):
		writeError(w, http.StatusBadRequest, "credentials_refused", "Brewfather refused this user ID and API key.")
	case errors.Is(err, importer.ErrNoBatchAccess):
		writeError(w, http.StatusBadRequest, "no_batch_access", "This API key cannot read batches; give it the batches.read scope.")
	case errors.Is(err, importer.ErrUnreachable), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusBadGateway, "provider_unreachable", "Brewfather could not be reached; try again.")
	case errors.As(err, &rl):
		secs := max(1, int64((rl.Until.Sub(deps.Now())+time.Second-1)/time.Second))
		w.Header().Set("Retry-After", strconv.FormatInt(secs, 10))
		writeJSON(w, http.StatusTooManyRequests, struct {
			errorBody
			RetryAt time.Time `json:"retry_at"`
		}{errorBody{Error: "provider_rate_limited", Message: "Brewfather's rate limit is reached; try again later."}, rl.Until.UTC()})
	default:
		writeBeerError(w, r, err)
	}
}
