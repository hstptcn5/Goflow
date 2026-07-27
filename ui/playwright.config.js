import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 45_000,
  workers: 1,
  expect: {
    timeout: 10_000,
  },
  use: {
    baseURL: process.env.GOFLOW_E2E_BASE_URL || 'http://127.0.0.1:18081',
    trace: 'retain-on-failure',
    viewport: { width: 1366, height: 768 },
    deviceScaleFactor: 1,
    colorScheme: 'light',
    extraHTTPHeaders: process.env.GOFLOW_E2E_API_KEY
      ? { Authorization: `Bearer ${process.env.GOFLOW_E2E_API_KEY}` }
      : {},
  },
  webServer: process.env.GOFLOW_E2E_NO_WEBSERVER === '1' ? undefined : {
    command: 'node tests/start-goflow-e2e.mjs',
    url: 'http://127.0.0.1:18081/workflows',
    timeout: 60_000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGKILL', timeout: 1000 },
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
