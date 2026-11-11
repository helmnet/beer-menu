import { FormEvent, type JSX, type ReactNode, useEffect, useId, useRef, useState } from "react";
import { ApiError, apiFetch, errorMessage, isUnauthenticated, type Menu, Settings } from "../api";
import { FONTS } from "../fonts";
import { DEFAULT_DISPLAY, displayOf, isMenu, pagesOf, SAMPLE_MENU, usePager } from "../tv";
import TvView from "./TvView";

type Field = keyof Settings;
/** Values is the form's draft: Settings with page_seconds as the typed text. */
type Values = Omit<Settings, "page_seconds"> & { page_seconds: string };
/** TextField and SwitchField are the fields drawn by a text control and by a checkbox. */
type TextField = { [F in Field]: Values[F] extends string ? F : never }[Field];
type SwitchField = { [F in Field]: Values[F] extends boolean ? F : never }[Field];

const text = (s: Settings): Values => ({ ...s, page_seconds: String(s.page_seconds) });
const { pageMs: _, ...tvDefaults } = DEFAULT_DISPLAY;
const EMPTY: Values = { ...tvDefaults, units: "us", page_seconds: "" };

/** THEME_TEXT is each TV theme's text color (--text in DisplayPage.css), shown by an unset color picker. */
const THEME_TEXT = { dark: "#f2f2f2", light: "#141416" };

/** SWITCHES are the tap detail checkboxes, in the order they show. */
const SWITCHES: [SwitchField, string][] = [
  ["show_style", "Style"],
  ["show_abv", "ABV"],
  ["show_ibu", "IBU"],
  ["show_srm", "SRM"],
  ["show_description", "Description"],
];

/**
 * SettingsPanel is the Display Settings page, the heading above one card that edits the display settings: the TV
 * title, the TV theme (Dark or Light), the admin app's volume units (US or Metric), the seconds
 * each TV page shows (5 to 300), the TV layout (Grid, Vertical list or Side by side), the
 * "Title" alignment, font and color, the "Text" font and color (fonts from FONTS; each color a
 * picker with a Reset back to the theme's color), and the "Tap details" switches (Style, ABV, IBU, SRM and Description, sent as JSON booleans). It loads
 * GET /api/settings on mount and calls `onChange` with the loaded settings, then on Save sends
 * PATCH /api/settings with only the changed fields and calls `onChange` with the reply; with
 * nothing changed Save sends nothing. The API is the only validator: its per-field messages are
 * shown beside each field and the typed values stay. A 401 calls `onSignedOut`.
 *
 * A 16:9 "TV preview" region, beside the form on wide screens and above it on narrow ones, draws
 * the unsaved form with `TvView`, the TV's own drawing, so it looks as the TV will after Save; a
 * field that is not valid yet (such as a half-typed page interval) draws its last good value.
 * It reads GET /api/menu once on mount, drawing `SAMPLE_MENU` when that fails or is not a menu,
 * turns pages by itself at the draft interval and has Previous and Next buttons that wrap. The
 * preview never saves: only Save changes the TV.
 */
export default function SettingsPanel({ onSignedOut, onChange }: { onSignedOut: () => void; onChange: (s: Settings) => void }) {
  const id = useId();
  // base is the last loaded or saved settings; Save sends only what differs from it.
  const [base, setBase] = useState<Values | null>(null);
  const [values, setValues] = useState<Values>(EMPTY);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [menu, setMenu] = useState<Menu | null>(null);
  // The last good draft, so a half-typed field keeps drawing its last good value. A ref, not
  // state: it only feeds the next render's fallback and never needs a render of its own.
  const lastGood = useRef(DEFAULT_DISPLAY);
  const display = displayOf({ ...values, page_seconds: Number(values.page_seconds) }, lastGood.current);
  lastGood.current = display;
  const pages = menu ? pagesOf(menu, display.layout) : [];
  const [page, setPage] = usePager(pages.length, display.pageMs);

  function fail(err: unknown) {
    if (isUnauthenticated(err)) onSignedOut();
    else setError(errorMessage(err));
  }

  function show(s: Settings) {
    setBase(text(s));
    setValues(text(s));
    onChange(s);
  }

  useEffect(() => {
    apiFetch<Settings>("GET", "/api/settings").then(show).catch(fail);
    // Any failure, or a body the TV would refuse, draws the sample: the preview is only a guide.
    apiFetch<unknown>("GET", "/api/menu")
      .then((m) => setMenu(isMenu(m) ? m : SAMPLE_MENU))
      .catch(() => setMenu(SAMPLE_MENU));
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!base) return;
    const body: Record<string, string | number | boolean> = {};
    for (const f of Object.keys(values) as Field[]) if (f !== "page_seconds" && values[f] !== base[f]) body[f] = values[f];
    // Non-numeric text goes as is so the API refuses it by field (NaN would become null).
    if (values.page_seconds !== base.page_seconds) {
      const n = Number(values.page_seconds);
      body.page_seconds = values.page_seconds.trim() === "" || Number.isNaN(n) ? values.page_seconds : n;
    }
    if (Object.keys(body).length === 0) return;
    setError("");
    setFieldErrors({});
    setSaving(true);
    try {
      show(await apiFetch<Settings>("PATCH", "/api/settings", body));
    } catch (err) {
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      fail(err);
    } finally {
      setSaving(false);
    }
  }

  /** field is one labelled control with its error, and an optional `note` it is described by and `extra` after it. */
  function field(name: TextField, label: string, control: (common: object) => JSX.Element, note?: string, extra?: ReactNode) {
    const errId = `${id}-${name}-error`;
    const noteId = `${id}-${name}-note`;
    const msg = fieldErrors[name];
    return (
      <div className="field">
        <label>
          {label}
          {control({
            value: values[name],
            "aria-invalid": msg ? true : undefined,
            "aria-describedby": [msg && errId, note && noteId].filter(Boolean).join(" ") || undefined,
            onChange: (e: { target: { value: string } }) => setValues({ ...values, [name]: e.target.value }),
          })}
        </label>
        {note && (
          <span id={noteId} className="muted">
            {note}
          </span>
        )}
        {extra}
        {msg && (
          <span id={errId} className="field-error">
            {msg}
          </span>
        )}
      </div>
    );
  }

  /**
   * colorField is a color picker for `name`. While it is "" (the theme's color) the picker shows the
   * draft theme's text color with a "Theme color" note; Reset sets it back to "". Colors are not
   * contrast-checked: the preview is how the owner judges them.
   */
  const colorField = (name: "title_color" | "text_color", label: string) =>
    field(
      name,
      label,
      (common) => <input {...common} type="color" value={values[name] || THEME_TEXT[values.theme]} />,
      values[name] ? undefined : "Theme color",
      <button
        type="button"
        className="btn"
        aria-label={`Reset ${label.toLowerCase()}`}
        disabled={!values[name]}
        onClick={() => setValues({ ...values, [name]: "" })}
      >
        Reset
      </button>,
    );

  const fontSelect = (common: object) => (
    <select {...common}>
      {FONTS.map((f) => (
        <option key={f.key} value={f.key}>
          {f.label}
        </option>
      ))}
    </select>
  );

  const step = (by: number) => setPage((page + by + pages.length) % pages.length);

  return (
    <section className="page">
      <h2 className="page-title">Display Settings</h2>
      <div className="settings-layout">
        <div className="preview-panel">
          <div className="tv-preview" role="region" aria-label="TV preview">
            <TvView menu={menu} display={display} pages={pages} page={page} failures={0} offlineAfter={Infinity} preview />
          </div>
          <div className="btn-row">
            <button type="button" className="btn" onClick={() => step(-1)} disabled={pages.length < 2}>
              Previous
            </button>
            <button type="button" className="btn" onClick={() => step(1)} disabled={pages.length < 2}>
              Next
            </button>
          </div>
        </div>
        <form className="card" onSubmit={submit} noValidate aria-label="Display settings">
          {field("title", "Title", (common) => (
            <input {...common} />
          ))}
          {field("theme", "Theme", (common) => (
            <select {...common}>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </select>
          ))}
          {field("units", "Units", (common) => (
            <select {...common}>
              <option value="us">US</option>
              <option value="metric">Metric</option>
            </select>
          ))}
          {field("page_seconds", "Page seconds", (common) => (
            <input {...common} type="number" min="5" max="300" step="1" />
          ))}
          {field("layout", "Layout", (common) => (
            <select {...common}>
              <option value="grid">Grid</option>
              <option value="list">Vertical list</option>
              <option value="columns">Side by side</option>
            </select>
          ))}
          <fieldset className="field">
            <legend>Title</legend>
            {field("title_align", "Title alignment", (common) => (
              <select {...common}>
                <option value="left">Left</option>
                <option value="center">Center</option>
                <option value="right">Right</option>
              </select>
            ))}
            {field("title_font", "Title font", fontSelect)}
            {colorField("title_color", "Title color")}
          </fieldset>
          <fieldset className="field">
            <legend>Text</legend>
            {field("text_font", "Text font", fontSelect)}
            {colorField("text_color", "Text color")}
          </fieldset>
          <fieldset className="field">
            <legend>Tap details</legend>
            {SWITCHES.map(([name, label]) => (
              <div key={name} className="field">
                <label>
                  <input
                    type="checkbox"
                    checked={values[name]}
                    aria-invalid={fieldErrors[name] ? true : undefined}
                    aria-describedby={fieldErrors[name] ? `${id}-${name}-error` : undefined}
                    onChange={(e) => setValues({ ...values, [name]: e.target.checked })}
                  />
                  {label}
                </label>
                {fieldErrors[name] && (
                  <span id={`${id}-${name}-error`} className="field-error">
                    {fieldErrors[name]}
                  </span>
                )}
              </div>
            ))}
          </fieldset>
          {error && <p role="alert">{error}</p>}
          <button type="submit" className="btn btn-primary" disabled={saving || !base}>
            Save
          </button>
        </form>
      </div>
    </section>
  );
}
