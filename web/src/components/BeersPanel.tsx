import { useEffect, useState } from "react";
import { apiFetch, Beer, errorMessage, isUnauthenticated } from "../api";
import BeerArt from "./BeerArt";
import BeerForm from "./BeerForm";

/** Editing is which form is open: none, the add form, or the edit form for one beer. */
type Editing = { kind: "none" } | { kind: "add" } | { kind: "edit"; beer: Beer };

/**
 * BeersPanel is the admin beer library: it lists current beers (GET /api/beers) with their
 * artwork, drawn from each beer's `color` with no extra request as a pint glass when `on_tap`
 * and a corny keg otherwise, name, style, ABV and status, and opens BeerForm to add a beer or edit one. "Show archived" switches to the
 * archived beers (GET /api/beers?archived=true); current beers can be archived and archived
 * ones restored (PATCH {"archived": ...}), and any beer deleted after a confirm prompt
 * (DELETE /api/beers/{id}). Beers imported from Brewfather are labelled "Brewfather", with
 * a note when Brewfather no longer has their batch or archived it while a keg is current, and
 * have a Re-sync button (POST /api/beers/{id}/sync). The list reloads after every change, and
 * whenever `refresh` changes (kegs changed or Brewfather imported or synced elsewhere), keeping
 * the view and any open form. Any 401 calls `onSignedOut`.
 */
export default function BeersPanel({ onSignedOut, refresh }: { onSignedOut: () => void; refresh?: number }) {
  const [beers, setBeers] = useState<Beer[]>([]);
  const [editing, setEditing] = useState<Editing>({ kind: "none" });
  const [error, setError] = useState("");
  const [archived, setArchived] = useState(false);

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  const fetchList = (archivedView: boolean) =>
    apiFetch<Beer[]>("GET", archivedView ? "/api/beers?archived=true" : "/api/beers");
  const load = async () => setBeers(await fetchList(archived));

  useEffect(() => {
    // A late answer for the view just left, or an older refresh, must not replace the newer list.
    let stale = false;
    run(async () => {
      const list = await fetchList(archived);
      if (!stale) setBeers(list);
    });
    return () => {
      stale = true;
    };
  }, [archived, refresh]);

  function setArchivedState(b: Beer, value: boolean) {
    run(async () => {
      await apiFetch("PATCH", `/api/beers/${b.id}`, { archived: value });
      await load();
    });
  }

  function resync(b: Beer) {
    run(async () => {
      await apiFetch("POST", `/api/beers/${b.id}/sync`);
      await load();
    });
  }

  function saved() {
    setEditing({ kind: "none" });
    run(load);
  }

  function remove(b: Beer) {
    if (!window.confirm(`Delete ${b.name} permanently?`)) return;
    run(async () => {
      await apiFetch("DELETE", `/api/beers/${b.id}`);
      await load();
    });
  }

  return (
    <section>
      <h2>Beer library</h2>
      <label>
        <input type="checkbox" checked={archived} onChange={(e) => setArchived(e.target.checked)} />
        Show archived
      </label>
      <table>
        <thead>
          <tr>
            <th>Artwork</th>
            <th>Name</th>
            <th>Style</th>
            <th>ABV</th>
            <th>Status</th>
            <th>Source</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {beers.map((b) => (
            <tr key={b.id}>
              <td>
                <BeerArt onTap={b.on_tap} color={b.color} name={b.name} height={64} />
              </td>
              <td>{b.name}</td>
              <td>{b.style}</td>
              <td>{b.abv === null ? "" : `${b.abv}%`}</td>
              <td>
                {b.status}
                {b.archived_at && (
                  <>
                    {" "}
                    <span>archived</span>
                  </>
                )}
              </td>
              <td>
                {b.source === "brewfather" && <span>Brewfather</span>}
                {b.sync?.not_found && (
                  <>
                    {" "}
                    <span>Not found in Brewfather</span>
                  </>
                )}
                {b.sync?.archive_pending && (
                  <>
                    {" "}
                    <span>Archived in Brewfather; kick its keg to archive</span>
                  </>
                )}
              </td>
              <td>
                <button type="button" onClick={() => setEditing({ kind: "edit", beer: b })}>
                  Edit
                </button>
                {b.source === "brewfather" && (
                  <button type="button" onClick={() => resync(b)}>
                    Re-sync
                  </button>
                )}
                {b.archived_at ? (
                  <button type="button" onClick={() => setArchivedState(b, false)}>
                    Restore
                  </button>
                ) : (
                  <button type="button" onClick={() => setArchivedState(b, true)}>
                    Archive
                  </button>
                )}
                <button type="button" onClick={() => remove(b)}>
                  Delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {editing.kind === "none" && (
        <button type="button" onClick={() => setEditing({ kind: "add" })}>
          Add beer
        </button>
      )}
      {editing.kind !== "none" && (
        <BeerForm
          key={editing.kind === "edit" ? editing.beer.id : "add"}
          beer={editing.kind === "edit" ? editing.beer : undefined}
          onSaved={saved}
          onCancel={() => setEditing({ kind: "none" })}
          onSignedOut={onSignedOut}
        />
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
