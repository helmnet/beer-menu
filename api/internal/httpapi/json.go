package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// maxBody bounds every JSON request body (research R12).
const maxBody = 8 << 10

// sessionCookieName is the browser session cookie.
const sessionCookieName = "bm_session"

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// decodeJSON reads r's body, at most 8 KiB, as exactly one JSON value into dst, refusing
// unknown fields and trailing data. On failure it writes 400 invalid_request and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
		err = errors.New("trailing data")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be one JSON object with only the documented fields, at most 8 KiB.")
		return false
	}
	return true
}

// writeJSON writes v as a JSON response with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.Error("encode response", "err", err)
		status, body = http.StatusInternalServerError, []byte(`{"error":"internal","message":"Internal error."}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

// writeError writes the contract's Error body {"error": code, "message": message} with status.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: code, Message: message})
}

// writeInternal logs err (which must hold no secrets) and answers 500.
func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Internal error.")
}

// setSessionCookie sends session id as the HttpOnly, SameSite=Strict bm_session cookie for
// the whole site, expiring at expires, with Secure when secure is set.
func setSessionCookie(w http.ResponseWriter, id string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: id, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secure,
	})
}

// clearSessionCookie tells the browser to drop bm_session.
func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secure,
	})
}
