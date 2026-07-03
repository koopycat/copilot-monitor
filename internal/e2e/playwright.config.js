import { defineConfig, devices } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..', '..');
const DB_PATH = path.join(ROOT, 'testdata.db');

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  workers: 1, // single binary on single port
  reporter: 'list',
  globalSetup: './setup/global-setup.js',
  use: {
    baseURL: 'http://127.0.0.1:7739',
    viewport: { width: 1440, height: 900 },
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: `go run ../../cmd/copilot-monitor serve --db ${DB_PATH} --addr 127.0.0.1:7739`,
    url: 'http://127.0.0.1:7739/api/health',
    timeout: 30_000,
    reuseExistingServer: false, // always start fresh against the seeded DB
    stdout: 'ignore',
    stderr: 'pipe',
  },
});
