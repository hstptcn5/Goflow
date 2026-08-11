import { expect, test } from '@playwright/test';

const sourceURL = process.env.GOFLOW_DAILYOPS_SOURCE_URL;
const chatID = process.env.GOFLOW_DAILYOPS_CHAT_ID || '@dailyops_e2e';
const tokenParts = (process.env.GOFLOW_DAILYOPS_TOKEN_PARTS || '').split('|');
const phase = process.env.GOFLOW_DAILYOPS_PHASE || 'setup';
const hasHarness = Boolean(sourceURL && process.env.GOFLOW_DAILYOPS_TOKEN_PARTS);

function fakeToken() {
  if (tokenParts.length !== 2 || !tokenParts[0] || !tokenParts[1]) {
    throw new Error('GOFLOW_DAILYOPS_TOKEN_PARTS must contain two token segments');
  }
  return `${tokenParts[0]}:${tokenParts[1]}`;
}

async function expectNoSecret(page) {
  await expect(page.locator('body')).not.toContainText(tokenParts[1]);
}

async function executionSnapshot(page) {
  const response = await page.request.get('/api/appliance/executions?limit=10');
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  return body.executions || [];
}

async function waitForNewSuccess(page, previousExecutions) {
  const previousIDs = new Set(previousExecutions.map((execution) => execution.id));
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const executions = await executionSnapshot(page);
    const succeeded = executions.find((execution) => !previousIDs.has(execution.id) && execution.status === 'SUCCESS');
    if (succeeded) {
      await page.reload();
      await expect(page.getByRole('heading', { name: 'Managed workflow' })).toBeVisible();
      await expect(page.getByLabel('Latest execution').getByText('SUCCESS')).toBeVisible();
      return succeeded;
    }
    await page.waitForTimeout(500);
  }
  throw new Error('A new successful DailyOps execution was not recorded');
}

test('DailyOps appliance completes real setup and execution with mocked Telegram', async ({ page }) => {
  test.skip(!hasHarness, 'DailyOps appliance harness is required');
  test.skip(phase !== 'setup', 'setup phase only');

  await page.goto('/workflows');
  await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram' })).toBeVisible();
  await expect(page.getByText('Unsigned pack integrity')).toBeVisible();

  await page.getByLabel('Source URL').fill(sourceURL);
  await page.getByLabel('Telegram chat ID').fill(chatID);
  await page.getByRole('button', { name: 'Save configuration' }).click();
  await expect(page.getByText('Configuration saved')).toBeVisible();

  await page.locator('input[type="password"]').fill(fakeToken());
  await page.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByText('Credential saved')).toBeVisible();
  await expectNoSecret(page);

  await page.getByRole('button', { name: 'Test' }).click();
  await expect(page.getByText('OK', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Complete setup' }).click();

  await expect(page.getByRole('heading', { name: 'Managed workflow' })).toBeVisible();
  const previousExecutions = await executionSnapshot(page);
  await page.getByRole('button', { name: 'Run now' }).click();
  await waitForNewSuccess(page, previousExecutions);
  await expect(page.getByLabel('Recent executions').getByText('SUCCESS').first()).toBeVisible();
  await page.getByRole('button', { name: 'Refresh' }).click();
  await expect(page.getByText('"secrets_hidden": true')).toBeVisible();
  await expect(page.getByText('"credential_ids_hidden": true')).toBeVisible();
  await expectNoSecret(page);
});

test('DailyOps appliance restart preserves setup without exposing decrypted secrets', async ({ page }) => {
  test.skip(!hasHarness, 'DailyOps appliance harness is required');
  test.skip(phase !== 'persist', 'persistence phase only');

  await page.goto('/workflows');
  await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Managed workflow' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Reconfigure' })).toBeVisible();
  await expect(page.getByLabel('Latest execution').getByText('SUCCESS')).toBeVisible();
  await expect(page.getByLabel('Recent executions').getByText('SUCCESS').first()).toBeVisible();

  const status = await page.request.get('/api/appliance/status');
  expect(status.ok()).toBeTruthy();
  const statusBody = await status.json();
  expect(statusBody.state).toBe('READY');
  expect(statusBody.setup_complete).toBe(true);

  const setup = await page.request.get('/api/appliance/setup');
  expect(setup.ok()).toBeTruthy();
  const body = await setup.json();
  expect(body.setup_complete).toBe(true);
  expect(body.decrypted_values_returned).toBe(false);
  expect(body.credential_requirements.find((requirement) => requirement.key === 'telegram')?.assigned).toBe(true);
  expect(JSON.stringify(body)).not.toContain(tokenParts[1]);
  await expect(page.locator('input[type="password"]')).toHaveCount(0);

  const previousExecutions = await executionSnapshot(page);
  await page.getByRole('button', { name: 'Run now' }).click();
  await waitForNewSuccess(page, previousExecutions);
  await expect(page.getByLabel('Recent executions').getByText('SUCCESS')).toHaveCount(previousExecutions.length + 1);
  await expectNoSecret(page);
});
