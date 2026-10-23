import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: [
    "ui.browser.spec.mjs",
    "terrain.browser.spec.mjs",
    "terrain-save.browser.spec.mjs",
  ],
  workers: 1,
  timeout: 90000,
  outputDir: ".tools/ui-browser-results",
  reporter: "list",
  use: {
    baseURL: process.env.KRIEMHILD_UI_URL || "http://127.0.0.1:8124",
    channel: process.env.KRIEMHILD_UI_BROWSER || "chrome",
    headless: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
