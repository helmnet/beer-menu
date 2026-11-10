package settings

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// Settings keys of the display settings.
const (
	keyTitle       = "display.title"
	keyTheme       = "display.theme"
	keyUnits       = "display.units"
	keyPageSeconds = "display.page_seconds"
)

// Limits of the display settings.
const (
	maxTitleLen    = 60
	minPageSeconds = 5
	maxPageSeconds = 300
)

// PageSecondsMessage is the user-facing error for a page_seconds that is not a whole number from
// 5 to 300. The HTTP layer reuses it for a page_seconds that is not a JSON integer.
const PageSecondsMessage = "Must be a whole number from 5 to 300."

// Display is every display setting, as GET and PATCH /api/settings return them.
type Display struct {
	// Title is shown at the top of every TV page; "" means no title row.
	Title string `json:"title"`
	// Theme is the TV theme, "dark" or "light".
	Theme string `json:"theme"`
	// Units is how the admin app shows and takes volumes, "us" or "metric". Never public.
	Units string `json:"units"`
	// PageSeconds is how long each TV page shows, 5 to 300.
	PageSeconds int `json:"page_seconds"`
}

// Public is the subset of Display the public GET /api/menu returns as display: no units.
type Public struct {
	Title       string `json:"title"`
	Theme       string `json:"theme"`
	PageSeconds int    `json:"page_seconds"`
}

// DisplayPatch names the display settings to change; a nil field is left as it is.
type DisplayPatch struct {
	Title       *string
	Theme       *string
	Units       *string
	PageSeconds *int
}

// Default is every display setting at its default.
var Default = Display{Title: "", Theme: "dark", Units: "us", PageSeconds: 15}

// Public returns the public subset of d.
func (d Display) Public() Public {
	return Public{Title: d.Title, Theme: d.Theme, PageSeconds: d.PageSeconds}
}

// GetDisplay returns every display setting. A missing row, or a stored value that fails
// validation, reads as its default, so a bad value never fails the read. On a database error it
// returns Default with the error.
func GetDisplay(q Querier) (Display, error) {
	d := Default
	vals := map[string]string{}
	for _, k := range []string{keyTitle, keyTheme, keyUnits, keyPageSeconds} {
		v, ok, err := Get(q, k)
		if err != nil {
			return Default, err
		}
		if ok {
			vals[k] = v
		}
	}
	if v, ok := vals[keyTitle]; ok {
		if s, msg := normalizeTitle(v); msg == "" {
			d.Title = s
		}
	}
	if v, ok := vals[keyTheme]; ok && validTheme(v) {
		d.Theme = v
	}
	if v, ok := vals[keyUnits]; ok && validUnits(v) {
		d.Units = v
	}
	if n, err := strconv.Atoi(vals[keyPageSeconds]); err == nil && validPageSeconds(n) {
		d.PageSeconds = n
	}
	return d, nil
}

// PatchDisplay validates every field p names and, if all are valid, writes them in one
// transaction with updated_at now and returns all four current values. Keys p does not name are
// not touched. It returns beers.FieldErrors keyed by JSON name, writing nothing, when any field
// is invalid, and beers.ErrEmptyPatch when p names nothing.
func PatchDisplay(db *sql.DB, p DisplayPatch, now time.Time) (Display, error) {
	fe := beers.FieldErrors{}
	set := map[string]string{}
	if p.Title != nil {
		s, msg := normalizeTitle(*p.Title)
		if msg != "" {
			fe["title"] = msg
		}
		set[keyTitle] = s
	}
	if p.Theme != nil {
		if !validTheme(*p.Theme) {
			fe["theme"] = "Must be dark or light."
		}
		set[keyTheme] = *p.Theme
	}
	if p.Units != nil {
		if !validUnits(*p.Units) {
			fe["units"] = "Must be us or metric."
		}
		set[keyUnits] = *p.Units
	}
	if p.PageSeconds != nil {
		if !validPageSeconds(*p.PageSeconds) {
			fe["page_seconds"] = PageSecondsMessage
		}
		set[keyPageSeconds] = strconv.Itoa(*p.PageSeconds)
	}
	if len(fe) > 0 {
		return Display{}, fe
	}
	if len(set) == 0 {
		return Display{}, beers.ErrEmptyPatch
	}
	tx, err := db.Begin()
	if err != nil {
		return Display{}, err
	}
	defer tx.Rollback()
	for k, v := range set {
		if err := Set(tx, k, v, now); err != nil {
			return Display{}, err
		}
	}
	d, err := GetDisplay(tx)
	if err != nil {
		return Display{}, err
	}
	return d, tx.Commit()
}

// normalizeTitle trims a title and checks it is at most 60 code points with no control
// characters. It returns the trimmed title and a user-facing message, empty when valid. Same
// rule as tap labels, but empty is allowed.
func normalizeTitle(s string) (string, string) {
	s = strings.TrimSpace(s)
	switch {
	case utf8.RuneCountInString(s) > maxTitleLen:
		return s, "Must be at most 60 characters."
	case strings.ContainsFunc(s, unicode.IsControl):
		return s, "Must not contain control characters."
	}
	return s, ""
}

func validTheme(s string) bool { return s == "dark" || s == "light" }

func validUnits(s string) bool { return s == "us" || s == "metric" }

func validPageSeconds(n int) bool { return n >= minPageSeconds && n <= maxPageSeconds }
