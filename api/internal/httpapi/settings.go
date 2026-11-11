package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/settings"
)

// getSettings handles GET /api/settings (session or any token): 200 with every display
// setting, each its stored value or its default when never set or not valid.
func getSettings(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := settings.GetDisplay(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	})
}

// updateSettings handles PATCH /api/settings (write): it applies the SettingsPatch body, at most
// 8 KiB, with merge-patch semantics and returns 200 with every setting. Every named field is
// validated before any is stored, so an invalid field changes nothing. A null or wrongly typed
// field (page_seconds must be a JSON integer, each show_* switch a JSON boolean, every other
// field a string) or an invalid value gives 400 with a fields map
// whose messages never echo the value; unknown fields and an empty object give 400.
func updateSettings(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Raw fields tell absent (nil) from null and keep a type error on one field from becoming
		// a body-level error without fields.
		var body struct {
			Title       json.RawMessage `json:"title"`
			Theme       json.RawMessage `json:"theme"`
			Units       json.RawMessage `json:"units"`
			PageSeconds json.RawMessage `json:"page_seconds"`

			Layout          json.RawMessage `json:"layout"`
			TitleAlign      json.RawMessage `json:"title_align"`
			TitleFont       json.RawMessage `json:"title_font"`
			TextFont        json.RawMessage `json:"text_font"`
			ShowStyle       json.RawMessage `json:"show_style"`
			ShowABV         json.RawMessage `json:"show_abv"`
			ShowIBU         json.RawMessage `json:"show_ibu"`
			ShowSRM         json.RawMessage `json:"show_srm"`
			ShowDescription json.RawMessage `json:"show_description"`
			TitleColor      json.RawMessage `json:"title_color"`
			TextColor       json.RawMessage `json:"text_color"`
		}
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		fe := beers.FieldErrors{}
		p := settings.DisplayPatch{
			Title:       rawField[string](fe, "title", body.Title, "Must be a string."),
			Theme:       rawField[string](fe, "theme", body.Theme, "Must be a string."),
			Units:       rawField[string](fe, "units", body.Units, "Must be a string."),
			PageSeconds: rawField[int](fe, "page_seconds", body.PageSeconds, settings.PageSecondsMessage),

			Layout:          rawField[string](fe, "layout", body.Layout, "Must be a string."),
			TitleAlign:      rawField[string](fe, "title_align", body.TitleAlign, "Must be a string."),
			TitleFont:       rawField[string](fe, "title_font", body.TitleFont, "Must be a string."),
			TextFont:        rawField[string](fe, "text_font", body.TextFont, "Must be a string."),
			ShowStyle:       rawField[bool](fe, "show_style", body.ShowStyle, settings.SwitchMessage),
			ShowABV:         rawField[bool](fe, "show_abv", body.ShowABV, settings.SwitchMessage),
			ShowIBU:         rawField[bool](fe, "show_ibu", body.ShowIBU, settings.SwitchMessage),
			ShowSRM:         rawField[bool](fe, "show_srm", body.ShowSRM, settings.SwitchMessage),
			ShowDescription: rawField[bool](fe, "show_description", body.ShowDescription, settings.SwitchMessage),
			TitleColor:      rawField[string](fe, "title_color", body.TitleColor, "Must be a string."),
			TextColor:       rawField[string](fe, "text_color", body.TextColor, "Must be a string."),
		}
		if len(fe) > 0 {
			writeBeerError(w, r, fe)
			return
		}
		d, err := settings.PatchDisplay(deps.DB, p, deps.Now())
		if err != nil {
			writeBeerError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "display settings updated")
		writeJSON(w, http.StatusOK, d)
	})
}

// rawField decodes raw, one field of a PATCH body, as a T. It returns nil when the field was not
// sent, and nil with msg recorded in fe under name when it is null or not a T.
func rawField[T any](fe beers.FieldErrors, name string, raw json.RawMessage, msg string) *T {
	if raw == nil {
		return nil
	}
	var v T
	if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &v) != nil {
		fe[name] = msg
		return nil
	}
	return &v
}
