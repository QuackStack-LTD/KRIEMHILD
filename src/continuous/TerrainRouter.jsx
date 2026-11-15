import { lazy, Suspense, useEffect, useState } from "react";
import { request } from "./api.js";
const Studio = lazy(() => import("./Studio.jsx"));
const ArchivedTerrain = lazy(() => import("./ArchivedTerrain.jsx"));

export default function TerrainRouter(props) {
  const [entry, setEntry] = useState(null),
    [error, setError] = useState("");
  useEffect(() => {
    const abort = new AbortController();
    const open = async () => {
      const query = new URLSearchParams(location.search);
      if (location.pathname === "/world/new") {
        const world = query.get("world"),
          age = query.get("age");
        if (!world || !age)
          throw Error("Open a World and choose an Age to generate terrain.");
        const doc = await request(
          `worlds/${world}?view=metadata`,
          undefined,
          "GET",
          abort.signal,
        );
        if (!doc.ages.some((a) => a.id === age))
          throw Error("Choose an existing Age in this World.");
        setEntry({});
        return;
      }
      const id = location.pathname.match(/^\/world\/([a-f0-9]{32})\//)?.[1];
      if (!id) throw Error("Map address not found");
      const context = await request(
        `map-context/${id}`,
        undefined,
        "GET",
        abort.signal,
      );
      if (!context.world || !context.map)
        throw Error("Open terrain from its World.");
      query.set("world", context.world.id);
      query.set("age", context.map.ageId);
      history.replaceState({}, "", `${location.pathname}?${query}`);
      const state = await request(
        `projects/${id}/open?view=terrain`,
        {},
        "POST",
        abort.signal,
      );
      setEntry({ state, archive: !state.continuous });
    };
    open().catch((e) => {
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
