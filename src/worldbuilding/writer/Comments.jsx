import { useEffect, useState } from "react";
import * as Y from "yjs";
import {
  absolutePositionToRelativePosition,
  relativePositionToAbsolutePosition,
  ySyncPluginKey,
} from "@tiptap/y-tiptap";
import { accountRequest } from "../../auth/api.js";
export default function Comments({ node, editor, room, role, version }) {
  const [comments, setComments] = useState([]),
    [body, setBody] = useState(""),
    [suggestion, setSuggestion] = useState(""),
    [error, setError] = useState("");
  const path = `/api/writer/projects/${node.projectId}/comments/${node.id}`;
  const refresh = () =>
    accountRequest(path)
      .then(setComments)
      .catch((e) => setError(e.message));
  useEffect(() => {
    void refresh();
  }, [node.id, version]);
  async function run(fn) {
    try {
      setError("");
      await fn();
      await refresh();
    } catch (e) {
      setError(e.message);
    }
  }
  function selection() {
    const state = ySyncPluginKey.getState(editor.state),
      { from, to } = editor.state.selection;
    if (from === to)
      throw new Error("Select the manuscript text to comment on.");
    return JSON.stringify({
      from: Y.relativePositionToJSON(
        absolutePositionToRelativePosition(
          from,
          state.type,
          state.binding.mapping,
        ),
      ),
      to: Y.relativePositionToJSON(
        absolutePositionToRelativePosition(
          to,
          state.type,
          state.binding.mapping,
        ),
      ),
      quote: editor.state.doc.textBetween(from, to),
    });
  }
  function positions(comment) {
    const anchor = JSON.parse(comment.anchor),
      state = ySyncPluginKey.getState(editor.state);
    const from = relativePositionToAbsolutePosition(
        room.doc,
        state.type,
        Y.createRelativePositionFromJSON(anchor.from),
        state.binding.mapping,
      ),
      to = relativePositionToAbsolutePosition(
        room.doc,
        state.type,
        Y.createRelativePositionFromJSON(anchor.to),
        state.binding.mapping,
      );
    if (from === null || to === null || from >= to)
      throw new Error(
        "The referenced text was deleted. The original quotation is preserved.",
      );
    return { from, to, quote: anchor.quote };
  }
  return (
    <section className="writer-comments">
      <h3>Comments & suggestions</h3>
      {error && <p role="alert">{error}</p>}
      {role !== "viewer" && (
        <>
          <label>
            Comment on selected text
            <textarea value={body} onChange={(e) => setBody(e.target.value)} />
          </label>
          <label>
            Suggested replacement (optional)
            <textarea
              value={suggestion}
              onChange={(e) => setSuggestion(e.target.value)}
            />
          </label>
          <button
            disabled={!body || !editor}
            onClick={() =>
              run(async () => {
                await accountRequest(path, {
                  body,
                  suggestion,
                  anchor: selection(),
                });
                setBody("");
                setSuggestion("");
              })
            }
          >
            Add comment
          </button>
        </>
      )}
      {comments.map((c) => (
        <article key={c.id} className="writer-comment">
          <strong>
            {c.authorName} · {c.status}
          </strong>
          <blockquote>
            {(() => {
              try {
                return JSON.parse(c.anchor).quote;
              } catch {
                return "Text anchor unavailable";
              }
            })()}
          </blockquote>
          <p>{c.body}</p>
          {c.suggestion && <p>Suggestion: {c.suggestion}</p>}
          <button
            onClick={() =>
              run(() => {
                const p = positions(c);
                editor.chain().focus().setTextSelection(p).run();
              })
            }
          >
            Find text
          </button>
          {role !== "viewer" && c.status === "open" && (
            <button
              onClick={() =>
                run(() =>
                  accountRequest(path, { id: c.id, status: "resolved" }),
                )
              }
            >
              Resolve
            </button>
          )}
          {["owner", "administrator", "editor"].includes(role) &&
            c.suggestion &&
            c.status === "open" && (
              <button
                onClick={() =>
                  run(async () => {
                    const p = positions(c);
                    if (editor.state.doc.textBetween(p.from, p.to) !== p.quote)
                      throw new Error(
                        "The selected text changed. Review the suggestion against the current manuscript.",
                      );
                    editor
                      .chain()
                      .focus()
                      .insertContentAt(
                        { from: p.from, to: p.to },
                        { type: "text", text: c.suggestion },
                      )
                      .run();
                    await room.flush();
                    await accountRequest(path, {
                      id: c.id,
                      status: "resolved",
                    });
                  })
                }
              >
                Accept suggestion
              </button>
            )}
        </article>
      ))}
    </section>
  );
}
