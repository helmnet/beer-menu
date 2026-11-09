package kegs

// KegType is one entry of the fixed keg type table: its key, display label, default size in
// litres and the shape clients draw it as ("corny" or "sanke").
type KegType struct {
	Type  string  `json:"type"`
	Label string  `json:"label"`
	SizeL float64 `json:"size_l"`
	Shape string  `json:"shape"`
}

// DefaultType is the keg type a new keg gets when none is sent.
const DefaultType = "corny_5_gal"

// Types is the keg type table served by GET /api/keg-types, in display order (research R4).
// Sizes use 1 US gal = 3.785411784 L and 1 US barrel = 31 US gal, rounded to 0.01 L. Adding a
// type needs no migration: the column has no CHECK and Go validates against this table.
var Types = []KegType{
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

// TypeByKey returns the keg type whose Type is key, and false when there is none.
func TypeByKey(key string) (KegType, bool) {
	for _, kt := range Types {
		if kt.Type == key {
			return kt, true
		}
	}
	return KegType{}, false
}
