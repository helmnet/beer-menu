package kegs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// addKeg adds a current keg of beer with type typ, size and remaining litres, on tap (nil for
// waiting), created at created, and returns its id. It bypasses validation so tests can set any
// level.
func addKeg(t *testing.T, db *sql.DB, beer int64, typ string, size, remaining float64, tap any, created int64) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("INSERT INTO kegs (beer_id, type, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id",
		beer, typ, size, remaining, tap, created, created).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// mustMenu returns GetMenu(db), failing t on error.
func mustMenu(t *testing.T, db *sql.DB) Menu {
	t.Helper()
	m, err := GetMenu(db)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	return m
}

// menuTaps returns the taps as "label:beer:shape:level" in menu order, "-" for an empty tap.
func menuTaps(m Menu) string {
	var s []string
	for _, tp := range m.Taps {
		switch {
		case tp.Beer == nil && tp.Keg == nil:
			s = append(s, tp.Label+":-")
		case tp.Beer == nil || tp.Keg == nil:
			s = append(s, tp.Label+":half")
		default:
			s = append(s, fmt.Sprintf("%s:%s:%s:%g", tp.Label, tp.Beer.Name, tp.Keg.Shape, tp.Keg.Level))
		}
	}
	return strings.Join(s, " ")
}

// onDeck returns the on-deck items as "beer:shape" for kegs and "beer:status" for beers.
func onDeck(m Menu) string {
	var s []string
	for _, it := range m.OnDeck {
		switch {
		case it.Keg != nil && it.Status == nil:
			s = append(s, it.Beer.Name+":"+it.Keg.Shape)
		case it.Keg == nil && it.Status != nil:
			s = append(s, it.Beer.Name+":"+*it.Status)
		default:
			s = append(s, it.Beer.Name+":bad")
		}
	}
	return strings.Join(s, " ")
}

func TestMenuFresh(t *testing.T) {
	m := mustMenu(t, openDB(t))
	if menuTaps(m) != "Tap 1:-" || m.OnDeck == nil || len(m.OnDeck) != 0 {
		t.Fatalf("fresh menu = %+v", m)
	}
	b, _ := json.Marshal(m)
	if string(b) != `{"taps":[{"label":"Tap 1","beer":null,"keg":null}],"on_deck":[]}` {
		t.Fatalf("fresh JSON = %s", b)
	}
}

func TestMenuTaps(t *testing.T) {
	db := openDB(t)
	pale := insertBeer(t, db, "Pale", 7.0)
	exec(t, db, "UPDATE beers SET style = 'APA', abv = 5.2, ibu = 35 WHERE id = ?", pale)
	plain := insertBeer(t, db, "Plain", nil)
	tap2, tap3, tap4 := newTap(t, db, "Tap 2"), newTap(t, db, "Tap 3"), newTap(t, db, "Tap 4")
	off, err := CreateTap(db, TapInput{Label: ptr("Off"), Enabled: ptr(false)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReorderTaps(db, []int64{tap3, 1, off.ID, tap4, tap2}, t0); err != nil {
		t.Fatal(err)
	}
	addKeg(t, db, pale, "corny_5_gal", 40, 10, int64(1), 1)
	addKeg(t, db, plain, "sanke_half", 58.67, 58.67, tap2, 1)
	addKeg(t, db, plain, "euro_20_l", 20, 25, tap3, 1)
	addKeg(t, db, pale, "corny_5_gal", 18.93, 5, off.ID, 1)
	insertKeg(t, db, pale, tap4, 5)

	m := mustMenu(t, db)
	if got := menuTaps(m); got != "Tap 3:Plain:sanke:1 Tap 1:Pale:corny:0.25 Tap 4:- Tap 2:Plain:sanke:1" {
		t.Fatalf("taps = %s", got)
	}
	b := m.Taps[1].Beer
	if b.Style == nil || *b.Style != "APA" || b.ABV == nil || *b.ABV != 5.2 || b.IBU == nil || *b.IBU != 35 ||
		b.Color == nil || *b.Color != *beers.ColorFor(ptr(7.0)) {
		t.Fatalf("Pale = %+v", b)
	}
	if b := m.Taps[0].Beer; b.Style != nil || b.ABV != nil || b.IBU != nil || b.Color != nil {
		t.Fatalf("Plain = %+v", b)
	}
}

func TestMenuLevel(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	for _, c := range []struct{ size, remaining, want float64 }{{40, 10, 0.25}, {30, 0.01, 0.01 / 30}, {10, 0, 0}, {10, -1, 0}, {10, 11, 1}, {0, 0, 0}, {0, 5, 0}} {
		exec(t, db, "DELETE FROM kegs")
		addKeg(t, db, beer, "corny_5_gal", c.size, c.remaining, int64(1), 1)
		if k := mustMenu(t, db).Taps[0].Keg; k == nil || k.Level != c.want {
			t.Errorf("level for %g of %g = %+v, want %g", c.remaining, c.size, k, c.want)
		}
	}
}

// TestMenuUnknownType draws a keg whose type key is not in Types (the column has no CHECK) as
// the default type's shape, never an empty shape.
func TestMenuUnknownType(t *testing.T) {
	db := openDB(t)
	addKeg(t, db, insertBeer(t, db, "Pale", nil), "gone", 20, 10, int64(1), 1)
	if got := menuTaps(mustMenu(t, db)); got != "Tap 1:Pale:corny:0.5" {
		t.Fatalf("taps = %s", got)
	}
}

// TestMenuArchivedBeerOnTap archives a tapped keg's beer by direct SQL, bypassing the feature 4
// guard, so the tap must show empty with both beer and keg null.
func TestMenuArchivedBeerOnTap(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Old", nil)
	addKeg(t, db, beer, "corny_5_gal", 18.93, 10, int64(1), 1)
	addKeg(t, db, beer, "corny_5_gal", 18.93, 10, nil, 1)
	exec(t, db, "UPDATE beers SET archived_at = 1, status = 'fermenting' WHERE id = ?", beer)
	m := mustMenu(t, db)
	if menuTaps(m) != "Tap 1:-" || len(m.OnDeck) != 0 {
		t.Fatalf("menu = %s / %s", menuTaps(m), onDeck(m))
	}
}

func TestMenuOnDeck(t *testing.T) {
	db := openDB(t)
	beer := func(name, status string) int64 {
		id := insertBeer(t, db, name, nil)
		exec(t, db, "UPDATE beers SET status = ? WHERE id = ?", status, id)
		return id
	}
	off, err := CreateTap(db, TapInput{Label: ptr("Off"), Enabled: ptr(false)}, t0)
	if err != nil {
		t.Fatal(err)
	}
	// Waiting kegs, oldest first, ties by id.
	addKeg(t, db, beer("Late", "ready"), "corny_5_gal", 18.93, 18.93, nil, 30)
	addKeg(t, db, beer("Cond keg", "conditioning"), "sanke_sixth", 19.56, 9.78, nil, 10)
	addKeg(t, db, beer("Tie", "ready"), "euro_30_l", 30, 30, nil, 10)
	// Beers still being made, conditioning first, then by name ignoring case, then id.
	beer("brown", "fermenting")
	beer("Amber", "fermenting")
	beer("zwickel", "conditioning")
	beer("Bock", "conditioning")
	beer("amber", "fermenting")
	// A kicked keg is not a current keg, so its beer still being made is on deck.
	insertKeg(t, db, beer("Refill", "fermenting"), nil, 5)
	// Not on deck: planned and ready beers without a keg, archived, on a tap, on a disabled tap,
	// kicked.
	beer("Planned", "planned")
	beer("Ready", "ready")
	exec(t, db, "UPDATE beers SET archived_at = 1 WHERE id = ?", beer("Archived", "fermenting"))
	addKeg(t, db, beer("On tap", "fermenting"), "corny_5_gal", 18.93, 18.93, int64(1), 1)
	addKeg(t, db, beer("On off tap", "fermenting"), "corny_5_gal", 18.93, 18.93, off.ID, 1)
	insertKeg(t, db, beer("Kicked", "ready"), nil, 5)

	m := mustMenu(t, db)
	want := "Cond keg:sanke Tie:sanke Late:corny Bock:conditioning zwickel:conditioning Amber:fermenting amber:fermenting brown:fermenting Refill:fermenting"
	if got := onDeck(m); got != want {
		t.Fatalf("on deck =\n%s\nwant\n%s", got, want)
	}
	if k := m.OnDeck[0].Keg; k.Level != 0.5 {
		t.Fatalf("Cond keg level = %g", k.Level)
	}
	if menuTaps(m) != "Tap 1:On tap:corny:1" {
		t.Fatalf("taps = %s", menuTaps(m))
	}
}

// keySet returns the sorted keys of the JSON object v.
func keySet(t *testing.T, v any) string {
	t.Helper()
	o, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	var ks []string
	for k := range o {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return strings.Join(ks, ",")
}

// TestMenuJSONKeys pins the menu's JSON to data-model.md at every level: a field added to the
// structs fails here, so nothing new goes public by accident.
func TestMenuJSONKeys(t *testing.T) {
	db := openDB(t)
	pale := insertBeer(t, db, "Pale", 5.0)
	stout := insertBeer(t, db, "Stout", nil)
	exec(t, db, "UPDATE beers SET status = 'fermenting' WHERE id = ?", stout)
	addKeg(t, db, pale, "corny_5_gal", 18.93, 9, int64(1), 1)
	addKeg(t, db, pale, "sanke_half", 58.67, 9, nil, 1)
	newTap(t, db, "Empty")

	b, err := json.Marshal(mustMenu(t, db))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"menu": "on_deck,taps", "tap": "beer,keg,label", "item": "beer,keg,status",
		"beer": "abv,color,ibu,name,style", "keg": "level,shape",
	}
	check := func(what string, v any) {
		t.Helper()
		if got := keySet(t, v); got != want[what] {
			t.Errorf("%s keys = %s, want %s (%s)", what, got, want[what], b)
		}
	}
	check("menu", m)
	taps, deck := m["taps"].([]any), m["on_deck"].([]any)
	if len(taps) != 2 || len(deck) != 2 {
		t.Fatalf("menu = %s", b)
	}
	for _, v := range taps {
		check("tap", v)
	}
	full, empty := taps[0].(map[string]any), taps[1].(map[string]any)
	check("beer", full["beer"])
	check("keg", full["keg"])
	if empty["beer"] != nil || empty["keg"] != nil {
		t.Errorf("empty tap = %v", empty)
	}
	for _, v := range deck {
		check("item", v)
		check("beer", v.(map[string]any)["beer"])
	}
	check("keg", deck[0].(map[string]any)["keg"])
	if deck[0].(map[string]any)["status"] != nil || deck[1].(map[string]any)["keg"] != nil || deck[1].(map[string]any)["status"] != "fermenting" {
		t.Errorf("on deck = %v", deck)
	}
}

// TestMenuScale reads a menu of 20 tapped taps, 50 on-deck items and 1,000 kicked kegs within
// SC-006's 1 s budget.
func TestMenuScale(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", 5.0)
	for i := range 19 {
		newTap(t, db, fmt.Sprintf("Tap %d", i+2))
	}
	exec(t, db, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000)
		INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, tap_label, tapped_at, kicked_at, created_at, updated_at)
		SELECT ?, 18.93, 1, (i % 20) + 1, 'Tap', i, i + 1, i, i FROM n`, beer)
	for i := range 20 {
		addKeg(t, db, beer, "corny_5_gal", 18.93, 10, int64(i+1), 1)
	}
	for i := range 25 {
		addKeg(t, db, beer, "corny_5_gal", 18.93, 18.93, nil, int64(i))
		exec(t, db, "INSERT INTO beers (name, status, created_at, updated_at) VALUES (?, 'fermenting', 1, 1)", fmt.Sprintf("Batch %d", i))
	}
	// SC-006's budget is 1 s on the home server; a tighter bound flakes on slow or -race CI.
	start := time.Now()
	m, err := GetMenu(db)
	if d := time.Since(start); err != nil || len(m.Taps) != 20 || m.Taps[19].Keg == nil || len(m.OnDeck) != 50 || d > time.Second {
		t.Fatalf("GetMenu = %d taps, %d on deck in %v (err %v)", len(m.Taps), len(m.OnDeck), d, err)
	}
}
