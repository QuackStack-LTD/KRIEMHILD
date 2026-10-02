import { defineConfig } from "@playwright/test";
import path from "node:path";
const root = path.resolve(__dirname, "../..");
const binary = path.join(
  root,
  "bin",
  process.platform === "win32" ? "kriemhild-dev.exe" : "kriemhild-dev",
);
export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  timeout: 60000,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI
    ? [
        ["github"],
        ["list"],
        ["html", { open: "never" }],
        ["junit", { outputFile: "test-results/junit.xml" }],
      ]
    : [["list"]],
  use: {
    baseURL: "http://127.0.0.1:4781",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: [
    {
      command: `"${binary}" -addr 127.0.0.1:4781 -data .test-worlds-dev -web apps/web/out-dev`,
      cwd: root,
      url: "http://127.0.0.1:4781",
      reuseExistingServer: false,
      timeout: 30000,
    },
    {
      command: `"${binary}" -addr 127.0.0.1:4783 -origin http://127.0.0.1:4783 -data .test-hosted-dev -web apps/web/out-dev`,
      cwd: root,
      env: { KRIEMHILD_ADMIN_PASSWORD: "browser-test-admin-password" },
      url: "http://127.0.0.1:4783",
      reuseExistingServer: false,
      timeout: 30000,
    },
  ],
});
