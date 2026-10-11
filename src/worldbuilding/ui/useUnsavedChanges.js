import { useEffect, useRef } from "react";
import { confirmAction, registerNavigationGuard } from "./feedback.js";

export default function useUnsavedChanges(
  dirty,
  onDiscard,
  message = "Discard unsaved changes before leaving?",
) {
  const latest = useRef({ dirty, onDiscard, message });
  latest.current = { dirty, onDiscard, message };
  async function confirmLeave() {
    const state = latest.current;
    if (!state.dirty) return true;
    if (!(await confirmAction(state.message, { accept: "Discard changes" })))
      return false;
    latest.current.dirty = false;
    state.onDiscard?.();
    return true;
  }
  useEffect(() => {
    const remove = registerNavigationGuard(confirmLeave);
    // Browser reload/close requires the browser's own prompt; in-app navigation does not.
    const unload = (e) => {
      if (latest.current.dirty) {
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
  return confirmLeave;
}
