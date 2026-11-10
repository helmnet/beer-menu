// Package settings holds all SQL on the settings table: the generic key/value Get and Set, and
// the display settings (title, TV theme, admin units, TV page interval) built on them.
package settings

import (
	"database/sql"
	"errors"
	"time"
)

// Querier is what Get and Set need. Both *sql.DB and *sql.Tx satisfy it, so several keys can be
// written in one transaction.
type Querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// Get returns the value of settings key and whether it exists. A missing key is ("", false, nil).
func Get(q Querier, key string) (string, bool, error) {
	var v string
	err := q.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// Set stores value under key at now, replacing any earlier value. The caller validates value;
// the table does not.
func Set(q Querier, key, value string, now time.Time) error {
	_, err := q.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now.Unix())
	return err
}
