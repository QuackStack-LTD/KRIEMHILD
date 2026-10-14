import { useEffect, useState } from "react";
import { request } from "./api.js";
import { archivePixels } from "./archive-preview.js";

export default function ArchivedTerrain({ initialState, onOpen, onExit }) {
  const [state, setState] = useState(initialState),
    [projects, setProjects] = useState([]),
    [entities, setEntities] = useState([]),
    [preview, setPreview] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const run = async (work) => {
    setBusy(true);
    setError("");
    try {
      await work();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    request("projects")
      .then((r) => setProjects(r.projects || []))
      .catch((e) => setError(e.message));
  }, []);
  useEffect(() => {
    if (!state) return;
    try {
      const { width, height, pixels } = archivePixels(state),
        canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      canvas
        .getContext("2d")
        .putImageData(new ImageData(pixels, width, height), 0, 0);
      setPreview(canvas.toDataURL());
    } catch (e) {
      setError(e.message);
    }
    request(`sessions/${state.id}/world/entities?detail=8`)
      .then((r) => setEntities(r.entities || []))
      .catch((e) => setError(e.message));
  }, [state?.id]);
  async function importFiles(files, folder) {
    if (!files.length) return;
    await run(async () => {
      let body = files[0],
        headers = { "Content-Type": "application/zip" };
      if (folder) {
        body = new FormData();
        headers = {};
        for (const f of files)
          body.append(f.webkitRelativePath || f.name, f, f.name);
      }
      const r = await fetch("/api/projects/import", {
          method: "POST",
          headers,
          body,
        }),
        next = await r.json();
      if (!r.ok) throw Error(next.error || "Import failed");
      if (next.continuous) onOpen(next);
      else setState(next);
    });
  }
  async function exportArchive() {
    const r = await fetch(`/api/sessions/${state.id}/project`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
    if (!r.ok) throw Error("Archive export failed");
    const url = URL.createObjectURL(await r.blob()),
      link = document.createElement("a");
    link.href = url;
    link.download = "KRIEMHILD-archived-terrain.zip";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
  }
  return (
    <main className="world-page">
      <header className="world-toolbar">
        <button onClick={() => onExit?.(state?.historyWorldId)}>
          Return to Worlds
        </button>
        <h1>{state?.project?.name || "Terrain archives"}</h1>
      </header>
      {error && <p role="alert">{error}</p>}
      {state ? (
        <>
          <p>
            This map uses the retired terrain format. Its saved cells and
            authored objects are shown below. The original terrain, explored
            details and project records remain available in its archive.
          </p>
          <p>
            New terrain and editing use the continuous workspace. Opening this
            archive does not regenerate or replace it.
          </p>
          <div className="form-actions">
            <button
              disabled={busy}
              onClick={() =>
                run(async () => {
                  await request(`sessions/${state.id}/save`, {});
                  setState({ ...state, unsaved: false });
                })
              }
            >
              {state.unsaved ? "Save imported archive" : "Save archive"}
            </button>
            <button disabled={busy} onClick={() => run(exportArchive)}>
              Export original archive
            </button>
            <button
              onClick={() => {
                const q = new URLSearchParams();
                if (state.historyWorldId) q.set("world", state.historyWorldId);
                if (state.historicalMap?.ageId)
                  q.set("age", state.historicalMap.ageId);
                location.assign(`/world/new?${q}`);
              }}
            >
              Create new terrain preview
            </button>
          </div>
          <svg
            role="img"
            aria-label="Saved terrain overview and authored objects"
            viewBox={`0 0 ${state.W} ${state.H}`}
            style={{ width: "100%", maxHeight: "65vh", background: "#101d2a" }}
          >
            <image href={preview} width={state.W} height={state.H} />
            {entities
              .filter((e) => !e.deleted)
              .map((e) =>
                e.geometry === "point" ? (
                  <circle
                    key={e.id}
                    cx={e.points[0][0]}
                    cy={e.points[0][1]}
                    r=".4"
                    fill={e.color}
                  >
                    <title>{e.name}</title>
                  </circle>
                ) : e.geometry === "polygon" ? (
                  <polygon
                    key={e.id}
                    points={e.points.map((p) => p.join(",")).join(" ")}
                    fill="none"
                    stroke={e.color}
                    strokeWidth=".2"
                  >
                    <title>{e.name}</title>
                  </polygon>
                ) : (
                  <polyline
                    key={e.id}
                    points={e.points.map((p) => p.join(",")).join(" ")}
                    fill="none"
                    stroke={e.color}
                    strokeWidth=".2"
                  >
                    <title>{e.name}</title>
                  </polyline>
                ),
              )}
          </svg>
          <p role="status">
            {busy
              ? "Working…"
              : state.unsaved
                ? "Imported archive has not been saved yet."
                : "Archive saved."}
          </p>
        </>
      ) : (
        <>
          <p>
            Open a saved terrain archive. Continuous maps open in the new
            editor; older maps open in the archive viewer.
          </p>
          <label className="button">
            Import terrain ZIP
            <input
              type="file"
              accept=".zip"
              disabled={busy}
              onChange={(e) => importFiles([...e.target.files], false)}
            />
          </label>
          <label className="button">
            Import project folder
            <input
              type="file"
              webkitdirectory=""
              multiple
              disabled={busy}
              onChange={(e) => importFiles([...e.target.files], true)}
            />
          </label>
          <h2>Saved terrain projects</h2>
          {projects.map((p) => (
            <button
              key={p.id}
              disabled={busy}
              onClick={() =>
                run(async () =>
                  onOpen(await request(`projects/${p.id}/open`, {})),
                )
              }
            >
              {p.name || p.id}
            </button>
          ))}
        </>
      )}
    </main>
  );
}
