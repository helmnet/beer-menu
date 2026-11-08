package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/store"
)

// TestRestore walks the README restore steps on files: back up, change data, replace the
// database with the backup, remove stale WAL files, start a newer release.
func TestRestore(t *testing.T) {
	dir := t.TempDir()
	db, err := startup(context.Background(), config{DataDir: dir}, v1, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('title', 'before')"); err != nil {
		t.Fatal(err)
	}
	backup, err := store.Backup(db, dir, 1, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE settings SET value = 'after' WHERE key = 'title'"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, store.DBFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}

	db, err = startup(context.Background(), config{DataDir: dir}, v2, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var title string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'title'").Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "before" {
		t.Fatalf("title = %q, want before", title)
	}
	if v, err := store.Version(db); err != nil || v != 2 {
		t.Fatalf("version = %d (err %v), want 2", v, err)
	}
}
