package brewfather

import (
	"strconv"
	"strings"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
)

// batchJSON is the part of a Brewfather batch the import uses (data-model.md).
type batchJSON struct {
	ID             string   `json:"_id"`
	Name           string   `json:"name"`
	BatchNo        *int64   `json:"batchNo"`
	Status         string   `json:"status"`
	BrewDate       *int64   `json:"brewDate"`
	MeasuredAbv    *float64 `json:"measuredAbv"`
	EstimatedIbu   *float64 `json:"estimatedIbu"`
	EstimatedColor *float64 `json:"estimatedColor"`
	BatchNotes     string   `json:"batchNotes"`
	TasteNotes     string   `json:"tasteNotes"`
	Recipe         struct {
		Name  string   `json:"name"`
		ABV   *float64 `json:"abv"`
		Style struct {
			Name *string `json:"name"`
		} `json:"style"`
	} `json:"recipe"`
}

// statuses maps Brewfather statuses to beer statuses (FR-013). Missing means planned, which
// keeps an unknown status off the TV.
var statuses = map[string]string{
	"Planning":     "planned",
	"Brewing":      "fermenting",
	"Fermenting":   "fermenting",
	"Conditioning": "conditioning",
	"Completed":    "ready",
	"Archived":     "ready",
}

// toBatch applies FR-012's fallbacks: name (blank or the default "Batch" counts as none), else
// recipe name, else "Batch <number>", else "Brewfather batch"; measured ABV, else recipe ABV;
// tasting notes, else batch notes. Values are not cleaned here; beers.Sanitize does that.
func (b batchJSON) toBatch() importer.Batch {
	out := importer.Batch{
		Ref:      b.ID,
		Number:   b.BatchNo,
		Status:   statuses[b.Status],
		Archived: b.Status == "Archived",
		Style:    b.Recipe.Style.Name,
		ABV:      b.MeasuredAbv,
		IBU:      b.EstimatedIbu,
		SRM:      b.EstimatedColor,
	}
	if out.Status == "" {
		out.Status = "planned"
	}
	// Brewfather names every new batch "Batch", so that default says nothing about the beer.
	switch name := strings.TrimSpace(b.Name); {
	case name != "" && !strings.EqualFold(name, "Batch"):
		out.Name = b.Name
	case strings.TrimSpace(b.Recipe.Name) != "":
		out.Name = b.Recipe.Name
	case b.BatchNo != nil:
		out.Name = "Batch " + strconv.FormatInt(*b.BatchNo, 10)
	default:
		out.Name = "Brewfather batch"
	}
	if out.ABV == nil {
		out.ABV = b.Recipe.ABV
	}
	for _, notes := range []string{b.TasteNotes, b.BatchNotes} {
		if strings.TrimSpace(notes) != "" {
			out.Notes = &notes
			break
		}
	}
	if b.BrewDate != nil && *b.BrewDate > 0 {
		d := time.UnixMilli(*b.BrewDate).UTC().Format(time.DateOnly)
		out.BrewDate = &d
	}
	return out
}
