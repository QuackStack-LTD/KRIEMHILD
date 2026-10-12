import { Mark, mergeAttributes } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { Markdown, MarkdownManager } from "@tiptap/markdown";
export const WorldReference = Mark.create({
  name: "worldReference",
  inclusive: false,
  addAttributes() {
    return {
      worldId: { default: null },
      entityId: { default: null },
      ageId: { default: null },
    };
  },
  parseHTML() {
    return [{ tag: "span[data-world-reference]" }];
  },
  renderHTML({ HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-world-reference": "true",
        class: "writer-reference",
      }),
      0,
    ];
  },
  renderMarkdown(node, helpers) {
    return helpers.renderChildren(node.content || []);
  },
});
export const extensions = [
  StarterKit.configure({
    link: { openOnClick: false, protocols: ["http", "https", "mailto"] },
  }),
  WorldReference,
  Markdown,
  Placeholder.configure({ placeholder: "Begin your scene…" }),
];
export const markdown = new MarkdownManager({ extensions });
