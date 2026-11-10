package beers

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// asInput turns cleaned Values back into an Input, as a client would send them.
func asInput(v Values) Input {
	in := Input{Name: v.Name, Style: v.Style, ABV: v.ABV, SRM: v.SRM, Description: v.Description, Brewer: v.Brewer,
		BrewDate: v.BrewDate, Status: &v.Status}
	if v.IBU != nil {
		in.IBU = ptr(float64(*v.IBU))
	}
	return in
}

func TestSanitize(t *testing.T) {
	long := strings.Repeat("ü", 150)
	longDesc := strings.Repeat("ß", 2500)
	cases := []struct {
		name     string
		in       Input
		want     Values
		adjusted []string
	}{
		{"clean values unchanged",
			Input{Name: "Pale", Style: ptr("APA"), ABV: ptr(5.4), IBU: ptr(35.0), SRM: ptr(6.5), Description: ptr("Line\n\tTwo"),
				Brewer: ptr("Helm"), BrewDate: ptr("2026-10-01"), Status: ptr("fermenting")},
			Values{Name: "Pale", Style: ptr("APA"), ABV: ptr(5.4), IBU: ptr[int64](35), SRM: ptr(6.5), Description: ptr("Line\n\tTwo"),
				Brewer: ptr("Helm"), BrewDate: ptr("2026-10-01"), Status: "fermenting"},
			[]string{}},
		{"trimming and blanks are not adjustments",
			Input{Name: "  Pale \n", Style: ptr("  "), Description: ptr("\n notes \n"), BrewDate: ptr(" ")},
			Values{Name: "Pale", Description: ptr("notes"), Status: "ready"},
			[]string{}},
		{"control characters stripped",
			Input{Name: "Pa\x00le\x1b", Style: ptr("A\tP\nA"), Brewer: ptr("He\u0085lm"), Description: ptr("a\x07b\n\tc\r")},
			Values{Name: "Pale", Style: ptr("APA"), Brewer: ptr("Helm"), Description: ptr("ab\n\tc"), Status: "ready"},
			[]string{"brewer", "description", "name", "style"}},
		{"shortened on rune boundaries",
			Input{Name: long, Style: ptr(long), Brewer: ptr(long), Description: ptr(longDesc)},
			Values{Name: long[:200], Style: ptr(long[:200]), Brewer: ptr(long[:200]), Description: ptr(longDesc[:4000]), Status: "ready"},
			[]string{"brewer", "description", "name", "style"}},
		{"cut then trimmed",
			Input{Name: strings.Repeat("a", 99) + " b"},
			Values{Name: strings.Repeat("a", 99), Status: "ready"},
			[]string{"name"}},
		{"rounded",
			Input{Name: "x", ABV: ptr(6.73), IBU: ptr(35.6), SRM: ptr(7.25)},
			Values{Name: "x", ABV: ptr(6.7), IBU: ptr[int64](36), SRM: ptr(7.3), Status: "ready"},
			[]string{"abv", "ibu", "srm"}},
		{"rounding up to the limit is kept",
			Input{Name: "x", ABV: ptr(24.96), SRM: ptr(100.04), IBU: ptr(200.4)},
			Values{Name: "x", ABV: ptr(25.0), SRM: ptr(100.0), IBU: ptr[int64](200), Status: "ready"},
			[]string{"abv", "ibu", "srm"}},
		{"out of range emptied",
			Input{Name: "x", ABV: ptr(25.1), SRM: ptr(100.1), IBU: ptr(200.6), BrewDate: ptr("2026-13-01")},
			Values{Name: "x", Status: "ready"},
			[]string{"abv", "brew_date", "ibu", "srm"}},
		{"negative emptied",
			Input{Name: "x", ABV: ptr(-0.1), SRM: ptr(-5.0), IBU: ptr(-1.0)},
			Values{Name: "x", Status: "ready"},
			[]string{"abv", "ibu", "srm"}},
		{"not a number emptied",
			Input{Name: "x", ABV: ptr(math.NaN()), SRM: ptr(math.Inf(1)), IBU: ptr(math.Inf(-1))},
			Values{Name: "x", Status: "ready"},
			[]string{"abv", "ibu", "srm"}},
		{"bad date formats emptied",
			Input{Name: "x", BrewDate: ptr("1/2/2026")},
			Values{Name: "x", Status: "ready"},
			[]string{"brew_date"}},
		{"empty name",
			Input{Name: ""},
			Values{Name: "Brewfather batch", Status: "ready"},
			[]string{"name"}},
		{"all-control name",
			Input{Name: "\x00\x01 \x02"},
			Values{Name: "Brewfather batch", Status: "ready"},
			[]string{"name"}},
		{"unknown status becomes planned",
			Input{Name: "x", Status: ptr("brewing")},
			Values{Name: "x", Status: "planned"},
			[]string{"status"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, adjusted := Sanitize(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Values = %s, want %s", show(got), show(c.want))
			}
			if !reflect.DeepEqual(adjusted, c.adjusted) {
				t.Errorf("adjusted = %v, want %v", adjusted, c.adjusted)
			}
		})
	}
}

// show prints v with pointers followed, for readable failures.
func show(v Values) string {
	d := func(p any) any {
		switch p := p.(type) {
		case *string:
			if p != nil {
				return *p
			}
		case *float64:
			if p != nil {
				return *p
			}
		case *int64:
			if p != nil {
				return *p
			}
		}
		return nil
	}
	return fmt.Sprintf("{%q style=%q abv=%v ibu=%v srm=%v desc=%q brewer=%q date=%q %s}", v.Name, d(v.Style), d(v.ABV), d(v.IBU),
		d(v.SRM), d(v.Description), d(v.Brewer), d(v.BrewDate), v.Status)
}

// TestSanitizePassesNormalize feeds ugly values through Sanitize and checks the result always
// passes Normalize unchanged, so imported data can never break the beer rules (research R6).
func TestSanitizePassesNormalize(t *testing.T) {
	ugly := []string{"", " ", "\x00", "\n\n", "\u200b", "\ufeffName", "a\x7fb", "\xff\xfe", "tab\tin\tname",
		strings.Repeat("🍺", 101), strings.Repeat("e\u0301", 120), strings.Repeat(" x", 1500), strings.Repeat("a\n", 1200),
		"2026-02-30", "2026-02-28", "0000-01-01", "9999-12-31T00:00:00Z", "<script>alert(1)</script>"}
	nums := []float64{-1, 0, 0.04, 0.05, 4.449, 4.45, 12.3456, 24.95, 24.96, 25, 25.04, 99.96, 100, 100.05, 199.5, 200.49,
		200.5, 1e308, math.NaN(), math.Inf(1), math.Inf(-1), math.SmallestNonzeroFloat64}
	statuses := []*string{nil, ptr("ready"), ptr("planned"), ptr("Brewing"), ptr("")}
	for i, s := range ugly {
		for j, n := range nums {
			in := Input{Name: s, Style: ptr(s), Description: ptr(s), Brewer: ptr(s), BrewDate: ptr(s),
				ABV: ptr(n), IBU: ptr(n), SRM: ptr(n), Status: statuses[(i+j)%len(statuses)]}
			v, _ := Sanitize(in)
			again, err := asInput(v).Normalize()
			if err != nil {
				t.Fatalf("Sanitize(%q, %v) = %s fails Normalize: %v", s, n, show(v), err)
			}
			if !reflect.DeepEqual(again, v) {
				t.Fatalf("Normalize changed Sanitize(%q, %v): %s, want %s", s, n, show(again), show(v))
			}
			for _, p := range []*string{&v.Name, v.Style, v.Description, v.Brewer} {
				if p != nil && !utf8.ValidString(*p) {
					t.Fatalf("Sanitize(%q) left invalid UTF-8 %q", s, *p)
				}
			}
		}
	}
}
