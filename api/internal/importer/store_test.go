package importer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/secrets"
	"github.com/helmnet/beer-menu/api/internal/settings"
	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)

// openDB returns a migrated database in a temporary folder.
func openDB(t *testing.T) *sql.DB {
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
	return db
}

// newBox returns a Box on a fresh random key.
func newBox(t *testing.T) *secrets.Box {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	b, err := secrets.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestInterval(t *testing.T) {
	db := openDB(t)
	if v, err := loadInterval(db); err != nil || v != "off" {
		t.Fatalf("missing interval = %q (err %v), want off", v, err)
	}
	for _, v := range []string{"15m", "1h", "6h", "24h", "off"} {
		if err := saveInterval(db, v, t0); err != nil {
			t.Fatalf("save %q: %v", v, err)
		}
		if got, err := loadInterval(db); err != nil || got != v {
			t.Fatalf("interval = %q (err %v), want %q", got, err, v)
		}
	}
	if err := saveInterval(db, "1h", t0); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "2h", "OFF", "1h ", "60m"} {
		if err := saveInterval(db, bad, t0); !errors.Is(err, ErrInvalidInterval) {
			t.Fatalf("save %q = %v, want ErrInvalidInterval", bad, err)
		}
	}
	if got, _ := loadInterval(db); got != "1h" {
		t.Fatalf("invalid save changed the interval to %q", got)
	}
	if err := settings.Set(db, intervalKey, "garbage", t0); err != nil {
		t.Fatal(err)
	}
	if got, err := loadInterval(db); err != nil || got != "off" {
		t.Fatalf("garbage stored interval = %q (err %v), want off", got, err)
	}
	for v, want := range map[string]time.Duration{"off": 0, "15m": 15 * time.Minute, "1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour} {
		if d, err := ParseInterval(v); err != nil || d != want {
			t.Fatalf("ParseInterval(%q) = %v (err %v), want %v", v, d, err, want)
		}
	}
}

func TestConnectionStore(t *testing.T) {
	db := openDB(t)
	if _, _, ok, err := loadConnection(db); err != nil || ok {
		t.Fatalf("empty load ok=%v err=%v", ok, err)
	}
	if err := saveConnection(db, "user-1", "v1:sealed-1", t0); err != nil {
		t.Fatal(err)
	}
	if err := saveConnection(db, "user-2", "v1:sealed-2", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	user, sealed, ok, err := loadConnection(db)
	if err != nil || !ok || user != "user-2" || sealed != "v1:sealed-2" {
		t.Fatalf("load = %q %q %v (err %v)", user, sealed, ok, err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM provider_connections").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows = %d (err %v), want 1", n, err)
	}
	if err := deleteConnection(db); err != nil {
		t.Fatal(err)
	}
	if err := deleteConnection(db); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if _, _, ok, _ := loadConnection(db); ok {
		t.Fatal("connection still there after delete")
	}
}

func TestCleanCredentials(t *testing.T) {
	user, key, err := cleanCredentials("  user \n", "\tfake-key ")
	if err != nil || user != "user" || key != "fake-key" {
		t.Fatalf("clean = %q %q (err %v)", user, key, err)
	}
	secret := "fake-key-" + rand.Text()
	cases := map[string][2]string{
		"empty user":    {"", secret},
		"blank key":     {"u", "   "},
		"long user":     {strings.Repeat("u", 201), secret},
		"long key":      {"u", strings.Repeat("k", 501)},
		"control user":  {"u\x00" + secret, secret},
		"control key":   {"u", secret + "\x1b"},
		"inner newline": {"u", secret + "\n" + secret},
	}
	for name, c := range cases {
		_, _, err := cleanCredentials(c[0], c[1])
		var fe beers.FieldErrors
		if !errors.As(err, &fe) || len(fe) != 1 {
			t.Errorf("%s: err = %v, want one field error", name, err)
			continue
		}
		raw, _ := json.Marshal(fe)
		if strings.Contains(string(raw), secret) || strings.Contains(err.Error(), secret) {
			t.Errorf("%s: error %s leaks the input", name, raw)
		}
	}
	if _, _, err := cleanCredentials(strings.Repeat("ü", 200), strings.Repeat("ü", 500)); err != nil {
		t.Fatalf("limits are in characters, not bytes: %v", err)
	}
}

func TestSyncRunStore(t *testing.T) {
	db := openDB(t)
	if run, err := loadRun(db); err != nil || run != nil {
		t.Fatalf("no run = %+v (err %v)", run, err)
	}
	if err := startRun(db, t0); err != nil {
		t.Fatal(err)
	}
	run, err := loadRun(db)
	if err != nil || run == nil || !run.StartedAt.Equal(t0) || run.FinishedAt != nil || run.OK != nil || run.Message != nil {
		t.Fatalf("running = %+v (err %v)", run, err)
	}
	t1 := t0.Add(3 * time.Second)
	if err := finishRun(db, t1, true, "Updated 4 beers, imported 1.", 4, 1, 2); err != nil {
		t.Fatal(err)
	}
	run, err = loadRun(db)
	if err != nil || !run.FinishedAt.Equal(t1) || !*run.OK || *run.Message != "Updated 4 beers, imported 1." ||
		run.Updated != 4 || run.Imported != 1 || run.NotFound != 2 {
		t.Fatalf("finished = %+v (err %v)", run, err)
	}
	t2 := t0.Add(time.Hour)
	if err := startRun(db, t2); err != nil {
		t.Fatal(err)
	}
	run, _ = loadRun(db)
	if !run.StartedAt.Equal(t2) || run.FinishedAt != nil || run.OK != nil || run.Message != nil || run.Updated != 0 || run.Imported != 0 || run.NotFound != 0 {
		t.Fatalf("restarted run kept old results: %+v", run)
	}
	if err := finishRun(db, t2, false, strings.Repeat("x", 600), 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if run, _ := loadRun(db); *run.OK || len(*run.Message) != 500 {
		t.Fatalf("failed run = ok %v, message length %d", *run.OK, len(*run.Message))
	}
}

func TestStatus(t *testing.T) {
	db := openDB(t)
	clock := t0
	box := newBox(t)
	s := New(db, box, nil, func() time.Time { return clock })
	ctx := context.Background()

	st, err := s.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Configured || st.Connected || st.ReconnectNeeded || st.Interval != "off" || st.SyncRunning || st.RetryAt != nil || st.LastSync != nil {
		t.Fatalf("fresh status = %+v", st)
	}
	if _, err := s.provider(ctx); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("provider with no connection = %v, want ErrNotConnected", err)
	}

	user, key := "fake-user-"+rand.Text(), "fake-key-"+rand.Text()
	sealed, err := box.Seal(key, sealPurpose)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveConnection(db, user, sealed, t0); err != nil {
		t.Fatal(err)
	}
	if err := saveInterval(db, "6h", t0); err != nil {
		t.Fatal(err)
	}
	var gotUser, gotKey string
	s.Factory = func(u, k string) Provider { gotUser, gotKey = u, k; return nil }
	if _, err := s.provider(ctx); err != nil || gotUser != user || gotKey != key {
		t.Fatalf("provider = %v; factory got the wrong credentials", err)
	}
	st, err = s.Status(ctx)
	if err != nil || !st.Connected || st.ReconnectNeeded || st.Interval != "6h" {
		t.Fatalf("connected status = %+v (err %v)", st, err)
	}

	other := New(db, newBox(t), s.Factory, s.Now)
	st, err = other.Status(ctx)
	if err != nil || st.Connected || !st.ReconnectNeeded || !st.Configured {
		t.Fatalf("status under another key = %+v (err %v)", st, err)
	}
	_, err = other.provider(ctx)
	if !errors.Is(err, ErrReconnectNeeded) {
		t.Fatalf("provider under another key = %v, want ErrReconnectNeeded", err)
	}
	raw, _ := json.Marshal(st)
	for _, secret := range []string{user, key, sealed} {
		if strings.Contains(string(raw), secret) || strings.Contains(err.Error(), secret) {
			t.Fatalf("status %s or error %v leaks a credential", raw, err)
		}
	}

	unconfigured := New(db, nil, s.Factory, s.Now)
	if st, err := unconfigured.Status(ctx); err != nil || st.Configured || st.Connected || st.ReconnectNeeded {
		t.Fatalf("unconfigured status = %+v (err %v)", st, err)
	}
	if _, err := unconfigured.provider(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("unconfigured provider = %v, want ErrNotConfigured", err)
	}
}

func TestStatusInterruptedRun(t *testing.T) {
	db := openDB(t)
	if err := startRun(db, t0); err != nil {
		t.Fatal(err)
	}
	s := New(db, newBox(t), nil, func() time.Time { return t0.Add(time.Minute) })
	st, err := s.Status(context.Background())
	if err != nil || st.LastSync == nil || st.LastSync.OK == nil || *st.LastSync.OK || st.LastSync.Message == nil ||
		*st.LastSync.Message != "Sync was interrupted" || st.LastSync.FinishedAt != nil || st.SyncRunning {
		t.Fatalf("interrupted run status = %+v (err %v)", st.LastSync, err)
	}
	raw, _ := json.Marshal(st)
	want := `{"configured":true,"connected":false,"reconnect_needed":false,"interval":"off","sync_running":false,"retry_at":null,` +
		`"last_sync":{"started_at":"2026-11-09T18:00:00Z","finished_at":null,"ok":false,"message":"Sync was interrupted","updated":0,"imported":0,"not_found":0}}`
	if string(raw) != want {
		t.Fatalf("status JSON = %s\nwant          %s", raw, want)
	}

	// A run this process is doing is running, not interrupted.
	s.syncing.Store(true)
	if st, _ := s.Status(context.Background()); !st.SyncRunning || st.LastSync.OK != nil || st.LastSync.Message != nil {
		t.Fatalf("running status = %+v %+v", st, st.LastSync)
	}
}

func TestRateGate(t *testing.T) {
	db := openDB(t)
	clock := t0
	s := New(db, newBox(t), nil, func() time.Time { return clock })
	if err := s.gate(); err != nil {
		t.Fatalf("open gate = %v", err)
	}
	until := t0.Add(2 * time.Minute)
	err := s.observe(&RateLimitedError{Until: until})
	var rl *RateLimitedError
	if !errors.As(err, &rl) || !rl.Until.Equal(until) {
		t.Fatalf("observe = %v", err)
	}
	if err := s.observe(ErrUnreachable); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("observe passes other errors through: %v", err)
	}
	if err := s.gate(); !errors.As(err, &rl) || !rl.Until.Equal(until) {
		t.Fatalf("closed gate = %v", err)
	}
	if st, _ := s.Status(context.Background()); st.RetryAt == nil || !st.RetryAt.Equal(until) {
		t.Fatalf("status retry_at = %v", st.RetryAt)
	}
	clock = until
	if err := s.gate(); err != nil {
		t.Fatalf("gate after Until = %v", err)
	}
	if st, _ := s.Status(context.Background()); st.RetryAt != nil {
		t.Fatalf("status retry_at after Until = %v", st.RetryAt)
	}
}
