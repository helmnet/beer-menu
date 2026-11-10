package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/store"
)

var (
	v1 = fstest.MapFS{
		"0001_baseline.sql": {Data: []byte("CREATE TABLE settings (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL);")},
	}
	v2 = fstest.MapFS{
		"0001_baseline.sql": v1["0001_baseline.sql"],
		"0002_beers.sql":    {Data: []byte("CREATE TABLE beers (id INTEGER PRIMARY KEY, name TEXT NOT NULL);")},
	}
	v2Broken = fstest.MapFS{
		"0001_baseline.sql": v1["0001_baseline.sql"],
		"0002_broken.sql":   {Data: []byte("CREATE TABLE beers (id INTEGER PRIMARY KEY); INSERT INTO missing VALUES (1);")},
	}
)

func fixedNow() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }

// runStartup runs startup on dataDir and closes the database again.
func runStartup(t *testing.T, dataDir string, fsys fstest.MapFS) error {
	t.Helper()
	db, err := startup(context.Background(), config{DataDir: dataDir}, fsys, fixedNow)
	if err == nil {
		db.Close()
	}
	return err
}

func userVersion(t *testing.T, dataDir string) int {
	t.Helper()
	db, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := store.Version(db)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func backups(t *testing.T, dataDir string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dataDir, store.BackupDir, "beermenu-*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestCheckNotRoot(t *testing.T) {
	cases := []struct {
		uid, gid int
		groups   []int
		wantErr  bool
	}{
		{0, 0, nil, true},
		{0, 1000, nil, true},
		{1000, 0, nil, true},
		{1000, 1000, nil, false},
		{1000, 1000, []int{1000}, false},
		{1000, 1000, []int{0, 1000}, true},
	}
	for _, c := range cases {
		err := checkNotRoot(c.uid, c.gid, c.groups)
		if (err != nil) != c.wantErr {
			t.Errorf("checkNotRoot(%d, %d, %v) = %v, want error %v", c.uid, c.gid, c.groups, err, c.wantErr)
		}
	}
}

func TestStartupBacksUpBeforeMigrating(t *testing.T) {
	dir := t.TempDir()
	if err := runStartup(t, dir, v1); err != nil {
		t.Fatal(err)
	}
	if err := runStartup(t, dir, v2); err != nil {
		t.Fatal(err)
	}
	got := backups(t, dir)
	if len(got) != 1 || filepath.Base(got[0]) != "beermenu-20261108T120000.000Z-v1.db" {
		t.Fatalf("backups = %v, want one named ...-v1.db", got)
	}
	if v := userVersion(t, dir); v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
}

func TestStartupNoPendingMakesNoBackup(t *testing.T) {
	dir := t.TempDir()
	for range 2 {
		if err := runStartup(t, dir, v1); err != nil {
			t.Fatal(err)
		}
	}
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("backups = %v, want none", got)
	}
}

func TestStartupNewDatabaseMigratesWithoutBackup(t *testing.T) {
	dir := t.TempDir()
	if err := runStartup(t, dir, v2); err != nil {
		t.Fatal(err)
	}
	if got := backups(t, dir); len(got) != 0 {
		t.Fatalf("backups = %v, want none", got)
	}
	if v := userVersion(t, dir); v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
}

func TestStartupBackupFailureAppliesNothing(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	dir := t.TempDir()
	if err := runStartup(t, dir, v1); err != nil {
		t.Fatal(err)
	}
	bdir := filepath.Join(dir, store.BackupDir)
	if err := os.Mkdir(bdir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(bdir, 0o700) })
	if err := runStartup(t, dir, v2); err == nil {
		t.Fatal("want error when the backup cannot be written")
	}
	if v := userVersion(t, dir); v != 1 {
		t.Fatalf("version = %d, want 1", v)
	}
}

func TestStartupFailedMigrationKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	if err := runStartup(t, dir, v1); err != nil {
		t.Fatal(err)
	}
	if err := runStartup(t, dir, v2Broken); err == nil {
		t.Fatal("want error from failing migration")
	}
	if v := userVersion(t, dir); v != 1 {
		t.Fatalf("version = %d, want 1", v)
	}
	if got := backups(t, dir); len(got) != 1 {
		t.Fatalf("backups = %v, want the pre-migration backup", got)
	}
}

func TestStartupRefusesNewerDatabase(t *testing.T) {
	dir := t.TempDir()
	if err := runStartup(t, dir, v2); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, store.DBFile)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := runStartup(t, dir, v1); !errors.Is(err, store.ErrNewerDatabase) {
		t.Fatalf("err = %v, want ErrNewerDatabase", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("database file changed")
	}
}

func TestStartupUnwritableDataFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := runStartup(t, dir, v1); err == nil {
		t.Fatal("want error for unwritable data folder")
	}
}

// captureLog sends slog output to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

// startAuth runs startup with the real migrations and then prepareAuth, as run does.
func startAuth(t *testing.T, cfg config) (*sql.DB, error) {
	t.Helper()
	db, err := startup(context.Background(), cfg, store.Migrations, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, prepareAuth(context.Background(), db, cfg, fixedNow)
}

func adminCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	n, err := auth.AdminCount(db)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestStartupCreatesFirstAdminFromEnv(t *testing.T) {
	logs := captureLog(t)
	pw := rand.Text()
	cfg := config{DataDir: t.TempDir(), AdminUsername: "owner", AdminPassword: pw}
	db, err := startAuth(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if adminCount(t, db) != 1 {
		t.Fatal("admin not created")
	}
	if _, err := auth.Authenticate(db, "owner", pw); err != nil {
		t.Fatalf("created admin cannot sign in: %v", err)
	}
	if !strings.Contains(logs.String(), "first admin created") {
		t.Errorf("log lacks 'first admin created': %s", logs)
	}
	if strings.Contains(logs.String(), pw) {
		t.Fatal("log contains the password")
	}
}

func TestStartupIgnoresEnvWhenAdminExists(t *testing.T) {
	logs := captureLog(t)
	dir := t.TempDir()
	first := rand.Text()
	if _, err := startAuth(t, config{DataDir: dir, AdminUsername: "owner", AdminPassword: first}); err != nil {
		t.Fatal(err)
	}
	second := rand.Text()
	db, err := startAuth(t, config{DataDir: dir, AdminUsername: "owner", AdminPassword: second})
	if err != nil {
		t.Fatal(err)
	}
	if adminCount(t, db) != 1 {
		t.Fatal("second admin created")
	}
	if _, err := auth.Authenticate(db, "owner", first); err != nil {
		t.Fatalf("existing password no longer verifies: %v", err)
	}
	if !strings.Contains(logs.String(), "first admin settings ignored, an admin exists") {
		t.Errorf("log lacks the ignored message: %s", logs)
	}
	if strings.Contains(logs.String(), first) || strings.Contains(logs.String(), second) {
		t.Fatal("log contains a password")
	}
}

func TestStartupRefusesBadAdminEnv(t *testing.T) {
	logs := captureLog(t)
	pw := rand.Text()
	cases := map[string]config{
		"username only":  {AdminUsername: "owner"},
		"password only":  {AdminPassword: pw},
		"short password": {AdminUsername: "owner", AdminPassword: pw[:11]},
		"bad username":   {AdminUsername: "bad name", AdminPassword: pw},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			cfg.DataDir = t.TempDir()
			db, err := startAuth(t, cfg)
			if err == nil {
				t.Fatal("want error")
			}
			if strings.Contains(err.Error(), pw[:11]) {
				t.Fatal("error contains the password")
			}
			if adminCount(t, db) != 0 {
				t.Fatal("admin created")
			}
		})
	}
	if strings.Contains(logs.String(), pw[:11]) {
		t.Fatal("log contains the password")
	}
}

func TestStartupWithoutAdminEnvCreatesNothing(t *testing.T) {
	db, err := startAuth(t, config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if adminCount(t, db) != 0 {
		t.Fatal("admin created without env vars")
	}
}

func TestLoadSecretKey(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	fileKey := make([]byte, 32)
	rand.Read(fileKey)
	enc, fileEnc := base64.StdEncoding.EncodeToString(key), base64.StdEncoding.EncodeToString(fileKey)
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(fileEnc+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := func(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }

	got, err := loadSecretKey(env(map[string]string{"BEER_MENU_SECRET_KEY_FILE": path, "BEER_MENU_SECRET_KEY": enc}), os.ReadFile)
	if err != nil || string(got) != string(fileKey) {
		t.Fatalf("file and env set = %x (err %v), want the file's key", got, err)
	}
	got, err = loadSecretKey(env(map[string]string{"BEER_MENU_SECRET_KEY": " " + enc + "\n"}), os.ReadFile)
	if err != nil || string(got) != string(key) {
		t.Fatalf("env only = %x (err %v)", got, err)
	}
	got, err = loadSecretKey(env(nil), os.ReadFile)
	if err != nil || got != nil {
		t.Fatalf("neither set = %x (err %v), want nil", got, err)
	}

	short := base64.StdEncoding.EncodeToString(key[:16])
	shortPath := filepath.Join(t.TempDir(), "short")
	if err := os.WriteFile(shortPath, []byte(short), 0o600); err != nil {
		t.Fatal(err)
	}
	junk := "not-base64-" + rand.Text()
	cases := []struct {
		name, variable string
		vars           map[string]string
		secret         string
	}{
		{"missing file", "BEER_MENU_SECRET_KEY_FILE", map[string]string{"BEER_MENU_SECRET_KEY_FILE": filepath.Join(t.TempDir(), "nope"), "BEER_MENU_SECRET_KEY": enc}, enc},
		{"short file", "BEER_MENU_SECRET_KEY_FILE", map[string]string{"BEER_MENU_SECRET_KEY_FILE": shortPath}, short},
		{"junk env", "BEER_MENU_SECRET_KEY", map[string]string{"BEER_MENU_SECRET_KEY": junk}, junk},
		{"short env", "BEER_MENU_SECRET_KEY", map[string]string{"BEER_MENU_SECRET_KEY": short}, short},
	}
	for _, c := range cases {
		got, err := loadSecretKey(env(c.vars), os.ReadFile)
		if err == nil || got != nil {
			t.Errorf("%s: = %x, %v; want an error", c.name, got, err)
			continue
		}
		if !strings.Contains(err.Error(), c.variable) {
			t.Errorf("%s: error %q does not name %s", c.name, err, c.variable)
		}
		if strings.Contains(err.Error(), c.secret) {
			t.Errorf("%s: error %q contains the value", c.name, err)
		}
	}
}
