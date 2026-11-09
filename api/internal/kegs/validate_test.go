package kegs

import (
	"sort"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// keys returns fe's keys sorted and joined by commas.
func keys(fe beers.FieldErrors) string {
	var ks []string
	for k := range fe {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, ",")
}

func TestRoundL(t *testing.T) {
	for in, want := range map[float64]float64{18.927: 18.93, 0.004: 0, 19.999: 20, 0.005: 0.01, 58.674: 58.67} {
		if got := roundL(in); got != want {
			t.Errorf("roundL(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestNormalizeLabel(t *testing.T) {
	for _, bad := range []string{"", "   ", strings.Repeat("a", 51), "a\x00", "a\nb"} {
		if _, msg := normalizeLabel(bad); msg == "" {
			t.Errorf("normalizeLabel(%q) passed", bad)
		}
	}
	for in, want := range map[string]string{" Nitro ": "Nitro", strings.Repeat("🍺", 50): strings.Repeat("🍺", 50), "a": "a"} {
		if got, msg := normalizeLabel(in); msg != "" || got != want {
			t.Errorf("normalizeLabel(%q) = %q %q", in, got, msg)
		}
	}
}

func TestVolumeRules(t *testing.T) {
	for _, x := range []float64{0, 0.004, 100.01, -1} {
		fe := beers.FieldErrors{}
		checkSize(fe, x)
		if fe["size_l"] == "" {
			t.Errorf("size %v passed", x)
		}
		fe = beers.FieldErrors{}
		checkAmount(fe, x)
		if fe["amount_l"] == "" {
			t.Errorf("amount %v passed", x)
		}
	}
	for x, want := range map[float64]float64{0.01: 0.01, 100: 100, 100.004: 100, 19.999: 20} {
		fe := beers.FieldErrors{}
		if got := checkSize(fe, x); len(fe) != 0 || got != want {
			t.Errorf("size %v = %v %v", x, got, fe)
		}
		if got := checkAmount(fe, x); len(fe) != 0 || got != want {
			t.Errorf("amount %v = %v %v", x, got, fe)
		}
	}
	for _, c := range []struct {
		x, size float64
		ok      bool
	}{{0, 18.93, true}, {18.93, 18.93, true}, {18.934, 18.93, true}, {18.94, 18.93, false}, {-0.01, 18.93, false}, {-0.004, 18.93, true}} {
		fe := beers.FieldErrors{}
		checkRemaining(fe, c.x, c.size)
		if (len(fe) == 0) != c.ok || (!c.ok && fe["remaining_l"] == "") {
			t.Errorf("remaining %v of %v: %v, want ok %v", c.x, c.size, fe, c.ok)
		}
	}

	fe := beers.FieldErrors{}
	if kt := checkType(fe, "sanke_half"); len(fe) != 0 || kt.SizeL != 58.67 {
		t.Fatalf("type sanke_half = %+v %v", kt, fe)
	}
	checkType(fe, "keg")
	checkSize(fe, 0)
	checkRemaining(fe, -1, 10)
	if keys(fe) != "remaining_l,size_l,type" {
		t.Fatalf("several bad fields = %v", fe)
	}
}
