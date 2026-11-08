package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

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
