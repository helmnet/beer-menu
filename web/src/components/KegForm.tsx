import { FormEvent, useEffect, useId, useState } from "react";
import { ApiError, apiFetch, Beer, errorMessage, isUnauthenticated, Keg, KegType, Units } from "../api";
import { inputToLitres, volumeText, volumeToLitres, volumeUnit } from "../units";
import BeerPicker from "./BeerPicker";

type Field = "beer_id" | "type" | "size_l" | "remaining_l";
type Values = Record<Field, string>;

/**
 * KegForm adds a keg (POST /api/kegs), on tap `tapId` when given or waiting otherwise, or,
 * given `keg`, corrects it (PATCH /api/kegs/{id}). It loads the current beers (GET /api/beers),
 * chosen by typing in BeerPicker (newest first), and the keg type table (GET /api/keg-types) on
 * mount. Size and remaining are shown and typed
 * in `units` (US gallons or litres, default US) and sent in litres; the form reads `units` once,
 * so a caller changing it remounts the form. On add, choosing a type prefills size and remaining
 * from the type's size; on edit it changes only the type, keeping the size (FR-020). Volumes are sent only when they differ from what was
 * prefilled, so a keg saved without touching them keeps its stored litres exactly; on edit only
 * changed fields are sent and no change just calls `onCancel`. The API is the only validator:
 * its per-field messages are shown next to each field. A 401 calls `onSignedOut`; success calls
 * `onSaved` with the stored keg.
 */
export default function KegForm({
  keg,
  tapId,
  units = "us",
  onSaved,
  onCancel,
  onSignedOut,
}: {
  keg?: Keg;
  tapId?: number;
  units?: Units;
  onSaved: (keg: Keg) => void;
  onCancel: () => void;
  onSignedOut: () => void;
}) {
  const id = useId();
  const [beers, setBeers] = useState<Beer[]>([]);
  const [types, setTypes] = useState<KegType[]>([]);
  // base holds the values the user has not changed: the keg on edit, the type's prefill on add.
  const [base, setBase] = useState<Values>(() =>
    keg
      ? { beer_id: String(keg.beer_id), type: keg.type, size_l: volumeText(keg.size_l, units), remaining_l: volumeText(keg.remaining_l, units) }
      : { beer_id: "", type: "corny_5_gal", size_l: "", remaining_l: "" },
  );
  const [values, setValues] = useState(base);
  const [fieldErrors, setFieldErrors] = useState<Partial<Record<string, string>>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  function fail(err: unknown) {
    if (isUnauthenticated(err)) onSignedOut();
    else setError(errorMessage(err));
  }

  useEffect(() => {
    Promise.all([apiFetch<Beer[]>("GET", "/api/beers"), apiFetch<KegType[]>("GET", "/api/keg-types")])
      .then(([b, t]) => {
        setBeers(b);
        setTypes(t);
        if (!keg) prefill(t, values.type, values);
      })
      .catch(fail);
  }, []);

  /** prefill sets type in v; on add it also sets the volumes from type's size as the unchanged base. */
  function prefill(table: KegType[], type: string, v: Values) {
    const t = table.find((k) => k.type === type);
    if (keg || !t) {
      setValues({ ...v, type });
      return;
    }
    const size = volumeText(t.size_l, units);
    setValues({ ...v, type, size_l: size, remaining_l: size });
    setBase({ ...base, size_l: size, remaining_l: size });
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    const body: Record<string, string | number> = {};
    if (values.beer_id !== base.beer_id) body.beer_id = Number(values.beer_id);
    if (!keg || values.type !== base.type) body.type = values.type;
    if (!keg && tapId !== undefined) body.tap_id = tapId;
    for (const f of ["size_l", "remaining_l"] as const) {
      if (values[f] !== base[f]) body[f] = inputToLitres(values[f], (n) => volumeToLitres(n, units));
    }
    if (keg && Object.keys(body).length === 0) {
      onCancel();
      return;
    }
    setError("");
    setFieldErrors({});
    setSaving(true);
    try {
      onSaved(
        keg
          ? await apiFetch<Keg>("PATCH", `/api/kegs/${keg.id}`, body)
          : await apiFetch<Keg>("POST", "/api/kegs", body),
      );
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
      <div className="field">
        <label>
          {label}
          {control({
            value: values[name],
            "aria-invalid": msg ? true : undefined,
            "aria-describedby": msg ? errId : undefined,
          })}
        </label>
        {msg && (
          <span id={errId} className="field-error">
            {msg}
          </span>
        )}
      </div>
    );
  }

  const volume = (name: "size_l" | "remaining_l") => (common: object) => (
    <input
      {...common}
      type="number"
      min="0"
      step="0.01"
      onChange={(e) => setValues({ ...values, [name]: e.target.value })}
    />
  );

  return (
    <form className="inline-form" onSubmit={submit} noValidate aria-label={keg ? `Edit keg of ${keg.beer_name}` : "Add keg"}>
      <div className="field">
        <BeerPicker
          beers={beers}
          value={values.beer_id ? Number(values.beer_id) : null}
          onChange={(beerId) => setValues({ ...values, beer_id: String(beerId) })}
          errorId={fieldErrors.beer_id ? `${id}-beer_id-error` : undefined}
        />
        {fieldErrors.beer_id && (
          <span id={`${id}-beer_id-error`} className="field-error">
            {fieldErrors.beer_id}
          </span>
        )}
      </div>
      {field("type", "Type", (common) => (
        <select {...common} onChange={(e) => prefill(types, e.target.value, values)}>
          {types.map((t) => (
            <option key={t.type} value={t.type}>
              {t.label}
            </option>
          ))}
        </select>
      ))}
      {field("size_l", `Size (${volumeUnit(units)})`, volume("size_l"))}
      {field("remaining_l", `Remaining (${volumeUnit(units)})`, volume("remaining_l"))}
      {error && <p role="alert">{error}</p>}
      <div className="btn-row">
        <button type="submit" className="btn btn-primary" disabled={saving}>
          Save
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}
