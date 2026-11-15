import { useEffect, useState } from "react";
import { request } from "./api.js";
import { waterKinds } from "./water-bodies.js";

export default function WaterBodies({
  state,
  revision,
  selectedSource,
  onNamed,
  onFocus,
  onProfile,
  onCreate,
}) {
  const [data, setData] = useState({ bodies: [], revision: 0 });
  const [source, setSource] = useState(selectedSource || "");
  const [search, setSearch] = useState("");
  const [name, setName] = useState("");
  const [kind, setKind] = useState("sea");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const body = data.bodies.find((b) => b.sourceId === source);
  useEffect(() => {
    const abort = new AbortController();
    request(
      `sessions/${state.id}/terrain/water-bodies`,
      undefined,
      "GET",
      abort.signal,
    )
      .then(setData)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => abort.abort();
  }, [state.id, revision]);
  useEffect(() => {
    if (selectedSource) setSource(selectedSource);
  }, [selectedSource]);
  useEffect(() => {
    setName(body?.entityId ? body.name : "");
  }, [body?.sourceId, body?.name, body?.entityId]);
  return (
    <section aria-label="Water body names">
      <h3>Water bodies</h3>
      <label>
        Find water body
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </label>
      <label>
        Generated water body
        <select value={source} onChange={(e) => setSource(e.target.value)}>
          <option value="">Choose an ocean, lake, or river</option>
          {data.bodies
            .filter(
              (b) =>
                b.sourceId === source ||
                `${b.name} ${b.kind} ${b.sourceId}`
                  .toLowerCase()
                  .includes(search.toLowerCase()),
            )
            .map((b) => (
              <option key={b.sourceId} value={b.sourceId}>
                {b.name} ({b.kind})
              </option>
            ))}
        </select>
      </label>
      {body && (
        <>
          <button onClick={() => onFocus(body.position)}>Find on map</button>
          <label>
            Water body name
            <input
              maxLength={160}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <button
            disabled={busy || !name.trim() || !state.historicalMap}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                const result = await request(
                  `sessions/${state.id}/terrain/water-bodies/${encodeURIComponent(source)}?view=metadata`,
                  {
                    name,
                    revision: Math.max(
                      data.revision,
                      state.historyRevision ?? 0,
                    ),
                  },
                );
                await onNamed(result.document);
              } catch (e) {
                setError(e.message);
              } finally {
                setBusy(false);
              }
            }}
          >
            {body.entityId ? "Rename water body" : "Name water body"}
          </button>
          {body.entityId && (
            <button onClick={() => onProfile(body.entityId)}>
              Open water body record
            </button>
          )}
        </>
      )}
      {!state.historicalMap && (
        <p>Save terrain to its World before naming water bodies.</p>
      )}
      {state.historicalMap && (
        <>
          <label>
            New water body type
            <select value={kind} onChange={(e) => setKind(e.target.value)}>
              {waterKinds.map((k) => (
                <option key={k}>{k}</option>
              ))}
            </select>
          </label>
          <button onClick={() => onCreate(kind)}>
            Place a named water body
          </button>
          <p>
            Select a sea, bay, or other water region by placing its name on the
            map.
          </p>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
