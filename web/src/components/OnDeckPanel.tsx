import { Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { apiFetch, Beer, errorMessage, isUnauthenticated, Keg, KegType, Units } from "../api";
import BeerArt from "./BeerArt";
import KegCard from "./KegCard";
import KegForm from "./KegForm";
import Modal from "./Modal";

/** Item is one On Deck entry: a waiting keg, or a beer being made that has no keg yet. */
type Item = { kind: "keg"; keg: Keg } | { kind: "beer"; beer: Beer };

const byName = (a: Beer, b: Beer) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" }) || a.id - b.id;

/**
 * onDeck builds the On Deck list in the TV's coming-soon order (research R5, GetMenu in
 * api/internal/kegs/menu.go): waiting kegs (current kegs off a tap) in API order, oldest first;
 * then conditioning, then fermenting beers with no current keg, each by name ignoring case,
 * then id. A beer with any current keg, on a tap or waiting, is not listed as a beer.
 */
function onDeck(kegs: Keg[], beers: Beer[]): Item[] {
  const kegged = new Set(kegs.map((k) => k.beer_id));
  const making = (status: Beer["status"]) =>
    beers
      .filter((b) => b.status === status && !kegged.has(b.id))
      .sort(byName)
      .map((beer): Item => ({ kind: "beer", beer }));
  return [
    ...kegs.filter((k) => k.tap_id === null).map((keg): Item => ({ kind: "keg", keg })),
    ...making("conditioning"),
    ...making("fermenting"),
  ];
}

const statusLabel: Record<string, string> = { conditioning: "Conditioning", fermenting: "Fermenting" };

/**
 * OnDeckPanel is the On Deck page: what the TV shows as coming soon, built from GET /api/kegs
 * (current kegs) and GET /api/beers (current beers) by `onDeck`, with keg type labels from GET
 * /api/keg-types. Waiting kegs show in KegCard with all their actions (pour, set level, the
 * inline Edit keg form, kick; no Take off, as they are on no tap), and the page reloads after
 * each. Beers show their artwork, name, style, ABV and status. An empty list says "Nothing on
 * deck". "Add keg" opens KegForm with no tap in the "Add keg" dialog, adding a waiting keg;
 * a save closes it and reloads. Volumes follow `units` (default US). Any 401 calls
 * `onSignedOut`; other load errors are shown. Given `actionSlot` (the phone top bar's slot), a
 * second Add keg is portaled there; CSS shows one of the two.
 */
export default function OnDeckPanel({
  onSignedOut,
  units = "us",
  actionSlot,
}: {
  onSignedOut: () => void;
  units?: Units;
  actionSlot?: HTMLElement | null;
}) {
  const [items, setItems] = useState<Item[] | null>(null);
  const [types, setTypes] = useState<KegType[]>([]);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState("");

  async function load() {
    setError("");
    try {
      const [kegs, beers] = await Promise.all([apiFetch<Keg[]>("GET", "/api/kegs"), apiFetch<Beer[]>("GET", "/api/beers")]);
      setItems(onDeck(kegs, beers));
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  useEffect(() => {
    load();
    apiFetch<KegType[]>("GET", "/api/keg-types")
      .then(setTypes)
      .catch((err) => {
        if (isUnauthenticated(err)) onSignedOut();
        else setError(errorMessage(err));
      });
  }, []);

  function saved() {
    setAdding(false);
    load();
  }

  const addKeg = (
    <button type="button" className="btn btn-primary page-action" onClick={() => setAdding(true)}>
      <Plus aria-hidden="true" />
      Add keg
    </button>
  );

  return (
    <section className="page">
      <div className="page-header">
        <h2>On Deck</h2>
        {addKeg}
      </div>
      {actionSlot && createPortal(addKeg, actionSlot)}
      {error && <p role="alert">{error}</p>}
      {items?.length === 0 && <p className="card muted">Nothing on deck</p>}
      {items && items.length > 0 && (
        <ol className="tap-grid" aria-label="On deck">
          {items.map((it) =>
            it.kind === "keg" ? (
              <li key={`keg-${it.keg.id}`}>
                <article className="card tap-card" aria-label={it.keg.beer_name}>
                  <KegCard keg={it.keg} types={types} units={units} onChanged={load} onSignedOut={onSignedOut} />
                </article>
              </li>
            ) : (
              <li key={`beer-${it.beer.id}`}>
                <article className="card tap-card" aria-label={it.beer.name}>
                  <div className="keg-summary">
                    <BeerArt onTap={false} color={it.beer.color} name={it.beer.name} height={64} />
                    <div className="keg-facts">
                      <p className="keg-beer">{it.beer.name}</p>
                      {it.beer.style && <p className="muted">{it.beer.style}</p>}
                      {it.beer.abv !== null && <p className="muted">{it.beer.abv}%</p>}
                      <p className="muted">{statusLabel[it.beer.status]}</p>
                    </div>
                  </div>
                </article>
              </li>
            ),
          )}
        </ol>
      )}
      {adding && (
        <Modal title="Add keg" onClose={() => setAdding(false)}>
          <KegForm key={units} units={units} onSaved={saved} onCancel={() => setAdding(false)} onSignedOut={onSignedOut} />
        </Modal>
      )}
    </section>
  );
}
