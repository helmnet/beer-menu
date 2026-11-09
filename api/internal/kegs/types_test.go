package kegs

import "testing"

func TestTypes(t *testing.T) {
	want := []KegType{
		{"corny_2_5_gal", "Corny 2.5 gal", 9.46, "corny"},
		{"corny_3_gal", "Corny 3 gal", 11.36, "corny"},
		{"corny_5_gal", "Corny 5 gal", 18.93, "corny"},
		{"sanke_sixth", "Sixth barrel", 19.56, "sanke"},
		{"sanke_quarter", "Quarter barrel", 29.34, "sanke"},
		{"sanke_half", "Half barrel", 58.67, "sanke"},
		{"euro_20_l", "Euro 20 L", 20.00, "sanke"},
		{"euro_30_l", "Euro 30 L", 30.00, "sanke"},
		{"euro_50_l", "Euro 50 L", 50.00, "sanke"},
	}
	if len(Types) != len(want) {
		t.Fatalf("Types has %d entries, want %d", len(Types), len(want))
	}
	for i, w := range want {
		if Types[i] != w {
			t.Errorf("Types[%d] = %+v, want %+v", i, Types[i], w)
		}
	}
	if kt, ok := TypeByKey("corny_5_gal"); !ok || kt.SizeL != 18.93 {
		t.Fatalf("TypeByKey(corny_5_gal) = %+v %v", kt, ok)
	}
	if _, ok := TypeByKey("keg"); ok {
		t.Fatal("TypeByKey(keg) found")
	}
	if DefaultType != "corny_5_gal" {
		t.Fatalf("DefaultType = %q", DefaultType)
	}
}
