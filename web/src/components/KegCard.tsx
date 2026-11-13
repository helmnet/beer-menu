import { FormEvent, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated, Keg, KegType, Units } from "../api";
import { formatVolume, inputToLitres, pourPrefill, pourToLitres, pourUnit, volumeText, volumeToLitres, volumeUnit } from "../units";
import BeerArt from "./BeerArt";
import { useConfirmAction } from "./ConfirmDialog";
import KegForm from "./KegForm";

/** Open is the one inline form open in the card: a pour, a set level, the keg edit form, or none. */
type Open = "pour" | "set" | "edit" | null;

/**
 * KegCard shows one current keg, on a tap or waiting, and its actions. It draws the beer
 * (BeerArt: a pint glass on a tap, else the keg type's shape, filled to remaining over size),
 * the beer name, keg type label (from `types`, the GET /api/keg-types table), a level bar and
 * "x of y" in `units` (default US). Its buttons, in order: Pour (prefilled 16 fl oz or 500 mL,
 * so a pour is two clicks; POST /api/kegs/{id}/pours), Set level (PATCH remaining_l, sent only
 * when changed), Edit keg (KegForm inline in this card), Take off (PATCH {"tap_id": null}) and
 * Kick (after a confirmation dialog; PATCH {"kicked": true}). Pour, Set level and Take off show only
 * on a tap, so a waiting keg has just Edit keg (which can still change its level) and Kick.
 * Volumes are always sent in litres. The inline forms are keyed by `units`, so a form open
 * during a units change restarts in the new units. After a successful change the form closes and `onChanged`
 * is called so the page reloads; server messages show in the card; a 401 calls `onSignedOut`.
 */
export default function KegCard({
  keg,
  types,
  units = "us",
  onChanged,
  onSignedOut,
}: {
  keg: Keg;
  types: KegType[];
  units?: Units;
  onChanged: () => void;
  onSignedOut: () => void;
}) {
  const [open, setOpen] = useState<Open>(null);
  const [error, setError] = useState("");
  const confirm = useConfirmAction();

  async function change(method: string, path: string, body?: unknown) {
    setError("");
    try {
      await apiFetch(method, path, body);
      setOpen(null);
      onChanged();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  function saveLevel(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const text = String(new FormData(e.currentTarget).get("amount") ?? "");
    if (open === "pour") {
      change("POST", `/api/kegs/${keg.id}/pours`, { amount_l: inputToLitres(text, (n) => pourToLitres(n, units)) });
    } else if (text === volumeText(keg.remaining_l, units)) {
      // An untouched level must not be resent: the two-decimal round trip would move it.
      setOpen(null);
    } else {
      change("PATCH", `/api/kegs/${keg.id}`, { remaining_l: inputToLitres(text, (n) => volumeToLitres(n, units)) });
    }
  }

  function kick() {
    confirm.ask({ title: "Kick keg", message: `Kick the ${keg.beer_name} keg? It moves to keg history.`, confirmLabel: "Kick" }, () =>
      change("PATCH", `/api/kegs/${keg.id}`, { kicked: true }),
    );
  }

  const type = types.find((t) => t.type === keg.type);
  const level = keg.remaining_l / keg.size_l;
  const onTap = keg.state === "on_tap";

  return (
    <div className="keg-card">
      <div className="keg-summary">
        <BeerArt
          onTap={onTap}
          kegShape={type?.shape}
          color={keg.beer_color}
          name={keg.beer_name}
          level={level}
          height={64}
        />
        <div className="keg-facts">
          <p className="keg-beer">{keg.beer_name}</p>
          <p className="muted">{type?.label ?? keg.type}</p>
          {/* Decorative: BeerArt's label and the text below already say the level. */}
          <progress className="level-bar" max={1} value={level} aria-hidden="true" />
          <p className="muted">
            {formatVolume(keg.remaining_l, units)} of {formatVolume(keg.size_l, units)}
          </p>
        </div>
      </div>
      <div className="btn-row keg-actions">
        {onTap && (
          <>
            <button type="button" className="btn btn-primary" onClick={() => setOpen("pour")}>
              Pour
            </button>
            <button type="button" className="btn" onClick={() => setOpen("set")}>
              Set level
            </button>
          </>
        )}
        <button type="button" className="btn" onClick={() => setOpen("edit")}>
          Edit keg
        </button>
        {onTap && (
          <button type="button" className="btn" onClick={() => change("PATCH", `/api/kegs/${keg.id}`, { tap_id: null })}>
            Take off
          </button>
        )}
        <button type="button" className="btn" onClick={kick}>
          Kick
        </button>
      </div>
      {(open === "pour" || open === "set") && (
        <form key={`${units}-${open}`} className="inline-form" onSubmit={saveLevel}>
          <label>
            {open === "pour" ? `Pour (${pourUnit(units)})` : `Remaining (${volumeUnit(units)})`}
            <input
              name="amount"
              type="number"
              min="0"
              step="any"
              defaultValue={open === "pour" ? pourPrefill(units) : volumeText(keg.remaining_l, units)}
            />
          </label>
          <div className="btn-row">
            <button type="submit" className="btn btn-primary">
              {open === "pour" ? "Save pour" : "Save level"}
            </button>
            <button type="button" className="btn" onClick={() => setOpen(null)}>
              Cancel
            </button>
          </div>
        </form>
      )}
      {open === "edit" && (
        <KegForm
          key={`${units}-${keg.id}`}
          keg={keg}
          units={units}
          onSaved={() => {
            setOpen(null);
            onChanged();
          }}
          onCancel={() => setOpen(null)}
          onSignedOut={onSignedOut}
        />
      )}
      {confirm.dialog}
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
