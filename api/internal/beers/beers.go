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

// ErrHasCurrentKeg means archiving was refused because a current keg, on a tap or waiting, is
// of the beer (409 beer_has_keg).
var ErrHasCurrentKeg = errors.New("beer has a current keg")

// ErrAlreadyImported means a beer is already linked to that source and ref, so an import made
// no second beer (FR-015).
var ErrAlreadyImported = errors.New("batch already imported")

// ErrHasKegs means deleting was refused because a keg, current or kicked, is of the beer (409
// beer_has_kegs).
var ErrHasKegs = errors.New("beer has kegs")

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
	// OnTap is true while a current keg of the beer is on a tap, so clients draw it as a glass.
	OnTap bool `json:"on_tap"`
	// Sync is nil for a manual beer and the import sync state otherwise.
	Sync      *BeerSync `json:"sync"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeerSync is an imported beer's sync state. The internal "batch archive applied" flag is not
// part of it.
type BeerSync struct {
	// SyncedAt is the last successful sync, nil if never.
	SyncedAt *time.Time `json:"synced_at"`
	// NotFound is true while the provider reports the batch missing.
	NotFound bool `json:"not_found"`
	// ArchivePending is true while an archive from the provider waits on a current keg.
	ArchivePending bool `json:"archive_pending"`
}

// columns is used in SELECT and RETURNING alike, so on_tap is a subquery rather than a join.
const columns = "id, name, style, abv, ibu, srm, description, brewer, brew_date, status, source, source_ref, archived_at, created_at, updated_at, " +
	"synced_at, sync_not_found, sync_archive_pending, " +
	"EXISTS (SELECT 1 FROM kegs WHERE kegs.beer_id = beers.id AND kegs.tap_id IS NOT NULL AND kegs.kicked_at IS NULL)"

// scan reads one row selected with columns into a Beer and derives its Color.
func scan(row interface{ Scan(...any) error }) (Beer, error) {
	var b Beer
	var archived, synced sql.NullInt64
	var created, updated int64
	var sync BeerSync
	err := row.Scan(&b.ID, &b.Name, &b.Style, &b.ABV, &b.IBU, &b.SRM, &b.Description, &b.Brewer, &b.BrewDate,
		&b.Status, &b.Source, &b.SourceRef, &archived, &created, &updated, &synced, &sync.NotFound, &sync.ArchivePending, &b.OnTap)
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
	if b.Source != "manual" {
		if synced.Valid {
			t := unixTime(synced.Int64)
			sync.SyncedAt = &t
		}
		b.Sync = &sync
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
// invalid, ErrNotFound for an unknown id, and ErrHasCurrentKeg, writing nothing, when archiving
// a beer with a current keg (feature 4 FR-030).
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
	where := " WHERE id = ?"
	archiving := p.Archived.Set && *p.Archived.Value
	if p.Archived.Set {
		if archiving {
			query += ", archived_at = COALESCE(archived_at, ?)"
			args = append(args, now.Unix())
			// In the same statement as the update, so a keg added meanwhile cannot slip past.
			where += " AND NOT EXISTS (SELECT 1 FROM kegs WHERE kegs.beer_id = beers.id AND kegs.kicked_at IS NULL)"
		} else {
			query += ", archived_at = NULL"
		}
	}
	b, err := scan(db.QueryRow(query+where+" RETURNING "+columns, append(args, id)...))
	if archiving && errors.Is(err, ErrNotFound) {
		if _, getErr := Get(db, id); getErr == nil {
			return Beer{}, ErrHasCurrentKeg
		}
	}
	return b, err
}

// Delete removes beer id permanently (FR-024), archived or not. It returns ErrNotFound for an
// unknown id and ErrHasKegs, deleting nothing, when any keg is of the beer (feature 4 FR-031);
// the kegs.beer_id foreign key is the backstop. Deleting an imported beer records its source
// and ref in import_dismissed at now, in the same transaction, so sync does not import it again
// (research R9).
func Delete(db *sql.DB, id int64, now time.Time) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO import_dismissed (source, ref, dismissed_at)
		SELECT source, source_ref, ? FROM beers WHERE id = ? AND source_ref IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM kegs WHERE beer_id = ?)`, now.Unix(), id, id); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM beers WHERE id = ? AND NOT EXISTS (SELECT 1 FROM kegs WHERE beer_id = ?)", id, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		if _, getErr := scan(tx.QueryRow("SELECT "+columns+" FROM beers WHERE id = ?", id)); getErr == nil {
			return ErrHasKegs
		}
		return ErrNotFound
	}
	return tx.Commit()
}

// CreateImported stores v at now as a beer from source with the provider's ref, synced at now,
// with brewer NULL. batchArchived records that the provider's archive is already applied. It
// returns ErrAlreadyImported, writing nothing, when a beer already has that source and ref. v
// must already be clean (Sanitize); it is not validated again.
func CreateImported(db *sql.DB, source, ref string, v Values, batchArchived bool, now time.Time) (Beer, error) {
	b, err := scan(db.QueryRow(`INSERT INTO beers (name, style, abv, ibu, srm, description, brew_date, status, source, source_ref,
		synced_at, sync_batch_archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (source, source_ref) WHERE source_ref IS NOT NULL DO NOTHING RETURNING `+columns,
		v.Name, v.Style, v.ABV, v.IBU, v.SRM, v.Description, v.BrewDate, v.Status, source, ref,
		now.Unix(), batchArchived, now.Unix(), now.Unix()))
	if errors.Is(err, ErrNotFound) {
		return Beer{}, ErrAlreadyImported
	}
	return b, err
}

// noCurrentKeg is Update's archive guard: no keg of the beer, on a tap or waiting, is unkicked.
const noCurrentKeg = "NOT EXISTS (SELECT 1 FROM kegs WHERE kegs.beer_id = beers.id AND kegs.kicked_at IS NULL)"

// ApplySync overwrites beer id's ABV, IBU, SRM, brew date and status from v, sets synced_at and
// updated_at to now and clears not_found; name, style, description and brewer stay as the owner
// left them (FR-017). When batchArchived and the provider's archive is not yet applied, it
// archives the beer if it has no current keg (keeping an earlier archived_at), else marks the
// archive pending for the next sync; once applied it never archives again, so the owner can
// unarchive. A batch not archived clears both flags and never unarchives (FR-017a). One
// statement, so a keg added meanwhile cannot slip past. ErrNotFound for an unknown id.
func ApplySync(db *sql.DB, id int64, v Values, batchArchived bool, now time.Time) (Beer, error) {
	// SET expressions read the row's old values, so each CASE sees sync_batch_archived before
	// this update.
	return scan(db.QueryRow(`UPDATE beers SET abv = ?, ibu = ?, srm = ?, brew_date = ?, status = ?,
		synced_at = ?, updated_at = ?, sync_not_found = 0,
		archived_at = CASE WHEN ? AND sync_batch_archived = 0 AND `+noCurrentKeg+` THEN COALESCE(archived_at, ?) ELSE archived_at END,
		sync_archive_pending = CASE WHEN NOT ? OR sync_batch_archived = 1 OR `+noCurrentKeg+` THEN 0 ELSE 1 END,
		sync_batch_archived = CASE WHEN NOT ? THEN 0 WHEN sync_batch_archived = 1 OR `+noCurrentKeg+` THEN 1 ELSE 0 END
		WHERE id = ? RETURNING `+columns,
		v.ABV, v.IBU, v.SRM, v.BrewDate, v.Status, now.Unix(), now.Unix(),
		batchArchived, now.Unix(), batchArchived, batchArchived, id))
}

// MarkNotFound records that the provider no longer has beer id's batch and changes nothing
// else, updated_at included. ErrNotFound for an unknown id.
func MarkNotFound(db *sql.DB, id int64) (Beer, error) {
	return scan(db.QueryRow("UPDATE beers SET sync_not_found = 1 WHERE id = ? RETURNING "+columns, id))
}

// ClearDismissal forgets that the owner deleted the beer imported from source and ref, so a
// hand import of that batch is not undone by sync (FR-022). Missing rows are not an error.
func ClearDismissal(db *sql.DB, source, ref string) error {
	_, err := db.Exec("DELETE FROM import_dismissed WHERE source = ? AND ref = ?", source, ref)
	return err
}

// DismissedRefs returns the refs of source whose imported beer the owner deleted, so a sync
// does not import them again (FR-022).
func DismissedRefs(db *sql.DB, source string) (map[string]bool, error) {
	rows, err := db.Query("SELECT ref FROM import_dismissed WHERE source = ?", source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := map[string]bool{}
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		refs[ref] = true
	}
	return refs, rows.Err()
}

// ImportedIDs returns the id of every beer imported from source, archived or not, keyed by its
// source_ref.
func ImportedIDs(db *sql.DB, source string) (map[string]int64, error) {
	rows, err := db.Query("SELECT source_ref, id FROM beers WHERE source = ? AND source_ref IS NOT NULL", source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var ref string
		var id int64
		if err := rows.Scan(&ref, &id); err != nil {
			return nil, err
		}
		ids[ref] = id
	}
	return ids, rows.Err()
}

// unixTime converts stored Unix seconds to a UTC time, which marshals as RFC 3339.
func unixTime(s int64) time.Time { return time.Unix(s, 0).UTC() }
