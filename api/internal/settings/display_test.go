package settings

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/store"
)

func ptr[T any](v T) *T { return &v }

// mustGet returns GetDisplay(db), failing t on error.
func mustGet(t *testing.T, db Querier) Display {
	t.Helper()
	d, err := GetDisplay(db)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// countKeys returns how many display.* rows are stored.
func countKeys(t *testing.T, q Querier) int {
	t.Helper()
	var n int
	if err := q.QueryRow("SELECT COUNT(*) FROM settings WHERE key LIKE 'display.%'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGetDisplayDefaults(t *testing.T) {
	db := openDB(t)
	if d := mustGet(t, db); d != Default {
		t.Fatalf("empty table = %+v, want %+v", d, Default)
	}
}

func TestGetDisplayBadStoredValues(t *testing.T) {
	good := map[string]string{
		keyTitle: "Helm", keyTheme: "light", keyUnits: "metric", keyPageSeconds: "20",
	}
	want := Display{Title: "Helm", Theme: "light", Units: "metric", PageSeconds: 20}
	cases := []struct {
		key, value string
		fix        func(*Display)
	}{
		{keyTheme, "blue", func(d *Display) { d.Theme = "dark" }},
		{keyUnits, "imperial", func(d *Display) { d.Units = "us" }},
		{keyPageSeconds, "4", func(d *Display) { d.PageSeconds = 15 }},
		{keyPageSeconds, "301", func(d *Display) { d.PageSeconds = 15 }},
		{keyPageSeconds, "15.5", func(d *Display) { d.PageSeconds = 15 }},
		{keyPageSeconds, "x", func(d *Display) { d.PageSeconds = 15 }},
		{keyTitle, strings.Repeat("é", 61), func(d *Display) { d.Title = "" }},
		{keyTitle, "Bell\x07", func(d *Display) { d.Title = "" }},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.value, func(t *testing.T) {
			db := openDB(t)
			for k, v := range good {
				if err := Set(db, k, v, t0); err != nil {
					t.Fatal(err)
				}
			}
			if err := Set(db, c.key, c.value, t0); err != nil {
				t.Fatal(err)
			}
			w := want
			c.fix(&w)
			if d := mustGet(t, db); d != w {
				t.Fatalf("got %+v, want %+v", d, w)
			}
		})
	}
}

func TestPatchDisplayAccepts(t *testing.T) {
	db := openDB(t)
	sixty := strings.Repeat("é", 60)
	for _, c := range []struct {
		patch DisplayPatch
		want  Display
	}{
		{DisplayPatch{Title: ptr(sixty)}, Display{Title: sixty, Theme: "dark", Units: "us", PageSeconds: 15}},
		{DisplayPatch{Title: ptr("  Helm  ")}, Display{Title: "Helm", Theme: "dark", Units: "us", PageSeconds: 15}},
		{DisplayPatch{Title: ptr("   ")}, Default},
		{DisplayPatch{PageSeconds: ptr(5)}, Display{Theme: "dark", Units: "us", PageSeconds: 5}},
		{DisplayPatch{PageSeconds: ptr(300), Theme: ptr("light"), Units: ptr("metric")}, Display{Theme: "light", Units: "metric", PageSeconds: 300}},
	} {
		got, err := PatchDisplay(db, c.patch, t0)
		if err != nil || got != c.want {
			t.Fatalf("patch %+v = %+v (err %v), want %+v", c.patch, got, err, c.want)
		}
		if read := mustGet(t, db); read != c.want {
			t.Fatalf("read back %+v, want %+v", read, c.want)
		}
	}
	if v, ok, _ := Get(db, keyTitle); !ok || v != "" {
		t.Fatalf("spaces-only title stored as %q %v, want empty", v, ok)
	}
}

func TestPatchDisplayRefuses(t *testing.T) {
	for _, c := range []struct {
		field string
		patch DisplayPatch
	}{
		{"title", DisplayPatch{Title: ptr(strings.Repeat("é", 61))}},
		{"title", DisplayPatch{Title: ptr("Helm\x07")}},
		{"theme", DisplayPatch{Theme: ptr("blue")}},
		{"units", DisplayPatch{Units: ptr("imperial")}},
		{"page_seconds", DisplayPatch{PageSeconds: ptr(4)}},
		{"page_seconds", DisplayPatch{PageSeconds: ptr(301)}},
	} {
		db := openDB(t)
		_, err := PatchDisplay(db, c.patch, t0)
		var fe beers.FieldErrors
		if !errors.As(err, &fe) || len(fe) != 1 || fe[c.field] == "" {
			t.Fatalf("patch %+v = %v, want a %s field error", c.patch, err, c.field)
		}
		if n := countKeys(t, db); n != 0 {
			t.Fatalf("refused patch stored %d rows", n)
		}
	}
	if _, err := PatchDisplay(openDB(t), DisplayPatch{}, t0); !errors.Is(err, beers.ErrEmptyPatch) {
		t.Fatalf("empty patch = %v, want ErrEmptyPatch", err)
	}
}

func TestPatchDisplayAllOrNothing(t *testing.T) {
	db := openDB(t)
	if _, err := PatchDisplay(db, DisplayPatch{Title: ptr("Old")}, t0); err != nil {
		t.Fatal(err)
	}
	_, err := PatchDisplay(db, DisplayPatch{Title: ptr("New"), PageSeconds: ptr(4)}, t0.Add(time.Hour))
	var fe beers.FieldErrors
	if !errors.As(err, &fe) || len(fe) != 1 || fe["page_seconds"] == "" {
		t.Fatalf("mixed patch = %v, want only a page_seconds error", err)
	}
	if d := mustGet(t, db); d.Title != "Old" {
		t.Fatalf("title after refused patch = %q, want Old", d.Title)
	}
	if u := updatedAt(t, db, keyTitle); u != t0.Unix() {
		t.Fatalf("updated_at changed to %d", u)
	}
}

func TestPatchDisplayTouchesOnlyNamedKeys(t *testing.T) {
	db := openDB(t)
	got, err := PatchDisplay(db, DisplayPatch{Title: ptr("Helm")}, t0)
	want := Display{Title: "Helm", Theme: "dark", Units: "us", PageSeconds: 15}
	if err != nil || got != want {
		t.Fatalf("patch = %+v (err %v), want %+v", got, err, want)
	}
	if n := countKeys(t, db); n != 1 {
		t.Fatalf("stored %d display rows, want 1", n)
	}
	got, err = PatchDisplay(db, DisplayPatch{Theme: ptr("light"), PageSeconds: ptr(20)}, t0.Add(time.Hour))
	want = Display{Title: "Helm", Theme: "light", Units: "us", PageSeconds: 20}
	if err != nil || got != want {
		t.Fatalf("second patch = %+v (err %v), want %+v", got, err, want)
	}
	if v, _, _ := Get(db, keyPageSeconds); v != "20" {
		t.Fatalf("page_seconds stored as %q, want decimal text 20", v)
	}
	if updatedAt(t, db, keyTheme) != updatedAt(t, db, keyPageSeconds) || updatedAt(t, db, keyTitle) != t0.Unix() {
		t.Fatal("one patch must write its keys with the same updated_at and leave others alone")
	}
}

func TestPublic(t *testing.T) {
	d := Display{Title: "Helm", Theme: "light", Units: "metric", PageSeconds: 20}
	if p := d.Public(); p != (Public{Title: "Helm", Theme: "light", PageSeconds: 20}) {
		t.Fatalf("Public = %+v", p)
	}
	b, err := json.Marshal(d.Public())
	if err != nil || string(b) != `{"title":"Helm","theme":"light","page_seconds":20}` {
		t.Fatalf("Public JSON = %s (err %v)", b, err)
	}
	b, err = json.Marshal(d)
	if err != nil || string(b) != `{"title":"Helm","theme":"light","units":"metric","page_seconds":20}` {
		t.Fatalf("Display JSON = %s (err %v)", b, err)
	}
}

func TestGetDisplayIgnoresOtherKeys(t *testing.T) {
	db := openDB(t)
	if err := Set(db, "brewfather.sync_interval", "1h", t0); err != nil {
		t.Fatal(err)
	}
	if d := mustGet(t, db); d != Default {
		t.Fatalf("got %+v, want Default", d)
	}
}

// TestDisplayDefaultsOnVersion5Data is the upgrade case: a version-5 database holding feature 4
// and 6 data and no display rows reads as all defaults and stays at version 5 (FR-014; there is
// no migration to test).
func TestDisplayDefaultsOnVersion5Data(t *testing.T) {
	db := openDB(t)
	for _, q := range []string{
		"INSERT INTO beers (name, created_at, updated_at) VALUES ('Pale', 1, 1)",
		"INSERT INTO taps (position, label, created_at, updated_at) VALUES ((SELECT COALESCE(MAX(position), 0) + 1 FROM taps), 'Upgrade', 1, 1)",
		"INSERT INTO kegs (beer_id, size_l, remaining_l, created_at, updated_at) VALUES ((SELECT id FROM beers WHERE name = 'Pale'), 18.93, 18.93, 1, 1)",
		"INSERT INTO settings (key, value, updated_at) VALUES ('brewfather.sync_interval', '1h', 7)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if v, err := store.Version(db); err != nil || v != 5 {
		t.Fatalf("version = %d (err %v), want 5", v, err)
	}
	if d := mustGet(t, db); d != Default {
		t.Fatalf("got %+v, want Default", d)
	}
	if v, err := store.Version(db); err != nil || v != 5 {
		t.Fatalf("version after read = %d (err %v), want 5", v, err)
	}
	var v string
	var u int64
	if err := db.QueryRow("SELECT value, updated_at FROM settings WHERE key = 'brewfather.sync_interval'").Scan(&v, &u); err != nil || v != "1h" || u != 7 {
		t.Fatalf("sync interval = %q %d (err %v)", v, u, err)
	}
}
