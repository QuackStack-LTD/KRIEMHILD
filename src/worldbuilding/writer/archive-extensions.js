import { Node, mergeAttributes } from "@tiptap/core";
import { extensions } from "./editor.js";
export const ArchiveImage = Node.create({
  name: "image",
  group: "block",
  atom: true,
  draggable: true,
  addAttributes() {
    return {
      src: { default: "" },
      alt: { default: "" },
      title: { default: "" },
    };
  },
  parseHTML() {
    return [
      {
        tag: "img[src]",
        getAttrs: (el) =>
          /^\/api\/(writer\/projects\/|worlds\/)/.test(el.getAttribute("src"))
            ? null
            : false,
      },
    ];
  },
  renderHTML({ HTMLAttributes }) {
    return ["img", mergeAttributes(HTMLAttributes)];
  },
  renderMarkdown(node) {
    return `![${node.attrs?.alt || "Image"}](${node.attrs?.src || ""})`;
  },
});
export const archiveExtensions = [
  ...extensions.filter((e) => e.name !== "placeholder"),
  ArchiveImage,
];
