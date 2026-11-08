package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenUsesWAL(t *testing.T) {
	db := openTemp(t)
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (err %v), want 1", fk, err)
	}
}

func TestOpenSetsFileModes(t *testing.T) {
	dir := t.TempDir()
	first, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	path := filepath.Join(dir, DBFile)
	// Loosen the modes as a database copied in from elsewhere might have them.
	if _, err := first.Exec("CREATE TABLE t (x INTEGER); INSERT INTO t VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	second, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %o, want 600", filepath.Base(p), fi.Mode().Perm())
		}
	}
}

func TestCheckWritable(t *testing.T) {
	dir := t.TempDir()
	if err := CheckWritable(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("probe file left behind: %v", entries)
	}
}

func TestCheckWritableReadOnlyFolder(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	err := CheckWritable(dir)
	if err == nil {
		t.Fatal("want error for read-only folder")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("error %q does not name the folder %s", err, dir)
	}
}
