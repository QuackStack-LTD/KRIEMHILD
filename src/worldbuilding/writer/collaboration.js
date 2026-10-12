import * as Y from "yjs";
import {
  prosemirrorJSONToYDoc,
  yXmlFragmentToProsemirrorJSON,
} from "@tiptap/y-tiptap";
import { textContent, inlineReferences } from "./model.js";

// Exports reconstruct all scenes from acknowledged updates, including scenes
// whose last REST snapshot predates a disconnected collaborator's final edit.
export function materializeManuscripts(bundle) {
  return bundle.nodes.map((node) => {
    const updates = bundle.updates?.[node.id];
    if (!updates?.length) return node;
    const doc = new Y.Doc();
    try {
      for (const update of updates)
        Y.applyUpdate(doc, decodeUpdate(update.data));
      let content = yXmlFragmentToProsemirrorJSON(
        doc.getXmlFragment("default"),
      );
      const old = bundle.project.importedWorldId,
        next = bundle.project.worldId;
      const remap = (value) => {
        if (!value || typeof value !== "object") return;
        if (value.worldId === old && old) value.worldId = next;
        for (const child of Object.values(value)) remap(child);
      };
      remap(content);
      return {
        ...node,
        content,
        text: textContent(content),
        references: [
          ...(node.references || []).filter((r) => r.type !== "inline"),
          ...inlineReferences(content),
        ],
      };
    } finally {
      doc.destroy();
    }
  });
}
export const encodeUpdate = (data) => {
  let text = "";
  for (const byte of data) text += String.fromCharCode(byte);
  return btoa(text);
};
export const decodeUpdate = (data) =>
  Uint8Array.from(atob(data), (c) => c.charCodeAt(0));

// The server acknowledges only durable updates. Reconnect replays the log and
// pending deltas; Yjs makes replay idempotent and merges concurrent text edits.
export class ManuscriptRoom {
  constructor({ worldId, node, userId, schema, onState, onComments }) {
    this.doc = new Y.Doc();
    this.node = node;
    this.schema = schema;
    this.worldId = worldId;
    this.onState = onState;
    this.onComments = onComments;
    this.pending = new Map();
    this.pendingEpochs = new Map();
    this.epoch = "";
    this.ready = false;
    this.synced = false;
    this.stopped = false;
    this.role = "viewer";
    this.presenceListeners = new Set();
    this.peers = [];
    this.storageKey = `kriemhild.collaboration.${userId}.${node.id}`;
    try {
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key.startsWith(this.storageKey + ".")) {
          const id = key.slice(this.storageKey.length + 1),
            raw = localStorage.getItem(key);
          let saved;
          try {
            saved = JSON.parse(raw);
          } catch {
            saved = { data: raw, epoch: "" };
          }
          this.pending.set(id, saved.data);
          this.pendingEpochs.set(id, saved.epoch);
        }
      }
    } catch {
      this.storageError = true;
    }
    this.doc.on(
      "update",
      (this.onUpdate = (update, origin) => {
        if (origin === this) return;
        const id = crypto.randomUUID();
        this.pending.set(id, encodeUpdate(update));
        this.pendingEpochs.set(id, this.epoch);
        this.persist();
        this.send({ type: "update", id, data: this.pending.get(id) });
        this.report();
      }),
    );
    this.connect();
  }
  report(error) {
    if (error !== undefined) this.error = error;
    this.onState?.({
      ready: this.ready,
      role: this.role,
      connected: this.synced,
      pending: this.pending.size,
      error:
        this.error ||
        (this.storageError
          ? "Offline recovery storage unavailable; keep this tab open until synced."
          : ""),
    });
  }
  persist() {
    try {
      for (const [id, data] of this.pending)
        localStorage.setItem(
          this.storageKey + "." + id,
          JSON.stringify({ data, epoch: this.pendingEpochs.get(id) }),
        );
      this.storageError = false;
    } catch {
      this.storageError = true;
    }
  }
  send(message) {
    if (this.synced && this.socket?.readyState === WebSocket.OPEN)
      this.socket.send(JSON.stringify(message));
  }
  connect() {
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    this.socket = new WebSocket(
      `${protocol}//${location.host}/api/collaboration/worlds/${this.worldId}?project=${encodeURIComponent(this.node.projectId)}&node=${encodeURIComponent(this.node.id)}`,
    );
    this.socket.onmessage = ({ data }) => {
      try {
        const message = JSON.parse(data);
        if (message.type === "sync") {
          this.synced = true;
          this.role = message.role;
          const seed =
            message.updates.find((u) => u.id.startsWith("seed")) ||
            message.updates[0];
          const epoch = seed
            ? seed.id === "seed"
              ? seed.data.slice(0, 96)
              : seed.id
            : "";
          if (this.ready && this.epoch !== epoch) {
            this.stopped = true;
            this.role = "viewer";
            this.report(
              "A manuscript revision was restored while you were away. Save your local recovery as a separate copy, then reload.",
            );
            this.socket.close();
            return;
          }
          this.epoch = epoch;
          this.error = "";
          if (!message.updates.length) {
            if (["owner", "administrator", "editor"].includes(this.role)) {
              const seed = prosemirrorJSONToYDoc(
                this.schema,
                message.node.content,
                "default",
              );
              this.send({
                type: "seed",
                id: "seed",
                data: encodeUpdate(Y.encodeStateAsUpdate(seed)),
                revision: message.node.revision,
              });
              seed.destroy();
            }
          } else {
            for (const u of message.updates)
              Y.applyUpdate(this.doc, decodeUpdate(u.data), this);
            this.ready = true;
            this.replay();
          }
          this.report();
        }
        if (message.type === "update") {
          if (message.id.startsWith("seed"))
            this.epoch =
              message.id === "seed" ? message.data.slice(0, 96) : message.id;
          Y.applyUpdate(this.doc, decodeUpdate(message.data), this);
          if (!this.ready) {
            this.ready = true;
            this.replay();
          }
          this.report();
        }
        if (message.type === "ack") {
          this.pending.delete(message.id);
          this.pendingEpochs.delete(message.id);
          try {
            localStorage.removeItem(this.storageKey + "." + message.id);
          } catch {}
          this.report();
        }
        if (message.type === "comments") this.onComments?.();
        if (message.type === "presence") {
          this.peers = message.members;
          for (const listener of this.presenceListeners) listener();
        }
        if (message.type === "reset") {
          this.stopped = true;
          this.synced = false;
          this.role = "viewer";
          this.report(
            "A manuscript revision was restored. Reload this page to open it; pending edits remain in local recovery.",
          );
          this.socket.close();
        }
        if (message.type === "error") {
          this.report(message.message);
          this.socket.close();
        }
      } catch {
        this.report(
          "Unable to read collaborative state. Your pending changes are retained.",
        );
        this.socket.close();
      }
    };
    this.socket.onclose = (event) => {
      this.synced = false;
      if (event.code === 1008) this.role = "viewer";
      this.report(
        event.code === 1008
          ? "Editing access was revoked. Pending changes remain in local recovery."
          : undefined,
      );
      if (!this.stopped && event.code !== 1008)
        this.timer = setTimeout(() => this.connect(), 3000);
    };
  }
  replay() {
    for (const [id, data] of this.pending) {
      if (
        this.pendingEpochs.get(id) &&
        this.pendingEpochs.get(id) !== this.epoch
      ) {
        this.pending.delete(id);
        this.error =
          "Edits from a restored manuscript remain in local recovery. Recover them as a separate copy.";
        continue;
      }
      Y.applyUpdate(this.doc, decodeUpdate(data), this);
      this.send({ type: "update", id, data });
    }
  }
  async flush() {
    const end = Date.now() + 8000;
    while (this.pending.size && !this.stopped && Date.now() < end)
      await new Promise((resolve) => setTimeout(resolve, 80));
    if (this.pending.size)
      throw new Error(
        "Live changes are awaiting synchronization. Keep this tab open or reconnect.",
      );
  }
  destroy() {
    this.stopped = true;
    clearTimeout(this.timer);
    clearTimeout(this.selectionTimer);
    this.socket?.close();
    this.doc.off("update", this.onUpdate);
    this.doc.destroy();
  }
}
