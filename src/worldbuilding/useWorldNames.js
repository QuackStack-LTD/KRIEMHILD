import { useEffect, useRef } from "react";
import { request } from "../continuous/api.js";

export function announceWorldChange(worldId) {
  window.dispatchEvent(
    new CustomEvent("world-records-changed", { detail: worldId }),
  );
  try {
    localStorage.setItem(
      "kriemhild:world-records-changed",
      JSON.stringify({ worldId, nonce: crypto.randomUUID() }),
    );
  } catch {
    /* Synchronization also uses focus and revision polling. */
  }
}

// Check a small revision response while idle; fetch the document only when it
// changes. References render from IDs against this current canonical document.
export default function useWorldNames(
  worldId,
  revision,
  onDocument,
  paused = false,
  view = "",
  onNameChange,
) {
  const latest = useRef({ worldId, revision, onDocument });
  latest.current = {
    worldId,
    paused,
    revision:
      latest.current.worldId === worldId
        ? Math.max(revision ?? -1, latest.current.revision ?? -1)
        : revision,
    onDocument,
    onNameChange,
  };
  useEffect(() => {
    if (!worldId) return;
    let stopped = false,
      pending = false;
    const abort = new AbortController();
    const refresh = async () => {
      if (
        stopped ||
        pending ||
        latest.current.paused ||
        document.visibilityState === "hidden"
      )
        return;
      pending = true;
      try {
        const value = await request(
          `worlds/${worldId}/names?revision=${latest.current.revision ?? -1}${view ? `&view=${view}` : ""}${latest.current.onNameChange ? "&delta=names" : ""}`,
          undefined,
          "GET",
          abort.signal,
        );
        if (
          !stopped &&
          value.nameChange &&
          value.nameChange.world.revision > (latest.current.revision ?? -1)
        ) {
          latest.current.onNameChange?.(value.nameChange);
        }
        if (
          !stopped &&
          value.document &&
          value.document.world.revision > (latest.current.revision ?? -1)
        ) {
          latest.current.onDocument(value.document);
        }
      } catch {
        /* A later focus or revision check retries a transient failure. */
      } finally {
        pending = false;
      }
    };
    const changed = (event) => {
      if (event.type === "storage") {
        if (event.key !== "kriemhild:world-records-changed") return;
        try {
          if (JSON.parse(event.newValue)?.worldId !== worldId) return;
        } catch {
          return;
        }
      } else if (
        event.type === "world-records-changed" &&
        event.detail !== worldId
      )
        return;
      void refresh();
    };
    const timer = setInterval(refresh, 4000);
    window.addEventListener("focus", changed);
    window.addEventListener("storage", changed);
    window.addEventListener("world-records-changed", changed);
    document.addEventListener("visibilitychange", changed);
    return () => {
      stopped = true;
      abort.abort();
      clearInterval(timer);
      window.removeEventListener("focus", changed);
      window.removeEventListener("storage", changed);
      window.removeEventListener("world-records-changed", changed);
      document.removeEventListener("visibilitychange", changed);
    };
  }, [worldId, view]);
  // A successful local command already returned this document. Remember it
  // before broadcasting, so the broadcast cannot download the World again.
  return (nextRevision) => {
    latest.current.revision = Math.max(
      latest.current.revision ?? -1,
      nextRevision,
    );
  };
}
