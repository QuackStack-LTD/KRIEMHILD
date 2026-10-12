import { useEffect, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import { extensions } from "./editor.js";
import { requestText } from "../ui/feedback.js";
export default function ManuscriptEditor({
  node,
  onChange,
  onBlur,
  onEditor,
  preferences,
  findOpen,
}) {
  const latest = useRef({ onChange, onBlur });
  latest.current = { onChange, onBlur };
  const [find, setFind] = useState(""),
    [replace, setReplace] = useState(""),
    [message, setMessage] = useState("");
  const editor = useEditor(
    {
      extensions,
      shouldRerenderOnTransaction: true,
      content: node.content,
      editorProps: {
        attributes: {
          role: "textbox",
          "aria-label": "Manuscript",
          "aria-multiline": "true",
          spellcheck: "true",
        },
      },
      onUpdate: ({ editor }) =>
        latest.current.onChange({
          content: editor.getJSON(),
          text: editor.getText(),
        }),
      onBlur: () => latest.current.onBlur(),
    },
    [node.id],
  );
  useEffect(() => {
    onEditor(editor);
    return () => onEditor(null);
  }, [editor]);
  if (!editor) return null;
  function matches() {
    const found = [];
    if (!find) return found;
    editor.state.doc.descendants((n, pos) => {
      if (!n.isTextblock) return;
      const text = n
          .textBetween(0, n.content.size, "", "\n")
          .toLocaleLowerCase(),
        term = find.toLocaleLowerCase();
      let i = 0;
      while ((i = text.indexOf(term, i)) >= 0) {
        found.push({ from: pos + 1 + i, to: pos + 1 + i + find.length });
        i += find.length;
      }
      return false;
    });
    return found;
  }
  function next() {
    const all = matches();
    const hit = all.find((x) => x.from >= editor.state.selection.to) || all[0];
    if (hit)
      editor.chain().focus().setTextSelection(hit).scrollIntoView().run();
    setMessage(`${all.length} matches`);
  }
  function replaceAll() {
    const all = matches();
    let tr = editor.state.tr;
    for (const hit of [...all].reverse())
      tr = replace
        ? tr.insertText(replace, hit.from, hit.to)
        : tr.delete(hit.from, hit.to);
    editor.view.dispatch(tr);
    setMessage(`Replaced ${all.length} matches`);
  }
  const buttons = [
    ["Bold", "bold", () => editor.chain().focus().toggleBold().run()],
    ["Italic", "italic", () => editor.chain().focus().toggleItalic().run()],
    [
      "Heading",
      "heading",
      () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
    ],
    [
      "Quote",
      "blockquote",
      () => editor.chain().focus().toggleBlockquote().run(),
    ],
    [
      "Bullets",
      "bulletList",
      () => editor.chain().focus().toggleBulletList().run(),
    ],
    [
      "Numbered",
      "orderedList",
      () => editor.chain().focus().toggleOrderedList().run(),
    ],
  ];
  return (
    <>
      <div
        className="writer-format"
        role="toolbar"
        aria-label="Text formatting"
      >
        {buttons.map(([name, mark, act]) => (
          <button key={name} aria-pressed={editor.isActive(mark)} onClick={act}>
            {name}
          </button>
        ))}
        <button
          onClick={async () => {
            const href = await requestText(
              "Link address (https://, http:// or mailto:)",
              editor.getAttributes("link").href || "",
            );
            if (href === null) return;
            if (!href) editor.chain().focus().unsetLink().run();
            else if (/^(https?:\/\/|mailto:)/i.test(href))
              editor.chain().focus().setLink({ href }).run();
          }}
        >
          Link
        </button>
        <button
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
        >
          Scene break
        </button>
        <button
          onClick={() => editor.chain().focus().undo().run()}
          disabled={!editor.can().undo()}
        >
          Undo
        </button>
        <button
          onClick={() => editor.chain().focus().redo().run()}
          disabled={!editor.can().redo()}
        >
          Redo
        </button>
      </div>
      {findOpen && (
        <div className="writer-find">
          <label>
            Find in scene
            <input value={find} onChange={(e) => setFind(e.target.value)} />
          </label>
          <label>
            Replace with
            <input
              value={replace}
              onChange={(e) => setReplace(e.target.value)}
            />
          </label>
          <button onClick={next}>Next match</button>
          <button onClick={replaceAll} disabled={!find}>
            Replace all in scene
          </button>
          <output>{message}</output>
        </div>
      )}
      <div
        className={`writer-paper ${preferences.theme || "paper"}`}
        style={{
          "--writer-font":
            preferences.font === "sans"
              ? "system-ui"
              : preferences.font === "mono"
                ? "monospace"
                : "Georgia, serif",
          "--writer-size": `${preferences.fontSize || 19}px`,
          "--writer-leading": preferences.lineHeight || 1.8,
        }}
      >
        <EditorContent editor={editor} />
      </div>
    </>
  );
}
