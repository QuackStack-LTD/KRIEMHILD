import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
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
function run(name, body, code, mode = "corpus-run") {
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
      "-corpus-definition",
      join(root, "sequence.json"),
      "-corpus-definition",
      join(root, "sequence-normalized.json"),
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
  console.log(JSON.stringify({ results }, null, 2));
} finally {
  assert.ok(!relative(resolve(".cache/languages"), temporary).startsWith(".."));
  rmSync(temporary, { recursive: true, force: true });
}
