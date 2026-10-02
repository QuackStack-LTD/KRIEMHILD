"use client";
import { useEffect, useState } from "react";
import type { Entry } from "./types";
import { draftStorageKey } from "./draftStorage";

type Recovery = { base: Entry; value: Entry };

// A browser draft is offered for review and never silently replaces canon.
// Components must remount this hook when the saved record changes.
export function useRecordDraft(key: string, saved: Entry, newRecord = false) {
  const [value, setValue] = useState(saved);
  const [recovery, setRecovery] = useState<Recovery | null>(null);
  const [ready, setReady] = useState(false);
  const [storageError, setStorageError] = useState("");
  const [storageKey, setStorageKey] = useState("");
  const [storageBlocked, setStorageBlocked] = useState(false);
  const dirty = JSON.stringify(value) !== JSON.stringify(saved);
  useEffect(() => {
    try {
      const scopedKey = draftStorageKey(key);
      setStorageKey(scopedKey);
      const raw = localStorage.getItem(scopedKey);
      if (raw) {
        const found = JSON.parse(raw) as Recovery;
        if (
          (newRecord ||
            (found.base?.id === saved.id && found.value?.id === saved.id)) &&
          found.value.kind === saved.kind
        ) {
          if (JSON.stringify(found.value) !== JSON.stringify(saved))
            setRecovery(found);
          else localStorage.removeItem(scopedKey);
        } else {
          setStorageBlocked(true);
          setStorageError(
            "The stored draft could not be read. Its browser copy has been retained.",
          );
        }
      }
    } catch {
      setStorageBlocked(true);
      setStorageError(
        "Browser draft recovery is unavailable. Keep this tab open until your save succeeds.",
      );
    }
    setReady(true);
  }, [key]); // Saved record is fixed for the lifetime of this hook.
  useEffect(() => {
    if (!ready || recovery || !storageKey || storageBlocked) return;
    try {
      if (dirty)
        localStorage.setItem(
          storageKey,
          JSON.stringify({ base: saved, value }),
        );
      else localStorage.removeItem(storageKey);
      setStorageError("");
    } catch {
      setStorageError(
        "The browser could not retain this draft. Keep this tab open until your save succeeds.",
      );
    }
  }, [ready, recovery, dirty, storageKey, storageBlocked, saved, value]);
  const clear = () => {
    try {
      if (storageKey) localStorage.removeItem(storageKey);
    } catch {
      /* The next load detects an already-saved identical draft. */
    }
    setRecovery(null);
    setStorageBlocked(false);
  };
  return {
    value,
    setValue,
    dirty,
    recovery,
    ready,
    storageError,
    pending: dirty || !!recovery,
    stale:
      !newRecord &&
      !!recovery &&
      JSON.stringify(recovery.base) !== JSON.stringify(saved),
    restore: () => {
      if (recovery) {
        setValue(
          newRecord ? { ...recovery.value, id: saved.id } : recovery.value,
        );
        setRecovery(null);
      }
    },
    discard: () => {
      clear();
      setValue(saved);
    },
    saved: clear,
  };
}
