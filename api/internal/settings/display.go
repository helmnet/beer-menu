package settings

import (
	"database/sql"
	"regexp"
	"slices"
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

	keyLayout          = "display.layout"
	keyTitleAlign      = "display.title_align"
	keyTitleFont       = "display.title_font"
	keyTextFont        = "display.text_font"
	keyShowStyle       = "display.show_style"
	keyShowABV         = "display.show_abv"
	keyShowIBU         = "display.show_ibu"
	keyShowSRM         = "display.show_srm"
	keyShowDescription = "display.show_description"
	keyTitleColor      = "display.title_color"
	keyTextColor       = "display.text_color"
)

// displayKeys is every display settings key, read together by GetDisplay.
var displayKeys = []string{
	keyTitle, keyTheme, keyUnits, keyPageSeconds,
	keyLayout, keyTitleAlign, keyTitleFont, keyTextFont,
	keyShowStyle, keyShowABV, keyShowIBU, keyShowSRM, keyShowDescription,
	keyTitleColor, keyTextColor,
}

// Allowed values of the feature 10 choices, in the order their error message lists them.
var (
	layouts     = []string{"grid", "list", "columns"}
	titleAligns = []string{"left", "center", "right"}
	fontKeys    = []string{"system", "inter", "lora", "roboto-slab", "oswald", "nunito", "bebas-neue", "pacifico"}
)

// colorPattern is the only color form stored. It is narrow on purpose: a stored color reaches
// the TV's CSS, so it must never carry CSS syntax (research R3).
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Limits of the display settings.
const (
	maxTitleLen    = 60
	minPageSeconds = 5
	maxPageSeconds = 300
)

// PageSecondsMessage is the user-facing error for a page_seconds that is not a whole number from
// 5 to 300. The HTTP layer reuses it for a page_seconds that is not a JSON integer.
const PageSecondsMessage = "Must be a whole number from 5 to 300."

// SwitchMessage is the user-facing error for a show_* switch that is not a JSON boolean. Only the
// HTTP layer can see that, as DisplayPatch holds the switches as *bool.
const SwitchMessage = "Must be true or false."

// colorMessage is the user-facing error for a color that is not "" or #rrggbb.
const colorMessage = "Must be a color like #1a2b3c, or empty for the theme color."

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
	// Layout is the TV layout: "grid", "list" or "columns".
	Layout string `json:"layout"`
	// TitleAlign is the title's alignment: "left", "center" or "right".
	TitleAlign string `json:"title_align"`
	// TitleFont and TextFont are font keys (fontKeys) for the title and for all other TV text.
	TitleFont string `json:"title_font"`
	TextFont  string `json:"text_font"`
	// ShowStyle to ShowDescription switch each detail on tap cards; on deck cards ignore them.
	ShowStyle       bool `json:"show_style"`
	ShowABV         bool `json:"show_abv"`
	ShowIBU         bool `json:"show_ibu"`
	ShowSRM         bool `json:"show_srm"`
	ShowDescription bool `json:"show_description"`
	// TitleColor and TextColor are lowercase "#rrggbb", or "" for the theme's own color.
	TitleColor string `json:"title_color"`
	TextColor  string `json:"text_color"`
}

// Public is the subset of Display the public GET /api/menu returns as display: every TV setting,
// never units.
type Public struct {
	Title           string `json:"title"`
	Theme           string `json:"theme"`
	PageSeconds     int    `json:"page_seconds"`
	Layout          string `json:"layout"`
	TitleAlign      string `json:"title_align"`
	TitleFont       string `json:"title_font"`
	TextFont        string `json:"text_font"`
	ShowStyle       bool   `json:"show_style"`
	ShowABV         bool   `json:"show_abv"`
	ShowIBU         bool   `json:"show_ibu"`
	ShowSRM         bool   `json:"show_srm"`
	ShowDescription bool   `json:"show_description"`
	TitleColor      string `json:"title_color"`
	TextColor       string `json:"text_color"`
}

// DisplayPatch names the display settings to change; a nil field is left as it is.
type DisplayPatch struct {
	Title           *string
	Theme           *string
	Units           *string
	PageSeconds     *int
	Layout          *string
	TitleAlign      *string
	TitleFont       *string
	TextFont        *string
	ShowStyle       *bool
	ShowABV         *bool
	ShowIBU         *bool
	ShowSRM         *bool
	ShowDescription *bool
	TitleColor      *string
	TextColor       *string
}

// Default is every display setting at its default: the TV as it looked before these settings.
var Default = Display{
	Title: "", Theme: "dark", Units: "us", PageSeconds: 15,
	Layout: "grid", TitleAlign: "left", TitleFont: "system", TextFont: "system",
	ShowStyle: true, ShowABV: true, ShowIBU: true, ShowSRM: false, ShowDescription: false,
	TitleColor: "", TextColor: "",
}

// Public returns the public subset of d.
func (d Display) Public() Public {
	return Public{
		Title: d.Title, Theme: d.Theme, PageSeconds: d.PageSeconds,
		Layout: d.Layout, TitleAlign: d.TitleAlign, TitleFont: d.TitleFont, TextFont: d.TextFont,
		ShowStyle: d.ShowStyle, ShowABV: d.ShowABV, ShowIBU: d.ShowIBU, ShowSRM: d.ShowSRM,
		ShowDescription: d.ShowDescription, TitleColor: d.TitleColor, TextColor: d.TextColor,
	}
}

// GetDisplay returns every display setting. A missing row, or a stored value that fails
// validation, reads as its default, so a bad value never fails the read. On a database error it
// returns Default with the error.
func GetDisplay(q Querier) (Display, error) {
	d := Default
	vals := map[string]string{}
	for _, k := range displayKeys {
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
	readChoice(vals, keyLayout, layouts, &d.Layout)
	readChoice(vals, keyTitleAlign, titleAligns, &d.TitleAlign)
	readChoice(vals, keyTitleFont, fontKeys, &d.TitleFont)
	readChoice(vals, keyTextFont, fontKeys, &d.TextFont)
	readSwitch(vals, keyShowStyle, &d.ShowStyle)
	readSwitch(vals, keyShowABV, &d.ShowABV)
	readSwitch(vals, keyShowIBU, &d.ShowIBU)
	readSwitch(vals, keyShowSRM, &d.ShowSRM)
	readSwitch(vals, keyShowDescription, &d.ShowDescription)
	readColor(vals, keyTitleColor, &d.TitleColor)
	readColor(vals, keyTextColor, &d.TextColor)
	return d, nil
}

// PatchDisplay validates every field p names and, if all are valid, writes them in one
// transaction with updated_at now and returns every current value. Switches are stored as "1" or
// "0" and colors lowercase. Keys p does not name are
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
	patchChoice(fe, set, "layout", keyLayout, p.Layout, layouts)
	patchChoice(fe, set, "title_align", keyTitleAlign, p.TitleAlign, titleAligns)
	patchChoice(fe, set, "title_font", keyTitleFont, p.TitleFont, fontKeys)
	patchChoice(fe, set, "text_font", keyTextFont, p.TextFont, fontKeys)
	patchSwitch(set, keyShowStyle, p.ShowStyle)
	patchSwitch(set, keyShowABV, p.ShowABV)
	patchSwitch(set, keyShowIBU, p.ShowIBU)
	patchSwitch(set, keyShowSRM, p.ShowSRM)
	patchSwitch(set, keyShowDescription, p.ShowDescription)
	patchColor(fe, set, "title_color", keyTitleColor, p.TitleColor)
	patchColor(fe, set, "text_color", keyTextColor, p.TextColor)
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

// readChoice sets *dst to the stored value of key when it is one of allowed.
func readChoice(vals map[string]string, key string, allowed []string, dst *string) {
	if v, ok := vals[key]; ok && slices.Contains(allowed, v) {
		*dst = v
	}
}

// readSwitch sets *dst from the stored "1" or "0" of key; anything else keeps the default.
func readSwitch(vals map[string]string, key string, dst *bool) {
	switch vals[key] {
	case "1":
		*dst = true
	case "0":
		*dst = false
	}
}

// readColor sets *dst to the stored color of key when it is "" or #rrggbb.
func readColor(vals map[string]string, key string, dst *string) {
	if v, ok := vals[key]; ok && validColor(v) {
		*dst = strings.ToLower(v)
	}
}

// patchChoice records p for key in set, and a field error under name when it is not one of
// allowed. A nil p is not named by the patch.
func patchChoice(fe beers.FieldErrors, set map[string]string, name, key string, p *string, allowed []string) {
	if p == nil {
		return
	}
	if !slices.Contains(allowed, *p) {
		fe[name] = "Must be one of: " + strings.Join(allowed, ", ") + "."
	}
	set[key] = *p
}

// patchSwitch records p for key in set as "1" or "0". A nil p is not named by the patch.
func patchSwitch(set map[string]string, key string, p *bool) {
	if p == nil {
		return
	}
	set[key] = "0"
	if *p {
		set[key] = "1"
	}
}

// patchColor records p, lowercased, for key in set, and a field error under name when it is not
// "" or #rrggbb. A nil p is not named by the patch.
func patchColor(fe beers.FieldErrors, set map[string]string, name, key string, p *string) {
	if p == nil {
		return
	}
	if !validColor(*p) {
		fe[name] = colorMessage
	}
	set[key] = strings.ToLower(*p)
}

func validColor(s string) bool { return s == "" || colorPattern.MatchString(s) }
