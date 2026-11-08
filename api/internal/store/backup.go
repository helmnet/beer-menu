package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// BackupDir is the backups folder name inside the data folder.
const BackupDir = "backups"

var backupName = regexp.MustCompile(`^beermenu-\d{8}T\d{6}\.\d{3}Z-v\d+\.db$`)

// Backup writes a consistent copy of db to <dataDir>/backups/beermenu-<UTC time>-v<from>.db
// with VACUUM INTO and returns its path. It creates the folder with mode 0700 if missing.
// The copy is written under a temporary name, set to 0600, then renamed, so a file with
// the final name is always complete. On error no file is left behind.
func Backup(db *sql.DB, dataDir string, fromVersion int, now time.Time) (string, error) {
	dir := filepath.Join(dataDir, BackupDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create backups folder %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*.db")
	if err != nil {
		return "", fmt.Errorf("create backup in %s: %w", dir, err)
	}
	tmp.Close()
	tmpPath := tmp.Name()
	final := filepath.Join(dir, fmt.Sprintf("beermenu-%s-v%d.db", now.UTC().Format("20060102T150405.000Z"), fromVersion))
	if err := writeBackup(db, tmpPath, final); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("backup to %s: %w", final, err)
	}
	return final, nil
}

func writeBackup(db *sql.DB, tmpPath, final string) error {
	// VACUUM INTO accepts an existing empty file, which CreateTemp gave us.
	if _, err := db.Exec("VACUUM INTO ?", tmpPath); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, final)
}

// PruneBackups deletes all but the newest keep files in dir whose names match the backup
// pattern. Name order is time order because the timestamp is UTC and fixed width. Files
// that do not match the pattern are never touched.
func PruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir) // sorted by name
	if err != nil {
		return fmt.Errorf("list backups in %s: %w", dir, err)
	}
	var backups []string
	for _, e := range entries {
		if e.Type().IsRegular() && backupName.MatchString(e.Name()) {
			backups = append(backups, e.Name())
		}
	}
	for len(backups) > keep {
		if err := os.Remove(filepath.Join(dir, backups[0])); err != nil {
			return fmt.Errorf("remove old backup: %w", err)
		}
		backups = backups[1:]
	}
	return nil
}
