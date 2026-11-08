package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/store"
)

func TestHealth(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(db, ms); err != nil {
		t.Fatal(err)
	}
	mux := NewMux(Deps{DB: db, Version: "1.2.3"})

	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
		return rec
	}
	check := func(rec *httptest.ResponseRecorder, code int, body string) {
		t.Helper()
		if rec.Code != code {
			t.Errorf("status = %d, want %d", rec.Code, code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if rec.Body.String() != body {
			t.Errorf("body = %s, want %s", rec.Body.String(), body)
		}
	}

	check(get(), http.StatusOK, `{"status":"ok","version":"1.2.3","database":"ok"}`)
	db.Close()
	check(get(), http.StatusServiceUnavailable, `{"status":"error","version":"1.2.3","database":"unreachable"}`)
}

func TestHealthRejectsOtherMethods(t *testing.T) {
	mux := NewMux(Deps{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}
