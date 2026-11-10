package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/importer/importertest"
	"github.com/helmnet/beer-menu/api/internal/secrets"
	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// testClock is a movable clock for Deps.Now and the limiter.
type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time          { return c.t }
func (c *testClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// newTestDeps returns Deps on a migrated database in a temporary folder, with a movable
// clock, a limiter on that clock, CookieSecure true as in production, and an import service
// with a fresh random encryption key and a fake provider on that clock. Tests that program the
// provider set deps.Importer.Factory to their own importertest.Fake. Cleanup waits for
// background syncs.
func newTestDeps(t *testing.T) (Deps, *testClock) {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(db, ms); err != nil {
		t.Fatal(err)
	}
	clock := &testClock{t: t0}
	key := make([]byte, 32)
	rand.Read(key)
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	imp := importer.New(db, box, importertest.New().Factory(), clock.Now)
	// Background syncs end before the database closes (cleanups run last-in, first-out).
	t.Cleanup(imp.Wait)
	return Deps{DB: db, Version: "test", CookieSecure: true, Now: clock.Now, Limiter: auth.NewLimiter(clock.Now), Importer: imp}, clock
}

// testPassword returns a random password so no fixed password is in the repo.
func testPassword() string { return rand.Text()[:20] }

// insertUser adds a user row with a placeholder hash and returns its ID.
func insertUser(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO users (username, password_hash, created_at) VALUES (?, 'x', ?)", username, t0.Unix())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// insertToken stores a token row for a fresh random secret and returns the secret and ID.
func insertToken(t *testing.T, db *sql.DB, scope auth.Scope, revoked bool) (string, int64) {
	t.Helper()
	b := make([]byte, 32)
	rand.Read(b)
	secret := "bmt_" + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(secret))
	var revokedAt any
	if revoked {
		revokedAt = t0.Unix()
	}
	res, err := db.Exec("INSERT INTO api_tokens (name, token_hash, scope, created_at, revoked_at) VALUES ('t', ?, ?, ?, ?)", sum[:], scope, t0.Unix(), revokedAt)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return secret, id
}

// do sends one request to h and returns the recorder. body may be empty.
func do(h http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie returns the bm_session cookie set by rec, or nil.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

// wantError fails unless rec has status code and an Error body with errCode.
func wantError(t *testing.T, rec *httptest.ResponseRecorder, code int, errCode string) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"error":"`+errCode+`"`) {
		t.Fatalf("body = %s, want error %q", rec.Body, errCode)
	}
}
