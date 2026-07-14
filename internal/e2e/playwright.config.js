import { defineConfig, devices } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..', '..');
const DB_PATH = path.join(ROOT, 'testdata.db');
const API_PORT = 7739;
const WEB_PORT = 5180;

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  workers: 1, // single binary on single port
  reporter: 'list',
  globalSetup: './setup/global-setup.js',
  use: {
    baseURL: `http://127.0.0.1:${WEB_PORT}`,
    viewport: { width: 1440, height: 900 },
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      // Go API server (backend)
      command: `go run ../../cmd/copilot-monitor serve --db ${DB_PATH} --addr 127.0.0.1:${API_PORT}`,
      url: `http://127.0.0.1:${API_PORT}/api/health`,
      timeout: 30_000,
      reuseExistingServer: false,
      stdout: 'pipe',
      stderr: 'pipe',
    },
    {
      // Vite dev server (dashboard with API proxy)
      command: `DASHBOARD_DEV_PORT=${WEB_PORT} DASHBOARD_API_TARGET=http://127.0.0.1:${API_PORT} pnpm --dir ../../dashboard dev`,
      url: `http://127.0.0.1:${WEB_PORT}`,
      timeout: 30_000,
      reuseExistingServer: false,
      stdout: 'pipe',
      stderr: 'pipe',
    },
  ],
});
