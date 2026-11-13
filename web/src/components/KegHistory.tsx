import { useEffect, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated, Keg, KegType, Units } from "../api";
import { formatVolume } from "../units";
import { useConfirmAction } from "./ConfirmDialog";

const day = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString() : "—");

/**
 * KegHistory is the Keg History page, the heading above one card: kicked kegs newest kicked first (GET
 * /api/kegs?state=kicked) with beer, the last tap's label, keg type (labels from GET
 * /api/keg-types), size in `units` (US gallons or litres, default US), and tapped and kicked
 * dates, an em dash when a keg was never on a tap. A keg is deleted after a confirmation dialog (DELETE
 * /api/kegs/{id}) and the list reloads. Any 401 calls `onSignedOut`.
 */
export default function KegHistory({ onSignedOut, units = "us" }: { onSignedOut: () => void; units?: Units }) {
  const [kegs, setKegs] = useState<Keg[]>([]);
  const [types, setTypes] = useState<KegType[]>([]);
  const [error, setError] = useState("");
  const confirm = useConfirmAction();

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  const load = async () => setKegs(await apiFetch<Keg[]>("GET", "/api/kegs?state=kicked"));

  useEffect(() => {
    run(async () => {
      const [, kegTypes] = await Promise.all([load(), apiFetch<KegType[]>("GET", "/api/keg-types")]);
      setTypes(kegTypes);
    });
  }, []);

  function remove(k: Keg) {
    confirm.ask(
      { title: "Delete keg", message: `Delete the ${k.beer_name} keg from history permanently?`, confirmLabel: "Delete" },
      () =>
        run(async () => {
          await apiFetch("DELETE", `/api/kegs/${k.id}`);
          await load();
        }),
    );
  }

  return (
    <section className="page">
      <h2 className="page-title">Keg History</h2>
      <div className="card">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Beer</th>
                <th>Tap</th>
                <th>Type</th>
                <th>Size</th>
                <th>Tapped</th>
                <th>Kicked</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {kegs.map((k) => (
                <tr key={k.id}>
                  <td>{k.beer_name}</td>
                  <td>{k.tap_label ?? "—"}</td>
                  <td>{types.find((t) => t.type === k.type)?.label ?? k.type}</td>
                  <td>{formatVolume(k.size_l, units)}</td>
                  <td>{day(k.tapped_at)}</td>
                  <td>{day(k.kicked_at)}</td>
                  <td>
                    <button type="button" className="btn btn-danger" onClick={() => remove(k)}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {confirm.dialog}
        {error && <p role="alert">{error}</p>}
      </div>
    </section>
  );
}
