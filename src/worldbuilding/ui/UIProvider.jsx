import { useState, useSyncExternalStore } from "react";
import Dialog from "./Dialog.jsx";
import { feedbackStore, settleFeedback } from "./feedback.js";

function Feedback({ item }) {
  const [value, setValue] = useState(item.initial);
  const cancel = () =>
    settleFeedback(item.id, item.kind === "prompt" ? null : false);
  const accept = () =>
    settleFeedback(item.id, item.kind === "prompt" ? value.trim() : true);
  return (
    <Dialog
      title={
        item.title ||
        (item.kind === "prompt"
          ? "Edit details"
          : item.kind === "notice"
            ? "Please note"
            : "Confirm action")
      }
      size="compact"
      role={item.kind === "confirm" ? "alertdialog" : "dialog"}
      trackChanges={false}
      onClose={cancel}
      onSubmit={(e) => {
        e.preventDefault();
        accept();
      }}
      actions={
        <>
          {item.kind !== "notice" && (
            <button type="button" autoFocus onClick={cancel}>
              Cancel
            </button>
          )}
          <button
            className={
              item.kind === "confirm" &&
              /delete|discard|throw away|replace|without saving/i.test(
                item.message,
              )
                ? "danger"
                : "primary"
            }
          >
            {item.accept ||
              (item.kind === "prompt"
                ? "Save"
                : item.kind === "notice"
                  ? "OK"
                  : "Confirm")}
          </button>
        </>
      }
    >
      {item.kind === "prompt" ? (
        <label>
          {item.message}
          <input
            type="text"
            autoFocus
            required
            maxLength={200}
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
        </label>
      ) : (
        <p>{item.message}</p>
      )}
    </Dialog>
  );
}
export default function UIProvider({ children }) {
  const queue = useSyncExternalStore(
    feedbackStore.subscribe,
    feedbackStore.snapshot,
  );
  return (
    <>
      {children}
      {queue[0] && <Feedback key={queue[0].id} item={queue[0]} />}
    </>
  );
}
