package beers

import "math"

// SRMColors is the display color for SRM 1 to 40: SRMColors[i] is SRM i+1, as lower-case
// "#rrggbb". It is the SRM-to-hex table widely used by brewing software, copied (lower-cased)
// from Kegbot Server's colors.py:
// https://github.com/Kegbot/kegbot-server/blob/ef983e6a90976501a9dcd73143f0ce493d05ad06/pykeg/core/colors.py
// The table is not strictly monotonic: SRM 39 is a shade lighter than SRM 38 in the source.
var SRMColors = [40]string{
	"#ffe699", "#ffd878", "#ffca5a", "#ffbf42", "#fbb123", "#f8a600", "#f39c00", "#ea8f00",
	"#e58500", "#de7c00", "#d77200", "#cf6900", "#cb6200", "#c35900", "#bb5100", "#b54c00",
	"#b04500", "#a63e00", "#a13700", "#9b3200", "#952d00", "#8e2900", "#882300", "#821e00",
	"#7b1a00", "#771900", "#701400", "#6a0e00", "#660d00", "#5e0b00", "#5a0a02", "#560a05",
	"#520907", "#4c0505", "#470606", "#440607", "#3f0708", "#3b0607", "#3a070b", "#36080a",
}

// ColorFor returns the display color for srm: srm rounded to the nearest integer (halves away
// from zero), clamped to 1..40, looked up in SRMColors. It returns nil for a nil srm, so a
// beer without SRM is drawn empty. The web app's colorForSrm must apply the same rule.
func ColorFor(srm *float64) *string {
	if srm == nil {
		return nil
	}
	i := min(max(int(math.Round(*srm)), 1), 40)
	c := SRMColors[i-1]
	return &c
}
