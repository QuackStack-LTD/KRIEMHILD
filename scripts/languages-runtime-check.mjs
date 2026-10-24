// Runs inside a disposable Node container sharing the test app's network.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
const base = "http://127.0.0.1:8124",
  phase = process.argv[2],
  existing = process.argv[3];
const json = async (name) =>
  JSON.parse(await readFile(`/fixtures/${name}`, "utf8"));
const me = await (await fetch(base + "/api/me")).json();
async function api(path, body) {
  const r = await fetch(base + path, {
    method: body === undefined ? "GET" : "POST",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": me.csrfToken,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await r.json();
  assert.ok(r.ok, `${path}: ${r.status} ${data.error || ""}`);
  return data;
}
assert.match(await (await fetch(base + "/")).text(), /id="root"/);
const definition = await json("reference.json"),
  script = await json("script.json"),
  productive = await json("productive.json");
let doc;
if (phase === "create") {
  doc = await api("/api/worlds", {
    name: "Disposable Languages container test",
  });
  for (const [kind, body] of [
    ["script", script],
    ["language", definition],
  ])
    doc = await api(`/api/worlds/${doc.world.id}/languages`, {
      ageId: doc.world.currentAge,
      revision: doc.world.revision,
      kind,
      definition: body,
      metadata: {},
      note: "Container fixture",
    });
  const path = `/api/worlds/${doc.world.id}/languages/${definition.languageId}`;
  const compiled = await api(path + "/compile", {
    ageId: doc.world.currentAge,
    revision: doc.world.revision,
    definitionRevision: doc.entities.find((e) => e.id === definition.languageId)
      .language.draftRevision,
    request: {},
  });
  doc = compiled.document;
  assert.equal(compiled.report.ready.parse, true);
  doc = await api(path + "/activate", {
    ageId: doc.world.currentAge,
    revision: doc.world.revision,
    definitionRevision: compiled.report.revision,
  });
  doc = await api(`/api/worlds/${doc.world.id}/languages`, {
    ageId: doc.world.currentAge,
    revision: doc.world.revision,
    kind: "language",
    definition: productive,
    metadata: {},
    note: "Productive A1 fixture",
  });
  const productivePath = `/api/worlds/${doc.world.id}/languages/${productive.languageId}`;
  const productiveCompile = await api(productivePath + "/compile", {
    ageId: doc.world.currentAge,
    revision: doc.world.revision,
    definitionRevision: (await json("productive-inflect.request.json"))
      .revision,
  });
  assert.equal(productiveCompile.report.compiler, "languages-compiler/2");
  assert.equal(productiveCompile.report.forms, 0);
  doc = await api(productivePath + "/activate", {
    ageId: doc.world.currentAge,
    revision: productiveCompile.document.world.revision,
    definitionRevision: productiveCompile.report.revision,
  });
} else {
  assert.equal(phase, "restart");
  doc = await api(`/api/worlds/${existing}`);
  assert.ok(
    doc.entities.find((e) => e.id === definition.languageId).language
      .activeRevision,
  );
}
const world = doc.world.id,
  age = doc.world.currentAge,
  path = `/api/worlds/${world}/languages/${definition.languageId}`;
for (const name of ["inflect", "seeded", "parse", "corpus"]) {
  const request = await json(`${name}.request.json`),
    expected = await json(`${name}.golden.json`);
  const actual = await api(path + "/run", {
    ageId: age,
    revision: doc.world.revision,
    definitionRevision: request.revision,
    request,
  });
  assert.deepEqual(
    actual.result,
    expected,
    `${name} result differs in real Linux runtime`,
  );
}
for (const name of ["productive-inflect", "productive-inverse"]) {
  const request = await json(`${name}.request.json`);
  const actual = await api(
    `/api/worlds/${world}/languages/${productive.languageId}/run`,
    {
      ageId: age,
      revision: doc.world.revision,
      definitionRevision: request.revision,
      request,
    },
  );
  assert.deepEqual(actual.result, await json(`${name}.golden.json`));
  assert.deepEqual(actual.context, {
    worldId: world,
    ageId: age,
    languageId: productive.languageId,
  });
}
const latest = await api(`/api/worlds/${world}`);
assert.equal(
  latest.world.revision,
  doc.world.revision,
  "Read/execute mutated the World",
);
const stored = await api(path + `?age=${age}`);
assert.deepEqual(stored, definition);
if (phase === "create") {
  const target = await json("descendant.json");
  target.history.sourceAge = age;
  doc = await api(`/api/worlds/${world}/languages`, {
    ageId: age,
    revision: doc.world.revision,
    kind: "language",
    definition: target,
    metadata: {},
    note: "Historical target",
  });
  const targetPath = `/api/worlds/${world}/languages/${target.languageId}`,
    targetHash = doc.entities.find((e) => e.id === target.languageId).language
      .draftRevision;
  const preview = await api(targetPath + "/history-preview", {
    ageId: age,
    revision: doc.world.revision,
    definitionRevision: targetHash,
    request: {},
  });
  assert.equal(preview.preview.status, "success");
  assert.equal(preview.preview.changes.length, 4);
  const applied = await api(targetPath + "/history-apply", {
    ageId: age,
    revision: doc.world.revision,
    definitionRevision: targetHash,
    previewHash: preview.preview.previewHash,
    approved: true,
    request: {},
  });
  doc = applied.document;
  assert.equal(
    doc.entities.find((e) => e.id === definition.languageId).language.revisions
      .length,
    1,
    "Source changed",
  );
}
const zip = await fetch(base + `/api/worlds/${world}/export`);
assert.ok(zip.ok);
const bytes = new Uint8Array(await zip.arrayBuffer());
assert.equal(bytes[0], 0x50);
assert.equal(bytes[1], 0x4b);
const importedResponse = await fetch(base + "/api/worlds/import", {
  method: "POST",
  headers: { "Content-Type": "application/zip", "X-CSRF-Token": me.csrfToken },
  body: bytes,
});
assert.ok(
  importedResponse.ok,
  `World ZIP import failed: ${importedResponse.status}`,
);
const imported = await importedResponse.json();
assert.notEqual(imported.world.id, world);
const importedDefinition = await api(
  `/api/worlds/${imported.world.id}/languages/${definition.languageId}?age=${age}`,
);
assert.deepEqual(importedDefinition, definition);
const replay = await api(
  `/api/worlds/${imported.world.id}/languages/${definition.languageId}/run`,
  {
    ageId: age,
    revision: imported.world.revision,
    definitionRevision: (await json("inflect.request.json")).revision,
    request: await json("inflect.request.json"),
  },
);
assert.deepEqual(replay.result, await json("inflect.golden.json"));
for (const name of ["productive-inflect", "productive-inverse"]) {
  const request = await json(`${name}.request.json`);
  const actual = await api(
    `/api/worlds/${imported.world.id}/languages/${productive.languageId}/run`,
    {
      ageId: age,
      revision: imported.world.revision,
      definitionRevision: request.revision,
      request,
    },
  );
  assert.deepEqual(actual.result, await json(`${name}.golden.json`));
}
assert.ok(
  (
    await fetch(base + `/api/worlds/${imported.world.id}`, {
      method: "DELETE",
      headers: { "X-CSRF-Token": me.csrfToken },
    })
  ).ok,
);
console.log(
  JSON.stringify({
    phase,
    world,
    age,
    revision: doc.world.revision,
    archiveBytes: bytes.length,
    checks: [
      "frontend",
      "independent-script",
      "compile",
      "inflect",
      "seeded",
      "parse",
      "corpus",
      "immutable-source",
      "portable-zip",
      "portable-import-replay",
      "productive-circumfix",
      "productive-inverse",
      "typed-traces",
      "schema-3-world-zip",
      phase === "create" ? "historical-apply" : "restart-persistence",
    ],
  }),
);
