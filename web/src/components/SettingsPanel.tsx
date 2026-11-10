import { FormEvent, useEffect, useId, useState } from "react";
import { ApiError, apiFetch, errorMessage, isUnauthenticated, Settings } from "../api";

type Field = keyof Settings;
type Values = Record<Field, string>;

const text = (s: Settings): Values => ({ ...s, page_seconds: String(s.page_seconds) });

/**
 * SettingsPanel edits the display settings: the TV title, the TV theme (Dark or Light), the
 * admin app's volume units (US or Metric) and the seconds each TV page shows (5 to 300). It
 * loads GET /api/settings on mount and calls `onChange` with the loaded settings, then on Save
 * sends PATCH /api/settings with only the changed fields and calls `onChange` with the reply;
 * with nothing changed Save sends nothing. The API is the only validator: its per-field
 * messages are shown beside each field and the typed values stay. A 401 calls `onSignedOut`.
 */
export default function SettingsPanel({
  onSignedOut,
  onChange,
}: {
  onSignedOut: () => void;
  onChange: (s: Settings) => void;
}) {
  const id = useId();
  // base is the last loaded or saved settings; Save sends only what differs from it.
  const [base, setBase] = useState<Values | null>(null);
  const [values, setValues] = useState<Values>({ title: "", theme: "dark", units: "us", page_seconds: "" });
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

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
  }, []);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!base) return;
    const body: Record<string, string | number> = {};
    for (const f of ["title", "theme", "units"] as const) if (values[f] !== base[f]) body[f] = values[f];
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

  function field(name: Field, label: string, control: (common: object) => JSX.Element) {
    const errId = `${id}-${name}-error`;
    const msg = fieldErrors[name];
    return (
      <div>
        <label>
          {label}
          {control({
            value: values[name],
            "aria-invalid": msg ? true : undefined,
            "aria-describedby": msg ? errId : undefined,
            onChange: (e: { target: { value: string } }) => setValues({ ...values, [name]: e.target.value }),
          })}
        </label>
        {msg && <span id={errId}>{msg}</span>}
      </div>
    );
  }

  return (
    <section>
      <h2>Display settings</h2>
      <form onSubmit={submit} noValidate aria-label="Display settings">
        {field("title", "Title", (common) => <input {...common} />)}
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
        {field("page_seconds", "Page seconds", (common) => <input {...common} type="number" min="5" max="300" step="1" />)}
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={saving || !base}>
          Save
        </button>
      </form>
    </section>
  );
}
