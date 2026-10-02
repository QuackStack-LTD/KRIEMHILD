"use client";
import { useEffect } from "react";

export function useDraftGuard(
  pending: boolean,
  onPending?: (pending: boolean) => void,
) {
  useEffect(() => {
    onPending?.(pending);
    return () => onPending?.(false);
  }, [pending, onPending]);
  useEffect(() => {
    if (!pending) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [pending]);
}
