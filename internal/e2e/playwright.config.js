import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  workers: 1, // single binary on single port
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:7739',
    viewport: { width: 1440, height: 900 },
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  webServer: {
    command: 'go run ../../cmd/copilot-monitor serve --db ../../testdata.db --addr 127.0.0.1:7739',
    url: 'http://127.0.0.1:7739/api/health',
    timeout: 30_000,
    reuseExistingServer: true,
    stdout: 'ignore',
    stderr: 'pipe',
  },
});
