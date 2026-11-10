package importer_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/importer/importertest"
)

var t0 = time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)

// fakeClock is a movable clock for Service.Now.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

// newService returns a Service on a fresh database and key with fake as its provider.
func newService(t *testing.T) (*importer.Service, *importertest.Fake, *fakeClock) {
	t.Helper()
	fake := importertest.New()
	clock := &fakeClock{t: t0}
	return importer.New(importer.OpenTestDB(t), importer.NewTestBox(t), fake.Factory(), clock.Now), fake, clock
}

// fakeCreds returns an obviously fake user ID and key.
func fakeCreds() (string, string) { return "fake-user-" + rand.Text(), "fake-key-" + rand.Text() }

// mustConnect connects s with fresh fake credentials and returns them.
func mustConnect(t *testing.T, s *importer.Service) (string, string) {
	t.Helper()
	user, key := fakeCreds()
	if err := s.Connect(context.Background(), user, key); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return user, key
}

// connectionRows counts stored connections.
func connectionRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM provider_connections").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConnect(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	user, key := mustConnect(t, s)
	if fake.Calls("Check") != 1 || connectionRows(t, s.DB) != 1 {
		t.Fatalf("Check calls %d, rows %d; want 1, 1", fake.Calls("Check"), connectionRows(t, s.DB))
	}
	if u, k := fake.Credentials(); u != user || k != key {
		t.Fatal("factory did not get the trimmed credentials")
	}
	if st, err := s.Status(ctx); err != nil || !st.Connected {
		t.Fatalf("status = %+v (err %v)", st, err)
	}

	// Failures save nothing and keep the existing connection.
	for _, fail := range []error{importer.ErrCredentialsRefused, importer.ErrNoBatchAccess, importer.ErrUnreachable,
		&importer.RateLimitedError{Until: t0.Add(time.Minute)}} {
		fake.SetErr("Check", fail)
		u2, k2 := fakeCreds()
		if err := s.Connect(ctx, u2, k2); !errors.Is(err, fail) {
			t.Fatalf("connect with %v = %v", fail, err)
		}
		fake.SetErr("Check", nil)
		if _, err := s.Status(ctx); err != nil {
			t.Fatal(err)
		}
		if u, _ := fake.Credentials(); u != u2 {
			t.Fatalf("%v: Check did not run on the new credentials", fail)
		}
	}
	// The rate limit above closed the gate, so a connect now makes no call at all.
	calls := fake.Calls("Check")
	var rl *importer.RateLimitedError
	if err := s.Connect(ctx, user, key); !errors.As(err, &rl) || fake.Calls("Check") != calls {
		t.Fatalf("connect behind a closed gate = %v, calls %d -> %d", err, calls, fake.Calls("Check"))
	}
	// The stored account is still the first one: the provider gets its credentials.
	s2 := importer.New(s.DB, s.Box, fake.Factory(), s.Now)
	if _, err := s2.Batches(ctx, true); err != nil {
		t.Fatal(err)
	}
	if u, k := fake.Credentials(); u != user || k != key {
		t.Fatal("a failed connect replaced the stored account")
	}

	// A second connect replaces the first.
	u3, k3 := mustConnect(t, s2)
	if _, err := s2.Batches(ctx, true); err != nil {
		t.Fatal(err)
	}
	if u, k := fake.Credentials(); u != u3 || k != k3 || connectionRows(t, s.DB) != 1 {
		t.Fatal("second connect did not replace the first")
	}
}

func TestConnectInvalidInput(t *testing.T) {
	s, fake, _ := newService(t)
	_, key := fakeCreds()
	err := s.Connect(context.Background(), "", key)
	if err == nil || fake.TotalCalls() != 0 || connectionRows(t, s.DB) != 0 {
		t.Fatalf("connect with empty user = %v, calls %d", err, fake.TotalCalls())
	}
}

func TestConnectNotConfigured(t *testing.T) {
	s, fake, _ := newService(t)
	s.Box = nil
	user, key := fakeCreds()
	if err := s.Connect(context.Background(), user, key); !errors.Is(err, importer.ErrNotConfigured) || fake.TotalCalls() != 0 {
		t.Fatalf("connect without a key = %v, calls %d", err, fake.TotalCalls())
	}
}

func TestDisconnect(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	if err := importer.SetSetting(s.DB, "brewfather.sync_interval", "1h", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO beers (name, source, source_ref, created_at, updated_at) VALUES ('B', 'brewfather', 'b1', 1, 1)"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Disconnect(ctx); err != nil {
			t.Fatalf("disconnect: %v", err)
		}
	}
	st, err := s.Status(ctx)
	if err != nil || st.Connected || st.ReconnectNeeded || st.Interval != "1h" || connectionRows(t, s.DB) != 0 {
		t.Fatalf("status after disconnect = %+v (err %v)", st, err)
	}
	var n int
	if err := s.DB.QueryRow("SELECT count(*) FROM beers").Scan(&n); err != nil || n != 1 {
		t.Fatalf("beers after disconnect = %d (err %v), want 1", n, err)
	}
	if _, err := s.Batches(ctx, false); !errors.Is(err, importer.ErrNotConnected) {
		t.Fatalf("batches after disconnect = %v", err)
	}
}

func TestReconnectAfterKeyChange(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	other := importer.New(s.DB, importer.NewTestBox(t), fake.Factory(), s.Now)
	if st, err := other.Status(ctx); err != nil || !st.ReconnectNeeded || st.Connected {
		t.Fatalf("status under a new key = %+v (err %v)", st, err)
	}
	mustConnect(t, other)
	if st, err := other.Status(ctx); err != nil || st.ReconnectNeeded || !st.Connected {
		t.Fatalf("status after reconnect = %+v (err %v)", st, err)
	}
}
