// Package store owns the SQLite database file: opening it, checking the data folder,
// running migrations and taking backups. Only the API process touches the database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DBFile is the database file name inside the data folder.
const DBFile = "beermenu.db"

// temp_store=MEMORY because the scratch image has no /tmp for SQLite temp files.
// _txlock=immediate so a transaction that reads before it writes waits on busy_timeout for the
// write lock instead of failing with SQLITE_BUSY at its first write (feature 4 research R3).
const pragmas = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)" +
	"&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_txlock=immediate"

// Open opens (creating if needed) <dataDir>/beermenu.db in WAL mode with the connection
// pragmas every connection needs, then sets the database file and any existing -wal and
// -shm files to mode 0600. The chmod covers a database restored from elsewhere; files the
// process creates are already 0600 through the umask set in main.
func Open(dataDir string) (*sql.DB, error) {
	path := filepath.Join(dataDir, DBFile)
	db, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	// sql.Open is lazy; Ping creates the file so its mode can be set below.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			db.Close()
			return nil, fmt.Errorf("set mode of %s: %w", p, err)
		}
	}
	return db, nil
}

// CheckWritable reports whether the process can create files in dir by creating and
// removing a probe file. The returned error names the folder.
func CheckWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".write-probe-*")
	if err != nil {
		return fmt.Errorf("data folder %s is not writable: %w", dir, err)
	}
	f.Close()
	if err := os.Remove(f.Name()); err != nil {
		return fmt.Errorf("data folder %s: remove probe file: %w", dir, err)
	}
	return nil
}
