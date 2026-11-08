package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 12, 30, 45, 123_000_000, time.UTC)

// migratedDB returns a database at version 1 with one settings row, and its data folder.
func migratedDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('title', 'Home Taps')"); err != nil {
		t.Fatal(err)
	}
	return db, dir
}

func TestBackup(t *testing.T) {
	db, dir := migratedDB(t)
	path, err := Backup(db, dir, 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, BackupDir, "beermenu-20261108T123045.123Z-v1.db"); path != want {
		t.Fatalf("path = %s, want %s", path, want)
	}
	if !backupName.MatchString(filepath.Base(path)) {
		t.Fatalf("name %s does not match the backup pattern", filepath.Base(path))
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("backup mode = %o, want 600", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Errorf("backups folder mode = %o, want 700", di.Mode().Perm())
	}

	copyDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var value string
	if err := copyDB.QueryRow("SELECT value FROM settings WHERE key = 'title'").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "Home Taps" {
		t.Fatalf("backup title = %q, want Home Taps", value)
	}
	if v := mustVersion(t, copyDB); v != 1 {
		t.Fatalf("backup version = %d, want 1", v)
	}
}

func TestBackupNamesUniqueWithinSecond(t *testing.T) {
	db, dir := migratedDB(t)
	a, err := Backup(db, dir, 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Backup(db, dir, 1, t0.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("both backups named %s", a)
	}
	entries, _ := os.ReadDir(filepath.Dir(a))
	if len(entries) != 2 {
		t.Fatalf("backups folder holds %d files, want 2", len(entries))
	}
}

func TestPruneBackupsKeepsNewestFive(t *testing.T) {
	db, dir := migratedDB(t)
	bdir := filepath.Join(dir, BackupDir)
	if err := os.Mkdir(bdir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := []string{
		"beermenu-20260101T000000.000Z-v1.db",
		"beermenu-20260201T000000.000Z-v1.db",
		"beermenu-20260301T000000.000Z-v1.db",
		"beermenu-20260401T000000.000Z-v1.db",
		"beermenu-20260501T000000.000Z-v1.db",
	}
	others := []string{"notes.txt", "beermenu-manual.db"}
	for _, name := range append(slices.Clone(old), others...) {
		if err := os.WriteFile(filepath.Join(bdir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	newest, err := Backup(db, dir, 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := PruneBackups(bdir, 5); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(bdir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := append(slices.Clone(old[1:]), filepath.Base(newest))
	want = append(want, others...)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("backups folder = %v, want %v", got, want)
	}
}

func TestBackupUnwritableFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	db, dir := migratedDB(t)
	bdir := filepath.Join(dir, BackupDir)
	if err := os.Mkdir(bdir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(bdir, 0o700) })
	if _, err := Backup(db, dir, 1, t0); err == nil {
		t.Fatal("want error for unwritable backups folder")
	}
	entries, _ := os.ReadDir(bdir)
	if len(entries) != 0 {
		t.Fatalf("files left in backups folder: %v", entries)
	}
}
