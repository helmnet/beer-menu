package beers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// openDB returns a migrated database in a temporary folder.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(db, ms); err != nil {
		t.Fatal(err)
	}
	return db
}

// insert adds a beer row named name, archived when archived is set, and returns its id.
func insert(t *testing.T, db *sql.DB, name string, archived bool) int64 {
	t.Helper()
	var archivedAt any
	if archived {
		archivedAt = t0.Unix()
	}
	res, err := db.Exec("INSERT INTO beers (name, archived_at, created_at, updated_at) VALUES (?, ?, ?, ?)", name, archivedAt, t0.Unix(), t0.Unix())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func names(bs []Beer) string {
	s := ""
	for _, b := range bs {
		s += fmt.Sprintf("%s#%d ", b.Name, b.ID)
	}
	return s
}

func TestGet(t *testing.T) {
	db := openDB(t)
	if _, err := Get(db, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
	id := insert(t, db, "Bare", false)
	b, err := Get(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != id || b.Name != "Bare" || b.Style != nil || b.ABV != nil || b.IBU != nil || b.SRM != nil || b.Color != nil ||
		b.Description != nil || b.Brewer != nil || b.BrewDate != nil || b.Status != "ready" || b.Source != "manual" ||
		b.SourceRef != nil || b.ArchivedAt != nil || !b.CreatedAt.Equal(t0) || !b.UpdatedAt.Equal(t0) {
		t.Fatalf("bare beer = %+v", b)
	}

	res, err := db.Exec(`INSERT INTO beers (name, style, abv, ibu, srm, description, brewer, brew_date, status, source, source_ref, archived_at, created_at, updated_at)
		VALUES ('Full', 'IPA', 6.7, 55, 7, 'Hoppy', 'Helm', '2026-10-01', 'fermenting', 'brewfather', 'batch-1', ?, ?, ?)`, t0.Unix()+2, t0.Unix(), t0.Unix()+1)
	if err != nil {
		t.Fatal(err)
	}
	id, _ = res.LastInsertId()
	b, err = Get(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if *b.Style != "IPA" || *b.ABV != 6.7 || *b.IBU != 55 || *b.SRM != 7 || *b.Color != SRMColors[6] || *b.Description != "Hoppy" ||
		*b.Brewer != "Helm" || *b.BrewDate != "2026-10-01" || b.Status != "fermenting" || b.Source != "brewfather" ||
		*b.SourceRef != "batch-1" || !b.ArchivedAt.Equal(t0.Add(2*time.Second)) || !b.UpdatedAt.Equal(t0.Add(time.Second)) {
		t.Fatalf("full beer = %+v", b)
	}
}

func TestList(t *testing.T) {
	db := openDB(t)
	got, err := List(db, false)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty List = %#v (err %v), want empty non-nil", got, err)
	}
	beta := insert(t, db, "beta", false)
	alpha1 := insert(t, db, "Alpha", false)
	alpha2 := insert(t, db, "alpha", false)
	old := insert(t, db, "Old", true)

	got, err = List(db, false)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Alpha#%d alpha#%d beta#%d ", alpha1, alpha2, beta)
	if names(got) != want {
		t.Fatalf("current = %s, want %s", names(got), want)
	}
	got, err = List(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("Old#%d ", old); names(got) != want {
		t.Fatalf("archived = %s, want %s", names(got), want)
	}
}

func TestList500(t *testing.T) {
	db := openDB(t)
	for i := range 500 {
		if _, err := db.Exec("INSERT INTO beers (name, srm, created_at, updated_at) VALUES (?, ?, 1, 1)", fmt.Sprintf("Beer %d", i), i%40+1); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	got, err := List(db, false)
	if err != nil || len(got) != 500 {
		t.Fatalf("List = %d beers (err %v)", len(got), err)
	}
	// SC-006's budget is 1 s on the home server; a tighter bound flakes on slow or -race CI.
	if d := time.Since(start); d > time.Second {
		t.Fatalf("List of 500 took %v, want under 1 s", d)
	}
}

func TestCreate(t *testing.T) {
	db := openDB(t)
	b, err := Create(db, Input{Name: " Pale "}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == 0 || b.Name != "Pale" || b.Status != "ready" || b.Source != "manual" || b.SourceRef != nil ||
		b.ABV != nil || !b.CreatedAt.Equal(t0) || !b.UpdatedAt.Equal(t0) || b.ArchivedAt != nil {
		t.Fatalf("name-only beer = %+v", b)
	}

	full := Input{Name: "Hönig 🍯 IPA", Style: ptr("American IPA"), ABV: ptr(6.7), IBU: ptr(55.0), SRM: ptr(7.5),
		Description: ptr("Line one\n\tLine two ✓"), Brewer: ptr("Helm's Brews"), BrewDate: ptr("2026-10-01"), Status: ptr("conditioning")}
	b, err = Create(db, full, t0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Get(db, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != full.Name || *got.Style != *full.Style || *got.ABV != 6.7 || *got.IBU != 55 || *got.SRM != 7.5 ||
		*got.Color != SRMColors[7] || *got.Description != *full.Description || *got.Brewer != *full.Brewer ||
		*got.BrewDate != "2026-10-01" || got.Status != "conditioning" {
		t.Fatalf("full beer = %+v", got)
	}

	twin, err := Create(db, Input{Name: "Pale"}, t0)
	if err != nil || twin.ID == 0 {
		t.Fatalf("duplicate name: %+v (err %v)", twin, err)
	}
	if list, _ := List(db, false); len(list) != 3 {
		t.Fatalf("list has %d beers, want 3", len(list))
	}

	if _, err := Create(db, Input{Name: "", ABV: ptr(150.0)}, t0); fieldKeysOf(err) != "abv,name" {
		t.Fatalf("invalid create err = %v", err)
	}
	if list, _ := List(db, false); len(list) != 3 {
		t.Fatalf("invalid create wrote a row")
	}
}

func TestUpdate(t *testing.T) {
	db := openDB(t)
	orig, err := Create(db, Input{Name: "Pale", Style: ptr("APA"), ABV: ptr(5.0), SRM: ptr(4.0)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(time.Hour)
	b, err := Update(db, orig.ID, Patch{ABV: Optional[float64]{Set: true, Value: ptr(5.4)}}, t1)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != orig.ID || *b.ABV != 5.4 || b.Name != "Pale" || *b.Style != "APA" || *b.SRM != 4 ||
		!b.CreatedAt.Equal(t0) || !b.UpdatedAt.Equal(t1) {
		t.Fatalf("after ABV edit = %+v", b)
	}

	b, err = Update(db, orig.ID, Patch{Style: Optional[string]{Set: true}, SRM: Optional[float64]{Set: true}}, t1)
	if err != nil || b.Style != nil || b.SRM != nil || b.Color != nil || *b.ABV != 5.4 {
		t.Fatalf("after clearing = %+v (err %v)", b, err)
	}

	if _, err := Update(db, 999, Patch{ABV: Optional[float64]{Set: true, Value: ptr(1.0)}}, t1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing id err = %v, want ErrNotFound", err)
	}

	before, _ := Get(db, orig.ID)
	if _, err := Update(db, orig.ID, Patch{Name: Optional[string]{Set: true, Value: ptr("New")}, ABV: Optional[float64]{Set: true, Value: ptr(150.0)}}, t1.Add(time.Hour)); fieldKeysOf(err) != "abv" {
		t.Fatalf("invalid update err = %v", err)
	}
	if after, _ := Get(db, orig.ID); after.Name != before.Name || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("invalid update changed the beer: %+v", after)
	}

	var all Patch
	if err := json.Unmarshal([]byte(`{"name":"Full","style":"IPA","abv":6,"ibu":40,"srm":9,"description":"d","brewer":"b","brew_date":"2026-01-02","status":"planned"}`), &all); err != nil {
		t.Fatal(err)
	}
	b, err = Update(db, orig.ID, all, t1)
	if err != nil || b.Name != "Full" || *b.Style != "IPA" || *b.ABV != 6 || *b.IBU != 40 || *b.SRM != 9 ||
		*b.Description != "d" || *b.Brewer != "b" || *b.BrewDate != "2026-01-02" || b.Status != "planned" {
		t.Fatalf("every field = %+v (err %v)", b, err)
	}
}

// fieldKeysOf returns the sorted FieldErrors keys of err joined by commas, or err's text.
func fieldKeysOf(err error) string {
	var fe FieldErrors
	if !errors.As(err, &fe) {
		return fmt.Sprint(err)
	}
	keys := make([]string, 0, len(fe))
	for k := range fe {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestArchiveAndRestore(t *testing.T) {
	db := openDB(t)
	orig, err := Create(db, Input{Name: "Pale", ABV: ptr(5.0)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	archive := func(v bool, at time.Time) Beer {
		t.Helper()
		b, err := Update(db, orig.ID, Patch{Archived: Optional[bool]{Set: true, Value: &v}}, at)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	t1, t2 := t0.Add(time.Hour), t0.Add(2*time.Hour)

	if b := archive(false, t0); b.ArchivedAt != nil {
		t.Fatalf("restoring a current beer archived it: %+v", b)
	}
	b := archive(true, t1)
	if b.ArchivedAt == nil || !b.ArchivedAt.Equal(t1) || b.Name != "Pale" || *b.ABV != 5 || b.ID != orig.ID {
		t.Fatalf("archived = %+v", b)
	}
	if b := archive(true, t2); !b.ArchivedAt.Equal(t1) {
		t.Fatalf("archiving again moved archived_at to %v", b.ArchivedAt)
	}
	b, err = Update(db, orig.ID, Patch{ABV: Optional[float64]{Set: true, Value: ptr(5.5)}}, t2)
	if err != nil || *b.ABV != 5.5 || !b.ArchivedAt.Equal(t1) {
		t.Fatalf("editing an archived beer = %+v (err %v)", b, err)
	}
	if b := archive(false, t2); b.ArchivedAt != nil || b.Name != "Pale" || *b.ABV != 5.5 {
		t.Fatalf("restored = %+v", b)
	}
}

func TestDelete(t *testing.T) {
	db := openDB(t)
	keep := insert(t, db, "Keep", false)
	gone := insert(t, db, "Gone", false)
	old := insert(t, db, "Old", true)
	if err := Delete(db, gone, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(db, gone); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get deleted = %v, want ErrNotFound", err)
	}
	if err := Delete(db, gone, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete = %v, want ErrNotFound", err)
	}
	if err := Delete(db, 999, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
	if err := Delete(db, old, t0); err != nil {
		t.Fatalf("Delete archived = %v", err)
	}
	if _, err := Get(db, keep); err != nil {
		t.Fatalf("other beer gone: %v", err)
	}
}

// insertKeg adds a keg of beer on tap (nil for waiting), kicked at kicked (nil for current), and
// returns its id. Migration 0004 seeds tap 1.
func insertKeg(t *testing.T, db *sql.DB, beer int64, tap, kicked any) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, kicked_at, created_at, updated_at) VALUES (?, 1, 1, ?, ?, 1, 1)", beer, tap, kicked)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestOnTap(t *testing.T) {
	db := openDB(t)
	id := insert(t, db, "Pale", false)
	onTap := func() bool {
		t.Helper()
		b, err := Get(db, id)
		if err != nil {
			t.Fatal(err)
		}
		list, err := List(db, false)
		if err != nil || len(list) != 1 || list[0].OnTap != b.OnTap {
			t.Fatalf("List = %+v (err %v), Get on_tap %v", list, err, b.OnTap)
		}
		return b.OnTap
	}
	if onTap() {
		t.Fatal("on_tap with no kegs")
	}
	keg := insertKeg(t, db, id, nil, nil)
	if onTap() {
		t.Fatal("on_tap with a waiting keg")
	}
	if _, err := db.Exec("UPDATE kegs SET tap_id = 1 WHERE id = ?", keg); err != nil {
		t.Fatal(err)
	}
	if !onTap() {
		t.Fatal("on_tap false with a keg on a tap")
	}
	if b, err := Update(db, id, Patch{ABV: Optional[float64]{Set: true, Value: ptr(5.0)}}, t0); err != nil || !b.OnTap {
		t.Fatalf("PATCH response on_tap = %+v (err %v)", b, err)
	}
	if _, err := db.Exec("UPDATE kegs SET tap_id = NULL WHERE id = ?", keg); err != nil {
		t.Fatal(err)
	}
	if onTap() {
		t.Fatal("on_tap after the keg was taken off")
	}
	if b, err := Create(db, Input{Name: "New"}, t0); err != nil || b.OnTap {
		t.Fatalf("created on_tap = %+v (err %v)", b, err)
	}
}

func TestArchiveWithCurrentKeg(t *testing.T) {
	db := openDB(t)
	archive := Patch{Archived: Optional[bool]{Set: true, Value: ptr(true)}, ABV: Optional[float64]{Set: true, Value: ptr(9.0)}}
	for name, tap := range map[string]any{"on a tap": 1, "waiting": nil} {
		id := insert(t, db, "Pale", false)
		keg := insertKeg(t, db, id, tap, nil)
		if _, err := Update(db, id, archive, t0.Add(time.Hour)); !errors.Is(err, ErrHasCurrentKeg) {
			t.Fatalf("%s: archive = %v, want ErrHasCurrentKeg", name, err)
		}
		if b, _ := Get(db, id); b.ArchivedAt != nil || b.ABV != nil || !b.UpdatedAt.Equal(t0) {
			t.Fatalf("%s: refused archive changed the beer: %+v", name, b)
		}
		if _, err := db.Exec("UPDATE kegs SET kicked_at = 2 WHERE id = ?", keg); err != nil {
			t.Fatal(err)
		}
		if b, err := Update(db, id, archive, t0); err != nil || b.ArchivedAt == nil {
			t.Fatalf("%s: archive with only kicked kegs = %+v (err %v)", name, b, err)
		}
	}
	if _, err := Update(db, 999, archive, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archive missing = %v", err)
	}
}

func TestDeleteWithKegs(t *testing.T) {
	db := openDB(t)
	for name, kicked := range map[string]any{"current": nil, "kicked": 2} {
		id := insert(t, db, "Pale", false)
		insertKeg(t, db, id, nil, kicked)
		if err := Delete(db, id, t0); !errors.Is(err, ErrHasKegs) {
			t.Fatalf("%s: delete = %v, want ErrHasKegs", name, err)
		}
		if _, err := Get(db, id); err != nil {
			t.Fatalf("%s: beer gone after refused delete: %v", name, err)
		}
	}
}

// dismissals returns the import_dismissed rows as "source/ref@time" joined by spaces.
func dismissals(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query("SELECT source, ref, dismissed_at FROM import_dismissed ORDER BY source, ref")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var source, ref string
		var at int64
		if err := rows.Scan(&source, &ref, &at); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s/%s@%d", source, ref, at))
	}
	return strings.Join(out, " ")
}

func TestCreateImported(t *testing.T) {
	db := openDB(t)
	manual, err := Create(db, Input{Name: "Manual"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if manual.Sync != nil {
		t.Fatalf("manual beer Sync = %+v, want nil", manual.Sync)
	}
	v := Values{Name: "Batch", Style: ptr("IPA"), ABV: ptr(6.5), IBU: ptr[int64](40), SRM: ptr(7.0), Description: ptr("Notes"),
		BrewDate: ptr("2026-10-01"), Status: StatusFermenting}
	b, err := CreateImported(db, "brewfather", "batch-1", v, false, t0)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Batch" || *b.Style != "IPA" || *b.ABV != 6.5 || *b.IBU != 40 || *b.SRM != 7 || *b.Description != "Notes" ||
		b.Brewer != nil || *b.BrewDate != "2026-10-01" || b.Status != StatusFermenting || b.Source != "brewfather" ||
		*b.SourceRef != "batch-1" || b.ArchivedAt != nil || !b.CreatedAt.Equal(t0) {
		t.Fatalf("imported = %+v", b)
	}
	if b.Sync == nil || b.Sync.SyncedAt == nil || !b.Sync.SyncedAt.Equal(t0) || b.Sync.NotFound || b.Sync.ArchivePending {
		t.Fatalf("imported Sync = %+v", b.Sync)
	}
	if got, err := Get(db, b.ID); err != nil || got.Sync == nil || !got.Sync.SyncedAt.Equal(t0) {
		t.Fatalf("Get imported Sync = %+v (err %v)", got.Sync, err)
	}
	raw, _ := json.Marshal(b)
	if !strings.Contains(string(raw), `"sync":{"synced_at":"2026-11-09T12:00:00Z","not_found":false,"archive_pending":false}`) {
		t.Fatalf("imported JSON = %s", raw)
	}
	if raw, _ := json.Marshal(manual); !strings.Contains(string(raw), `"sync":null`) {
		t.Fatalf("manual JSON = %s", raw)
	}

	v.Name = "Again"
	if _, err := CreateImported(db, "brewfather", "batch-1", v, false, t0); !errors.Is(err, ErrAlreadyImported) {
		t.Fatalf("second import = %v, want ErrAlreadyImported", err)
	}
	if list, _ := List(db, false); len(list) != 2 {
		t.Fatalf("list has %d beers after duplicate import, want 2", len(list))
	}
	if _, err := CreateImported(db, "brewfather", "batch-2", v, false, t0); err != nil {
		t.Fatalf("other batch: %v", err)
	}
}

func TestDeleteRecordsDismissal(t *testing.T) {
	db := openDB(t)
	t1 := t0.Add(time.Hour)
	manual := insert(t, db, "Manual", false)
	if err := Delete(db, manual, t1); err != nil {
		t.Fatal(err)
	}
	withKeg, err := CreateImported(db, "brewfather", "kegged", Values{Name: "K", Status: StatusReady}, false, t0)
	if err != nil {
		t.Fatal(err)
	}
	insertKeg(t, db, withKeg.ID, nil, 2)
	if err := Delete(db, withKeg.ID, t1); !errors.Is(err, ErrHasKegs) {
		t.Fatalf("delete with keg = %v, want ErrHasKegs", err)
	}
	if got := dismissals(t, db); got != "" {
		t.Fatalf("dismissals after manual and refused deletes = %q, want none", got)
	}
	imported, err := CreateImported(db, "brewfather", "batch-1", Values{Name: "B", Status: StatusReady}, false, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(db, imported.ID, t1); err != nil {
		t.Fatal(err)
	}
	if got, want := dismissals(t, db), fmt.Sprintf("brewfather/batch-1@%d", t1.Unix()); got != want {
		t.Fatalf("dismissals = %q, want %q", got, want)
	}
	if refs, err := DismissedRefs(db, "brewfather"); err != nil || len(refs) != 1 || !refs["batch-1"] {
		t.Fatalf("DismissedRefs = %v (err %v)", refs, err)
	}
	if refs, err := DismissedRefs(db, "other"); err != nil || len(refs) != 0 {
		t.Fatalf("DismissedRefs of another source = %v (err %v)", refs, err)
	}
	if err := ClearDismissal(db, "brewfather", "batch-1"); err != nil {
		t.Fatal(err)
	}
	if err := ClearDismissal(db, "brewfather", "never"); err != nil {
		t.Fatalf("clearing a missing dismissal: %v", err)
	}
	if got := dismissals(t, db); got != "" {
		t.Fatalf("dismissals after clear = %q", got)
	}
}

// importedBeer creates an imported, fermenting beer with every field set.
func importedBeer(t *testing.T, db *sql.DB, ref string) Beer {
	t.Helper()
	b, err := CreateImported(db, "brewfather", ref, Values{Name: "Local name", Style: ptr("Local style"), ABV: ptr(5.0),
		IBU: ptr[int64](30), SRM: ptr(5.0), Description: ptr("Local notes"), BrewDate: ptr("2026-09-01"), Status: StatusFermenting}, false, t0)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// syncFlags reads the beer's three sync flags as "not_found pending batch_archived".
func syncFlags(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()
	var nf, pending, batch int
	if err := db.QueryRow("SELECT sync_not_found, sync_archive_pending, sync_batch_archived FROM beers WHERE id = ?", id).Scan(&nf, &pending, &batch); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(nf, pending, batch)
}

func TestApplySync(t *testing.T) {
	db := openDB(t)
	b := importedBeer(t, db, "batch-1")
	if _, err := db.Exec("UPDATE beers SET brewer = 'Local brewer', sync_not_found = 1 WHERE id = ?", b.ID); err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(time.Hour)
	upstream := Values{Name: "Upstream", Style: ptr("Upstream style"), ABV: ptr(6.1), IBU: ptr[int64](45), SRM: ptr(9.0),
		Description: ptr("Upstream notes"), Brewer: ptr("Upstream brewer"), BrewDate: ptr("2026-10-02"), Status: StatusConditioning}
	got, err := ApplySync(db, b.ID, upstream, false, t1)
	if err != nil {
		t.Fatal(err)
	}
	if *got.ABV != 6.1 || *got.IBU != 45 || *got.SRM != 9 || *got.BrewDate != "2026-10-02" || got.Status != StatusConditioning ||
		*got.Color != SRMColors[8] || !got.UpdatedAt.Equal(t1) || got.ArchivedAt != nil {
		t.Fatalf("synced = %+v", got)
	}
	if got.Name != "Local name" || *got.Style != "Local style" || *got.Description != "Local notes" || *got.Brewer != "Local brewer" {
		t.Fatalf("sync overwrote local fields: %+v", got)
	}
	if got.Sync == nil || !got.Sync.SyncedAt.Equal(t1) || got.Sync.NotFound || got.Sync.ArchivePending {
		t.Fatalf("Sync = %+v", got.Sync)
	}
	if f := syncFlags(t, db, b.ID); f != "0 0 0" {
		t.Fatalf("flags = %s", f)
	}

	empty := Values{Name: "x", Status: StatusReady}
	if got, err := ApplySync(db, b.ID, empty, false, t1); err != nil || got.ABV != nil || got.IBU != nil || got.SRM != nil ||
		got.BrewDate != nil || got.Color != nil || got.Status != StatusReady {
		t.Fatalf("sync to empty values = %+v (err %v)", got, err)
	}
	if _, err := ApplySync(db, 999, empty, false, t1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id = %v, want ErrNotFound", err)
	}
}

func TestApplySyncArchive(t *testing.T) {
	db := openDB(t)
	v := Values{Name: "x", Status: StatusReady}
	t1, t2, t3 := t0.Add(time.Hour), t0.Add(2*time.Hour), t0.Add(3*time.Hour)
	apply := func(id int64, archived bool, at time.Time) Beer {
		t.Helper()
		b, err := ApplySync(db, id, v, archived, at)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	t.Run("no keg archives once", func(t *testing.T) {
		id := importedBeer(t, db, "a").ID
		if b := apply(id, true, t1); b.ArchivedAt == nil || !b.ArchivedAt.Equal(t1) || b.Sync.ArchivePending {
			t.Fatalf("archived batch = %+v %+v", b, b.Sync)
		}
		if f := syncFlags(t, db, id); f != "0 0 1" {
			t.Fatalf("flags = %s, want 0 0 1", f)
		}
		if _, err := db.Exec("UPDATE beers SET archived_at = NULL WHERE id = ?", id); err != nil {
			t.Fatal(err)
		}
		if b := apply(id, true, t2); b.ArchivedAt != nil {
			t.Fatalf("archived again after the owner unarchived: %v", b.ArchivedAt)
		}
		if b := apply(id, false, t2); b.ArchivedAt != nil || syncFlags(t, db, id) != "0 0 0" {
			t.Fatalf("batch unarchived: %+v flags %s", b, syncFlags(t, db, id))
		}
		if b := apply(id, true, t3); b.ArchivedAt == nil || !b.ArchivedAt.Equal(t3) {
			t.Fatalf("re-archived batch did not archive: %+v", b)
		}
	})

	for name, tap := range map[string]any{"on a tap": 1, "waiting": nil} {
		t.Run("current keg "+name, func(t *testing.T) {
			id := importedBeer(t, db, "keg-"+name).ID
			keg := insertKeg(t, db, id, tap, nil)
			if b := apply(id, true, t1); b.ArchivedAt != nil || !b.Sync.ArchivePending {
				t.Fatalf("with current keg = %+v %+v", b, b.Sync)
			}
			if f := syncFlags(t, db, id); f != "0 1 0" {
				t.Fatalf("flags = %s, want 0 1 0", f)
			}
			if _, err := db.Exec("UPDATE kegs SET kicked_at = 5, tap_id = NULL WHERE id = ?", keg); err != nil {
				t.Fatal(err)
			}
			if b := apply(id, true, t2); b.ArchivedAt == nil || !b.ArchivedAt.Equal(t2) || b.Sync.ArchivePending {
				t.Fatalf("after kick = %+v %+v", b, b.Sync)
			}
			if f := syncFlags(t, db, id); f != "0 0 1" {
				t.Fatalf("flags after kick = %s, want 0 0 1", f)
			}
		})
	}

	t.Run("not archived never unarchives", func(t *testing.T) {
		id := importedBeer(t, db, "local").ID
		if _, err := db.Exec("UPDATE beers SET archived_at = 3, sync_archive_pending = 1, sync_batch_archived = 1 WHERE id = ?", id); err != nil {
			t.Fatal(err)
		}
		if b := apply(id, false, t1); b.ArchivedAt == nil || b.ArchivedAt.Unix() != 3 {
			t.Fatalf("unarchived by sync: %+v", b)
		}
		if f := syncFlags(t, db, id); f != "0 0 0" {
			t.Fatalf("flags = %s, want 0 0 0", f)
		}
	})

	t.Run("archived locally keeps its time", func(t *testing.T) {
		id := importedBeer(t, db, "early").ID
		if _, err := db.Exec("UPDATE beers SET archived_at = 3 WHERE id = ?", id); err != nil {
			t.Fatal(err)
		}
		if b := apply(id, true, t1); b.ArchivedAt.Unix() != 3 || syncFlags(t, db, id) != "0 0 1" {
			t.Fatalf("archived beer = %+v flags %s", b, syncFlags(t, db, id))
		}
	})
}

func TestMarkNotFound(t *testing.T) {
	db := openDB(t)
	b := importedBeer(t, db, "gone")
	got, err := MarkNotFound(db, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Sync.NotFound || got.Sync.ArchivePending || !got.Sync.SyncedAt.Equal(t0) {
		t.Fatalf("Sync = %+v", got.Sync)
	}
	got.Sync, b.Sync = nil, nil
	if g, w := mustJSON(t, got), mustJSON(t, b); g != w {
		t.Fatalf("MarkNotFound changed more than the flag:\n%s\n%s", g, w)
	}
	if f := syncFlags(t, db, b.ID); f != "1 0 0" {
		t.Fatalf("flags = %s, want 1 0 0", f)
	}
	if _, err := MarkNotFound(db, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id = %v, want ErrNotFound", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
