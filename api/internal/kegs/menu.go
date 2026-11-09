package kegs

import (
	"database/sql"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// Menu is the public menu GET /api/menu serves to the TV display, with display fields only: no
// ids, timestamps, sizes or volumes (research R3). It is a compatibility contract: fields may be
// added but not removed or renamed within a major version. These structs are separate from Tap,
// Keg and beers.Beer so a field added to a credentialed type can never leak here. Taps and
// OnDeck are never nil, so they marshal as [] when empty.
type Menu struct {
	// Taps are the enabled taps in position order.
	Taps []MenuTap `json:"taps"`
	// OnDeck is waiting kegs (oldest first), then conditioning beers, then fermenting beers,
	// each by name.
	OnDeck []OnDeckItem `json:"on_deck"`
}

// MenuTap is one enabled tap. Beer and Keg are both nil, marshalled as null, when the tap has no
// current keg of a live beer.
type MenuTap struct {
	Label string    `json:"label"`
	Beer  *MenuBeer `json:"beer"`
	Keg   *MenuKeg  `json:"keg"`
}

// OnDeckItem is a waiting keg (Keg set, Status nil) or a beer still being made with no current
// keg (Keg nil, Status "fermenting" or "conditioning").
type OnDeckItem struct {
	Beer   MenuBeer `json:"beer"`
	Keg    *MenuKeg `json:"keg"`
	Status *string  `json:"status"`
}

// MenuBeer is the beer as the display draws it. Optional fields are nil when unset and marshal
// as null. Color is beers.ColorFor of the beer's SRM.
type MenuBeer struct {
	Name  string   `json:"name"`
	Style *string  `json:"style"`
	ABV   *float64 `json:"abv"`
	IBU   *float64 `json:"ibu"`
	Color *string  `json:"color"`
}

// MenuKeg is a keg as the display draws it: its shape ("corny" or "sanke") and Level, the
// fraction left (remaining over size), clamped to 0..1 and not rounded (research R4).
type MenuKeg struct {
	Shape string  `json:"shape"`
	Level float64 `json:"level"`
}

// menuBeerCols and menuKegCols are scanned by scanBeer and scanKeg below, in this order.
const (
	menuBeerCols = "b.name, b.style, b.abv, b.ibu, b.srm"
	menuKegCols  = "k.type, k.size_l, k.remaining_l"
)

// GetMenu reads the public menu with three plain SELECTs (research R3, R7): enabled taps with their
// current keg and its beer, waiting kegs, and beers being made that have no current keg. No
// transaction: the display re-reads every 10 s, so a menu spanning two instants is harmless.
func GetMenu(db *sql.DB) (Menu, error) {
	m := Menu{Taps: []MenuTap{}, OnDeck: []OnDeckItem{}}

	rows, err := db.Query(`SELECT t.label, k.id, b.id, ` + menuKegCols + `, ` + menuBeerCols + `
		FROM taps t
		LEFT JOIN kegs k ON k.tap_id = t.id AND k.kicked_at IS NULL
		LEFT JOIN beers b ON b.id = k.beer_id AND b.archived_at IS NULL
		WHERE t.enabled = 1 ORDER BY t.position, t.id`)
	if err != nil {
		return Menu{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var tp MenuTap
		var kegID, beerID sql.NullInt64
		var kt sql.NullString
		var size, remaining sql.NullFloat64
		var name sql.NullString
		var b MenuBeer
		var srm *float64
		if err := rows.Scan(&tp.Label, &kegID, &beerID, &kt, &size, &remaining, &name, &b.Style, &b.ABV, &b.IBU, &srm); err != nil {
			return Menu{}, err
		}
		// Both or neither: a keg whose beer is archived shows as an empty tap, never half filled.
		if kegID.Valid && beerID.Valid {
			b.Name, b.Color = name.String, beers.ColorFor(srm)
			k := menuKeg(kt.String, size.Float64, remaining.Float64)
			tp.Beer, tp.Keg = &b, &k
		}
		m.Taps = append(m.Taps, tp)
	}
	if err := rows.Err(); err != nil {
		return Menu{}, err
	}
	rows.Close()

	rows, err = db.Query(`SELECT ` + menuKegCols + `, ` + menuBeerCols + `
		FROM kegs k JOIN beers b ON b.id = k.beer_id
		WHERE k.tap_id IS NULL AND k.kicked_at IS NULL AND b.archived_at IS NULL
		ORDER BY k.created_at, k.id`)
	if err != nil {
		return Menu{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var kt string
		var size, remaining float64
		var it OnDeckItem
		var srm *float64
		if err := rows.Scan(&kt, &size, &remaining, &it.Beer.Name, &it.Beer.Style, &it.Beer.ABV, &it.Beer.IBU, &srm); err != nil {
			return Menu{}, err
		}
		it.Beer.Color = beers.ColorFor(srm)
		k := menuKeg(kt, size, remaining)
		it.Keg = &k
		m.OnDeck = append(m.OnDeck, it)
	}
	if err := rows.Err(); err != nil {
		return Menu{}, err
	}
	rows.Close()

	rows, err = db.Query(`SELECT b.status, ` + menuBeerCols + `
		FROM beers b
		WHERE b.archived_at IS NULL AND b.status IN ('conditioning','fermenting')
		AND NOT EXISTS (SELECT 1 FROM kegs k WHERE k.beer_id = b.id AND k.kicked_at IS NULL)
		ORDER BY b.status = 'fermenting', b.name COLLATE NOCASE, b.id`)
	if err != nil {
		return Menu{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var it OnDeckItem
		var srm *float64
		if err := rows.Scan(&it.Status, &it.Beer.Name, &it.Beer.Style, &it.Beer.ABV, &it.Beer.IBU, &srm); err != nil {
			return Menu{}, err
		}
		it.Beer.Color = beers.ColorFor(srm)
		m.OnDeck = append(m.OnDeck, it)
	}
	if err := rows.Err(); err != nil {
		return Menu{}, err
	}
	return m, nil
}

// menuKeg builds a MenuKeg from a keg's type key, size and remaining litres.
func menuKeg(typ string, size, remaining float64) MenuKeg {
	kt, ok := TypeByKey(typ)
	if !ok {
		// The column has no CHECK; an unknown key still draws, as the default type's shape.
		kt, _ = TypeByKey(DefaultType)
	}
	// Go validation keeps size above 0 (feature 4 FR-012) but the column has no CHECK, and 0/0
	// is NaN, which JSON cannot encode; the clamp guards against float noise.
	level := 0.0
	if size > 0 {
		level = min(max(remaining/size, 0), 1)
	}
	return MenuKeg{Shape: kt.Shape, Level: level}
}
