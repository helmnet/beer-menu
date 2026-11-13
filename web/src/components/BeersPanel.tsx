import { Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { apiFetch, Beer, errorMessage, isUnauthenticated } from "../api";
import BeerArt from "./BeerArt";
import BeerForm from "./BeerForm";
import Modal from "./Modal";

/** Editing is which dialog is open: none, Add beer, or Edit for one beer. */
type Editing = { kind: "none" } | { kind: "add" } | { kind: "edit"; beer: Beer };

/**
 * BeersPanel is the Beer Library page, the heading with "Add beer" above one card: it lists current
 * beers (GET /api/beers) with their artwork, drawn from each beer's `color` with no extra request
 * as a pint glass when `on_tap` and a corny keg otherwise, name, style, ABV and status. "Add beer"
 * and each beer's "Edit" open BeerForm in a dialog titled "Add beer" or "Edit <name>"; a server
 * error keeps it open with the field messages, and a save closes it and reloads the list. "Show archived" switches to the archived beers (GET
 * /api/beers?archived=true); current beers can be archived and archived ones restored (PATCH
 * {"archived": ...}), and any beer deleted after a confirm prompt (DELETE /api/beers/{id}). Beers
 * imported from Brewfather are labelled "Brewfather", with a note when Brewfather no longer has
 * their batch or archived it while a keg is current, and have a Re-sync button (POST
 * /api/beers/{id}/sync). The list sits in a sideways-scrolling wrapper and reloads after every
 * change; the page mounts fresh each time it is shown, so changes made on other pages show then.
 * Any 401 calls `onSignedOut`. Given `actionSlot` (the phone top bar's slot), a second Add beer
 * is portaled there; CSS shows one of the two.
 */
export default function BeersPanel({ onSignedOut, actionSlot }: { onSignedOut: () => void; actionSlot?: HTMLElement | null }) {
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
    // A late answer for the view just left must not replace the newer list.
    let stale = false;
    run(async () => {
      const list = await fetchList(archived);
      if (!stale) setBeers(list);
    });
    return () => {
      stale = true;
    };
  }, [archived]);

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

  const addBeer = (
    <button type="button" className="btn btn-primary page-action" onClick={() => setEditing({ kind: "add" })}>
      <Plus aria-hidden="true" />
      Add beer
    </button>
  );

  return (
    <section className="page">
      <div className="page-header">
        <h2>Beer Library</h2>
        {addBeer}
      </div>
      {actionSlot && createPortal(addBeer, actionSlot)}
      <div className="card">
        <label>
          <input type="checkbox" checked={archived} onChange={(e) => setArchived(e.target.checked)} />
          Show archived
        </label>
        <div className="table-wrap">
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
                    <button type="button" className="btn" onClick={() => setEditing({ kind: "edit", beer: b })}>
                      Edit
                    </button>
                    {b.source === "brewfather" && (
                      <button type="button" className="btn" onClick={() => resync(b)}>
                        Re-sync
                      </button>
                    )}
                    {b.archived_at ? (
                      <button type="button" className="btn" onClick={() => setArchivedState(b, false)}>
                        Restore
                      </button>
                    ) : (
                      <button type="button" className="btn" onClick={() => setArchivedState(b, true)}>
                        Archive
                      </button>
                    )}
                    <button type="button" className="btn btn-danger" onClick={() => remove(b)}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {editing.kind !== "none" && (
          <Modal
            title={editing.kind === "edit" ? `Edit ${editing.beer.name}` : "Add beer"}
            onClose={() => setEditing({ kind: "none" })}
          >
            <BeerForm
              beer={editing.kind === "edit" ? editing.beer : undefined}
              onSaved={saved}
              onCancel={() => setEditing({ kind: "none" })}
              onSignedOut={onSignedOut}
            />
          </Modal>
        )}
        {error && <p role="alert">{error}</p>}
      </div>
    </section>
  );
}
