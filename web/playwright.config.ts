import { defineConfig, devices } from "@playwright/test";

const port = 18080;

// E2E against the real Go binary (with the embedded frontend) and a fresh database on every run.
// Locally: E2E_CHANNEL=chrome uses the installed Chrome; in CI the Chromium comes from `playwright install`.
export default defineConfig({
  testDir: "e2e",
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: { baseURL: `http://127.0.0.1:${port}`, channel: process.env.E2E_CHANNEL, trace: "retain-on-failure" },
  // the same flows run on a desktop viewport and on a phone (bottom nav and floating + button instead of the sidebar)
  projects: [
    { name: "desktop" },
    { name: "mobile", use: { ...devices["Pixel 5"] } },
  ],
  webServer: {
    command: "go run ./cmd/gastos",
    cwd: "..",
    url: `http://127.0.0.1:${port}/healthz`,
    reuseExistingServer: false,
    timeout: 120_000,
    env: {
      GASTOS_ADDR: `127.0.0.1:${port}`,
      GASTOS_DB_PATH: "web/e2e-data/gastos.db",
      GASTOS_BACKUP_DIR: "web/e2e-data/backups",
      GASTOS_COOKIE_SECURE: "false",
      GASTOS_INVITE_CODE: "e2e-invite-code",
    },
  },
});
