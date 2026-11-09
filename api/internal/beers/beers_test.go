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
	if err := Delete(db, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(db, gone); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get deleted = %v, want ErrNotFound", err)
	}
	if err := Delete(db, gone); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete = %v, want ErrNotFound", err)
	}
	if err := Delete(db, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
	if err := Delete(db, old); err != nil {
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
		if err := Delete(db, id); !errors.Is(err, ErrHasKegs) {
			t.Fatalf("%s: delete = %v, want ErrHasKegs", name, err)
		}
		if _, err := Get(db, id); err != nil {
			t.Fatalf("%s: beer gone after refused delete: %v", name, err)
		}
	}
}
