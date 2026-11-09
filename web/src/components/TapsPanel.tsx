import { FormEvent, useEffect, useId, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated, Keg, KegType, Tap } from "../api";
import { flOzToLitres, formatGallons, gallons, gallonsToLitres, inputToLitres } from "../units";
import BeerArt from "./BeerArt";
import KegForm from "./KegForm";

/** Editing is the open keg form: none, add (on tap `tapId` or waiting), or edit one keg. */
type Editing = { kind: "none" } | { kind: "add"; tapId?: number } | { kind: "edit"; keg: Keg };

/** Level is the open inline level form of one keg: a pour or a set level. */
type Level = { kegId: number; kind: "pour" | "set" } | null;

/**
 * TapsPanel is the admin taps panel. It lists taps in position order (GET /api/taps), adds a
 * tap at the end (POST /api/taps), renames and enables or disables one (PATCH
 * /api/taps/{id}), moves one up or down by sending every id in the new order (PUT
 * /api/taps/order), and removes one after a confirm prompt (DELETE /api/taps/{id}). Each tap
 * shows its keg: the beer as a pint glass filled to the keg's level (remaining over size, also
 * in the drawing's label), beer name, keg type, and level in US gallons, with Pour (US fl oz, prefilled 16, so a pour is
 * two clicks; POST /api/kegs/{id}/pours), Set level (gallons; PATCH remaining_l, sent only when
 * changed), Kick (after a confirm prompt; PATCH {"kicked": true}), Take off and Edit keg; an empty tap offers Add keg and, while kegs are
 * waiting, Put on tap. An On deck list shows waiting kegs (GET /api/kegs?state=waiting) in
 * their keg type's drawing, filled to their level, with the same keg actions but Take off, and Add keg for a waiting
 * keg. Keg labels and drawings come from GET /api/keg-types, loaded once. Keg changes go to
 * PATCH /api/kegs/{id} or KegForm. Everything reloads after every change and server messages,
 * such as conflicts, are shown. Every successful keg change that can move a beer on or off a tap or into
 * keg history (create, edit, put on or take off a tap, kick) calls `onKegsChanged`, so keg
 * history and beer artwork can reload; pours, level sets and tap-only changes do not. Any 401 calls `onSignedOut`.
 */
export default function TapsPanel({
  onSignedOut,
  onKegsChanged,
}: {
  onSignedOut: () => void;
  onKegsChanged?: () => void;
}) {
  const deckId = useId();
  const [taps, setTaps] = useState<Tap[]>([]);
  const [waiting, setWaiting] = useState<Keg[]>([]);
  const [types, setTypes] = useState<KegType[]>([]);
  const [editing, setEditing] = useState<Editing>({ kind: "none" });
  const [level, setLevel] = useState<Level>(null);
  const [error, setError] = useState("");

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  async function load() {
    const [t, w] = await Promise.all([
      apiFetch<Tap[]>("GET", "/api/taps"),
      apiFetch<Keg[]>("GET", "/api/kegs?state=waiting"),
    ]);
    setTaps(t);
    setWaiting(w);
  }

  useEffect(() => {
    run(async () => {
      const [, kegTypes] = await Promise.all([load(), apiFetch<KegType[]>("GET", "/api/keg-types")]);
      setTypes(kegTypes);
    });
  }, []);

  /** change sends one write, reports it when `kegsChanged`, then closes any level form and reloads. */
  function change(method: string, path: string, body?: unknown, kegsChanged = false) {
    run(async () => {
      await apiFetch(method, path, body);
      if (kegsChanged) onKegsChanged?.();
      setLevel(null);
      await load();
    });
  }

  function rename(t: Tap, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const label = String(new FormData(e.currentTarget).get("label") ?? "");
    if (label !== t.label) change("PATCH", `/api/taps/${t.id}`, { label });
  }

  function move(i: number, by: -1 | 1) {
    const ids = taps.map((t) => t.id);
    [ids[i], ids[i + by]] = [ids[i + by], ids[i]];
    change("PUT", "/api/taps/order", { ids });
  }

  function remove(t: Tap) {
    if (window.confirm(`Remove ${t.label}?`)) change("DELETE", `/api/taps/${t.id}`);
  }

  function putOnTap(t: Tap, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    change("PATCH", `/api/kegs/${new FormData(e.currentTarget).get("keg")}`, { tap_id: t.id }, true);
  }

  function saveLevel(k: Keg, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const text = String(new FormData(e.currentTarget).get("amount") ?? "");
    if (level?.kind === "pour") {
      change("POST", `/api/kegs/${k.id}/pours`, { amount_l: inputToLitres(text, flOzToLitres) });
    } else if (text === gallons(k.remaining_l)) {
      // An untouched level must not be resent: the gallons round trip would move it.
      setLevel(null);
    } else {
      change("PATCH", `/api/kegs/${k.id}`, { remaining_l: inputToLitres(text, gallonsToLitres) });
    }
  }

  function kick(k: Keg) {
    if (window.confirm(`Kick the ${k.beer_name} keg? It moves to keg history.`)) {
      change("PATCH", `/api/kegs/${k.id}`, { kicked: true }, true);
    }
  }

  function saved() {
    setEditing({ kind: "none" });
    onKegsChanged?.();
    run(load);
  }

  const typeOf = (k: Keg) => types.find((t) => t.type === k.type);
  const typeLabel = (k: Keg) => typeOf(k)?.label ?? k.type;

  /** kegView is a keg's artwork, beer, type and level, then its actions. */
  const kegView = (k: Keg) => (
    <div>
      <BeerArt
        onTap={k.state === "on_tap"}
        kegShape={typeOf(k)?.shape}
        color={k.beer_color}
        name={k.beer_name}
        level={k.remaining_l / k.size_l}
        height={64}
      />
      <p>{k.beer_name}</p>
      <p>{typeLabel(k)}</p>
      <p>
        {formatGallons(k.remaining_l)} of {formatGallons(k.size_l)}
      </p>
      <button type="button" onClick={() => setLevel({ kegId: k.id, kind: "pour" })}>
        Pour
      </button>
      <button type="button" onClick={() => setLevel({ kegId: k.id, kind: "set" })}>
        Set level
      </button>
      {level?.kegId === k.id && (
        <form key={level.kind} onSubmit={(e) => saveLevel(k, e)}>
          <label>
            {level.kind === "pour" ? "Pour (fl oz)" : "Remaining (gal)"}
            <input
              name="amount"
              type="number"
              min="0"
              step="any"
              defaultValue={level.kind === "pour" ? "16" : gallons(k.remaining_l)}
            />
          </label>
          <button type="submit">{level.kind === "pour" ? "Save pour" : "Save level"}</button>
          <button type="button" onClick={() => setLevel(null)}>
            Cancel
          </button>
        </form>
      )}
      <button type="button" onClick={() => kick(k)}>
        Kick
      </button>
      {k.state === "on_tap" && (
        <button type="button" onClick={() => change("PATCH", `/api/kegs/${k.id}`, { tap_id: null }, true)}>
          Take off
        </button>
      )}
      <button type="button" onClick={() => setEditing({ kind: "edit", keg: k })}>
        Edit keg
      </button>
    </div>
  );

  return (
    <section>
      <h2>Taps</h2>
      <ol aria-label="Taps">
        {taps.map((t, i) => (
          <li key={t.id} aria-label={t.label}>
            <h3>
              {t.position}. {t.label}
              {!t.enabled && " (disabled)"}
            </h3>
            {/* Keyed by label so a reload after a rename resets the input. */}
            <form key={t.label} onSubmit={(e) => rename(t, e)}>
              <label>
                Label
                <input name="label" defaultValue={t.label} />
              </label>
              <button type="submit">Rename</button>
            </form>
            <button type="button" onClick={() => change("PATCH", `/api/taps/${t.id}`, { enabled: !t.enabled })}>
              {t.enabled ? "Disable" : "Enable"}
            </button>
            {i > 0 && (
              <button type="button" onClick={() => move(i, -1)}>
                Move up
              </button>
            )}
            {i < taps.length - 1 && (
              <button type="button" onClick={() => move(i, 1)}>
                Move down
              </button>
            )}
            <button type="button" onClick={() => remove(t)}>
              Remove
            </button>
            {t.keg ? (
              kegView(t.keg)
            ) : (
              <div>
                <button type="button" onClick={() => setEditing({ kind: "add", tapId: t.id })}>
                  Add keg
                </button>
                {waiting.length > 0 && (
                  <form onSubmit={(e) => putOnTap(t, e)}>
                    <label>
                      Waiting keg
                      <select name="keg">
                        {waiting.map((k) => (
                          <option key={k.id} value={k.id}>
                            {k.beer_name}, {typeLabel(k)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <button type="submit">Put on tap</button>
                  </form>
                )}
              </div>
            )}
          </li>
        ))}
      </ol>
      <button type="button" onClick={() => change("POST", "/api/taps", {})}>
        Add tap
      </button>
      <section aria-labelledby={deckId}>
        <h3 id={deckId}>On deck</h3>
        <ul>
          {waiting.map((k) => (
            <li key={k.id}>{kegView(k)}</li>
          ))}
        </ul>
        <button type="button" onClick={() => setEditing({ kind: "add" })}>
          Add keg
        </button>
      </section>
      {editing.kind !== "none" && (
        <KegForm
          key={editing.kind === "edit" ? editing.keg.id : `add-${editing.tapId}`}
          keg={editing.kind === "edit" ? editing.keg : undefined}
          tapId={editing.kind === "add" ? editing.tapId : undefined}
          onSaved={saved}
          onCancel={() => setEditing({ kind: "none" })}
          onSignedOut={onSignedOut}
        />
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
