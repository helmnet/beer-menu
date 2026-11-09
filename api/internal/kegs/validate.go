// Package kegs holds the rules and SQL for taps, kegs and the keg type table, with no HTTP.
// Taps and kegs share invariants (one current keg per tap, a tap with a keg cannot be removed),
// so they live in one package. Volumes are litres rounded to 0.01.
package kegs

import (
	"errors"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// ErrNotFound means no tap or keg has the given id (404 not_found).
var ErrNotFound = errors.New("not found")

// ErrTapOccupied means a keg was put on a tap that already holds a current keg (409
// tap_occupied).
var ErrTapOccupied = errors.New("tap already has a keg")

// ErrTapHasKeg means a tap holding a current keg was asked to be removed (409 tap_has_keg).
var ErrTapHasKeg = errors.New("tap has a keg")

// ErrLastTap means the only tap was asked to be removed (409 last_tap).
var ErrLastTap = errors.New("last tap")

// ErrKegKicked means a kicked keg was asked to change, other than a repeat kick or delete (409
// keg_kicked).
var ErrKegKicked = errors.New("keg is kicked")

// maxVolumeL bounds keg sizes and pours in litres.
const maxVolumeL = 100

// roundL rounds a volume in litres to 0.01, so clients may send converted gallons as they are.
func roundL(x float64) float64 {
	r := math.Round(x*100) / 100
	if r == 0 {
		return 0 // never -0, which would marshal as "-0"
	}
	return r
}

// normalizeLabel trims a tap label and checks it is 1 to 50 code points with no control
// characters. It returns the trimmed label and a user-facing message, empty when valid.
func normalizeLabel(s string) (string, string) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return s, "Label is required."
	case utf8.RuneCountInString(s) > 50:
		return s, "Must be at most 50 characters."
	case strings.ContainsFunc(s, unicode.IsControl):
		return s, "Must not contain control characters."
	}
	return s, ""
}

// checkSize rounds a keg size and records a size_l error in fe unless it is over 0 and at
// most 100 litres.
func checkSize(fe beers.FieldErrors, x float64) float64 {
	x = roundL(x)
	if x <= 0 || x > maxVolumeL {
		fe["size_l"] = "Must be more than 0 and at most 100 litres."
	}
	return x
}

// checkRemaining rounds a remaining volume and records a remaining_l error in fe unless it is
// 0 to size litres.
func checkRemaining(fe beers.FieldErrors, x, size float64) float64 {
	x = roundL(x)
	if x < 0 || x > size {
		fe["remaining_l"] = "Must be 0 to the keg size."
	}
	return x
}

// checkAmount rounds a pour and records an amount_l error in fe unless it is over 0 and at most
// 100 litres.
func checkAmount(fe beers.FieldErrors, x float64) float64 {
	x = roundL(x)
	if x <= 0 || x > maxVolumeL {
		fe["amount_l"] = "Must be more than 0 and at most 100 litres."
	}
	return x
}

// checkType returns the keg type named key, recording a type error in fe when there is none.
func checkType(fe beers.FieldErrors, key string) KegType {
	kt, ok := TypeByKey(key)
	if !ok {
		fe["type"] = "Must be one of the keg types."
	}
	return kt
}
