import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";

test("World ZIP survives deletion of the original session and a Go server process restart", async () => {
  const root = path.resolve(import.meta.dirname, ".."),
    binary = path.join(
      root,
      "bin",
      process.platform === "win32"
        ? "kriemhild-project-test.exe"
        : "kriemhild-project-test",
    );
  const local = path.join(
    root,
    ".tools/go/bin",
    process.platform === "win32" ? "go.exe" : "go",
  );
  const build = spawnSync(
    fs.existsSync(local) ? local : "go",
    ["build", "-o", binary, "./cmd/kriemhild"],
    { cwd: root, encoding: "utf8", windowsHide: true },
  );
  assert.equal(build.status, 0, build.stderr);
  const origin = "http://127.0.0.1:18126";
  let server;
  fs.mkdirSync(path.join(root, ".tools"), { recursive: true });
  const databaseDir = fs.mkdtempSync(path.join(root, ".tools", "restart-db-"));
  const launch = async () => {
    server = spawn(binary, ["-addr", "127.0.0.1:18126"], {
      cwd: root,
      stdio: "pipe",
      windowsHide: true,
      env: {
        ...process.env,
        KRIEMHILD_DATA_DIR: databaseDir,
        DATABASE_URL: "",
        DATABASE_URL_FILE: "",
      },
    });
    for (let n = 0; n < 100; n++) {
      try {
        if ((await fetch(origin + "/api/health")).ok) return;
      } catch {}
      await delay(50);
    }
    throw new Error("Test server did not start");
  };
  const stop = async () => {
    if (server && server.exitCode === null) {
      const closed = once(server, "exit");
      server.kill();
      await closed;
    }
  };
  const json = async (route, body) => {
    const r = await fetch(origin + "/api/" + route, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const value = await r.json();
    assert.ok(r.ok, JSON.stringify(value));
    return value;
  };
  try {
    await launch();
    const { defaults } = await import("../src/continuous/api.js");
    const job = await json("terrain/generation-jobs", {
      spec: { ...defaults, surfaceWidth: 1024, seed: "cold-process-project" },
    });
    let result;
    for (let n = 0; n < 1800; n++) {
      result = await fetch(
        origin + "/api/terrain/generation-jobs/" + job.id,
      ).then((r) => r.json());
      if (result.status === "ready" || result.status === "failed") break;
      await delay(100);
    }
    assert.equal(result.status, "ready", JSON.stringify(result));
    const initial = await fetch(
      origin + "/api/sessions/" + result.session + "/world/state",
    ).then((r) => r.json());
    const center = [0.45 * (initial.W - 1), 0.45 * (initial.H - 1)];
    const samplePath = "/terrain/sample?lon=-18&lat=9";
    const before = await fetch(
      origin + "/api/sessions/" + initial.id + samplePath,
    ).then((r) => r.json());
    await json("sessions/" + initial.id + "/world", {
      kind: "operation",
      revision: initial.project.revision,
      operation: {
        id: "local-edit",
        kind: "raise",
        layer: "elevation",
        center,
        radius: 2,
        amount: 120,
      },
    });
    const sample = await fetch(
      origin + "/api/sessions/" + initial.id + samplePath,
    ).then((r) => r.json());
    assert.ok(Math.abs(sample.bed - before.bed - 120) < 1e-8);
    const tilePath = "/terrain/tiles/2/3/1?style=natural";
    const tile = Buffer.from(
      await fetch(origin + "/api/sessions/" + initial.id + tilePath).then((r) =>
        r.arrayBuffer(),
      ),
    );
    // Draft work must not enter the database; explicit Save survives a real restart.
    await json(`sessions/${initial.id}/view`, {
      ui: { seedUsed: 123, camera2d: { center: { x: 12, y: 10 }, scale: 200 } },
    });
    assert.equal(
      (await fetch(origin + "/api/projects").then((r) => r.json())).projects
        .length,
      0,
    );
    await json(`sessions/${initial.id}/save`, {});
    const worlds = await fetch(origin + "/api/projects").then((r) => r.json());
    await stop();
    await launch();
    const stored = await json(`projects/${worlds.projects[0].id}/open`, {});
    assert.deepEqual(stored.continuous.spec, initial.continuous.spec);
    assert.deepEqual(
      await fetch(origin + "/api/sessions/" + stored.id + samplePath).then(
        (r) => r.json(),
      ),
      sample,
    );
    assert.equal(stored.projectUI.camera2d.scale, 200);
    assert.deepEqual(
      Buffer.from(
        await fetch(origin + `/api/sessions/${stored.id}` + tilePath).then(
          (r) => r.arrayBuffer(),
        ),
      ),
      tile,
    );
    initial.id = stored.id;
    const saved = await fetch(origin + `/api/sessions/${initial.id}/project`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        ui: {
          seedUsed: 123,
          camera2d: { center: { x: 12, y: 10 }, scale: 200 },
        },
      }),
    });
    assert.equal(saved.status, 200);
    const archive = Buffer.from(await saved.arrayBuffer());
    assert.ok(
      archive.includes(Buffer.from("terrain/continuous.json")),
      "ZIP omitted the continuous terrain source",
    );
    await fetch(origin + `/api/sessions/${initial.id}`, { method: "DELETE" });
    await stop();
    fs.rmSync(databaseDir, { recursive: true, force: true });
    await launch();
    const reopened = await fetch(origin + "/api/projects/import", {
      method: "POST",
      headers: { "Content-Type": "application/zip" },
      body: archive,
    });
    const restored = await reopened.json();
    assert.equal(reopened.status, 201, JSON.stringify(restored));
    assert.deepEqual(restored.continuous.spec, initial.continuous.spec);
    assert.deepEqual(
      await fetch(origin + "/api/sessions/" + restored.id + samplePath).then(
        (r) => r.json(),
      ),
      sample,
    );
    assert.equal(restored.projectUI.camera2d.scale, 200);
    assert.equal(restored.storedDetailCount, 0);
    const restoredTile = Buffer.from(
      await fetch(origin + `/api/sessions/${restored.id}` + tilePath).then(
        (r) => r.arrayBuffer(),
      ),
    );
    assert.deepEqual(restoredTile, tile);
    const missing = await fetch(
      origin + `/api/sessions/${restored.id}/terrain/tiles/3/2/2`,
    );
    assert.equal(missing.status, 200);
    assert.equal(
      (await fetch(origin + "/api/projects").then((r) => r.json())).projects
        .length,
      0,
      "Import/exploration must stay draft-only",
    );
    await fetch(origin + `/api/sessions/${restored.id}`, { method: "DELETE" });
  } finally {
    await stop();
    fs.rmSync(databaseDir, { recursive: true, force: true });
  }
});
