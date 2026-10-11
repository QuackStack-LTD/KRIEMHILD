import Dialog from "../worldbuilding/ui/Dialog.jsx";
import { useEffect, useMemo, useRef, useState } from "react";
import { worldRequest } from "../project.js";
import {
  renderMedievalMap,
  artworkPNG,
  downloadArtwork,
} from "./medieval-map-export.js";
import "./illustration.css";

export default function IllustratedMapDialog({ solver, flush, onClose }) {
  const active = useRef(true),
    [data, setData] = useState(null),
    [name, setName] = useState(""),
    [saved, setSaved] = useState([]),
    [chosen, setChosen] = useState(""),
    [busy, setBusy] = useState(true),
    [error, setError] = useState(""),
    [status, setStatus] = useState(
      "Saving source map and preparing illustration...",
    ),
    [resolution, setResolution] = useState(4800);
  const render = useMemo(
    () => (data ? renderMedievalMap(data, { title: name || data.name }) : null),
    [data, name],
  );
  const artwork = chosen ? saved.find((a) => a.id === chosen) : render;
  const [url, setURL] = useState("");
  useEffect(() => {
    if (!artwork) return;
    const url = URL.createObjectURL(
      new Blob([artwork.svg], { type: "image/svg+xml" }),
    );
    setURL(url);
    return () => URL.revokeObjectURL(url);
  }, [artwork]);
  async function generate() {
    setBusy(true);
    setError("");
    setStatus("Saving source map and preparing illustration...");
    try {
      await flush();
      const result = await worldRequest(solver.id, "/illustration", {});
      if (!active.current) return;
      solver.setSaveState(result);
      setData(result);
      setName(result.name);
      setChosen("");
      const list = await worldRequest(solver.id, "/artworks");
      if (!active.current) return;
      setSaved(list.artworks);
      setStatus(
        "Preview ready. The source map is saved; save this artwork separately.",
      );
    } catch (e) {
      if (active.current) setError(e.message);
    } finally {
      if (active.current) setBusy(false);
    }
  }
  useEffect(() => {
    active.current = true;
    void generate();
    return () => {
      active.current = false;
    };
  }, [solver.id]);
  async function save() {
    if (!artwork || chosen) return;
    setBusy(true);
    setError("");
    try {
      const result = await worldRequest(solver.id, "/artworks", artwork);
      solver.setSaveState(result);
      setSaved((a) => [result.artwork, ...a]);
      setChosen(result.artwork.id);
      setStatus(
        "Illustration saved to this map and Age. It is included in World ZIP exports.",
      );
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  async function png() {
    if (!artwork) return;
    setBusy(true);
    setError("");
    setStatus("Rendering high-resolution PNG...");
    try {
      downloadArtwork(
        await artworkPNG(artwork, resolution),
        `${filename()}.png`,
      );
      setStatus("PNG exported.");
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  function filename() {
    return (
      (artwork?.name || "illustrated-map")
        .replace(/[^\p{L}\p{N} _-]/gu, "")
        .slice(0, 100) || "illustrated-map"
    );
  }
  return (
    <Dialog
      title="Illustrated map"
      size="wizard"
      className="illustration-dialog"
      busy={busy}
      onClose={onClose}
      trackChanges={false}
      actions={
        <>
          {" "}
          <div className="illustration-actions">
            <button
              className="primary"
              disabled={busy || !artwork || !!chosen}
              onClick={save}
            >
              Save to World
            </button>
            <button
              disabled={busy || !artwork}
              onClick={() =>
                downloadArtwork(
                  new Blob([artwork.svg], {
                    type: "image/svg+xml;charset=utf-8",
                  }),
                  `${filename()}.svg`,
                )
              }
            >
              Export SVG
            </button>
            <button disabled={busy || !artwork} onClick={png}>
              Export PNG
            </button>
            <label>
              PNG size
              <select
                value={resolution}
                disabled={busy}
                onChange={(e) => setResolution(Number(e.target.value))}
              >
                <option value={3200}>3,200 px</option>
                <option value={4800}>4,800 px</option>
                <option value={6400}>6,400 px</option>
                <option value={8192}>8,192 px</option>
              </select>
            </label>
          </div>
          <p role="status">
            {busy ? "Working... " : ""}
            {status}
          </p>
          {error && (
            <p role="alert" className="illustration-error">
              {error}
            </p>
          )}
          <small>
            White paper | SVG vectors | {artwork?.width || 1600} x{" "}
            {artwork?.height || 1100} page units. Saved illustrations are
            independent snapshots.
          </small>{" "}
        </>
      }
    >
      {" "}
      <div className="illustration-controls">
        <label>
          Artwork title
          <input
            type="text"
            value={chosen ? artwork?.name || "" : name}
            maxLength={160}
            disabled={busy || !!chosen}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label>
          Saved artwork
          <select
            value={chosen}
            disabled={busy}
            onChange={(e) => setChosen(e.target.value)}
          >
            <option value="">New preview</option>
            {saved.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} | {new Date(a.createdAt).toLocaleString()}
              </option>
            ))}
          </select>
        </label>
        <button disabled={busy} onClick={generate}>
          Regenerate from map
        </button>
      </div>
      <div className="illustration-paper">
        {url ? (
          <img
            src={url}
            alt={`Medieval illustration of ${artwork?.name || name}`}
          />
        ) : (
          <p>Preparing the complete map...</p>
        )}
      </div>
    </Dialog>
  );
}
