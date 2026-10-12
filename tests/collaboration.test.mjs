import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { getSchema } from "@tiptap/core";
import { yXmlFragmentToProsemirrorJSON } from "@tiptap/y-tiptap";
import { extensions } from "../src/worldbuilding/writer/editor.js";
import { ManuscriptRoom, materializeManuscripts } from "../src/worldbuilding/writer/collaboration.js";

test("two manuscript clients merge edits, replay offline changes and recover durable state after restart", { timeout: 120000 }, async () => {
  const root = path.resolve(import.meta.dirname, ".."), temp = fs.mkdtempSync(path.join(os.tmpdir(), "kriemhild-collab-"));
  const binary = path.join(temp, process.platform === "win32" ? "server.exe" : "server"), local = path.join(root, ".tools/go/bin", process.platform === "win32" ? "go.exe" : "go");
  const build = spawnSync(fs.existsSync(local) ? local : "go", ["build", "-o", binary, "./cmd/kriemhild"], { cwd: root, encoding: "utf8", windowsHide: true });
  assert.equal(build.status, 0, build.stderr);
  const origin = "http://127.0.0.1:18137"; let server; const rooms = [];
  const storage = new Map(); globalThis.localStorage = { get length() { return storage.size; }, key(i) { return [...storage.keys()][i]; }, getItem: (k) => storage.get(k) ?? null, setItem: (k, v) => storage.set(k, v), removeItem: (k) => storage.delete(k) }; globalThis.location = new URL(origin);
  const wait = async (fn) => { for (let i = 0; i < 160; i++) { if (fn()) return; await delay(50); } throw new Error("Timed out waiting for collaboration"); };
  const launch = async () => { server = spawn(binary, ["-addr", "127.0.0.1:18137", "-data", path.join(temp, "data")], { cwd: root, stdio: "pipe", windowsHide: true, env: { ...process.env, KRIEMHILD_AUTH_MODE: "local", DATABASE_URL: "", DATABASE_URL_FILE: "" } }); for (let i = 0; i < 100; i++) { try { if ((await fetch(origin + "/api/ready")).ok) return; } catch {} await delay(50); } throw new Error("Server did not start"); };
  const stop = async () => { if (server && server.exitCode === null) { const stopped = once(server, "exit"); server.kill(); await stopped; } };
  const request = async (route, body) => { const response = await fetch(origin + route, body === undefined ? undefined : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await response.json(); assert.ok(response.ok, JSON.stringify(data)); return data; };
  const contents = (room) => JSON.stringify(yXmlFragmentToProsemirrorJSON(room.doc.getXmlFragment("default")));
  try {
    await launch();
    const world = await request("/api/worlds", { name: "Collaboration test" });
    const project = await request("/api/writer/projects", { id: "novel", worldId: world.world.id, name: "Novel", settings: {} });
    const node = await request("/api/writer/projects/novel/nodes", { node: { id: "scene", projectId: project.id, title: "Opening", type: "scene", position: 0, status: "Drafting", contentSchema: 1, content: { type: "doc", content: [{ type: "paragraph", content: [{ type: "text", text: "Shared words" }] }] } } });
    const open = () => { const room = new ManuscriptRoom({ worldId: world.world.id, node, userId: "test", schema: getSchema(extensions), onState() {} }); rooms.push(room); return room; };
    const a = open(), b = open(); await wait(() => a.ready && b.ready);
    a.doc.getXmlFragment("default").get(0).get(0).insert(0, "Alice ");
    b.doc.getXmlFragment("default").get(0).get(0).insert(0, "Bob ");
    await Promise.all([a.flush(), b.flush()]); await wait(() => contents(a) === contents(b));
    assert.match(contents(a), /Alice/); assert.match(contents(a), /Bob/);
    const before = contents(a); assert.equal((before.match(/Shared words/g) || []).length, 1, "initial manuscript duplicated");
    a.socket.close(); await wait(() => !a.synced); a.doc.getXmlFragment("default").get(0).get(0).insert(0, "Offline "); await wait(() => a.synced); await a.flush(); await wait(() => contents(a) === contents(b));
    const expected = contents(a); for (const room of rooms) room.destroy(); await stop(); await launch();
    const c = open(); await wait(() => c.ready); assert.equal(contents(c), expected);
    const backup = await request("/api/writer/projects/novel?backup=1"); assert.ok(backup.updates.scene.length >= 3, "backup omitted collaborative state");
    assert.match(materializeManuscripts(backup)[0].text,/Offline/);assert.match(materializeManuscripts(backup)[0].text,/Alice/);assert.match(materializeManuscripts(backup)[0].text,/Bob/);
    await request('/api/writer/projects/novel/recovery/scene',{version:1,revision:1});await wait(()=>c.stopped);
    const restored=open();await wait(()=>restored.ready);assert.doesNotMatch(contents(restored),/Alice|Offline/);
    restored.socket.close();await wait(()=>!restored.synced);restored.doc.getXmlFragment('default').get(0).get(0).insert(0,'Unsynced recovery ');
    await request('/api/writer/projects/novel/recovery/scene',{version:1,revision:2});
    const fresh=open();await wait(()=>fresh.ready);await wait(()=>restored.stopped);assert.match(contents(restored),/Unsynced recovery/);assert.doesNotMatch(contents(fresh),/Unsynced recovery/,'stale offline edits crossed a restoration boundary');
  } finally { for (const room of rooms) room.destroy(); await stop(); fs.rmSync(temp, { recursive: true, force: true }); delete globalThis.localStorage; delete globalThis.location; }
});
