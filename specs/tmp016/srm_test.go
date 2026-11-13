package beers

import (
	"regexp"
	"strconv"
	"testing"
)

func TestSRMColorsTable(t *testing.T) {
	if len(SRMColors) != 40 {
		t.Fatalf("table has %d entries, want 40", len(SRMColors))
	}
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	// The published table is not strictly monotonic: its SRM 39 is 4 RGB units lighter than
	// SRM 38. A larger rise means a mis-copied entry.
	const tolerance = 4
	prev := 3 * 255
	for i, c := range SRMColors {
		if !hex.MatchString(c) {
			t.Fatalf("entry %d = %q, want #rrggbb lower case", i+1, c)
		}
		sum := 0
		for _, p := range []string{c[1:3], c[3:5], c[5:7]} {
			v, _ := strconv.ParseUint(p, 16, 8)
			sum += int(v)
		}
		if sum > prev+tolerance {
			t.Errorf("entry %d (%s) is lighter than entry %d", i+1, c, i)
		}
		prev = sum
	}
}

func TestColorFor(t *testing.T) {
	if got := ColorFor(nil); got != nil {
		t.Fatalf("ColorFor(nil) = %q, want nil", *got)
	}
	want := func(srm float64, index int) {
		t.Helper()
		got := ColorFor(&srm)
		if got == nil || *got != SRMColors[index] {
			t.Errorf("ColorFor(%v) = %v, want entry %d (%s)", srm, got, index+1, SRMColors[index])
		}
	}
	want(0, 0)
	for n := 1; n <= 40; n++ {
		want(float64(n), n-1)
	}
	want(4.4, 3)
	want(4.5, 4)
	want(60, 39)
	want(100, 39)
}
