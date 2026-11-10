package beers

import (
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// importedName replaces a name that is empty after cleaning.
const importedName = "Brewfather batch"

// Sanitize cleans imported data so it always passes Normalize, rather than refusing it, because
// the owner cannot fix the source data from here (research R6). Text is trimmed, stripped of
// control characters (a description keeps newlines and tabs) and shortened by runes to its
// limit; ABV and SRM are rounded to one decimal and IBU to a whole number; a number still out
// of range, a negative number or a bad brew date becomes nil; an empty name becomes
// "Brewfather batch"; an unknown status becomes planned and a missing one ready. It returns the
// JSON names of the fields it changed, sorted and never nil. Trimming surrounding whitespace
// and blank text becoming nil are not changes: Normalize does both for typed input too.
func Sanitize(in Input) (Values, []string) {
	adjusted := []string{}
	note := func(field string, changed bool) {
		if changed {
			adjusted = append(adjusted, field)
		}
	}
	v := Values{Status: StatusReady}

	trimmed := strings.TrimSpace(in.Name)
	v.Name = cleanText(in.Name, maxNameLen, false)
	if v.Name == "" {
		v.Name = importedName
	}
	note("name", v.Name != trimmed)

	var changed bool
	v.Style, changed = cleanOptional(in.Style, maxStyleLen, false)
	note("style", changed)
	v.Description, changed = cleanOptional(in.Description, maxDescriptionLen, true)
	note("description", changed)
	v.Brewer, changed = cleanOptional(in.Brewer, maxBrewerLen, false)
	note("brewer", changed)

	v.ABV, changed = roundTo(in.ABV, 10, maxABV)
	note("abv", changed)
	v.SRM, changed = roundTo(in.SRM, 10, maxSRM)
	note("srm", changed)
	var ibu *float64
	ibu, changed = roundTo(in.IBU, 1, maxIBU)
	note("ibu", changed)
	if ibu != nil {
		n := int64(*ibu)
		v.IBU = &n
	}

	if in.BrewDate != nil {
		if s := strings.TrimSpace(*in.BrewDate); s != "" {
			_, err := time.Parse(time.DateOnly, s)
			note("brew_date", err != nil)
			if err == nil {
				v.BrewDate = &s
			}
		}
	}

	if in.Status != nil {
		fe := FieldErrors{}
		v.Status = fe.status(*in.Status)
		if len(fe) > 0 {
			v.Status = StatusPlanned
			note("status", true)
		}
	}

	slices.Sort(adjusted)
	return v, adjusted
}

// cleanText drops invalid UTF-8 and refused control characters, trims, and cuts s to maxLen
// runes, trimming again so a cut never leaves trailing space.
func cleanText(s string, maxLen int, multiline bool) string {
	s = strings.Map(func(r rune) rune {
		if isControl(r, multiline) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxLen {
		s = strings.TrimSpace(string([]rune(s)[:maxLen]))
	}
	return s
}

// cleanOptional is cleanText for an optional field: nil or blank gives nil, unreported.
func cleanOptional(p *string, maxLen int, multiline bool) (*string, bool) {
	if p == nil {
		return nil, false
	}
	trimmed := strings.TrimSpace(*p)
	if trimmed == "" {
		return nil, false
	}
	s := cleanText(*p, maxLen, multiline)
	if s == "" {
		return nil, true
	}
	return &s, s != trimmed
}

// roundTo rounds *p to 1/scale and returns nil for a negative value or one outside 0 to maxVal
// after rounding; NaN and infinities fail the range test too. It reports whether the value
// changed.
func roundTo(p *float64, scale, maxVal float64) (*float64, bool) {
	if p == nil {
		return nil, false
	}
	r := math.Round(*p*scale) / scale
	if *p < 0 || !(r >= 0 && r <= maxVal) {
		return nil, true
	}
	return &r, r != *p
}
