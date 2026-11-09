import { FormEvent, useEffect, useId, useState } from "react";
import { ApiError, apiFetch, Beer, BeerStatus, errorMessage, isUnauthenticated } from "../api";
import BeerArt, { colorForSrm } from "./BeerArt";

type Field = "name" | "style" | "abv" | "ibu" | "srm" | "description" | "brewer" | "brew_date" | "status";
type Values = Record<Field, string>;

const numeric: Field[] = ["abv", "ibu", "srm"];
const statuses: BeerStatus[] = ["planned", "fermenting", "conditioning", "ready"];

/** initialValues turns a beer into input strings; null fields become empty inputs. */
function initialValues(beer?: Beer): Values {
  const s = (v: string | number | null | undefined) => (v === null || v === undefined ? "" : String(v));
  return {
    name: s(beer?.name),
    style: s(beer?.style),
    abv: s(beer?.abv),
    ibu: s(beer?.ibu),
    srm: s(beer?.srm),
    description: s(beer?.description),
    brewer: s(beer?.brewer),
    brew_date: s(beer?.brew_date),
    status: beer?.status ?? "ready",
  };
}

/**
 * parse turns an input string into the JSON value to send. Numeric fields become numbers; a
 * value that is not a number is sent as text so the API refuses it, because JSON.stringify
 * would turn NaN into null and silently clear the field.
 */
function parse(field: Field, value: string): string | number {
  if (!numeric.includes(field)) return value;
  const n = Number(value);
  return Number.isNaN(n) ? value : n;
}

/**
 * BeerForm adds a beer (POST /api/beers) or, given `beer`, edits it (PATCH /api/beers/{id}).
 * On add it sends only filled-in fields, plus the status. On edit it sends only the fields that
 * changed, with null for a cleared optional field; with no change it just calls `onCancel`.
 * The API is the only validator: its per-field messages are shown next to each field. A 401
 * calls `onSignedOut`; success calls `onSaved` with the stored beer. A BeerArt preview follows
 * the SRM input, colored from GET /api/srm-colors, fetched once on mount; if that fails the
 * preview stays empty rather than showing an error.
 */
export default function BeerForm({
  beer,
  onSaved,
  onCancel,
  onSignedOut,
}: {
  beer?: Beer;
  onSaved: (beer: Beer) => void;
  onCancel: () => void;
  onSignedOut: () => void;
}) {
  const id = useId();
  const [initial] = useState(() => initialValues(beer));
  const [values, setValues] = useState(initial);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [colors, setColors] = useState<string[]>([]);

  useEffect(() => {
    apiFetch<{ colors: string[] }>("GET", "/api/srm-colors")
      .then((t) => setColors(t.colors))
      .catch(() => setColors([]));
  }, []);

  const srm = values.srm === "" ? null : Number(values.srm);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const body: Record<string, string | number | null> = {};
    for (const field of Object.keys(values) as Field[]) {
      const value = values[field];
      if (!beer) {
        if (value !== "") body[field] = parse(field, value);
      } else if (value !== initial[field]) {
        body[field] = value === "" && field !== "name" ? null : parse(field, value);
      }
    }
    if (beer && Object.keys(body).length === 0) {
      onCancel();
      return;
    }
    setError("");
    setFieldErrors({});
    setSaving(true);
    try {
      const saved = beer
        ? await apiFetch<Beer>("PATCH", `/api/beers/${beer.id}`, body)
        : await apiFetch<Beer>("POST", "/api/beers", body);
      onSaved(saved);
    } catch (err) {
      if (isUnauthenticated(err)) {
        onSignedOut();
        return;
      }
      if (err instanceof ApiError && err.fields) setFieldErrors(err.fields);
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  function input(field: Field, label: string, props: Record<string, string> = {}) {
    const errId = `${id}-${field}-error`;
    const msg = fieldErrors[field];
    const common = {
      value: values[field],
      "aria-invalid": msg ? true : undefined,
      "aria-describedby": msg ? errId : undefined,
      onChange: (e: { target: { value: string } }) => setValues({ ...values, [field]: e.target.value }),
    };
    return (
      <div>
        <label>
          {label}
          {field === "description" ? (
            <textarea {...common} rows={4} />
          ) : field === "status" ? (
            <select {...common}>
              {statuses.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          ) : (
            <input {...common} {...props} />
          )}
        </label>
        {msg && <span id={errId}>{msg}</span>}
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate aria-label={beer ? `Edit ${beer.name}` : "Add beer"}>
      <BeerArt
        color={srm === null || Number.isNaN(srm) ? null : colorForSrm(colors, srm)}
        name={values.name || "New beer"}
        height={96}
      />
      {input("name", "Name")}
      {input("style", "Style")}
      {input("abv", "ABV", { type: "number", min: "0", max: "25", step: "0.1" })}
      {input("ibu", "IBU", { type: "number", min: "0", max: "200", step: "1" })}
      {input("srm", "SRM", { type: "number", min: "0", max: "100", step: "0.1" })}
      {input("description", "Description")}
      {input("brewer", "Brewer")}
      {input("brew_date", "Brew date", { type: "date" })}
      {input("status", "Status")}
      {error && <p role="alert">{error}</p>}
      <button type="submit" disabled={saving}>
        Save
      </button>
      <button type="button" onClick={onCancel}>
        Cancel
      </button>
    </form>
  );
}
