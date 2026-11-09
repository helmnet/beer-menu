package beers

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// fieldKeys returns the sorted keys of err, which must be a FieldErrors (or nil).
func fieldKeys(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var fe FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("error %v is not FieldErrors", err)
	}
	keys := make([]string, 0, len(fe))
	for k, msg := range fe {
		if msg == "" {
			t.Errorf("field %s has an empty message", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestInputNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		want string // comma-separated FieldErrors keys; "" means valid
	}{
		{"name only", Input{Name: "Pale"}, ""},
		{"empty name", Input{Name: ""}, "name"},
		{"blank name", Input{Name: "   "}, "name"},
		{"name 101 code points", Input{Name: strings.Repeat("é", 101)}, "name"},
		{"name control char", Input{Name: "Pa\x07le"}, "name"},
		{"name newline", Input{Name: "Pa\nle"}, "name"},
		{"name 100 emoji", Input{Name: strings.Repeat("🍺", 100)}, ""},
		{"style 100", Input{Name: "x", Style: ptr(strings.Repeat("s", 100))}, ""},
		{"style 101", Input{Name: "x", Style: ptr(strings.Repeat("s", 101))}, "style"},
		{"style control", Input{Name: "x", Style: ptr("I\tPA")}, "style"},
		{"brewer 101", Input{Name: "x", Brewer: ptr(strings.Repeat("b", 101))}, "brewer"},
		{"brewer control", Input{Name: "x", Brewer: ptr("a\x00b")}, "brewer"},
		{"description 2000 multi-byte", Input{Name: "x", Description: ptr(strings.Repeat("ü", 2000))}, ""},
		{"description 2001", Input{Name: "x", Description: ptr(strings.Repeat("ü", 2001))}, "description"},
		{"description newline and tab", Input{Name: "x", Description: ptr("a\n\tb")}, ""},
		{"description carriage return", Input{Name: "x", Description: ptr("a\r\nb")}, "description"},
		{"description NUL", Input{Name: "x", Description: ptr("a\x00b")}, "description"},
		{"abv -0.1", Input{Name: "x", ABV: ptr(-0.1)}, "abv"},
		{"abv 25.1", Input{Name: "x", ABV: ptr(25.1)}, "abv"},
		{"abv 6.75", Input{Name: "x", ABV: ptr(6.75)}, "abv"},
		{"abv 0", Input{Name: "x", ABV: ptr(0.0)}, ""},
		{"abv 25", Input{Name: "x", ABV: ptr(25.0)}, ""},
		{"abv 6.7", Input{Name: "x", ABV: ptr(6.7)}, ""},
		{"ibu -1", Input{Name: "x", IBU: ptr(-1.0)}, "ibu"},
		{"ibu 201", Input{Name: "x", IBU: ptr(201.0)}, "ibu"},
		{"ibu 55.5", Input{Name: "x", IBU: ptr(55.5)}, "ibu"},
		{"ibu 0 and 200", Input{Name: "x", IBU: ptr(200.0)}, ""},
		{"srm 100.1", Input{Name: "x", SRM: ptr(100.1)}, "srm"},
		{"srm 4.25", Input{Name: "x", SRM: ptr(4.25)}, "srm"},
		{"srm -1", Input{Name: "x", SRM: ptr(-1.0)}, "srm"},
		{"srm 100 and 4.5", Input{Name: "x", SRM: ptr(100.0)}, ""},
		{"brew_date Feb 30", Input{Name: "x", BrewDate: ptr("2026-02-30")}, "brew_date"},
		{"brew_date no padding", Input{Name: "x", BrewDate: ptr("2026-1-5")}, "brew_date"},
		{"brew_date US", Input{Name: "x", BrewDate: ptr("01/05/2026")}, "brew_date"},
		{"brew_date future", Input{Name: "x", BrewDate: ptr("2099-12-31")}, ""},
		{"status brewing", Input{Name: "x", Status: ptr("brewing")}, "status"},
		{"status empty", Input{Name: "x", Status: ptr("")}, "status"},
		{"several", Input{Name: "", ABV: ptr(150.0), Status: ptr("x")}, "abv,name,status"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.in.Normalize()
			if got := strings.Join(fieldKeys(t, err), ","); got != c.want {
				t.Fatalf("FieldErrors keys = %q, want %q", got, c.want)
			}
		})
	}
	for _, s := range []string{"planned", "fermenting", "conditioning", "ready"} {
		if _, err := (Input{Name: "x", Status: ptr(s)}).Normalize(); err != nil {
			t.Errorf("status %s: %v", s, err)
		}
	}
}

func TestInputNormalizeValues(t *testing.T) {
	v, err := Input{Name: "  Pale  ", Style: ptr("  "), Brewer: ptr(" Helm "), Description: ptr("\n line \n"), BrewDate: ptr(" ")}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Pale" || v.Style != nil || v.Brewer == nil || *v.Brewer != "Helm" || v.Description == nil || *v.Description != "line" || v.BrewDate != nil {
		t.Fatalf("values = %+v", v)
	}
	if v.Status != "ready" {
		t.Fatalf("default status = %q, want ready", v.Status)
	}
	v, err = Input{Name: "x", IBU: ptr(55.0)}.Normalize()
	if err != nil || v.IBU == nil || *v.IBU != 55 {
		t.Fatalf("ibu = %v (err %v), want 55", v.IBU, err)
	}
}

func TestPatchNormalize(t *testing.T) {
	decode := func(s string) Patch {
		t.Helper()
		var p Patch
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			t.Fatalf("decode %s: %v", s, err)
		}
		return p
	}
	cases := []struct {
		body string
		want string
	}{
		{`{"name":null}`, "name"},
		{`{"status":null}`, "status"},
		{`{"archived":null}`, "archived"},
		{`{"name":" "}`, "name"},
		{`{"abv":150,"srm":4.25}`, "abv,srm"},
		{`{"style":null,"abv":null,"ibu":null,"srm":null,"description":null,"brewer":null,"brew_date":null}`, ""},
		{`{"archived":true}`, ""},
	}
	for _, c := range cases {
		_, err := decode(c.body).Normalize()
		if got := strings.Join(fieldKeys(t, err), ","); got != c.want {
			t.Errorf("%s: FieldErrors keys = %q, want %q", c.body, got, c.want)
		}
	}
	if _, err := decode(`{}`).Normalize(); !errors.Is(err, ErrEmptyPatch) {
		t.Errorf("{}: err = %v, want ErrEmptyPatch", err)
	}

	set, err := decode(`{"name":" New ","abv":5.4,"style":null,"status":"fermenting"}`).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 4 || set["name"] != "New" || *(set["abv"].(*float64)) != 5.4 || set["style"].(*string) != nil || set["status"] != "fermenting" {
		t.Fatalf("set = %#v", set)
	}
}
