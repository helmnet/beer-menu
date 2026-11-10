import { FormEvent, useEffect, useId, useState } from "react";
import {
  ApiError,
  apiFetch,
  BatchSummary,
  Beer,
  BrewfatherStatus,
  errorMessage,
  ImportResult,
  isUnauthenticated,
  SyncInterval,
} from "../api";

const POLL_MS = 2000;

const intervals: [SyncInterval, string][] = [
  ["off", "Off"],
  ["15m", "Every 15 minutes"],
  ["1h", "Every hour"],
  ["6h", "Every 6 hours"],
  ["24h", "Every 24 hours"],
];

const outcomes: Record<ImportResult["outcome"], string> = {
  imported: "Imported",
  already_imported: "Already imported",
  archived: "Archived in Brewfather, not imported",
  not_found: "Not found in Brewfather",
  rate_limited: "Rate limited, not imported",
  failed: "Failed",
};

/**
 * BrewfatherPanel is the Brewfather page. It loads GET /api/brewfather and shows a
 * not-set-up note when the server has no encryption key; otherwise two cards. "Connection":
 * the connection state with a connect form (PUT /api/brewfather/connection) when not connected
 * or a reconnect is needed, or a Disconnect button (confirm, then DELETE) when connected; the
 * sync interval (PUT /api/brewfather/schedule); and, while connected, "Sync now" (POST
 * /api/brewfather/sync) and the last sync result. "Batches", only while connected: the batch
 * list (GET /api/brewfather/batches, Refresh with ?refresh=true) with checkboxes to import
 * (POST /api/brewfather/imports) and per-batch results. The API key field is a password field
 * that is never prefilled and is cleared on submit, whatever the answer. While a sync runs it
 * polls the status every 2 s, stopping on unmount; a finished sync reloads the batch list. Any
 * 401 calls `onSignedOut`.
 */
export default function BrewfatherPanel({ onSignedOut }: { onSignedOut: () => void }) {
  const id = useId();
  const [status, setStatus] = useState<BrewfatherStatus | null>(null);
  const [userId, setUserId] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [batches, setBatches] = useState<BatchSummary[]>([]);
  // Names of the beers batches were imported as, by beer id; batches only carry the id.
  const [beerNames, setBeerNames] = useState<Map<number, string>>(new Map());
  const [selected, setSelected] = useState<Set<string>>(new Set());
  // Each import result with its batch's name, kept from before the import since the list then reloads.
  const [results, setResults] = useState<{ name: string; result: ImportResult }[]>([]);
  // A count, not a flag, since actions can overlap (an import while a finished sync reloads the list).
  const [busy, setBusy] = useState(0);
  const [error, setError] = useState("");

  async function run(action: () => Promise<void>) {
    setError("");
    setBusy((n) => n + 1);
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    } finally {
      setBusy((n) => n - 1);
    }
  }

  const loadStatus = async () => setStatus(await apiFetch<BrewfatherStatus>("GET", "/api/brewfather"));

  async function loadBatches(refresh = false) {
    const list = await apiFetch<{ batches: BatchSummary[] }>(
      "GET",
      refresh ? "/api/brewfather/batches?refresh=true" : "/api/brewfather/batches",
    );
    const names = new Map<number, string>();
    if (list.batches.some((b) => b.beer_id !== null)) {
      const lists = await Promise.all([
        apiFetch<Beer[]>("GET", "/api/beers"),
        apiFetch<Beer[]>("GET", "/api/beers?archived=true"),
      ]);
      for (const b of lists.flat()) names.set(b.id, b.name);
    }
    setBatches(list.batches);
    setBeerNames(names);
    setSelected(new Set());
  }

  useEffect(() => {
    run(loadStatus);
  }, []);

  const listable = !!status?.connected && !status.reconnect_needed;
  useEffect(() => {
    if (listable) run(() => loadBatches());
    else setBatches([]);
  }, [listable]);

  const running = !!status?.sync_running;
  useEffect(() => {
    if (!running) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = () => {
      timer = setTimeout(async () => {
        try {
          const next = await apiFetch<BrewfatherStatus>("GET", "/api/brewfather");
          if (stopped) return;
          setStatus(next);
          if (next.sync_running) poll();
          else {
            // A sync imports new batches, so the imported markers may have changed.
            if (next.connected && !next.reconnect_needed) run(() => loadBatches());
          }
        } catch (err) {
          if (stopped) return;
          if (isUnauthenticated(err)) onSignedOut();
          else {
            setError(errorMessage(err));
            // Keep polling, or a blip leaves the panel stuck showing the sync as running.
            poll();
          }
        }
      }, POLL_MS);
    };
    poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [running]);

  function connect(e: FormEvent) {
    e.preventDefault();
    const credentials = { user_id: userId, api_key: apiKey };
    setApiKey("");
    run(async () => {
      setStatus(await apiFetch<BrewfatherStatus>("PUT", "/api/brewfather/connection", credentials));
      setUserId("");
    });
  }

  function disconnect() {
    if (!window.confirm("Disconnect Brewfather? Imported beers stay, and scheduled sync stops.")) return;
    run(async () => {
      await apiFetch("DELETE", "/api/brewfather/connection");
      setResults([]);
      await loadStatus();
    });
  }

  function changeInterval(interval: SyncInterval) {
    run(async () => setStatus(await apiFetch<BrewfatherStatus>("PUT", "/api/brewfather/schedule", { interval })));
  }

  function syncNow() {
    run(async () => {
      try {
        setStatus(await apiFetch<BrewfatherStatus>("POST", "/api/brewfather/sync"));
      } catch (err) {
        if (!(err instanceof ApiError && err.code === "sync_running")) throw err;
        // A scheduled sync started first: reload the status so the poll follows that sync.
        await loadStatus();
      }
    });
  }

  function toggle(id: string, on: boolean) {
    const next = new Set(selected);
    if (on) next.add(id);
    else next.delete(id);
    setSelected(next);
  }

  function importSelected() {
    const chosen = batches.filter((b) => selected.has(b.id));
    const names = new Map(chosen.map((b) => [b.id, b.name]));
    run(async () => {
      const answer = await apiFetch<{ results: ImportResult[] }>("POST", "/api/brewfather/imports", {
        batch_ids: chosen.map((b) => b.id),
      });
      setResults(answer.results.map((result) => ({ name: names.get(result.batch_id) ?? result.batch_id, result })));
      await loadBatches();
    });
  }

  if (!status) {
    return (
      <div className="page">
        <h2 className="page-title">Brewfather</h2>
        {error && <p role="alert">{error}</p>}
      </div>
    );
  }

  if (!status.configured) {
    return (
      <div className="page">
        <h2 className="page-title">Brewfather</h2>
        <section className="card">
          <p>
            Brewfather import is not set up on this server. Set BEER_MENU_SECRET_KEY_FILE (or
            BEER_MENU_SECRET_KEY) on the API and restart it.
          </p>
        </section>
      </div>
    );
  }

  const last = status.last_sync;

  return (
    <div className="page">
      <h2 className="page-title">Brewfather</h2>
      <section className="card" aria-labelledby={`${id}-connection`}>
        <h3 id={`${id}-connection`}>Connection</h3>
        <p>{listable ? "Connected" : status.reconnect_needed ? "Reconnect needed" : "Not connected"}</p>
        {listable ? (
          <div className="btn-row">
            <button type="button" className="btn" onClick={disconnect}>
              Disconnect
            </button>
          </div>
        ) : (
          <form className="stack-form" onSubmit={connect}>
            <label>
              User ID
              <input value={userId} onChange={(e) => setUserId(e.target.value)} autoComplete="off" required />
            </label>
            <label>
              API key
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                autoComplete="off"
                required
              />
            </label>
            <button type="submit" className="btn btn-primary" disabled={busy > 0}>
              Connect
            </button>
          </form>
        )}
        <div className="field">
          <label>
            Sync interval
            <select value={status.interval} onChange={(e) => changeInterval(e.target.value as SyncInterval)}>
              {intervals.map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
        </div>
        {status.retry_at && <p>Brewfather's rate limit is reached. Calls resume at {new Date(status.retry_at).toLocaleString()}.</p>}
        {listable && (
          <div className="btn-row">
            <button type="button" className="btn" onClick={syncNow} disabled={status.sync_running}>
              Sync now
            </button>
            {status.sync_running && <p>Sync running…</p>}
          </div>
        )}
        {last && !status.sync_running && (
          <p className="muted">
            {last.ok === false ? "Last sync failed" : "Last sync"}{" "}
            {new Date(last.finished_at ?? last.started_at).toLocaleString()}: {last.message} Updated {last.updated},
            imported {last.imported}, not found {last.not_found}.
          </p>
        )}
      </section>
      {listable && (
        <section className="card" aria-labelledby={`${id}-batches`}>
          <div className="page-header">
            <h3 id={`${id}-batches`}>Batches</h3>
            <button type="button" className="btn" onClick={() => run(() => loadBatches(true))} disabled={busy > 0}>
              Refresh
            </button>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Import</th>
                  <th>Name</th>
                  <th>Number</th>
                  <th>Status</th>
                  <th>Brew date</th>
                </tr>
              </thead>
              <tbody>
                {batches.map((b) => (
                  <tr key={b.id}>
                    <td>
                      {b.beer_id === null ? (
                        <input
                          type="checkbox"
                          aria-label={`Import ${b.name}`}
                          checked={selected.has(b.id)}
                          onChange={(e) => toggle(b.id, e.target.checked)}
                        />
                      ) : beerNames.has(b.beer_id) ? (
                        `Imported as ${beerNames.get(b.beer_id)}`
                      ) : (
                        "Imported"
                      )}
                    </td>
                    <td>{b.name}</td>
                    <td>{b.number}</td>
                    <td>{b.status}</td>
                    <td>{b.brew_date}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="btn-row">
            <button
              type="button"
              className="btn btn-primary"
              onClick={importSelected}
              disabled={busy > 0 || selected.size === 0}
            >
              Import selected
            </button>
          </div>
          {results.length > 0 && (
            <ul aria-label="Import results">
              {results.map(({ name, result: r }) => (
                <li key={r.batch_id}>
                  {name}: {outcomes[r.outcome]}
                  {r.beer && ` as ${r.beer.name}`}.{r.adjusted.length > 0 && ` Adjusted: ${r.adjusted.join(", ")}.`}
                  {r.message && ` ${r.message}`}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
