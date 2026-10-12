import { Extension } from "@tiptap/core";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import * as Y from "yjs";
import {
  ySyncPluginKey,
  absolutePositionToRelativePosition,
  relativePositionToAbsolutePosition,
} from "@tiptap/y-tiptap";

export function publishSelection(editor, room) {
  const sync = ySyncPluginKey.getState(editor.state);
  if (!sync?.binding) return;
  const { from, to } = editor.state.selection;
  const selection = JSON.stringify({
    client: room.doc.clientID,
    from: Y.relativePositionToJSON(
      absolutePositionToRelativePosition(from, sync.type, sync.binding.mapping),
    ),
    to: Y.relativePositionToJSON(
      absolutePositionToRelativePosition(to, sync.type, sync.binding.mapping),
    ),
  });
  clearTimeout(room.selectionTimer);
  room.selectionTimer = setTimeout(
    () => room.send({ type: "selection", selection }),
    100,
  );
}

export function remoteSelections(room) {
  return Extension.create({
    name: "remoteSelections",
    addProseMirrorPlugins() {
      return [
        new Plugin({
          key: new PluginKey("remoteSelections"),
          props: {
            decorations(state) {
              const sync = ySyncPluginKey.getState(state);
              if (!sync?.binding) return null;
              const decorations = [];
              for (const peer of room.peers || []) {
                if (peer.nodeId !== room.node.id || !peer.selection) continue;
                try {
                  const selection = JSON.parse(peer.selection);
                  if (selection.client === room.doc.clientID) continue;
                  const from = relativePositionToAbsolutePosition(
                      room.doc,
                      sync.type,
                      Y.createRelativePositionFromJSON(selection.from),
                      sync.binding.mapping,
                    ),
                    to = relativePositionToAbsolutePosition(
                      room.doc,
                      sync.type,
                      Y.createRelativePositionFromJSON(selection.to),
                      sync.binding.mapping,
                    );
                  if (from === null || to === null) continue;
                  if (from < to)
                    decorations.push(
                      Decoration.inline(from, to, {
                        class: "remote-selection",
                      }),
                    );
                  decorations.push(
                    Decoration.widget(
                      to,
                      () => {
                        const cursor = document.createElement("span"),
                          label = document.createElement("span");
                        cursor.className = "remote-cursor";
                        cursor.setAttribute(
                          "aria-label",
                          `${peer.name}'s cursor`,
                        );
                        label.textContent = peer.name;
                        cursor.append(label);
                        return cursor;
                      },
                      { key: peer.userId + ":" + selection.client, side: 1 },
                    ),
                  );
                } catch {
                  /* A removed text anchor has no cursor position. */
                }
              }
              return DecorationSet.create(state.doc, decorations);
            },
          },
        }),
      ];
    },
  });
}
