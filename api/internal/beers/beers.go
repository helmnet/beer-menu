package beers

import (
	"database/sql"
	"errors"
	"maps"
	"slices"
	"time"
)

// ErrNotFound means no beer has the given id.
var ErrNotFound = errors.New("beer not found")

// Beer is a beer as the API returns it. Optional fields are nil when empty and marshal as null,
// so every field is always present (FR-003). Color is derived from SRM, never stored.
type Beer struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Style       *string    `json:"style"`
	ABV         *float64   `json:"abv"`
	IBU         *int64     `json:"ibu"`
	SRM         *float64   `json:"srm"`
	Color       *string    `json:"color"`
	Description *string    `json:"description"`
	Brewer      *string    `json:"brewer"`
	BrewDate    *string    `json:"brew_date"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	SourceRef   *string    `json:"source_ref"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const columns = "id, name, style, abv, ibu, srm, description, brewer, brew_date, status, source, source_ref, archived_at, created_at, updated_at"

// scan reads one row selected with columns into a Beer and derives its Color.
func scan(row interface{ Scan(...any) error }) (Beer, error) {
	var b Beer
	var archived sql.NullInt64
	var created, updated int64
	err := row.Scan(&b.ID, &b.Name, &b.Style, &b.ABV, &b.IBU, &b.SRM, &b.Description, &b.Brewer, &b.BrewDate,
		&b.Status, &b.Source, &b.SourceRef, &archived, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Beer{}, ErrNotFound
	}
	if err != nil {
		return Beer{}, err
	}
	b.Color = ColorFor(b.SRM)
	b.CreatedAt, b.UpdatedAt = unixTime(created), unixTime(updated)
	if archived.Valid {
		t := unixTime(archived.Int64)
		b.ArchivedAt = &t
	}
	return b, nil
}

// Get returns beer id, archived or not, or ErrNotFound.
func Get(db *sql.DB, id int64) (Beer, error) {
	return scan(db.QueryRow("SELECT "+columns+" FROM beers WHERE id = ?", id))
}

// List returns the current beers, or only the archived ones when archived is set, sorted by
// name ignoring the case of A to Z, then by id (FR-014). The slice is never nil, so it
// marshals as [] when empty.
func List(db *sql.DB, archived bool) ([]Beer, error) {
	where := "archived_at IS NULL"
	if archived {
		where = "archived_at IS NOT NULL"
	}
	rows, err := db.Query("SELECT " + columns + " FROM beers WHERE " + where + " ORDER BY name COLLATE NOCASE, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Beer{}
	for rows.Next() {
		b, err := scan(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

// Create validates in and stores it at now as a manual beer, returning the stored beer. It
// returns FieldErrors, and writes nothing, when in is invalid.
func Create(db *sql.DB, in Input, now time.Time) (Beer, error) {
	v, err := in.Normalize()
	if err != nil {
		return Beer{}, err
	}
	return scan(db.QueryRow(`INSERT INTO beers (name, style, abv, ibu, srm, description, brewer, brew_date, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+columns,
		v.Name, v.Style, v.ABV, v.IBU, v.SRM, v.Description, v.Brewer, v.BrewDate, v.Status, now.Unix(), now.Unix()))
}

// Update applies p to beer id at now in one statement and returns the updated beer; the id
// never changes (FR-007). Archived true sets archived_at to now unless already set, so
// archiving twice keeps the first time; false clears it (FR-023). It returns FieldErrors or ErrEmptyPatch, writing nothing, when p is
// invalid, and ErrNotFound for an unknown id.
func Update(db *sql.DB, id int64, p Patch, now time.Time) (Beer, error) {
	set, err := p.Normalize()
	if err != nil {
		return Beer{}, err
	}
	query := "UPDATE beers SET updated_at = ?"
	args := []any{now.Unix()}
	// set's keys are column-name literals from Patch.Normalize, never client text; sorted for a stable query.
	for _, col := range slices.Sorted(maps.Keys(set)) {
		query += ", " + col + " = ?"
		args = append(args, set[col])
	}
	if p.Archived.Set {
		if *p.Archived.Value {
			query += ", archived_at = COALESCE(archived_at, ?)"
			args = append(args, now.Unix())
		} else {
			query += ", archived_at = NULL"
		}
	}
	return scan(db.QueryRow(query+" WHERE id = ? RETURNING "+columns, append(args, id)...))
}

// Delete removes beer id permanently (FR-024), archived or not. It returns ErrNotFound for an
// unknown id.
func Delete(db *sql.DB, id int64) error {
	res, err := db.Exec("DELETE FROM beers WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		err = ErrNotFound
	}
	return err
}

// unixTime converts stored Unix seconds to a UTC time, which marshals as RFC 3339.
func unixTime(s int64) time.Time { return time.Unix(s, 0).UTC() }
