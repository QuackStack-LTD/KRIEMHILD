import { useEffect, useMemo, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import { getSchema } from "@tiptap/core";
import Collaboration from "@tiptap/extension-collaboration";
import { archiveExtensions } from "./archive-extensions.js";
import { ManuscriptRoom } from "./collaboration.js";
import { useAuth } from "../../auth/AuthProvider.jsx";
import { words, textContent } from "./model.js";
import { requestText } from "../ui/feedback.js";
import Icon from "./ArchiveIcon.jsx";
export default function ArchiveEditor(props) {
  const auth = useAuth(),
    schema = useMemo(() => getSchema(archiveExtensions), []);
  const [room, setRoom] = useState(null),
    [state, setState] = useState({ ready: !auth });
  useEffect(() => {
    if (!auth || props.inworld) return;
    const live = new ManuscriptRoom({
      worldId: props.worldId,
      node: props.node,
      userId: auth.user.id,
      schema,
      onState: setState,
    });
    setRoom(live);
    props.onRoom?.(live);
    return () => {
      props.onRoom?.(null);
      live.destroy();
    };
  }, [props.worldId, props.node.id, auth?.user.id]);
  if (auth && !props.inworld && !state.ready)
    return (
      <div className="archive-editor-loading" role="status">
        {state.error || "Opening document…"}
      </div>
    );
  return (
    <DocumentCanvas
      {...props}
      room={room}
      liveState={state}
      editable={
        !props.readOnly &&
        (props.inworld ||
          !auth ||
          ["owner", "administrator", "editor"].includes(state.role))
      }
    />
  );
}
function DocumentCanvas({
  worldId,
  sourceWorldId,
  node,
  onChange,
  onSave,
  onEditor,
  onImage,
  room,
  liveState,
  editable,
}) {
  const latest = useRef({ onChange, onSave });
  latest.current = { onChange, onSave };
  const [outline, setOutline] = useState(true),
    [find, setFind] = useState(false),
    [term, setTerm] = useState(""),
    [replacement, setReplacement] = useState(""),
    [zoom, setZoom] = useState(100),
    [font, setFont] = useState("Georgia"),
    [searchMessage, setSearchMessage] = useState("");
  const imageInput = useRef(null);
  const editor = useEditor(
    {
      extensions: room
        ? [
            ...archiveExtensions.map((e) =>
              e.name === "starterKit" ? e.configure({ undoRedo: false }) : e,
            ),
            Collaboration.configure({ document: room.doc }),
          ]
        : archiveExtensions,
      content: room ? undefined : node.content,
      editable,
      shouldRerenderOnTransaction: true,
      editorProps: {
        attributes: {
          role: "textbox",
          "aria-label": "Document body",
          "aria-multiline": "true",
          spellcheck: "true",
        },
      },
      onUpdate: ({ editor: e }) => {
        if (editable)
          latest.current.onChange({ content: e.getJSON(), text: e.getText() });
      },
      onBlur: () => void latest.current.onSave(),
    },
    [node.id, room],
  );
  useEffect(() => {
    onEditor?.(editor);
    return () => onEditor?.(null);
  }, [editor]);
  useEffect(() => {
    if (!editor || !editable || !sourceWorldId || sourceWorldId === worldId)
      return;
    let tr = editor.state.tr;
    editor.state.doc.descendants((n, pos) => {
      if (
        n.type.name === "image" &&
        n.attrs.src?.startsWith(`/api/worlds/${sourceWorldId}/`)
      )
        tr = tr.setNodeMarkup(pos, undefined, {
          ...n.attrs,
          src:
            `/api/worlds/${worldId}/` +
            n.attrs.src.slice(`/api/worlds/${sourceWorldId}/`.length),
        });
      for (const mark of n.marks)
        if (
          mark.type.name === "worldReference" &&
          mark.attrs.worldId === sourceWorldId
        )
          tr = tr
            .removeMark(pos, pos + n.nodeSize, mark.type)
            .addMark(
              pos,
              pos + n.nodeSize,
              mark.type.create({ ...mark.attrs, worldId }),
            );
    });
    if (tr.docChanged) editor.view.dispatch(tr);
  }, [editor, editable, sourceWorldId, worldId]);
  useEffect(() => {
    const key = (e) => {
      if (
        (e.ctrlKey || e.metaKey) &&
        ["s", "f"].includes(e.key.toLowerCase())
      ) {
        e.preventDefault();
        if (e.key.toLowerCase() === "s") void latest.current.onSave();
        else setFind((v) => !v);
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, []);
  if (!editor) return null;
  const headings = [];
  editor.state.doc.descendants((n, pos) => {
    if (n.type.name === "heading")
      headings.push({ text: n.textContent, level: n.attrs.level, pos });
  });
  const act = (fn) => {
    fn();
    document
      .querySelectorAll(".archive-editor-menu[open]")
      .forEach((el) => el.removeAttribute("open"));
  };
  const command = (name) => editable && editor.chain().focus()[name]().run();
  async function link() {
    const href = await requestText(
      "Link address",
      editor.getAttributes("link").href || "https://",
    );
    if (href === null) return;
    if (!href) editor.chain().focus().unsetLink().run();
    else if (/^(https?:\/\/|mailto:)/i.test(href))
      editor.chain().focus().setLink({ href }).run();
  }
  function matches() {
    const out = [];
    if (!term) return out;
    editor.state.doc.descendants((n, pos) => {
      if (!n.isTextblock) return;
      const text = n.textBetween(0, n.content.size, "", "\n").toLowerCase(),
        needle = term.toLowerCase();
      for (let i = 0; (i = text.indexOf(needle, i)) >= 0; i += needle.length)
        out.push({ from: pos + 1 + i, to: pos + 1 + i + term.length });
      return false;
    });
    return out;
  }
  function nextMatch() {
    const all = matches(),
      hit = all.find((x) => x.from >= editor.state.selection.to) || all[0];
    if (hit)
      editor.chain().focus().setTextSelection(hit).scrollIntoView().run();
    setSearchMessage(`${all.length} matches`);
  }
  const tools = [
    ["Bold", "bold", "B", "toggleBold"],
    ["Italic", "italic", "I", "toggleItalic"],
    ["Underline", "underline", "U", "toggleUnderline"],
    ["Strikethrough", "strike", "S", "toggleStrike"],
  ];
  return (
    <>
      <div
        className="archive-editor-menubar"
        role="menubar"
        aria-label="Document menu"
      >
        <details className="archive-editor-menu">
          <summary>Edit</summary>
          <div>
            <button onClick={() => act(() => command("undo"))}>
              Undo <kbd>Ctrl Z</kbd>
            </button>
            <button onClick={() => act(() => command("redo"))}>
              Redo <kbd>Ctrl Shift Z</kbd>
            </button>
            <button onClick={() => act(() => setFind(true))}>
              Find and replace <kbd>Ctrl F</kbd>
            </button>
            <button
              onClick={() =>
                act(() => editor.chain().focus().selectAll().run())
              }
            >
              Select all <kbd>Ctrl A</kbd>
            </button>
          </div>
        </details>
        <details className="archive-editor-menu">
          <summary>View</summary>
          <div>
            <button onClick={() => act(() => setOutline((v) => !v))}>
              {outline ? "Hide" : "Show"} document outline
            </button>
            <button onClick={() => act(() => setZoom(100))}>Reset zoom</button>
          </div>
        </details>
        <details className="archive-editor-menu">
          <summary>Insert</summary>
          <div>
            <button
              disabled={!editable}
              onClick={() => act(() => imageInput.current?.click())}
            >
              Image from your device
            </button>
            <button disabled={!editable} onClick={() => act(() => void link())}>
              Link
            </button>
            <button
              disabled={!editable}
              onClick={() => act(() => command("setHorizontalRule"))}
            >
              Horizontal line
            </button>
          </div>
        </details>
        <details className="archive-editor-menu">
          <summary>Format</summary>
          <div>
            <button onClick={() => act(() => command("toggleBold"))}>
              Bold
            </button>
            <button onClick={() => act(() => command("toggleItalic"))}>
              Italic
            </button>
            <button onClick={() => act(() => command("toggleUnderline"))}>
              Underline
            </button>
            <button
              onClick={() =>
                act(() =>
                  editor.chain().focus().unsetAllMarks().clearNodes().run(),
                )
              }
            >
              Clear formatting
            </button>
          </div>
        </details>
        <span className="archive-menu-status">
          {liveState.error ||
            (liveState.pending
              ? "Synchronizing…"
              : !editable
                ? "View only"
                : "Changes save automatically")}
        </span>
      </div>
      <fieldset
        className="archive-format-toolbar"
        disabled={!editable}
        aria-label="Document formatting"
      >
        <button
          title="Undo"
          aria-label="Undo"
          disabled={!editor.can().undo()}
          onClick={() => command("undo")}
        >
          ↶
        </button>
        <button
          title="Redo"
          aria-label="Redo"
          disabled={!editor.can().redo()}
          onClick={() => command("redo")}
        >
          ↷
        </button>
        <span className="archive-toolbar-divider" />
        <select
          aria-label="Document zoom"
          value={zoom}
          onChange={(e) => setZoom(+e.target.value)}
        >
          {[75, 90, 100, 125, 150].map((v) => (
            <option key={v} value={v}>
              {v}%
            </option>
          ))}
        </select>
        <select
          aria-label="Paragraph style"
          value={
            editor.isActive("heading")
              ? editor.getAttributes("heading").level
              : "paragraph"
          }
          onChange={(e) =>
            e.target.value === "paragraph"
              ? editor.chain().focus().setParagraph().run()
              : editor
                  .chain()
                  .focus()
                  .setHeading({ level: +e.target.value })
                  .run()
          }
        >
          <option value="paragraph">Normal text</option>
          {[1, 2, 3, 4].map((n) => (
            <option key={n} value={n}>
              Heading {n}
            </option>
          ))}
        </select>
        <select
          aria-label="Reading font"
          title="Reading font"
          value={font}
          onChange={(e) => setFont(e.target.value)}
        >
          <option value="Georgia">Serif</option>
          <option value="system-ui">Sans serif</option>
          <option value="monospace">Monospace</option>
        </select>
        <span className="archive-toolbar-divider" />
        {tools.map(([label, mark, symbol, fn]) => (
          <button
            key={mark}
            className={`archive-tool-${mark}`}
            title={label}
            aria-label={label}
            aria-pressed={editor.isActive(mark)}
            onClick={() => command(fn)}
          >
            {symbol}
          </button>
        ))}
        <span className="archive-toolbar-divider" />
        <button title="Insert link" aria-label="Insert link" onClick={link}>
          <Icon name="link" size={17} />
        </button>
        <button
          title="Insert image"
          aria-label="Insert image"
          onClick={() => imageInput.current?.click()}
        >
          <Icon name="image" size={18} />
        </button>
        <button
          title="Bulleted list"
          aria-label="Bulleted list"
          aria-pressed={editor.isActive("bulletList")}
          onClick={() => command("toggleBulletList")}
        >
          ☷
        </button>
        <button
          title="Numbered list"
          aria-label="Numbered list"
          aria-pressed={editor.isActive("orderedList")}
          onClick={() => command("toggleOrderedList")}
        >
          1.
        </button>
        <button
          title="Quote"
          aria-label="Quote"
          aria-pressed={editor.isActive("blockquote")}
          onClick={() => command("toggleBlockquote")}
        >
          ❞
        </button>
      </fieldset>
      {find && (
        <div className="archive-find">
          <input
            aria-label="Find in document"
            placeholder="Find in document"
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") nextMatch();
            }}
          />
          <input
            aria-label="Replace with"
            placeholder="Replace with"
            value={replacement}
            onChange={(e) => setReplacement(e.target.value)}
          />
          <button onClick={nextMatch}>Find</button>
          <button
            disabled={!term || !editable}
            onClick={() => {
              const all = matches();
              let tr = editor.state.tr;
              for (const h of all.reverse())
                tr = replacement
                  ? tr.insertText(replacement, h.from, h.to)
                  : tr.delete(h.from, h.to);
              editor.view.dispatch(tr);
              setSearchMessage(`Replaced ${all.length} matches`);
            }}
          >
            Replace all
          </button>
          <output>{searchMessage}</output>
          <button aria-label="Close find" onClick={() => setFind(false)}>
            <Icon name="close" />
          </button>
        </div>
      )}
      <div
        className={`archive-editor-workspace ${outline ? "with-outline" : ""}`}
      >
        {outline && (
          <aside className="archive-document-outline">
            <span>Document outline</span>
            {headings.length ? (
              headings.map((h) => (
                <button
                  key={h.pos}
                  style={{ paddingLeft: 12 + (h.level - 1) * 12 }}
                  onClick={() =>
                    editor
                      .chain()
                      .focus()
                      .setTextSelection(h.pos + 1)
                      .scrollIntoView()
                      .run()
                  }
                >
                  {h.text || "Untitled heading"}
                </button>
              ))
            ) : (
              <p>Headings you add will appear here.</p>
            )}
          </aside>
        )}
        <div className="archive-paper-scroll">
          <div className="archive-ruler" aria-hidden="true">
            1　　2　　3　　4　　5　　6　　7　　8　　9　　10　　11　　12　　13　　14　　15
          </div>
          <article
            className="archive-paper"
            style={{ zoom: zoom / 100, "--archive-reading-font": font }}
          >
            <EditorContent editor={editor} />
          </article>
        </div>
      </div>
      <footer className="archive-document-footer">
        <span>
          {words(textContent(editor.getJSON())).toLocaleString()} words
        </span>
        <span>{editor.getText().length.toLocaleString()} characters</span>
        <span>
          {node.format?.toUpperCase() || "Document"} · Writer's Archive
        </span>
      </footer>
      <input
        type="file"
        accept="image/*"
        ref={imageInput}
        hidden
        onChange={async (e) => {
          const file = e.target.files?.[0];
          e.target.value = "";
          if (file) {
            const src = await onImage(file);
            if (src && !editor.isDestroyed)
              editor
                .chain()
                .focus()
                .insertContent({
                  type: "image",
                  attrs: { src, alt: file.name },
                })
                .run();
          }
        }}
      />
    </>
  );
}
