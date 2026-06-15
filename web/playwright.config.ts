import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: /.*\.spec\.ts/,
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:3007",
    trace: "on-first-retry",
  },
  webServer: {
    command: "npm run dev -- --port 3007",
    port: 3007,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
