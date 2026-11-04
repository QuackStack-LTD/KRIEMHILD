// Temporary named containers/volumes only. No production ports, data or pushes.
import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { resolve } from "node:path";
import { writeFile } from "node:fs/promises";
const prefix = `kriemhild-languages-${randomBytes(5).toString("hex")}`,
  network = prefix + "-net",
  app = prefix + "-app",
  pg = prefix + "-pg",
  image = process.argv[2] || "kriemhild-languages-runtime:integration",
  volumes = [prefix + "-sqlite", prefix + "-pg-cache"];
const docker = (...args) =>
  execFileSync("docker", args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  }).trim();
const reports = [];
async function start(volume, dsn) {
  docker(
    "run",
    "-d",
    "--name",
    app,
    "--network",
    network,
    "--read-only",
    "--tmpfs",
    "/tmp:rw,size=64m",
    "--cap-drop",
    "ALL",
    "--security-opt",
    "no-new-privileges:true",
    "--memory",
    "256m",
    "-v",
    `${volume}:/data`,
    ...(dsn ? ["-e", `DATABASE_URL=${dsn}`] : []),
    image,
  );
  for (let i = 0; i < 60; i++) {
    try {
      docker("exec", app, "/app/kriemhild", "-healthcheck");
      return;
    } catch {}
    await new Promise((r) => setTimeout(r, 250));
  }
  throw Error("Runtime readiness failed");
}
function check(phase, world = "") {
  const output = docker(
    "run",
    "--rm",
    "--network",
    `container:${app}`,
    "--memory",
    "128m",
    "-v",
    `${resolve("scripts/languages-runtime-check.mjs")}:/checks.mjs:ro`,
    "-v",
    `${resolve("internal/languages/testdata")}:/fixtures:ro`,
    "node:24-alpine",
    "node",
    "/checks.mjs",
    phase,
    ...(world ? [world] : []),
  );
  return JSON.parse(output);
}
function stop() {
  docker("stop", "-t", "30", app);
  docker("rm", app);
}
try {
  docker("network", "create", network);
  docker(
    "run",
    "-d",
    "--name",
    pg,
    "--network",
    network,
    "--network-alias",
    "postgres",
    "--memory",
    "256m",
    "-e",
    "POSTGRES_USER=kriemhild",
    "-e",
    "POSTGRES_DB=kriemhild",
    "-e",
    "POSTGRES_PASSWORD=languages-smoke-only",
    "postgres:17-alpine",
  );
  let ready = false;
  for (let i = 0; i < 60; i++) {
    try {
      docker("exec", pg, "pg_isready", "-U", "kriemhild", "-d", "kriemhild");
      ready = true;
      break;
    } catch {}
    await new Promise((r) => setTimeout(r, 250));
  }
  if (!ready) throw Error("Temporary PostgreSQL did not start");
  for (const [driver, volume, dsn] of [
    ["sqlite", volumes[0], undefined],
    [
      "postgresql",
      volumes[1],
      "postgres://kriemhild:languages-smoke-only@postgres:5432/kriemhild?sslmode=disable",
    ],
  ]) {
    await start(volume, dsn);
    const created = check("create");
    stop();
    await start(volume, dsn);
    const restored = check("restart", created.world);
    stop();
    reports.push({ driver, created, restored });
  }
  await writeFile(
    ".cache/languages/container-smoke.json",
    JSON.stringify(reports, null, 2) + "\n",
  );
  console.log(JSON.stringify(reports));
} finally {
  for (const name of [app, pg])
    try {
      docker("rm", "-f", name);
    } catch {}
  for (const name of volumes)
    try {
      docker("volume", "rm", name);
    } catch {}
  try {
    docker("network", "rm", network);
  } catch {}
}
