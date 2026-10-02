import { Extension } from "@tiptap/core";
import { Plugin, PluginKey } from "@tiptap/pm/state";

export function stableBlocks(remoteKey?: PluginKey) {
  return Extension.create({
    name: "stableBlocks",
    addGlobalAttributes() {
      return [
        {
          types: [
            "paragraph",
            "heading",
            "blockquote",
            "codeBlock",
            "listItem",
          ],
          attributes: { blockId: { default: null } },
        },
      ];
    },
    addProseMirrorPlugins() {
      return [
        new Plugin({
          appendTransaction(transactions, _old, state) {
            if (
              !transactions.some((t) => t.docChanged) ||
              (remoteKey && transactions.some((t) => t.getMeta(remoteKey)))
            )
              return null;
            const seen = new Set<string>(),
              tr = state.tr;
            let changed = false;
            state.doc.descendants((node, pos) => {
              if (
                ![
                  "paragraph",
                  "heading",
                  "blockquote",
                  "codeBlock",
                  "listItem",
                ].includes(node.type.name)
              )
                return;
              let id = node.attrs.blockId as string;
              if (!id || seen.has(id)) {
                id = crypto.randomUUID();
                tr.setNodeMarkup(pos, undefined, {
                  ...node.attrs,
                  blockId: id,
                });
                changed = true;
              }
              seen.add(id);
            });
            return changed ? tr : null;
          },
        }),
      ];
    },
  });
}
