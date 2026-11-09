package kegs

import (
	"database/sql"
	"errors"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// Tap is a tap as the API returns it: a fixed slot at Position 1..N with its current keg, or
// Keg nil (null) when it has none.
type Tap struct {
	ID        int64     `json:"id"`
	Position  int64     `json:"position"`
	Label     string    `json:"label"`
	Enabled   bool      `json:"enabled"`
	Keg       *Keg      `json:"keg"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TapInput is the body of a tap create request. A nil Label names the tap "Tap N", N being its
// new position; a nil Enabled means enabled.
type TapInput struct {
	Label   *string `json:"label"`
	Enabled *bool   `json:"enabled"`
}

// TapPatch is the body of a tap PATCH request: an unset field is kept. Neither field may be null.
type TapPatch struct {
	Label   beers.Optional[string] `json:"label"`
	Enabled beers.Optional[bool]   `json:"enabled"`
}

const tapColumns = "id, position, label, enabled, created_at, updated_at"

// scanTap reads one row selected with tapColumns into a Tap with no keg.
func scanTap(row interface{ Scan(...any) error }) (Tap, error) {
	var tp Tap
	var created, updated int64
	err := row.Scan(&tp.ID, &tp.Position, &tp.Label, &tp.Enabled, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Tap{}, ErrNotFound
	}
	tp.CreatedAt, tp.UpdatedAt = unixTime(created), unixTime(updated)
	return tp, err
}

// ListTaps returns every tap in position order, each with its current keg. The slice is never
// nil, so it marshals as [] (there is always at least one tap).
func ListTaps(db *sql.DB) ([]Tap, error) {
	rows, err := db.Query("SELECT " + tapColumns + " FROM taps ORDER BY position, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Tap{}
	for rows.Next() {
		tp, err := scanTap(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, tp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, attachKegs(db, list)
}

// GetTap returns tap id with its current keg, or ErrNotFound.
func GetTap(db *sql.DB, id int64) (Tap, error) {
	return withKeg(db)(scanTap(db.QueryRow("SELECT "+tapColumns+" FROM taps WHERE id = ?", id)))
}

// CreateTap validates in and adds a tap at the end at now, returning it. It returns FieldErrors,
// writing nothing, for an invalid label.
func CreateTap(db *sql.DB, in TapInput, now time.Time) (Tap, error) {
	var label *string
	if in.Label != nil {
		l, msg := normalizeLabel(*in.Label)
		if msg != "" {
			return Tap{}, beers.FieldErrors{"label": msg}
		}
		label = &l
	}
	enabled := in.Enabled == nil || *in.Enabled
	// One statement, so two concurrent adds cannot take the same position.
	return scanTap(db.QueryRow(`INSERT INTO taps (position, label, enabled, created_at, updated_at)
		SELECT p, COALESCE(?, 'Tap ' || p), ?, ?, ? FROM (SELECT COALESCE(MAX(position), 0) + 1 AS p FROM taps)
		RETURNING `+tapColumns, label, enabled, now.Unix(), now.Unix()))
}

// UpdateTap applies p to tap id at now and returns the tap with its keg. Disabling keeps the
// keg. It returns FieldErrors or beers.ErrEmptyPatch, writing nothing, when p is invalid, and
// ErrNotFound for an unknown id.
func UpdateTap(db *sql.DB, id int64, p TapPatch, now time.Time) (Tap, error) {
	fe := beers.FieldErrors{}
	query := "UPDATE taps SET updated_at = ?"
	args := []any{now.Unix()}
	if p.Label.Set {
		if p.Label.Value == nil {
			fe["label"] = "Label is required."
		} else if l, msg := normalizeLabel(*p.Label.Value); msg != "" {
			fe["label"] = msg
		} else {
			query += ", label = ?"
			args = append(args, l)
		}
	}
	if p.Enabled.Set {
		if p.Enabled.Value == nil {
			fe["enabled"] = "Must be true or false."
		} else {
			query += ", enabled = ?"
			args = append(args, *p.Enabled.Value)
		}
	}
	if len(fe) > 0 {
		return Tap{}, fe
	}
	if !p.Label.Set && !p.Enabled.Set {
		return Tap{}, beers.ErrEmptyPatch
	}
	return withKeg(db)(scanTap(db.QueryRow(query+" WHERE id = ? RETURNING "+tapColumns, append(args, id)...)))
}

// ReorderTaps sets the positions of all taps to 1..N in the order of ids at now, and returns the
// taps in that order. It returns FieldErrors on ids, changing nothing, unless ids lists every
// tap exactly once.
func ReorderTaps(db *sql.DB, ids []int64, now time.Time) ([]Tap, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow("SELECT count(*) FROM taps").Scan(&n); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	ok := len(ids) == n && len(seen) == n
	for i, id := range ids {
		if !ok {
			break
		}
		res, err := tx.Exec("UPDATE taps SET position = ?, updated_at = CASE WHEN position = ? THEN updated_at ELSE ? END WHERE id = ?", i+1, i+1, now.Unix(), id)
		if err != nil {
			return nil, err
		}
		m, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		ok = m == 1
	}
	if !ok {
		return nil, beers.FieldErrors{"ids": "Must list every tap exactly once."}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return ListTaps(db)
}

// DeleteTap removes tap id and closes the gap in positions. Kicked kegs that were on it keep
// their tap label. It returns ErrNotFound for an unknown id, ErrLastTap for the only tap and
// ErrTapHasKeg while a current keg is on it.
func DeleteTap(db *sql.DB, id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var position, taps int64
	var hasKeg bool
	err = tx.QueryRow(`SELECT position, (SELECT count(*) FROM taps),
		EXISTS (SELECT 1 FROM kegs WHERE tap_id = taps.id AND kicked_at IS NULL)
		FROM taps WHERE id = ?`, id).Scan(&position, &taps, &hasKeg)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return err
	case hasKeg:
		return ErrTapHasKeg
	case taps == 1:
		return ErrLastTap
	}
	if _, err := tx.Exec("DELETE FROM taps WHERE id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE taps SET position = position - 1 WHERE position > ?", position); err != nil {
		return err
	}
	return tx.Commit()
}

// withKeg returns a function that fills a scanned tap's current keg, passing errors through.
func withKeg(db *sql.DB) func(Tap, error) (Tap, error) {
	return func(tp Tap, err error) (Tap, error) {
		if err != nil {
			return Tap{}, err
		}
		list := []Tap{tp}
		err = attachKegs(db, list)
		return list[0], err
	}
}

// unixTime converts stored Unix seconds to a UTC time, which marshals as RFC 3339.
func unixTime(s int64) time.Time { return time.Unix(s, 0).UTC() }
