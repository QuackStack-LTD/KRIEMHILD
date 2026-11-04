import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdtempSync,
  readFileSync,
  writeFileSync,
  rmSync,
  readdirSync,
} from "node:fs";
import { resolve, join, relative } from "node:path";

const executable = resolve(
  process.argv[2] || ".cache/languages/brief5/cli.exe",
);
const root = resolve("internal/languages/testdata");
const temporary = mkdtempSync(resolve(".cache/languages/corpus-cli-"));
const corpus = JSON.parse(
  readFileSync(join(root, "sequence.corpus.json"), "utf8"),
);
const results = [];
function run(
  name,
  body,
  code,
  mode = "corpus-run",
  definitions = ["sequence.json", "sequence-normalized.json"],
) {
  const file = join(temporary, name + ".json");
  writeFileSync(
    file,
    typeof body === "string" || Buffer.isBuffer(body)
      ? body
      : JSON.stringify(body),
  );
  const r = spawnSync(
    executable,
    [
      "-operation",
      mode,
      "-corpus",
      file,
      ...definitions.flatMap((name) => [
        "-corpus-definition",
        join(root, name),
      ]),
      "-script",
      join(root, "sequence-script.json"),
    ],
    { encoding: "utf8", timeout: 15000 },
  );
  assert.equal(r.status, code, `${name}: ${r.stderr}`);
  const report = JSON.parse(r.stdout);
  results.push({
    name,
    exit: r.status,
    status: report.status,
    calls: report.calls,
  });
  return { report, summary: r.stderr };
}
try {
  assert.equal(run("reviewed", corpus, 0).report.status, "success");
  assert.equal(run("validate", corpus, 0, "corpus-validate").report.calls, 0);
  let c = structuredClone(corpus);
  c.cases[0].expected.spelling.steps++;
  assert.match(run("mismatch", c, 1).summary, /result.spelling.steps/);
  c = structuredClone(corpus);
  c.bounds.work = 100000;
  assert.equal(run("budget", c, 3).report.status, "limit_reached");
  c = structuredClone(corpus);
  c.cases[0].review.status = "pending";
  assert.equal(
    run("pending-review", c, 1).report.cases[0].status,
    "review_required",
  );
  c = structuredClone(corpus);
  c.cases[0].pin.revision = "a".repeat(64);
  assert.equal(run("contradictory-pin", c, 2).report.calls, 0);
  run("unknown-field", { ...corpus, unknown: true }, 2);
  run(
    "duplicate-field",
    JSON.stringify(corpus).replace('"version":', '"version":"bad","version":'),
    2,
  );
  run("invalid-utf8", Buffer.from([0x22, 0xff, 0x22]), 2);
  run("oversized", Buffer.alloc((2 << 20) + 1, 0x20), 2);
  const coverage = join(root, "sequence-corpus");
  const definitions = [
    "sequence.json",
    "sequence-normalized.json",
    ...[
      "contexts",
      "priority",
      "topology",
      "nfd",
      "deletion",
      "inverse-disabled",
    ].map((name) => `sequence-corpus/${name}.definition.json`),
  ];
  for (const file of readdirSync(coverage)
    .filter((name) => name.endsWith(".corpus.json"))
    .sort()) {
    const c = JSON.parse(readFileSync(join(coverage, file), "utf8"));
    const defs =
      c.cases[0].phase === "definition-validation"
        ? [`sequence-corpus/${c.cases[0].id}.definition.json`]
        : file.startsWith("topology-missing-")
          ? ["sequence-corpus/topology-missing.definition.json"]
          : definitions;
    assert.equal(
      run(file + "-validate", c, 0, "corpus-validate", defs).report.calls,
      0,
    );
    const report = run(file, c, 0, "corpus-run", defs).report;
    assert.equal(report.cases[0].passed, true);
    assert.deepEqual(
      report.cases[0].validation ?? report.cases[0].actual,
      c.cases[0].validation ?? c.cases[0].expected,
    );
    if (c.version === "sequence-corpus/2") {
      const old = structuredClone(c);
      old.version = "sequence-corpus/1";
      run(file + "-downgraded", old, 2, "corpus-run", defs);
    }
  }
  c = JSON.parse(
    readFileSync(join(coverage, "invalid-layer.corpus.json"), "utf8"),
  );
  c.cases[0].review.status = "pending";
  assert.equal(
    run("pending-definition-validation", c, 1, "corpus-run", [
      "sequence-corpus/invalid-layer.definition.json",
    ]).report.cases[0].status,
    "review_required",
  );
  console.log(JSON.stringify({ results }, null, 2));
} finally {
  assert.ok(!relative(resolve(".cache/languages"), temporary).startsWith(".."));
  rmSync(temporary, { recursive: true, force: true });
}
