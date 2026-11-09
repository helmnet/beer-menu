// Package beers holds the beer library's rules, SQL and SRM color table, with no HTTP, so the
// API handlers and a later importer share one implementation.
package beers

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Beer statuses, as stored in beers.status. Status does not pick the artwork vessel.
const (
	StatusPlanned      = "planned"
	StatusFermenting   = "fermenting"
	StatusConditioning = "conditioning"
	StatusReady        = "ready"
)

// ErrEmptyPatch means a PATCH body named no field, so there is nothing to change.
var ErrEmptyPatch = errors.New("patch changes nothing")

// FieldErrors maps each invalid field, by its JSON name, to a user-facing message. Messages
// never echo the submitted value.
type FieldErrors map[string]string

// Error lists the invalid field names, sorted.
func (fe FieldErrors) Error() string {
	keys := make([]string, 0, len(fe))
	for k := range fe {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "invalid fields: " + strings.Join(keys, ", ")
}

// Input is the body of a beer create request. Absent and null optional fields are both nil.
// IBU is decoded as a number so a fractional value is a field error, not a decode error.
type Input struct {
	Name        string   `json:"name"`
	Style       *string  `json:"style"`
	ABV         *float64 `json:"abv"`
	IBU         *float64 `json:"ibu"`
	SRM         *float64 `json:"srm"`
	Description *string  `json:"description"`
	Brewer      *string  `json:"brewer"`
	BrewDate    *string  `json:"brew_date"`
	Status      *string  `json:"status"`
}

// Values is a validated, normalized beer: text trimmed, empty optional text nil, IBU whole.
type Values struct {
	Name        string
	Style       *string
	ABV         *float64
	IBU         *int64
	SRM         *float64
	Description *string
	Brewer      *string
	BrewDate    *string
	Status      string
}

// Normalize validates in by research R4 and returns its Values, with status ready when none
// was sent. On failure it returns FieldErrors naming every invalid field.
func (in Input) Normalize() (Values, error) {
	fe := FieldErrors{}
	v := Values{
		Name:        fe.name(in.Name),
		Style:       fe.text("style", in.Style, 100, false),
		ABV:         fe.decimal("abv", in.ABV, 25, "Must be 0 to 25 with at most one decimal place."),
		IBU:         fe.ibu(in.IBU),
		SRM:         fe.decimal("srm", in.SRM, 100, "Must be 0 to 100 with at most one decimal place."),
		Description: fe.text("description", in.Description, 2000, true),
		Brewer:      fe.text("brewer", in.Brewer, 100, false),
		BrewDate:    fe.date(in.BrewDate),
		Status:      StatusReady,
	}
	if in.Status != nil {
		v.Status = fe.status(*in.Status)
	}
	if len(fe) > 0 {
		return Values{}, fe
	}
	return v, nil
}

// Patch is the body of a beer PATCH request with merge-patch semantics: an unset field is
// kept, null clears an optional field, a value sets it. Name, status and archived refuse null.
type Patch struct {
	Name        Optional[string]  `json:"name"`
	Style       Optional[string]  `json:"style"`
	ABV         Optional[float64] `json:"abv"`
	IBU         Optional[float64] `json:"ibu"`
	SRM         Optional[float64] `json:"srm"`
	Description Optional[string]  `json:"description"`
	Brewer      Optional[string]  `json:"brewer"`
	BrewDate    Optional[string]  `json:"brew_date"`
	Status      Optional[string]  `json:"status"`
	Archived    Optional[bool]    `json:"archived"`
}

// Normalize validates p by research R4 and returns the columns to set, keyed by column name
// (equal to the JSON field name), with nil pointers meaning NULL. Archived is not in the map;
// callers read p.Archived. It returns FieldErrors naming every invalid field, or ErrEmptyPatch
// when p sets nothing.
func (p Patch) Normalize() (map[string]any, error) {
	fe := FieldErrors{}
	set := map[string]any{}
	if p.Name.Set {
		if p.Name.Value == nil {
			fe["name"] = "Name is required."
		} else {
			set["name"] = fe.name(*p.Name.Value)
		}
	}
	if p.Status.Set {
		if p.Status.Value == nil {
			fe["status"] = "Must be planned, fermenting, conditioning or ready."
		} else {
			set["status"] = fe.status(*p.Status.Value)
		}
	}
	if p.Archived.Set && p.Archived.Value == nil {
		fe["archived"] = "Must be true or false."
	}
	if p.Style.Set {
		set["style"] = fe.text("style", p.Style.Value, 100, false)
	}
	if p.ABV.Set {
		set["abv"] = fe.decimal("abv", p.ABV.Value, 25, "Must be 0 to 25 with at most one decimal place.")
	}
	if p.IBU.Set {
		set["ibu"] = fe.ibu(p.IBU.Value)
	}
	if p.SRM.Set {
		set["srm"] = fe.decimal("srm", p.SRM.Value, 100, "Must be 0 to 100 with at most one decimal place.")
	}
	if p.Description.Set {
		set["description"] = fe.text("description", p.Description.Value, 2000, true)
	}
	if p.Brewer.Set {
		set["brewer"] = fe.text("brewer", p.Brewer.Value, 100, false)
	}
	if p.BrewDate.Set {
		set["brew_date"] = fe.date(p.BrewDate.Value)
	}
	if len(fe) > 0 {
		return nil, fe
	}
	if len(set) == 0 && !p.Archived.Set {
		return nil, ErrEmptyPatch
	}
	return set, nil
}

func (fe FieldErrors) name(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		fe["name"] = "Name is required."
	case utf8.RuneCountInString(s) > 100:
		fe["name"] = "Must be at most 100 characters."
	case hasControl(s, false):
		fe["name"] = "Must not contain control characters."
	}
	return s
}

// text trims an optional text field and returns nil when it is empty. multiline allows
// newline and tab, the only control characters a description may hold.
func (fe FieldErrors) text(field string, p *string, maxLen int, multiline bool) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	if utf8.RuneCountInString(s) > maxLen {
		fe[field] = "Must be at most " + strconv.Itoa(maxLen) + " characters."
	} else if hasControl(s, multiline) {
		fe[field] = "Must not contain control characters."
	}
	return &s
}

// decimal checks 0 <= *p <= maxVal with at most one decimal place. The tolerance absorbs binary
// rounding, so 6.7 passes although 6.7*10 is not exactly 67.
func (fe FieldErrors) decimal(field string, p *float64, maxVal float64, msg string) *float64 {
	if p == nil {
		return nil
	}
	x := *p
	if x < 0 || x > maxVal || math.Abs(x*10-math.Round(x*10)) >= 1e-9 {
		fe[field] = msg
	}
	return p
}

func (fe FieldErrors) ibu(p *float64) *int64 {
	if p == nil {
		return nil
	}
	if *p < 0 || *p > 200 || *p != math.Trunc(*p) {
		fe["ibu"] = "Must be a whole number from 0 to 200."
		return nil
	}
	n := int64(*p)
	return &n
}

// date accepts a calendar date as YYYY-MM-DD, future dates included; empty means none.
func (fe FieldErrors) date(p *string) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		fe["brew_date"] = "Must be a date as YYYY-MM-DD."
	}
	return &s
}

func (fe FieldErrors) status(s string) string {
	switch s {
	case StatusPlanned, StatusFermenting, StatusConditioning, StatusReady:
	default:
		fe["status"] = "Must be planned, fermenting, conditioning or ready."
	}
	return s
}

func hasControl(s string, multiline bool) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\t'))
	})
}
