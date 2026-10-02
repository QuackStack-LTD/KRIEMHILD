"use client";
import { useEffect, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import { getSchema } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Collaboration from "@tiptap/extension-collaboration";
import { prosemirrorJSONToYDoc, ySyncPluginKey } from "y-prosemirror";
import * as Y from "yjs";
import { api, Command, Doc, Entry, newID, State, textOf } from "../lib/types";
type Response = {
  sequence: number;
  updates: string[];
  document: Doc;
  peers: string[];
  conflict: boolean;
  revision: string;
};
import { stableBlocks } from "../lib/blocks";
const attributes = stableBlocks(ySyncPluginKey);
const encode = (u: Uint8Array) => {
  let s = "";
  for (let i = 0; i < u.length; i += 8192)
    s += String.fromCharCode(...u.subarray(i, i + 8192));
  return btoa(s);
};
const decode = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
export default function LiveWriting({
  state,
  run,
  install,
}: {
  state: State;
  run: (c: Command) => Promise<boolean>;
  install: (s: State) => void;
}) {
  const [selected, setSelected] = useState("");
  const scenes = Object.values(state.records).filter((r) => r.kind === "scene");
  const scene = scenes.find((r) => r.id === selected) || scenes[0];
  return (
    <div>
      <div className="eyebrow">Shared manuscript / {state.age.name}</div>
      <h2>Write together, preserve revisions.</h2>
      <p>
        Changes merge in a durable shared draft. Save a revision to add the
        current manuscript to Age history and exports. A conflicting edit from
        the ordinary editor is reported, never overwritten.
      </p>
      <label>
        Shared scene
        <select
          value={scene?.id || ""}
          onChange={(e) => setSelected(e.target.value)}
        >
          {scenes.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      {scene ? (
        <Room
          key={state.age.id + scene.id}
          scene={scene}
          state={state}
          run={run}
          install={install}
        />
      ) : (
        <p>Create a scene in Writing first.</p>
      )}
    </div>
  );
}
function Room({
  scene,
  state,
  run,
  install,
}: {
  scene: Entry;
  state: State;
  run: (c: Command) => Promise<boolean>;
  install: (s: State) => void;
}) {
  const [doc, setDoc] = useState<Y.Doc | null>(null),
    [status, setStatus] = useState("Opening shared draft…"),
    [error, setError] = useState(""),
    [peers, setPeers] = useState<string[]>([]),
    [conflict, setConflict] = useState(false),
    [pendingCount, setPendingCount] = useState(0),
    [saving, setSaving] = useState(false),
    [comment, setComment] = useState(""),
    [anchor, setAnchor] = useState("");
  const client = useRef(newID()),
    lastRevision = useRef(state.revision),
    seq = useRef(0),
    pending = useRef<Uint8Array[]>([]),
    pumping = useRef(false),
    getContent = useRef<() => Doc>(() => scene.document!),
    pump = useRef<() => Promise<void>>(async () => {});
  const path = `/projects/${state.root.world.id}/live`,
    key = `kriemhild:shared:${state.root.world.id}:${state.age.id}:${scene.id}`;
  const post = (action: string, extra: Record<string, unknown> = {}) =>
    api<Response>(path, {
      method: "POST",
      body: JSON.stringify({
        action,
        age: state.age.id,
        scene: scene.id,
        client: client.current,
        after: seq.current,
        ...extra,
      }),
    });
  useEffect(() => {
    let disposed = false;
    const ydoc = new Y.Doc();
    let timer: ReturnType<typeof setInterval> | undefined;
    const apply = (r: Response) => {
      for (const u of r.updates) Y.applyUpdate(ydoc, decode(u), "remote");
      seq.current = Math.max(seq.current, r.sequence);
      setPeers(r.peers);
      setConflict(r.conflict);
      if (lastRevision.current !== r.revision) {
        lastRevision.current = r.revision;
        void api<State>(
          `/projects/${state.root.world.id}/state?age=${state.age.id}`,
        )
          .then((next) => {
            if (!disposed) install(next);
          })
          .catch((e) => setError(e.message));
      }
    };
    const poll = () =>
      api<Response>(
        `${path}?age=${state.age.id}&scene=${scene.id}&client=${client.current}&after=${seq.current}`,
      );
    const update = (u: Uint8Array, origin: unknown) => {
      if (origin === "remote") return;
      try {
        localStorage.setItem(key, encode(Y.encodeStateAsUpdate(ydoc)));
      } catch {
        setError(
          "Browser recovery storage is unavailable. Keep this tab open until synchronization succeeds.",
        );
      }
      pending.current.push(u);
      setPendingCount(pending.current.length);
      setStatus("Synchronizing draft…");
      setTimeout(() => void pump.current(), 50);
    };
    pump.current = async () => {
      if (pumping.current || disposed) return;
      pumping.current = true;
      const batch = pending.current.splice(0);
      try {
        const r = batch.length
          ? await post("update", { update: encode(Y.mergeUpdates(batch)) })
          : await poll();
        if (disposed) return;
        apply(r);
        setPendingCount(pending.current.length);
        if (batch.length)
          setStatus("Draft synchronized · save a revision for history");
        setError("");
      } catch (e) {
        pending.current.unshift(...batch);
        setPendingCount(pending.current.length);
        setError((e as Error).message);
        setStatus("Connection interrupted · browser draft retained");
      } finally {
        pumping.current = false;
      }
    };
    void (async () => {
      try {
        let r = await poll();
        if (disposed) return;
        while (!r.updates.length && state.readOnly && !disposed) {
          setStatus("Waiting for an editor to initialize the shared draft.");
          await new Promise((resolve) => setTimeout(resolve, 1000));
          if (!disposed) r = await poll();
        }
        if (disposed) return;
        if (!r.updates.length) {
          const schema = getSchema([
            StarterKit.configure({ link: false }),
            attributes,
          ]);
          const initial = prosemirrorJSONToYDoc(schema, r.document, "default");
          try {
            r = await post("initialize", {
              update: encode(Y.encodeStateAsUpdate(initial)),
            });
          } catch {
            r = await poll();
          } finally {
            initial.destroy();
          }
        }
        if (disposed) return;
        apply(r);
        ydoc.on("update", update);
        const recovered = localStorage.getItem(key);
        if (recovered && !state.readOnly)
          Y.applyUpdate(ydoc, decode(recovered), "recovery");
        setDoc(ydoc);
        setStatus("Shared draft open");
        timer = setInterval(() => void pump.current(), 1000);
      } catch (e) {
        setError((e as Error).message);
      }
    })();
    const before = (e: BeforeUnloadEvent) => {
      if (pending.current.length || pumping.current) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", before);
    return () => {
      disposed = true;
      if (timer) clearInterval(timer);
      window.removeEventListener("beforeunload", before);
      ydoc.off("update", update);
      ydoc.destroy();
    };
  }, [path, state.age.id, scene.id]);
  const notes = Object.values(state.records).filter(
    (r) => r.kind === "note" && r.properties?.commentOn === scene.id,
  );
  const anchors: Doc[] = [];
  const collectAnchors = (node: Doc) => {
    if (node.attrs?.blockId && anchors.length < 500) anchors.push(node);
    node.content?.forEach(collectAnchors);
  };
  if (scene.document) collectAnchors(scene.document);
  return (
    <section className="card">
      <h3>{scene.name}</h3>
      <p role="status">{status}</p>
      <p className="muted">Present: {peers.join(", ") || "connecting"}</p>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {conflict && (
        <p className="notice">
          The saved scene changed outside this shared draft. The draft is
          preserved. Save a separate scene to reconcile it deliberately.
        </p>
      )}
      {doc && (
        <SharedEditor
          doc={doc}
          readOnly={!!state.readOnly || saving}
          content={getContent}
        />
      )}
      <div className="map-tools">
        <button
          disabled={
            !doc || saving || pendingCount > 0 || conflict || state.readOnly
          }
          onClick={async () => {
            setSaving(true);
            try {
              for (
                let i = 0;
                i < 200 && (pumping.current || pending.current.length);
                i++
              ) {
                await pump.current();
                await new Promise((resolve) => setTimeout(resolve, 50));
              }
              if (pumping.current || pending.current.length)
                throw new Error(
                  "Synchronization has not finished; your draft is retained.",
                );
              await post("checkpoint", {
                sequence: seq.current,
                update: encode(Y.encodeStateAsUpdate(doc!)),
                document: getContent.current(),
              });
              localStorage.removeItem(key);
              install(
                await api<State>(
                  `/projects/${state.root.world.id}/state?age=${state.age.id}`,
                ),
              );
              setStatus("Shared revision saved to Age history");
            } catch (e) {
              setError((e as Error).message);
            } finally {
              setSaving(false);
            }
          }}
        >
          Save shared revision
        </button>
        <button
          disabled={!doc || state.readOnly}
          onClick={async () => {
            const record: Entry = {
              ...scene,
              id: newID(),
              name: scene.name + " — reconciled draft",
              document: getContent.current(),
              properties: {
                ...scene.properties,
                _collaborationState: encode(Y.encodeStateAsUpdate(doc!)),
              },
            };
            if (await run({ action: "put", record })) {
              setStatus("Saved as a separate scene");
              localStorage.removeItem(key);
            }
          }}
        >
          Save draft as separate scene
        </button>
      </div>
      <h4>Comments</h4>
      {notes.map((n) => (
        <article key={n.id}>
          <strong>{n.name}</strong>
          <p>{n.notes}</p>
          {!!n.properties?.blockID && (
            <small>
              {textOf(
                anchors.find((b) => b.attrs?.blockId === n.properties?.blockID),
              ).slice(0, 100) ||
                "Paragraph anchor unavailable; the comment is preserved."}
            </small>
          )}
        </article>
      ))}
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (
            await run({
              action: "put",
              record: {
                id: newID(),
                kind: "note",
                name: "Manuscript comment",
                notes: comment,
                properties: { commentOn: scene.id, blockID: anchor },
              },
            })
          )
            setComment("");
        }}
      >
        <label>
          Comment
          <textarea
            required
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
        </label>
        <label>
          Comment on
          <select
            aria-label="Comment on"
            value={anchor}
            onChange={(e) => setAnchor(e.target.value)}
          >
            <option value="">Whole scene</option>
            {anchors.map((b) => (
              <option
                key={String(b.attrs!.blockId)}
                value={String(b.attrs!.blockId)}
              >
                {textOf(b).slice(0, 100) || "Empty paragraph"}
              </option>
            ))}
          </select>
        </label>
        <button disabled={state.readOnly}>Add manuscript comment</button>
      </form>
    </section>
  );
}
function SharedEditor({
  doc,
  readOnly,
  content,
}: {
  doc: Y.Doc;
  readOnly: boolean;
  content: React.RefObject<() => Doc>;
}) {
  const editor = useEditor(
    {
      extensions: [
        StarterKit.configure({ link: false, undoRedo: false }),
        attributes,
        Collaboration.configure({ document: doc, field: "default" }),
      ],
      immediatelyRender: false,
      editable: !readOnly,
      editorProps: {
        attributes: {
          "aria-label": "Shared manuscript",
          role: "textbox",
          "aria-multiline": "true",
          dir: "auto",
          class: "manuscript",
        },
      },
    },
    [doc],
  );
  useEffect(() => {
    content.current = () => editor?.getJSON() as Doc;
  }, [editor, content]);
  useEffect(() => {
    editor?.setEditable(!readOnly);
  }, [editor, readOnly]);
  return (
    <div className="editor-shell">
      <div className="formatbar">
        <button
          disabled={readOnly}
          onClick={() => editor?.chain().focus().toggleBold().run()}
        >
          Bold
        </button>
        <button
          disabled={readOnly}
          onClick={() => editor?.chain().focus().toggleItalic().run()}
        >
          Italic
        </button>
        <button
          disabled={readOnly}
          onClick={() => editor?.chain().focus().undo().run()}
        >
          Undo my typing
        </button>
        <button
          disabled={readOnly}
          onClick={() => editor?.chain().focus().redo().run()}
        >
          Redo my typing
        </button>
      </div>
      <EditorContent editor={editor} />
    </div>
  );
}
