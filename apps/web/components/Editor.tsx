"use client";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { useEffect, useRef } from "react";
import type { Doc } from "../lib/types";

import { stableBlocks } from "../lib/blocks";

export default function Editor({
  value,
  onChange,
}: {
  value: Doc;
  onChange: (d: Doc) => void;
}) {
  const callback = useRef(onChange);
  callback.current = onChange;
  const editor = useEditor({
    extensions: [StarterKit.configure({ link: false }), stableBlocks()],
    content: value,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        "aria-label": "Manuscript",
        role: "textbox",
        "aria-multiline": "true",
        dir: "auto",
        class: "manuscript",
      },
    },
    onUpdate: ({ editor }) => callback.current(editor.getJSON() as Doc),
  });
  useEffect(() => {
    if (editor && JSON.stringify(editor.getJSON()) !== JSON.stringify(value))
      editor.commands.setContent(value, { emitUpdate: false });
  }, [editor, value]);
  if (!editor) return <p>Opening manuscript…</p>;
  return (
    <div className="editor-shell">
      <div className="formatbar" aria-label="Text formatting">
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleBold().run()}
          aria-label="Bold"
        >
          <b>B</b>
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleItalic().run()}
          aria-label="Italic"
        >
          <i>I</i>
        </button>
        <button
          type="button"
          onClick={() =>
            editor.chain().focus().toggleHeading({ level: 2 }).run()
          }
        >
          Heading
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleBulletList().run()}
        >
          List
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
        >
          Quote
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().undo().run()}
        >
          Undo typing
        </button>
        <button
          type="button"
          onClick={() => editor.chain().focus().redo().run()}
        >
          Redo typing
        </button>
      </div>
      <EditorContent editor={editor} />
    </div>
  );
}
