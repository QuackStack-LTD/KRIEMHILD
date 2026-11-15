import { useEffect, useRef, useState } from "react";
import { registerNavigationGuard } from "../ui/feedback.js";
import ArchiveEditor from "./ArchiveEditor.jsx";
import Icon from "./ArchiveIcon.jsx";
import { importDocument, exportDocument, mimeFor } from "./archive-files.js";
import {
  baseName,
  libraryURL,
  sizeLabel,
  archiveRequest,
  uploadArchiveFile,
  fileURL,
  imageURL,
} from "./archive-model.js";
import { download, plainDocument, newNode } from "./model.js";

export default function ArchiveFilePreview({
  node,
  url,
  bytes: given,
  onReplace,
}) {
  const [objectURL, setURL] = useState(""),
    [error, setError] = useState(""),
    [bytes, setBytes] = useState(given || null),
    [zoom, setZoom] = useState(100);
  const mime = node.mime || "application/octet-stream",
    name = node.fileName || node.title;
  useEffect(() => {
    if (given) return;
    const controller = new AbortController();
    fetch(url, { signal: controller.signal })
      .then(async (r) => {
        if (!r.ok) throw Error("Unable to open file");
        return new Uint8Array(await r.arrayBuffer());
      })
      .then(setBytes)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      });
    return () => controller.abort();
  }, [url, given]);
  useEffect(() => {
    if (!bytes) return;
    const value = URL.createObjectURL(new Blob([bytes], { type: mime }));
    setURL(value);
    return () => URL.revokeObjectURL(value);
  }, [bytes, mime]);
  return (
    <section className="archive-preview">
      <div className="archive-preview-toolbar">
        <span>
          {name} · {sizeLabel(bytes?.length)}
        </span>
        {mime.startsWith("image/") && (
          <select
            aria-label="Image zoom"
            value={zoom}
            onChange={(e) => setZoom(+e.target.value)}
          >
            {[50, 75, 100, 150, 200].map((z) => (
              <option key={z} value={z}>
                {z}%
              </option>
            ))}
          </select>
        )}
        <button disabled={!bytes} onClick={() => download(name, bytes, mime)}>
          <Icon name="download" size={17} />
          Download original
        </button>
        {onReplace && (
          <button onClick={onReplace}>
            <Icon name="upload" size={17} />
            Replace file
          </button>
        )}
      </div>
      {error ? (
        <p role="alert">{error}</p>
      ) : !objectURL ? (
        <p role="status">Loading file…</p>
      ) : mime.startsWith("image/") ? (
        <div className="archive-image-stage">
          <img
            src={objectURL}
            alt={name}
            style={{ transform: `scale(${zoom / 100})` }}
          />
        </div>
      ) : mime === "application/pdf" ? (
        <iframe className="archive-pdf-preview" title={name} src={objectURL} />
      ) : /^audio\//.test(mime) ? (
        <audio controls src={objectURL} />
      ) : /^video\//.test(mime) ? (
        <video controls src={objectURL} />
      ) : (
        <div className="archive-empty">
          <Icon name="document" size={54} />
          <h2>{name}</h2>
          <p>
            This file is stored in its original format. Download it to open in a
            compatible application, or replace it with a revised file.
          </p>
        </div>
      )}
    </section>
  );
}

export function LinkedArchiveFile({
  item,
  world,
  onBack,
  onLibrary,
  onResources,
  onDocument,
  onRefresh,
}) {
  const [draft, setDraft] = useState(null),
    [bytes, setBytes] = useState(null),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [status, setStatus] = useState("Loading…"),
    [original, setOriginal] = useState(false);
  const state = useRef({ draft: null, dirty: false, generation: 0 }),
    flight = useRef(null),
    expectedHash = useRef(item.file?.versions.at(-1)?.hash);
  const latestVersion = item.file?.versions.at(-1),
    url = item.file ? libraryURL(world, item.file) : "",
    name = latestVersion?.name || item.title,
    locked = ["lost", "destroyed"].includes(item.file?.status);
  useEffect(() => {
    let live = true;
    const controller = new AbortController();
    (async () => {
      try {
        let parsed;
        if (url) {
          const r = await fetch(url, { signal: controller.signal });
          if (!r.ok) throw Error("Unable to load Library document");
          const raw = new Uint8Array(await r.arrayBuffer());
          if (!live) return;
          setBytes(raw);
          try {
            parsed = await importDocument(raw, name);
          } catch (e) {
            setNotice(
              `Original file preserved. Text conversion was unavailable: ${e.message}`,
            );
          }
        } else
          parsed = {
            content: plainDocument(
              item.entity?.properties?.summary ||
                item.entity?.properties?.description ||
                item.title,
            ),
            format: "md",
          };
        if (!live) return;
        const n = {
          id: item.key,
          title: item.title,
          format: parsed?.format || "",
          content: parsed?.content || null,
        };
        state.current.draft = n;
        setDraft(n);
        setNotice((n) => n || parsed?.notice || "");
        setStatus(locked ? item.file.status : "Saved");
      } catch (e) {
        if (e.name !== "AbortError" && live) setError(e.message);
      }
    })();
    return () => {
      live = false;
      controller.abort();
    };
  }, [item.key]);
  function change(patch) {
    const next = { ...state.current.draft, ...patch };
    state.current = {
      ...state.current,
      draft: next,
      dirty: true,
      generation: state.current.generation + 1,
    };
    setDraft(next);
    setStatus("Saving…");
  }
  async function flush() {
    if (flight.current) {
      const ok = await flight.current;
      return ok && state.current.dirty ? flush() : ok;
    }
    if (!state.current.dirty) return true;
    const n = state.current.draft,
      g = state.current.generation;
    flight.current = (async () => {
      try {
        const base = await archiveRequest(world, "/inworld"),
          currentFile = base.libraryFiles.find(
            (f) => f.id === item.id && f.ageId === item.ageId,
          ),
          currentEntity = base.entities.find(
            (e) => e.id === item.id && e.ageId === item.ageId && !e.deleted,
          );
        if (!currentEntity || !currentFile)
          throw Error(
            "This Library document was removed. Your edits remain in this tab.",
          );
        if (currentFile.versions.at(-1)?.hash !== expectedHash.current)
          throw Error(
            "This Library document changed in another session. Download your edits before reopening it.",
          );
        const out = await exportDocument(n, n.format);
        const form = new FormData();
        form.append(
          "metadata",
          JSON.stringify({
            revision: base.world.revision,
            entity: currentEntity,
            archiveId: item.file.archiveId,
          }),
        );
        form.append("file", new Blob([out]), baseName(name) + "." + n.format);
        const response = await fetch(`/api/worlds/${world}/library`, {
            method: "POST",
            body: form,
          }),
          result = await response.json();
        if (!response.ok)
          throw Error(result.error || "Unable to save Library document");
        expectedHash.current = result.libraryFiles
          .find((f) => f.id === item.id && f.ageId === item.ageId)
          ?.versions.at(-1)?.hash;
        onDocument?.(result);
        if (g === state.current.generation) {
          state.current.dirty = false;
          setStatus("Saved to Library");
        }
        setError("");
        return true;
      } catch (e) {
        setError(e.message);
        setStatus("Save failed · edits retained in this tab");
        return false;
      } finally {
        flight.current = null;
      }
    })();
    const ok = await flight.current;
    return ok && state.current.dirty ? flush() : ok;
  }
  const handlers = useRef({ flush });
  handlers.current = { flush };
  useEffect(() => {
    if (!state.current.dirty) return;
    const timer = setTimeout(() => void flush(), 1100);
    return () => clearTimeout(timer);
  }, [draft]);
  useEffect(() => {
    const remove = registerNavigationGuard(() => handlers.current.flush());
    const unload = (e) => {
      if (state.current.dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", unload);
    return () => {
      remove();
      window.removeEventListener("beforeunload", unload);
    };
  }, []);
  const readonly = locked || !item.file;
  async function back() {
    if (await flush()) {
      await onRefresh?.();
      onBack();
    }
  }
  async function exportFile(format) {
    try {
      download(
        baseName(name) + "." + format,
        await exportDocument(state.current.draft, format),
        mimeFor(format),
      );
    } catch (e) {
      setError(e.message);
    }
  }
  return (
    <section className="archive-document">
      <header className="archive-document-header">
        <button
          className="archive-icon-button"
          aria-label="Back to archive"
          onClick={back}
        >
          <Icon name="back" />
        </button>
        <span className="archive-document-symbol">
          <Icon name="document" size={30} />
        </span>
        <div className="archive-document-heading">
          <h2>{item.title}</h2>
          <small>
            <Icon name="link" size={12} />
            Inworld Library <span role="status">{status}</span>
          </small>
        </div>
        <button
          onClick={async () => {
            if (await flush()) onLibrary();
          }}
        >
          Open in Library
        </button>
        {item.file?.resourceSurvey && onResources && (
          <button
            disabled={locked}
            onClick={async () => {
              if (await flush()) onResources();
            }}
          >
            Review terrain area
          </button>
        )}
        {draft?.content && (
          <details className="archive-item-menu archive-export-menu">
            <summary>
              <Icon name="download" size={17} />
              Download
            </summary>
            <div>
              {["docx", "pdf", "txt", "md"].map((f) => (
                <button key={f} onClick={() => exportFile(f)}>
                  {f.toUpperCase()}
                </button>
              ))}
            </div>
          </details>
        )}
      </header>
      {error && (
        <div className="archive-alert" role="alert">
          {error}
          <button onClick={() => void flush()}>Retry save</button>
        </div>
      )}
      {notice && (
        <div className="archive-notice">
          {notice}
          <button aria-label="Dismiss notice" onClick={() => setNotice("")}>
            <Icon name="close" size={16} />
          </button>
        </div>
      )}
      {bytes && draft?.content && (
        <div className="archive-linked-notice">
          <button onClick={() => setOriginal((v) => !v)}>
            {original ? "Return to editor" : "View original file"}
          </button>
          <span>
            {readonly
              ? "Read only · manage this record in the Library"
              : "Edits are saved to the same Library document."}
          </span>
        </div>
      )}
      {item.file?.resourceSurvey && (
        <p className="archive-linked-notice">
          This document is linked to its observed terrain circle. Resource
          charts use the captured observations; text edits do not change survey
          data.
        </p>
      )}
      {!draft ? (
        <div className="archive-editor-loading" role="status">
          Loading document…
        </div>
      ) : original || !draft.content ? (
        <ArchiveFilePreview
          node={{
            title: item.title,
            fileName: name,
            mime: latestVersion?.mime,
          }}
          url={url}
          bytes={bytes}
        />
      ) : (
        <ArchiveEditor
          key={item.key}
          inworld
          worldId={world}
          node={draft}
          readOnly={readonly}
          onChange={change}
          onSave={flush}
          onImage={async (file) => {
            try {
              if (file.size > 64 * 1024 * 1024)
                throw Error("Image exceeds 64 MiB");
              const project = await archiveRequest(world, "", {});
              const n = await uploadArchiveFile(
                {
                  ...newNode(project.id, "asset", "", Date.now()),
                  title: file.name,
                },
                new Uint8Array(await file.arrayBuffer()),
                file.name,
              );
              return imageURL(world, n.fileHash);
            } catch (e) {
              setError(e.message);
              return null;
            }
          }}
        />
      )}
    </section>
  );
}
