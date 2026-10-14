import { lazy, Suspense, useEffect, useState } from "react";
import { request } from "./api.js";
const Studio = lazy(() => import("./Studio.jsx"));
const ArchivedTerrain = lazy(() => import("./ArchivedTerrain.jsx"));

export default function TerrainRouter(props) {
  const [entry, setEntry] = useState(null),
    [error, setError] = useState("");
  useEffect(() => {
    const abort = new AbortController();
    if (new URLSearchParams(location.search).has("legacyImport")) {
      setEntry({ archive: true });
    } else if (location.pathname === "/world/new") {
      setEntry({});
    } else {
      const id = location.pathname.match(/^\/world\/([a-f0-9]{32})\//)?.[1];
      if (!id) setError("Map address not found");
      else
        request(`projects/${id}/open`, {}, "POST", abort.signal)
          .then((state) => setEntry({ state, archive: !state.continuous }))
          .catch((e) => {
            if (e.name !== "AbortError") setError(e.message);
          });
    }
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
      {!entry ? (
        <main className="world-page" role="status">
          Reading saved terrain…
        </main>
      ) : entry.archive ? (
        <ArchivedTerrain
          key={entry.state?.id || "archives"}
          {...props}
          initialState={entry.state}
          onOpen={(state) => setEntry({ state, archive: !state.continuous })}
        />
      ) : (
        <Studio {...props} initialState={entry.state} />
      )}
    </Suspense>
  );
}
