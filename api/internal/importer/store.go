package importer

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// providerName is the provider_connections and sync_runs key, and the source of imported beers.
const providerName = "brewfather"

// intervalKey is the settings key of the scheduled sync interval.
const intervalKey = "brewfather.sync_interval"

// intervals are the allowed sync intervals; "off" disables the schedule.
var intervals = map[string]time.Duration{"off": 0, "15m": 15 * time.Minute, "1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour}

// ErrInvalidInterval means an interval other than off, 15m, 1h, 6h or 24h.
var ErrInvalidInterval = errors.New("interval must be off, 15m, 1h, 6h or 24h")

// maxRunMessage caps a stored sync message, in characters.
const maxRunMessage = 500

// GetSetting returns the value of settings key and whether it exists.
func GetSetting(db *sql.DB, key string) (string, bool, error) {
	var v string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// SetSetting stores value under key at now, replacing any earlier value. The caller validates
// value; the table does not.
func SetSetting(db *sql.DB, key, value string, now time.Time) error {
	_, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now.Unix())
	return err
}

// ParseInterval returns the duration of a sync interval, 0 for off, or ErrInvalidInterval.
func ParseInterval(s string) (time.Duration, error) {
	d, ok := intervals[s]
	if !ok {
		return 0, ErrInvalidInterval
	}
	return d, nil
}

// loadInterval returns the stored sync interval; missing or invalid reads as off.
func loadInterval(db *sql.DB) (string, error) {
	v, _, err := GetSetting(db, intervalKey)
	if err != nil {
		return "", err
	}
	if _, err := ParseInterval(v); err != nil {
		return "off", nil
	}
	return v, nil
}

// saveInterval stores a valid sync interval, or returns ErrInvalidInterval writing nothing.
func saveInterval(db *sql.DB, v string, now time.Time) error {
	if _, err := ParseInterval(v); err != nil {
		return err
	}
	return SetSetting(db, intervalKey, v, now)
}

// cleanCredentials trims userID and apiKey and checks their limits: 1 to 200 and 1 to 500
// characters, no control characters. It returns beers.FieldErrors keyed by JSON name, whose
// messages never echo the values.
func cleanCredentials(userID, apiKey string) (string, string, error) {
	fe := beers.FieldErrors{}
	check := func(field, s string, maxLen int) string {
		s = strings.TrimSpace(s)
		switch {
		case s == "" || utf8.RuneCountInString(s) > maxLen:
			fe[field] = "Must be 1 to " + strconv.Itoa(maxLen) + " characters."
		case strings.ContainsFunc(s, unicode.IsControl):
			fe[field] = "Must not contain control characters."
		}
		return s
	}
	userID = check("user_id", userID, 200)
	apiKey = check("api_key", apiKey, 500)
	if len(fe) > 0 {
		return "", "", fe
	}
	return userID, apiKey, nil
}

// saveConnection stores the account, replacing any earlier one.
func saveConnection(db *sql.DB, userID, sealedKey string, now time.Time) error {
	_, err := db.Exec(`INSERT INTO provider_connections (provider, user_id, api_key_sealed, connected_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (provider) DO UPDATE SET user_id = excluded.user_id, api_key_sealed = excluded.api_key_sealed,
		connected_at = excluded.connected_at`, providerName, userID, sealedKey, now.Unix())
	return err
}

// loadConnection returns the stored user ID and sealed key, and whether an account is stored.
func loadConnection(db *sql.DB) (userID, sealedKey string, ok bool, err error) {
	err = db.QueryRow("SELECT user_id, api_key_sealed FROM provider_connections WHERE provider = ?", providerName).Scan(&userID, &sealedKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	return userID, sealedKey, err == nil, err
}

// deleteConnection forgets the account. Deleting none is not an error.
func deleteConnection(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM provider_connections WHERE provider = ?", providerName)
	return err
}

// LastSync is the last sync run as the status reports it.
type LastSync struct {
	StartedAt time.Time `json:"started_at"`
	// FinishedAt, OK and Message are nil while the run is going.
	FinishedAt *time.Time `json:"finished_at"`
	OK         *bool      `json:"ok"`
	Message    *string    `json:"message"`
	Updated    int64      `json:"updated"`
	Imported   int64      `json:"imported"`
	NotFound   int64      `json:"not_found"`
}

// startRun records a run started at now, clearing the previous run's results.
func startRun(db *sql.DB, now time.Time) error {
	_, err := db.Exec(`INSERT INTO sync_runs (provider, started_at) VALUES (?, ?)
		ON CONFLICT (provider) DO UPDATE SET started_at = excluded.started_at, finished_at = NULL, ok = NULL, message = NULL,
		updated = 0, imported = 0, not_found = 0`, providerName, now.Unix())
	return err
}

// finishRun records the end of the current run. message is cut to 500 characters.
func finishRun(db *sql.DB, now time.Time, ok bool, message string, updated, imported, notFound int64) error {
	if utf8.RuneCountInString(message) > maxRunMessage {
		message = string([]rune(message)[:maxRunMessage])
	}
	_, err := db.Exec("UPDATE sync_runs SET finished_at = ?, ok = ?, message = ?, updated = ?, imported = ?, not_found = ? WHERE provider = ?",
		now.Unix(), ok, message, updated, imported, notFound, providerName)
	return err
}

// loadRun returns the last run, or nil if none ever started.
func loadRun(db *sql.DB) (*LastSync, error) {
	var r LastSync
	var started int64
	var finished sql.NullInt64
	var ok sql.NullBool
	var msg sql.NullString
	err := db.QueryRow("SELECT started_at, finished_at, ok, message, updated, imported, not_found FROM sync_runs WHERE provider = ?", providerName).
		Scan(&started, &finished, &ok, &msg, &r.Updated, &r.Imported, &r.NotFound)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.StartedAt = time.Unix(started, 0).UTC()
	if finished.Valid {
		t := time.Unix(finished.Int64, 0).UTC()
		r.FinishedAt = &t
	}
	if ok.Valid {
		r.OK = &ok.Bool
	}
	if msg.Valid {
		r.Message = &msg.String
	}
	return &r, nil
}
