import test from "node:test";
import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { resolve, join, sep } from "node:path";
import { spawnSync } from "node:child_process";

const bash =
  process.platform === "win32" &&
  existsSync("C:/Program Files/Git/bin/bash.exe")
    ? "C:/Program Files/Git/bin/bash.exe"
    : "bash";
const revision = "0123456789abcdef0123456789abcdef01234567";
const available =
  spawnSync(bash, ["--version"], { windowsHide: true }).status === 0;
async function run(t, script, body, env = {}) {
  await mkdir(resolve(".tools"), { recursive: true });
  const dir = await mkdtemp(resolve(".tools/ci-scripts-")),
    log = join(dir, "commands");
  t.after(async () => {
    assert.ok(dir.startsWith(resolve(".tools") + sep));
    await rm(dir, { recursive: true, force: true });
  });
  const result = spawnSync(
    bash,
    ["-c", `source scripts/ci/${script}; ${body}`],
    {
      cwd: process.cwd(),
      windowsHide: true,
      encoding: "utf8",
      env: {
        ...process.env,
        GITHUB_SHA: revision,
        GITHUB_REPOSITORY: "QuackStack-LTD/KRIEMHILD",
        GITHUB_REF: "refs/heads/main",
        GITHUB_EVENT_NAME: "push",
        CI_TEST_LOG: log.replaceAll("\\", "/"),
        KRIEMHILD_BUILDX_CACHE: "",
        ...env,
      },
    },
  );
  const commands = await readFile(log, "utf8").catch(() => "");
  return { ...result, commands };
}
const operations = `
check_image() { printf 'check:%s\\n' "$1" >> "$CI_TEST_LOG"; [[ \${FAIL_CHECK:-} != 1 ]]; }
publish_image() { printf 'publish:%s:%s\\n' "$1" "$2" >> "$CI_TEST_LOG"; }
release_image() { printf 'release:%s:%s\\n' "$1" "$2" >> "$CI_TEST_LOG"; }
main`;

test(
  "pull requests only check images, including when a token is present",
  { skip: !available },
  async (t) => {
    const r = await run(t, "pull-request.sh", operations, {
      GH_TOKEN: "test-only",
      GITHUB_EVENT_NAME: "pull_request",
      GITHUB_REF: "refs/pull/1/merge",
    });
    assert.equal(r.status, 0, r.stderr);
    assert.equal(
      r.commands,
      `check:kriemhild:check-${revision.slice(0, 12)}\n`,
    );
  },
);
test(
  "main publishes and releases only after a successful image check",
  { skip: !available },
  async (t) => {
    const r = await run(t, "build.sh", operations);
    assert.equal(r.status, 0, r.stderr);
    assert.deepEqual(r.commands.trim().split("\n"), [
      `check:kriemhild:check-${revision.slice(0, 12)}`,
      `publish:ghcr.io/quackstack-ltd/kriemhild:${revision}`,
      `release:ghcr.io/quackstack-ltd/kriemhild:${revision}`,
    ]);
    const failed = await run(t, "build.sh", operations, { FAIL_CHECK: "1" });
    assert.notEqual(failed.status, 0);
    assert.ok(
      !failed.commands.includes("publish:") &&
        !failed.commands.includes("release:"),
    );
  },
);
test(
  "a branch or pull request cannot invoke the main publishing entry point",
  { skip: !available },
  async (t) => {
    const r = await run(t, "build.sh", operations, {
      GITHUB_EVENT_NAME: "pull_request",
      GITHUB_REF: "refs/pull/1/merge",
    });
    assert.notEqual(r.status, 0);
    assert.equal(r.commands, "");
  },
);
test(
  "Buildx checks load one architecture and publishing pushes the multi-platform image",
  { skip: !available },
  async (t) => {
    for (const mode of ["check", "publish"]) {
      const r = await run(
        t,
        "image.sh",
        `docker() { printf '%s\\0' "$@" >> "$CI_TEST_LOG"; }; build_image ${mode} /tmp/metadata kriemhild:test`,
      );
      assert.equal(r.status, 0, r.stderr);
      const args = r.commands.split("\0");
      assert.equal(
        args[args.indexOf("--platform") + 1],
        mode === "check" ? "linux/amd64" : "linux/amd64,linux/arm64",
      );
      assert.equal(args.includes("--push"), mode === "publish");
      assert.equal(args.includes("--load"), mode === "check");
      assert.ok(args.includes(`org.opencontainers.image.revision=${revision}`));
    }
  },
);
