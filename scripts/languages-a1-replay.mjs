import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { resolve } from "node:path";

// Run from the repository root after building the CLI and Dockerfile.engine.
// Raw UTF-8 bytes, complete provenance and typed traces must match, not just forms.
const image = process.argv[2] || "kriemhild-languages:a1";
const executable = resolve(process.argv[3] || ".cache/languages/a1-cli.exe");
const fixtures = resolve("internal/languages/testdata");
const requests = resolve(".cache/languages/a1-replay-requests");
mkdirSync(requests, { recursive: true });
const seed = JSON.parse(
  readFileSync(`${fixtures}/productive-inverse.request.json`, "utf8"),
);
seed.policy = "seeded";
seed.seed = "authored-choice";
writeFileSync(`${requests}/seeded.json`, JSON.stringify(seed));
const cases = [
  ["legacy-compile", "reference.json", null],
  ...["inflect", "seeded", "parse", "corpus"].map((name) => [
    name,
    "reference.json",
    `${name}.request.json`,
  ]),
  ["productive-compile", "productive.json", null],
  ...["productive-inflect", "productive-inverse"].map((name) => [
    name,
    "productive.json",
    `${name}.request.json`,
  ]),
  ["productive-seeded", "productive.json", "@seeded.json"],
  ["indexed-compile", "indexed.json", null],
  ["indexed-inverse", "indexed.json", "indexed-inverse.request.json"],
];
const results = [];
for (const [name, definition, request] of cases) {
  const args = (root, requestRoot) => [
    "-definition",
    `${root}/${definition}`,
    ...(definition === "reference.json"
      ? ["-script", `${root}/script.json`]
      : []),
    ...(request
      ? [
          "-operation",
          "run",
          "-request",
          request.startsWith("@")
            ? `${requestRoot}/${request.slice(1)}`
            : `${root}/${request}`,
        ]
      : []),
  ];
  let previous;
  for (let repeat = 0; repeat < 2; repeat++) {
    const windows = execFileSync(executable, args(fixtures, requests));
    const linux = execFileSync("docker", [
      "run",
      "--rm",
      "--network",
      "none",
      "-v",
      `${fixtures}:/fixtures:ro`,
      "-v",
      `${requests}:/requests:ro`,
      image,
      ...args("/fixtures", "/requests"),
    ]);
    assert.ok(windows.equals(linux), `${name}: platform bytes differ`);
    if (previous)
      assert.ok(previous.equals(windows), `${name}: process replay differs`);
    if (request && !request.startsWith("@")) {
      assert.deepEqual(
        JSON.parse(windows),
        JSON.parse(readFileSync(`${fixtures}/${name}.golden.json`, "utf8")),
        `${name}: full golden differs`,
      );
    }
    previous = windows;
  }
  results.push({
    name,
    bytes: previous.length,
    sha256: createHash("sha256").update(previous).digest("hex"),
  });
}
mkdirSync(resolve(".cache/languages"), { recursive: true });
writeFileSync(
  resolve(".cache/languages/a1-platform-replay.json"),
  JSON.stringify({ image, repeats: 2, results }, null, 2),
);
console.log(JSON.stringify({ image, repeats: 2, results }, null, 2));
