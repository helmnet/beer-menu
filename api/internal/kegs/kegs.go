package kegs

import (
	"database/sql"
	"errors"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// Keg states, derived and never stored: kicked once kicked_at is set, else on_tap while it has a
// tap, else waiting (on deck).
const (
	StateOnTap   = "on_tap"
	StateWaiting = "waiting"
	StateKicked  = "kicked"
	// StateCurrent is a list filter, not a keg state: kegs on a tap and waiting.
	StateCurrent = "current"
)

// Keg is a keg as the API returns it. Volumes are litres to 0.01. TapLabel is the tap's live
// label while the keg is current and on a tap, else the label of the last tap it was on.
// Optional fields are nil when empty and marshal as null, so every field is always present.
type Keg struct {
	ID         int64      `json:"id"`
	BeerID     int64      `json:"beer_id"`
	BeerName   string     `json:"beer_name"`
	BeerColor  *string    `json:"beer_color"`
	Type       string     `json:"type"`
	SizeL      float64    `json:"size_l"`
	RemainingL float64    `json:"remaining_l"`
	TapID      *int64     `json:"tap_id"`
	TapLabel   *string    `json:"tap_label"`
	State      string     `json:"state"`
	TappedAt   *time.Time `json:"tapped_at"`
	KickedAt   *time.Time `json:"kicked_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// KegInput is the body of a keg create request. Type defaults to DefaultType, SizeL to the
// type's size and RemainingL to SizeL. TapID absent or null creates a waiting keg.
type KegInput struct {
	BeerID     int64                 `json:"beer_id"`
	Type       *string               `json:"type"`
	SizeL      *float64              `json:"size_l"`
	RemainingL *float64              `json:"remaining_l"`
	TapID      beers.Optional[int64] `json:"tap_id"`
}

// KegPatch is the body of a keg PATCH request with merge-patch semantics: an unset field is
// kept. TapID null takes the keg off its tap; every other field refuses null. Kicked accepts only
// true.
type KegPatch struct {
	BeerID     beers.Optional[int64]   `json:"beer_id"`
	Type       beers.Optional[string]  `json:"type"`
	SizeL      beers.Optional[float64] `json:"size_l"`
	RemainingL beers.Optional[float64] `json:"remaining_l"`
	TapID      beers.Optional[int64]   `json:"tap_id"`
	Kicked     beers.Optional[bool]    `json:"kicked"`
}

// KegFilter selects kegs for ListKegs. State is StateCurrent (also when empty), StateWaiting or
// StateKicked; a zero BeerID or TapID does not filter.
type KegFilter struct {
	State  string
	BeerID int64
	TapID  int64
}

// queryRower is a *sql.DB or *sql.Tx.
type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

// kegSelect reads kegs with their beer and live tap label, for scanKeg.
const kegSelect = `SELECT k.id, k.beer_id, b.name, b.srm, k.type, k.size_l, k.remaining_l, k.tap_id,
	COALESCE(CASE WHEN k.kicked_at IS NULL THEN t.label END, k.tap_label), k.tapped_at, k.kicked_at, k.created_at, k.updated_at
	FROM kegs k JOIN beers b ON b.id = k.beer_id LEFT JOIN taps t ON t.id = k.tap_id`

// scanKeg reads one row selected with kegSelect into a Keg, deriving its color and state.
func scanKeg(row interface{ Scan(...any) error }) (Keg, error) {
	var k Keg
	var srm *float64
	var tapped, kicked sql.NullInt64
	var created, updated int64
	err := row.Scan(&k.ID, &k.BeerID, &k.BeerName, &srm, &k.Type, &k.SizeL, &k.RemainingL, &k.TapID, &k.TapLabel,
		&tapped, &kicked, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Keg{}, ErrNotFound
	}
	if err != nil {
		return Keg{}, err
	}
	k.BeerColor = beers.ColorFor(srm)
	k.CreatedAt, k.UpdatedAt = unixTime(created), unixTime(updated)
	k.TappedAt, k.KickedAt = optTime(tapped), optTime(kicked)
	switch {
	case k.KickedAt != nil:
		k.State = StateKicked
	case k.TapID != nil:
		k.State = StateOnTap
	default:
		k.State = StateWaiting
	}
	return k, nil
}

// GetKeg returns keg id, or ErrNotFound.
func GetKeg(db *sql.DB, id int64) (Keg, error) { return getKeg(db, id) }

func getKeg(q queryRower, id int64) (Keg, error) {
	return scanKeg(q.QueryRow(kegSelect+" WHERE k.id = ?", id))
}

// ListKegs returns the kegs f selects: current (on a tap and waiting) and waiting kegs oldest
// first, kicked kegs newest kicked first. TapID matches kegs on that tap or kicked on it, not
// kegs taken off it. The slice is never nil, so it marshals as [] when empty.
func ListKegs(db *sql.DB, f KegFilter) ([]Keg, error) {
	var where, order string
	switch f.State {
	case "", StateCurrent:
		where, order = "k.kicked_at IS NULL", "k.created_at, k.id"
	case StateWaiting:
		where, order = "k.kicked_at IS NULL AND k.tap_id IS NULL", "k.created_at, k.id"
	case StateKicked:
		where, order = "k.kicked_at IS NOT NULL", "k.kicked_at DESC, k.id DESC"
	default:
		return nil, errors.New("unknown keg state " + f.State)
	}
	var args []any
	if f.BeerID != 0 {
		where += " AND k.beer_id = ?"
		args = append(args, f.BeerID)
	}
	if f.TapID != 0 {
		where += " AND k.tap_id = ?"
		args = append(args, f.TapID)
	}
	return queryKegs(db, kegSelect+" WHERE "+where+" ORDER BY "+order, args...)
}

func queryKegs(db *sql.DB, query string, args ...any) ([]Keg, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Keg{}
	for rows.Next() {
		k, err := scanKeg(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, k)
	}
	return list, rows.Err()
}

// attachKegs sets each tap's Keg to its current keg with one query for all taps.
func attachKegs(db *sql.DB, taps []Tap) error {
	list, err := queryKegs(db, kegSelect+" WHERE k.kicked_at IS NULL AND k.tap_id IS NOT NULL")
	if err != nil {
		return err
	}
	byTap := make(map[int64]*Keg, len(list))
	for i := range list {
		byTap[*list[i].TapID] = &list[i]
	}
	for i := range taps {
		taps[i].Keg = byTap[taps[i].ID]
	}
	return nil
}

// CreateKeg validates in and stores a keg at now, on its tap with tapped_at now when TapID is
// set, and returns it. It returns FieldErrors, writing nothing, for invalid fields, a beer that
// is missing or archived, or a missing tap, and ErrTapOccupied when the tap has a current keg.
// A disabled tap may take a keg.
func CreateKeg(db *sql.DB, in KegInput, now time.Time) (Keg, error) {
	fe := beers.FieldErrors{}
	kt := checkType(fe, DefaultType)
	if in.Type != nil {
		kt = checkType(fe, *in.Type)
	}
	size := kt.SizeL
	if in.SizeL != nil {
		size = checkSize(fe, *in.SizeL)
	}
	remaining := size
	if in.RemainingL != nil {
		remaining = checkRemaining(fe, *in.RemainingL, size)
	}
	var tapID *int64
	if in.TapID.Set {
		tapID = in.TapID.Value
	}

	tx, err := db.Begin()
	if err != nil {
		return Keg{}, err
	}
	defer tx.Rollback()
	if err := checkBeer(tx, fe, in.BeerID); err != nil {
		return Keg{}, err
	}
	taken, err := checkTap(tx, fe, tapID, 0)
	if err != nil {
		return Keg{}, err
	}
	if len(fe) > 0 {
		return Keg{}, fe
	}
	if taken {
		return Keg{}, ErrTapOccupied
	}
	var id int64
	err = tx.QueryRow(`INSERT INTO kegs (beer_id, type, size_l, remaining_l, tap_id, tap_label, tapped_at, created_at, updated_at)
		VALUES (?1, ?2, ?3, ?4, ?5, (SELECT label FROM taps WHERE id = ?5), CASE WHEN ?5 IS NOT NULL THEN ?6 END, ?6, ?6) RETURNING id`,
		in.BeerID, kt.Type, size, remaining, tapID, now.Unix()).Scan(&id)
	if err != nil {
		return Keg{}, occupied(err)
	}
	k, err := getKeg(tx, id)
	if err != nil {
		return Keg{}, err
	}
	return k, tx.Commit()
}

// UpdateKeg applies p to keg id at now in one transaction and returns the keg. Going on a tap
// sets tapped_at the first time; leaving one, or being kicked, keeps that tap's label. Type
// alone keeps the size; a size below the remaining volume lowers it, unless p also sets
// remaining_l, which is then checked against the new size. Kicked true sets kicked_at and keeps
// tap_id and remaining_l. It returns FieldErrors or beers.ErrEmptyPatch, writing nothing, when p
// is invalid; ErrNotFound for an unknown id; ErrTapOccupied for a tap with another current keg;
// and ErrKegKicked for any change to a kicked keg except a repeat kick, which returns the keg
// unchanged.
func UpdateKeg(db *sql.DB, id int64, p KegPatch, now time.Time) (Keg, error) {
	fe := beers.FieldErrors{}
	if p.BeerID.Set && p.BeerID.Value == nil {
		fe["beer_id"] = "Must be a current beer."
	}
	if p.Type.Set {
		if p.Type.Value == nil {
			fe["type"] = "Must be one of the keg types."
		} else {
			checkType(fe, *p.Type.Value)
		}
	}
	var size *float64
	if p.SizeL.Set {
		if p.SizeL.Value == nil {
			fe["size_l"] = "Must be more than 0 and at most 100 litres."
		} else {
			s := checkSize(fe, *p.SizeL.Value)
			size = &s
		}
	}
	if p.RemainingL.Set && p.RemainingL.Value == nil {
		fe["remaining_l"] = "Must be 0 to the keg size."
	}
	if p.Kicked.Set && (p.Kicked.Value == nil || !*p.Kicked.Value) {
		fe["kicked"] = "Can only be true: a kicked keg stays kicked."
	}
	if len(fe) > 0 {
		return Keg{}, fe
	}
	kick := p.Kicked.Set
	if !p.BeerID.Set && !p.Type.Set && !p.SizeL.Set && !p.RemainingL.Set && !p.TapID.Set && !kick {
		return Keg{}, beers.ErrEmptyPatch
	}

	tx, err := db.Begin()
	if err != nil {
		return Keg{}, err
	}
	defer tx.Rollback()
	var curSize float64
	var curTap *int64
	var kicked bool
	err = tx.QueryRow("SELECT size_l, tap_id, kicked_at IS NOT NULL FROM kegs WHERE id = ?", id).Scan(&curSize, &curTap, &kicked)
	if errors.Is(err, sql.ErrNoRows) {
		return Keg{}, ErrNotFound
	}
	if err != nil {
		return Keg{}, err
	}
	if kicked {
		if kick && !p.BeerID.Set && !p.Type.Set && !p.SizeL.Set && !p.RemainingL.Set && !p.TapID.Set {
			return getKeg(tx, id)
		}
		return Keg{}, ErrKegKicked
	}

	query := "UPDATE kegs SET updated_at = ?"
	args := []any{now.Unix()}
	if p.BeerID.Set {
		if err := checkBeer(tx, fe, *p.BeerID.Value); err != nil {
			return Keg{}, err
		}
		query += ", beer_id = ?"
		args = append(args, *p.BeerID.Value)
	}
	if p.Type.Set {
		query += ", type = ?"
		args = append(args, *p.Type.Value)
	}
	if size != nil {
		query += ", size_l = ?"
		args = append(args, *size)
		curSize = *size
	}
	switch {
	case p.RemainingL.Set:
		query += ", remaining_l = ?"
		args = append(args, checkRemaining(fe, *p.RemainingL.Value, curSize))
	case size != nil:
		query += ", remaining_l = MIN(remaining_l, ?)"
		args = append(args, *size)
	}
	tap, taken := curTap, false
	if p.TapID.Set {
		tap = p.TapID.Value
		if taken, err = checkTap(tx, fe, tap, id); err != nil {
			return Keg{}, err
		}
		query += ", tap_id = ?, tapped_at = CASE WHEN ? IS NOT NULL THEN COALESCE(tapped_at, ?) ELSE tapped_at END"
		args = append(args, tap, tap, now.Unix())
	}
	if len(fe) > 0 {
		return Keg{}, fe
	}
	if taken {
		return Keg{}, ErrTapOccupied
	}
	// Record the label of the tap the keg is on now, or of the one it just left, so history
	// keeps it after the tap is renamed or removed.
	query += ", tap_label = COALESCE((SELECT label FROM taps WHERE id = ?), (SELECT label FROM taps WHERE id = ?), tap_label)"
	args = append(args, tap, curTap)
	if kick {
		query += ", kicked_at = ?"
		args = append(args, now.Unix())
	}
	if _, err := tx.Exec(query+" WHERE id = ?", append(args, id)...); err != nil {
		return Keg{}, occupied(err)
	}
	k, err := getKeg(tx, id)
	if err != nil {
		return Keg{}, err
	}
	return k, tx.Commit()
}

// Pour subtracts amount litres, rounded to 0.01, from keg id at now in one statement, so
// concurrent pours all count. The level stops at 0 and the keg is never kicked by a pour; a
// waiting keg may be poured. It returns FieldErrors unless amount is over 0 and at most 100,
// ErrNotFound for an unknown id and ErrKegKicked for a kicked keg.
func Pour(db *sql.DB, id int64, amount float64, now time.Time) (Keg, error) {
	fe := beers.FieldErrors{}
	amount = checkAmount(fe, amount)
	if len(fe) > 0 {
		return Keg{}, fe
	}
	err := db.QueryRow(`UPDATE kegs SET remaining_l = MAX(0.0, ROUND(remaining_l - ?, 2)), updated_at = ?
		WHERE id = ? AND kicked_at IS NULL RETURNING id`, amount, now.Unix(), id).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// The UPDATE skips only missing and kicked kegs; GetKeg tells them apart.
		if _, err := GetKeg(db, id); err != nil {
			return Keg{}, err
		}
		return Keg{}, ErrKegKicked
	}
	if err != nil {
		return Keg{}, err
	}
	return GetKeg(db, id)
}

// DeleteKeg removes keg id permanently, in any state. It returns ErrNotFound for an unknown id.
func DeleteKeg(db *sql.DB, id int64) error {
	res, err := db.Exec("DELETE FROM kegs WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		err = ErrNotFound
	}
	return err
}

// checkBeer records a beer_id error in fe unless beer names a beer that is not archived.
func checkBeer(q queryRower, fe beers.FieldErrors, beer int64) error {
	var ok bool
	err := q.QueryRow("SELECT EXISTS (SELECT 1 FROM beers WHERE id = ? AND archived_at IS NULL)", beer).Scan(&ok)
	if err == nil && !ok {
		fe["beer_id"] = "Must be a current beer."
	}
	return err
}

// checkTap records a tap_id error in fe when tap is set and names no tap, and reports whether
// the tap holds a current keg other than keg (0 for a new keg). A nil tap passes.
func checkTap(q queryRower, fe beers.FieldErrors, tap *int64, keg int64) (taken bool, err error) {
	if tap == nil {
		return false, nil
	}
	var exists bool
	err = q.QueryRow(`SELECT EXISTS (SELECT 1 FROM taps WHERE id = ?1),
		EXISTS (SELECT 1 FROM kegs WHERE tap_id = ?1 AND kicked_at IS NULL AND id != ?2)`, *tap, keg).Scan(&exists, &taken)
	if err == nil && !exists {
		fe["tap_id"] = "Must be an existing tap."
	}
	return taken, err
}

// occupied maps a violation of the kegs_one_per_tap index, the only unique index a keg write can
// break, to ErrTapOccupied, and returns other errors unchanged. The explicit check in checkTap
// runs first; this covers a writer the check did not see.
func occupied(err error) error {
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return ErrTapOccupied
	}
	return err
}

// optTime converts nullable stored Unix seconds to a UTC time, or nil.
func optTime(s sql.NullInt64) *time.Time {
	if !s.Valid {
		return nil
	}
	t := unixTime(s.Int64)
	return &t
}
