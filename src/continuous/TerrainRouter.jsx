import { lazy, Suspense, useEffect, useState } from "react";
import { request } from "./api.js";
const LegacyTerrain = lazy(() => import("../TerrainApp.jsx"));
const Studio = lazy(() => import("./Studio.jsx"));
export default function TerrainRouter(props) {
  const [entry, setEntry] = useState(null),
    [error, setError] = useState("");
  useEffect(() => {
    const abort = new AbortController();
    if (new URLSearchParams(location.search).has("legacyImport")) {
      setEntry({ legacy: true });
      return;
    }
    if (location.pathname === "/world/new") {
      setEntry({});
      return;
    }
    const id = location.pathname.match(/^\/world\/([a-f0-9]{32})\//)?.[1];
    if (!id) {
      setEntry({ legacy: true });
      return;
    }
    request(`projects/${id}/open`, {}, "POST", abort.signal)
      .then((state) =>
        setEntry(state.continuous ? { state } : { legacy: true }),
      )
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => abort.abort();
  }, []);
  if (error)
    return (
      <main className="world-page">
        <p role="alert">{error}</p>
        <button onClick={() => props.onExit?.()}>Return to Worlds</button>
      </main>
    );
  return (
    <Suspense
      fallback={
        <main className="world-page" role="status">
          Opening terrain…
        </main>
      }
    >
      {entry ? (
        entry.legacy ? (
          <LegacyTerrain {...props} />
        ) : (
          <Studio {...props} initialState={entry.state} />
        )
      ) : (
        <main className="world-page" role="status">
          Reading saved terrain…
        </main>
      )}
    </Suspense>
  );
}
