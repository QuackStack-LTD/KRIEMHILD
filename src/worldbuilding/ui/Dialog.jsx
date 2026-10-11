import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { confirmAction, registerNavigationGuard } from "./feedback.js";

const stack = [];
let originalOverflow = "";
const focusable =
  'button:not(:disabled), [href], input:not(:disabled):not([type="hidden"]), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';
export default function Dialog({
  title,
  children,
  actions,
  onClose,
  onSubmit,
  busy = false,
  size = "form",
  className = "",
  role = "dialog",
  trackChanges = true,
  ...props
}) {
  const id = useId(),
    ref = useRef(null),
    opener = useRef(document.activeElement);
  const [edited, setEdited] = useState(false),
    [pending, setPending] = useState(false),
    [error, setError] = useState("");
  const locked = busy || pending,
    latest = useRef({}),
    submitting = useRef(false),
    closing = useRef(false);
  async function close() {
    if (locked || closing.current || !onClose) return;
    closing.current = true;
    if (
      trackChanges &&
      edited &&
      !(await confirmAction("Discard the unsaved changes in this dialog?", {
        accept: "Discard changes",
      }))
    ) {
      closing.current = false;
      return;
    }
    closing.current = false;
    onClose();
  }
  latest.current = { close, locked, edited, trackChanges };
  useEffect(() => {
    const remove = registerNavigationGuard(async () => {
      // A successful form may deliberately change Age or open the created profile.
      if (submitting.current) return true;
      const state = latest.current;
      if (state.locked) return false;
      if (!state.trackChanges || !state.edited) return true;
      if (
        !(await confirmAction("Discard the unsaved changes in this dialog?", {
          accept: "Discard changes",
        }))
      )
        return false;
      latest.current.edited = false;
      setEdited(false);
      return true;
    });
    const unload = (e) => {
      if (
        latest.current.trackChanges &&
        latest.current.edited &&
        !submitting.current
      ) {
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
  useEffect(() => {
    const panel = ref.current,
      root = document.getElementById("root");
    if (!stack.length) {
      originalOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      if (root) root.inert = true;
    }
    const previous = stack.at(-1);
    if (previous) previous.inert = true;
    stack.push(panel);
    const timer = requestAnimationFrame(() => {
      if (!panel.contains(document.activeElement)) {
        (
          panel.querySelector(
            ".ui-dialog-body input:not(:disabled), .ui-dialog-body select:not(:disabled), .ui-dialog-body textarea:not(:disabled)",
          ) ||
          panel.querySelector(focusable) ||
          panel
        ).focus();
      }
    });
    function keydown(e) {
      if (stack.at(-1) !== panel) return;
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        void latest.current.close();
      }
      if (e.key !== "Tab") return;
      const controls = [...panel.querySelectorAll(focusable)].filter(
        (el) => el.getClientRects().length && !el.closest("[inert]"),
      );
      const first = controls[0],
        last = controls.at(-1);
      if (!first) {
        e.preventDefault();
        panel.focus();
      } else if (
        e.shiftKey &&
        (document.activeElement === first ||
          !panel.contains(document.activeElement))
      ) {
        e.preventDefault();
        last.focus();
      } else if (
        !e.shiftKey &&
        (document.activeElement === last ||
          !panel.contains(document.activeElement))
      ) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener("keydown", keydown, true);
    return () => {
      cancelAnimationFrame(timer);
      document.removeEventListener("keydown", keydown, true);
      const index = stack.indexOf(panel);
      if (index >= 0) stack.splice(index, 1);
      if (stack.length) stack.at(-1).inert = false;
      else {
        document.body.style.overflow = originalOverflow;
        if (root) root.inert = false;
      }
      if (opener.current?.isConnected && !opener.current.closest("[inert]"))
        opener.current.focus();
    };
  }, []);
  const Tag = onSubmit ? "form" : "section";
  return createPortal(
    <div className="ui-root ui-dialog-backdrop">
      <Tag
        {...props}
        ref={ref}
        tabIndex={-1}
        role={role}
        aria-modal="true"
        aria-labelledby={id}
        aria-busy={locked}
        className={`ui-dialog ui-dialog--${size} ${className}`}
        onChangeCapture={() => setEdited(true)}
        onInputCapture={() => setEdited(true)}
        onClickCapture={(e) => {
          if (e.target.closest("[data-dialog-dismiss]")) {
            e.preventDefault();
            e.stopPropagation();
            void close();
          } else if (e.target.closest(".ui-dialog-body button"))
            setEdited(true);
        }}
        onSubmit={
          onSubmit
            ? async (e) => {
                e.preventDefault();
                if (locked || submitting.current) return;
                submitting.current = true;
                setPending(true);
                setError("");
                try {
                  await onSubmit(e);
                } catch (err) {
                  setError(err.message || "Unable to save. Please try again.");
                } finally {
                  submitting.current = false;
                  setPending(false);
                }
              }
            : undefined
        }
      >
        <header className="ui-dialog-header">
          <h2 id={id}>{title}</h2>
          {locked && <span role="status">Working…</span>}
          {onClose && (
            <button
              type="button"
              className="ui-icon-button"
              aria-label="Close dialog"
              disabled={locked}
              onClick={close}
            >
              ×
            </button>
          )}
        </header>
        <div className="ui-dialog-body">
          <fieldset className="ui-dialog-fields" disabled={locked}>
            {children}
          </fieldset>
          {error && (
            <p role="alert" className="ui-error">
              {error}
            </p>
          )}
        </div>
        {actions && (
          <footer className="dialog-actions">
            <fieldset disabled={locked}>{actions}</fieldset>
          </footer>
        )}
      </Tag>
    </div>,
    document.body,
  );
}
