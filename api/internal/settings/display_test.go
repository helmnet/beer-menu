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

// with returns Default changed by fix.
func with(fix func(*Display)) Display {
	d := Default
	fix(&d)
	return d
}

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
	want := with(func(d *Display) { d.Title, d.Theme, d.Units, d.PageSeconds = "Helm", "light", "metric", 20 })
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
		{DisplayPatch{Title: ptr(sixty)}, with(func(d *Display) { d.Title = sixty })},
		{DisplayPatch{Title: ptr("  Helm  ")}, with(func(d *Display) { d.Title = "Helm" })},
		{DisplayPatch{Title: ptr("   ")}, Default},
		{DisplayPatch{PageSeconds: ptr(5)}, with(func(d *Display) { d.PageSeconds = 5 })},
		{DisplayPatch{PageSeconds: ptr(300), Theme: ptr("light"), Units: ptr("metric")}, with(func(d *Display) { d.Theme, d.Units, d.PageSeconds = "light", "metric", 300 })},
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
	want := with(func(d *Display) { d.Title = "Helm" })
	if err != nil || got != want {
		t.Fatalf("patch = %+v (err %v), want %+v", got, err, want)
	}
	if n := countKeys(t, db); n != 1 {
		t.Fatalf("stored %d display rows, want 1", n)
	}
	got, err = PatchDisplay(db, DisplayPatch{Theme: ptr("light"), PageSeconds: ptr(20)}, t0.Add(time.Hour))
	want = with(func(d *Display) { d.Title, d.Theme, d.PageSeconds = "Helm", "light", 20 })
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
	d := Display{
		Title: "Helm", Theme: "light", Units: "metric", PageSeconds: 20,
		Layout: "columns", TitleAlign: "center", TitleFont: "pacifico", TextFont: "oswald",
		ShowStyle: false, ShowABV: true, ShowIBU: false, ShowSRM: true, ShowDescription: true,
		TitleColor: "#ffaa00", TextColor: "#102030",
	}
	want := Public{
		Title: "Helm", Theme: "light", PageSeconds: 20,
		Layout: "columns", TitleAlign: "center", TitleFont: "pacifico", TextFont: "oswald",
		ShowStyle: false, ShowABV: true, ShowIBU: false, ShowSRM: true, ShowDescription: true,
		TitleColor: "#ffaa00", TextColor: "#102030",
	}
	if p := d.Public(); p != want {
		t.Fatalf("Public = %+v", p)
	}
	tv := `"layout":"columns","title_align":"center","title_font":"pacifico","text_font":"oswald",` +
		`"show_style":false,"show_abv":true,"show_ibu":false,"show_srm":true,"show_description":true,` +
		`"title_color":"#ffaa00","text_color":"#102030"}`
	b, err := json.Marshal(d.Public())
	if err != nil || string(b) != `{"title":"Helm","theme":"light","page_seconds":20,`+tv {
		t.Fatalf("Public JSON = %s (err %v)", b, err)
	}
	b, err = json.Marshal(d)
	if err != nil || string(b) != `{"title":"Helm","theme":"light","units":"metric","page_seconds":20,`+tv {
		t.Fatalf("Display JSON = %s (err %v)", b, err)
	}
}

// TestTVDefaults pins the feature 10 defaults to data-model.md: today's look.
func TestTVDefaults(t *testing.T) {
	want := Display{
		Title: "", Theme: "dark", Units: "us", PageSeconds: 15,
		Layout: "grid", TitleAlign: "left", TitleFont: "system", TextFont: "system",
		ShowStyle: true, ShowABV: true, ShowIBU: true, ShowSRM: false, ShowDescription: false,
		TitleColor: "", TextColor: "",
	}
	if Default != want {
		t.Fatalf("Default = %+v, want %+v", Default, want)
	}
}

// TestGetDisplayBadTVValues stores a good value for every feature 10 key, then one bad value,
// which must read as its default while the other keys keep their stored values.
func TestGetDisplayBadTVValues(t *testing.T) {
	good := map[string]string{
		keyLayout: "list", keyTitleAlign: "right", keyTitleFont: "lora", keyTextFont: "nunito",
		keyShowStyle: "0", keyShowABV: "0", keyShowIBU: "0", keyShowSRM: "1", keyShowDescription: "1",
		keyTitleColor: "#aabbcc", keyTextColor: "#112233",
	}
	want := with(func(d *Display) {
		d.Layout, d.TitleAlign, d.TitleFont, d.TextFont = "list", "right", "lora", "nunito"
		d.ShowStyle, d.ShowABV, d.ShowIBU, d.ShowSRM, d.ShowDescription = false, false, false, true, true
		d.TitleColor, d.TextColor = "#aabbcc", "#112233"
	})
	if d := func() Display {
		db := openDB(t)
		for k, v := range good {
			if err := Set(db, k, v, t0); err != nil {
				t.Fatal(err)
			}
		}
		return mustGet(t, db)
	}(); d != want {
		t.Fatalf("good values read as %+v, want %+v", d, want)
	}
	cases := []struct {
		key, value string
		fix        func(*Display)
	}{
		{keyLayout, "tiles", func(d *Display) { d.Layout = "grid" }},
		{keyLayout, "GRID", func(d *Display) { d.Layout = "grid" }},
		{keyTitleAlign, "middle", func(d *Display) { d.TitleAlign = "left" }},
		{keyTitleFont, "comic-sans", func(d *Display) { d.TitleFont = "system" }},
		{keyTextFont, "", func(d *Display) { d.TextFont = "system" }},
		{keyShowStyle, "yes", func(d *Display) { d.ShowStyle = true }},
		{keyShowABV, "true", func(d *Display) { d.ShowABV = true }},
		{keyShowIBU, "", func(d *Display) { d.ShowIBU = true }},
		{keyShowSRM, "yes", func(d *Display) { d.ShowSRM = false }},
		{keyShowDescription, "2", func(d *Display) { d.ShowDescription = false }},
		{keyTitleColor, "red", func(d *Display) { d.TitleColor = "" }},
		{keyTitleColor, "#abc", func(d *Display) { d.TitleColor = "" }},
		{keyTextColor, "#1122334", func(d *Display) { d.TextColor = "" }},
		{keyTextColor, "#11223g", func(d *Display) { d.TextColor = "" }},
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

func TestPatchDisplayAcceptsTV(t *testing.T) {
	db := openDB(t)
	for _, v := range []string{"grid", "list", "columns"} {
		if d, err := PatchDisplay(db, DisplayPatch{Layout: ptr(v)}, t0); err != nil || d.Layout != v {
			t.Fatalf("layout %s = %+v (err %v)", v, d, err)
		}
	}
	for _, v := range []string{"left", "center", "right"} {
		if d, err := PatchDisplay(db, DisplayPatch{TitleAlign: ptr(v)}, t0); err != nil || d.TitleAlign != v {
			t.Fatalf("title_align %s = %+v (err %v)", v, d, err)
		}
	}
	for _, v := range []string{"system", "inter", "lora", "roboto-slab", "oswald", "nunito", "bebas-neue", "pacifico"} {
		if d, err := PatchDisplay(db, DisplayPatch{TitleFont: ptr(v), TextFont: ptr(v)}, t0); err != nil || d.TitleFont != v || d.TextFont != v {
			t.Fatalf("font %s = %+v (err %v)", v, d, err)
		}
	}
	all := DisplayPatch{ShowStyle: ptr(false), ShowABV: ptr(false), ShowIBU: ptr(false), ShowSRM: ptr(true), ShowDescription: ptr(true)}
	d, err := PatchDisplay(db, all, t0)
	if err != nil || d.ShowStyle || d.ShowABV || d.ShowIBU || !d.ShowSRM || !d.ShowDescription {
		t.Fatalf("switches = %+v (err %v)", d, err)
	}
	for k, want := range map[string]string{keyShowStyle: "0", keyShowABV: "0", keyShowIBU: "0", keyShowSRM: "1", keyShowDescription: "1"} {
		if v, _, _ := Get(db, k); v != want {
			t.Errorf("%s stored as %q, want %q", k, v, want)
		}
	}
	d, err = PatchDisplay(db, DisplayPatch{ShowStyle: ptr(true), ShowSRM: ptr(false)}, t0)
	if err != nil || !d.ShowStyle || d.ShowSRM {
		t.Fatalf("switches back = %+v (err %v)", d, err)
	}
	if v, _, _ := Get(db, keyShowStyle); v != "1" {
		t.Errorf("show_style stored as %q, want 1", v)
	}

	d, err = PatchDisplay(db, DisplayPatch{TitleColor: ptr("#FFAA00"), TextColor: ptr("#a1B2c3")}, t0)
	if err != nil || d.TitleColor != "#ffaa00" || d.TextColor != "#a1b2c3" {
		t.Fatalf("colors = %+v (err %v)", d, err)
	}
	if v, _, _ := Get(db, keyTitleColor); v != "#ffaa00" {
		t.Fatalf("title_color stored as %q, want #ffaa00", v)
	}
	d, err = PatchDisplay(db, DisplayPatch{TitleColor: ptr("")}, t0)
	if err != nil || d.TitleColor != "" || d.TextColor != "#a1b2c3" {
		t.Fatalf("reset title color = %+v (err %v)", d, err)
	}
	if read := mustGet(t, db); read != d {
		t.Fatalf("read back %+v, want %+v", read, d)
	}
}

func TestPatchDisplayRefusesTV(t *testing.T) {
	const fonts = "Must be one of: system, inter, lora, roboto-slab, oswald, nunito, bebas-neue, pacifico."
	const color = "Must be a color like #1a2b3c, or empty for the theme color."
	for _, c := range []struct {
		field, msg string
		patch      DisplayPatch
	}{
		{"layout", "Must be one of: grid, list, columns.", DisplayPatch{Layout: ptr("tiles")}},
		{"layout", "Must be one of: grid, list, columns.", DisplayPatch{Layout: ptr("Grid")}},
		{"title_align", "Must be one of: left, center, right.", DisplayPatch{TitleAlign: ptr("middle")}},
		{"title_font", fonts, DisplayPatch{TitleFont: ptr("comic-sans")}},
		{"text_font", fonts, DisplayPatch{TextFont: ptr("")}},
		{"title_color", color, DisplayPatch{TitleColor: ptr("red")}},
		{"title_color", color, DisplayPatch{TitleColor: ptr("#abc")}},
		{"text_color", color, DisplayPatch{TextColor: ptr("#1122334")}},
		{"text_color", color, DisplayPatch{TextColor: ptr("#11223g")}},
		{"text_color", color, DisplayPatch{TextColor: ptr("#112233\n")}},
		{"text_color", color, DisplayPatch{TextColor: ptr("#112233;}")}},
	} {
		db := openDB(t)
		// A valid field in the same patch must not be stored either (all-or-nothing).
		c.patch.ShowSRM = ptr(true)
		_, err := PatchDisplay(db, c.patch, t0)
		var fe beers.FieldErrors
		if !errors.As(err, &fe) || len(fe) != 1 || fe[c.field] != c.msg {
			t.Fatalf("patch %+v = %v, want %s: %q", c.patch, err, c.field, c.msg)
		}
		if n := countKeys(t, db); n != 0 {
			t.Fatalf("refused patch stored %d rows", n)
		}
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
