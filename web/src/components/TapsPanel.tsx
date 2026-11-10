import { ArrowDown, ArrowUp, GlassWater, Pencil, Plus, Power, Trash2 } from "lucide-react";
import { FormEvent, useEffect, useId, useState } from "react";
import { createPortal } from "react-dom";
import { ApiError, apiFetch, errorMessage, isUnauthenticated, Keg, KegType, Tap, Units } from "../api";
import KegCard from "./KegCard";
import KegForm from "./KegForm";
import Modal from "./Modal";

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/**
 * TapsPanel is the Taps page. Its header has the "Taps" heading, a summary (taps, how many
 * have a keg, kegs waiting) and "Add tap", which opens the "Add tap" dialog: an optional Label
 * (1 to 50 characters) and Save, which adds a tap at the end (POST /api/taps, body `{}` when the
 * label is empty so the API names it, else `{"label"}`); a server error keeps it open.
 * Taps (GET /api/taps) show in position order as a grid of cards, each named by its label,
 * headed "<position>. <label>" (plus "(disabled)") with icon buttons: Rename (reveals an
 * inline label form; PATCH /api/taps/{id}, sent only when changed), Move up and Move down
 * (PUT /api/taps/order with every id in the new order; absent at the ends), Disable or Enable
 * (PATCH enabled) and Remove (after a confirm prompt; DELETE /api/taps/{id}). A tap with a keg
 * shows it in KegCard (pour, set level, edit, take off, kick). An empty tap shows "Empty tap",
 * Add keg (KegForm for this tap in the "Add keg" dialog) and, while kegs wait (GET
 * /api/kegs?state=waiting), a "Waiting keg" select with Put on tap (PATCH /api/kegs/{id}
 * {"tap_id"}). Keg labels come from GET /api/keg-types, loaded once. Everything reloads after
 * every change, and server messages, such as conflicts, are shown. Volumes follow `units`
 * (default US); KegForm is keyed by it so an open form restarts in new units. Any 401 calls
 * `onSignedOut`. Waiting kegs themselves are managed on the On Deck page. Given `actionSlot`
 * (the phone top bar's slot), a second Add tap is portaled there; CSS shows one of the two.
 */
export default function TapsPanel({
  onSignedOut,
  units = "us",
  actionSlot,
}: {
  onSignedOut: () => void;
  units?: Units;
  actionSlot?: HTMLElement | null;
}) {
  const [taps, setTaps] = useState<Tap[]>([]);
  const [waiting, setWaiting] = useState<Keg[]>([]);
  const [types, setTypes] = useState<KegType[]>([]);
  // The tap whose Add keg dialog or rename form is open, if any.
  const [addingTo, setAddingTo] = useState<number | null>(null);
  const [addingTap, setAddingTap] = useState(false);
  const [renaming, setRenaming] = useState<number | null>(null);
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

  /** change sends one write, then reloads. */
  function change(method: string, path: string, body?: unknown, after?: () => void) {
    run(async () => {
      await apiFetch(method, path, body);
      after?.();
      await load();
    });
  }

  function rename(t: Tap, e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const label = String(new FormData(e.currentTarget).get("label") ?? "");
    if (label === t.label) setRenaming(null);
    else change("PATCH", `/api/taps/${t.id}`, { label }, () => setRenaming(null));
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
    change("PATCH", `/api/kegs/${new FormData(e.currentTarget).get("keg")}`, { tap_id: t.id });
  }

  function saved() {
    setAddingTo(null);
    setAddingTap(false);
    run(load);
  }

  const addTap = (
    <button type="button" className="btn btn-primary page-action" onClick={() => setAddingTap(true)}>
      <Plus aria-hidden="true" />
      Add tap
    </button>
  );

  const typeLabel = (k: Keg) => types.find((t) => t.type === k.type)?.label ?? k.type;
  const withKeg = taps.filter((t) => t.keg).length;

  return (
    <section className="page">
      <div className="page-header">
        <div>
          <h2>Taps</h2>
          <p className="muted">
            {plural(taps.length, "tap", "taps")}, {withKeg} with a keg, {plural(waiting.length, "keg", "kegs")} waiting
          </p>
        </div>
        {addTap}
      </div>
      {actionSlot && createPortal(addTap, actionSlot)}
      {error && <p role="alert">{error}</p>}
      <ol className="tap-grid" aria-label="Taps">
        {taps.map((t, i) => (
          <li key={t.id}>
            <article className="card tap-card" aria-label={t.label}>
              <div className="tap-head">
                <h3>
                  {t.position}. {t.label}
                  {!t.enabled && " (disabled)"}
                </h3>
                <div className="tap-tools">
                  <IconButton label="Rename" onClick={() => setRenaming(renaming === t.id ? null : t.id)}>
                    <Pencil aria-hidden="true" />
                  </IconButton>
                  {i > 0 && (
                    <IconButton label="Move up" onClick={() => move(i, -1)}>
                      <ArrowUp aria-hidden="true" />
                    </IconButton>
                  )}
                  {i < taps.length - 1 && (
                    <IconButton label="Move down" onClick={() => move(i, 1)}>
                      <ArrowDown aria-hidden="true" />
                    </IconButton>
                  )}
                  <IconButton
                    label={t.enabled ? "Disable" : "Enable"}
                    onClick={() => change("PATCH", `/api/taps/${t.id}`, { enabled: !t.enabled })}
                  >
                    <Power aria-hidden="true" />
                  </IconButton>
                  <IconButton label="Remove" onClick={() => remove(t)}>
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </div>
              </div>
              {renaming === t.id && (
                <form className="inline-form" aria-label={`Rename ${t.label}`} onSubmit={(e) => rename(t, e)}>
                  <label>
                    Label
                    <input name="label" defaultValue={t.label} />
                  </label>
                  <div className="btn-row">
                    <button type="submit" className="btn btn-primary">
                      Save
                    </button>
                    <button type="button" className="btn" onClick={() => setRenaming(null)}>
                      Cancel
                    </button>
                  </div>
                </form>
              )}
              {t.keg ? (
                <KegCard
                  key={t.keg.id}
                  keg={t.keg}
                  types={types}
                  units={units}
                  onChanged={() => run(load)}
                  onSignedOut={onSignedOut}
                />
              ) : (
                <div className="empty-tap">
                  <div className="empty-glass" aria-hidden="true">
                    <GlassWater />
                  </div>
                  <p className="muted">Empty tap</p>
                  <div className="btn-row">
                    <button type="button" className="btn btn-primary" onClick={() => setAddingTo(t.id)}>
                      Add keg
                    </button>
                  </div>
                  {waiting.length > 0 && (
                    <form className="inline-form put-on-tap" onSubmit={(e) => putOnTap(t, e)}>
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
                      <button type="submit" className="btn">
                        Put on tap
                      </button>
                    </form>
                  )}
                </div>
              )}
            </article>
          </li>
        ))}
      </ol>
      {addingTo !== null && (
        <Modal title="Add keg" onClose={() => setAddingTo(null)}>
          <KegForm
            key={units}
            tapId={addingTo}
            units={units}
            onSaved={saved}
            onCancel={() => setAddingTo(null)}
            onSignedOut={onSignedOut}
          />
        </Modal>
      )}
      {addingTap && (
        <Modal title="Add tap" onClose={() => setAddingTap(false)}>
          <AddTapForm onSaved={saved} onCancel={() => setAddingTap(false)} onSignedOut={onSignedOut} />
        </Modal>
      )}
    </section>
  );
}

/**
 * AddTapForm is the Add tap dialog's form: an optional Label (1 to 50 characters; the API is
 * the validator and its field message shows under it) and Save, which sends POST /api/taps with
 * `{}` when the label is blank, so the API names the tap "Tap N", else `{"label"}`. Success calls
 * `onSaved`, a 401 `onSignedOut`.
 */
function AddTapForm({ onSaved, onCancel, onSignedOut }: { onSaved: () => void; onCancel: () => void; onSignedOut: () => void }) {
  const id = useId();
  const [label, setLabel] = useState("");
  const [fieldError, setFieldError] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setFieldError("");
    setSaving(true);
    try {
      await apiFetch("POST", "/api/taps", label.trim() ? { label } : {});
      onSaved();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else if (err instanceof ApiError && err.fields?.label) setFieldError(err.fields.label);
      else setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="inline-form" onSubmit={submit} noValidate aria-label="Add tap">
      <div className="field">
        <label>
          Label
          <input
            value={label}
            maxLength={50}
            aria-invalid={fieldError ? true : undefined}
            aria-describedby={fieldError ? `${id}-label-error` : undefined}
            onChange={(e) => setLabel(e.target.value)}
          />
        </label>
        {fieldError && (
          <span id={`${id}-label-error`} className="field-error">
            {fieldError}
          </span>
        )}
      </div>
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

/** IconButton is a square icon-only button named by `label` (also its tooltip). */
function IconButton({ label, onClick, children }: { label: string; onClick: () => void; children: JSX.Element }) {
  return (
    <button type="button" className="icon-btn" aria-label={label} title={label} onClick={onClick}>
      {children}
    </button>
  );
}
