import { expect, test } from '@playwright/test';

async function createWorkflow(page, data) {
  const res = await page.request.post('/api/v1/workflows', { data });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

async function latestExecutionFor(page, workflowId) {
  const res = await page.request.get(`/api/v1/workflows/${workflowId}/executions`);
  expect(res.ok()).toBeTruthy();
  const executions = await res.json();
  expect(executions.length).toBeGreaterThan(0);
  return executions[0];
}

test('Test Workflow saves dirty graph before triggering backend execution', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX Dirty Run',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'delay_saved', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 120, y: 120 }, params: { seconds: '1' } },
    ]),
    edges_json: '[]',
  });

  await page.goto(`/workflows/${workflow.id}`);
  await page.getByText('Delay / Sleep').click();
  await page.getByLabel('Delay Duration (Seconds)').fill('2');
  await expect(page.getByText('Unsaved changes')).toBeVisible();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);

  const saved = await (await page.request.get(`/api/v1/workflows/${workflow.id}`)).json();
  const savedNodes = JSON.parse(saved.nodes_json);
  expect(savedNodes[0].params.seconds).toBe('2');

  const execution = await latestExecutionFor(page, workflow.id);
  const logs = JSON.parse(execution.logs_json || '[]');
  expect(logs.some((log) => log.node_id === 'delay_saved')).toBeTruthy();
});

test('Save allows incomplete draft, reload keeps it, Test blocks until configured', async ({ page }) => {
  await page.goto('/workflows');
  await page.getByLabel('Workflow name').fill('UX Draft Save');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();
  await expect(page).toHaveURL(/\/workflows\/.+/);
  const workflowId = page.url().split('/').pop();

  await page.getByRole('button', { name: 'Add first step' }).click();
  await page.getByLabel('Search nodes').fill('IF');
  await page.getByRole('option', { name: /IF \/ ELSE Condition/ }).first().click();
  await expect(page.getByText('Missing required fields')).toBeVisible();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();

  await page.reload();
  await expect(page.getByText('IF / ELSE Condition', { exact: true })).toBeVisible();
  await page.getByLabel('Inactive').click();
  await expect(page.getByText('Fix workflow validation issues before activating.')).toBeVisible();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page.getByText('Fix workflow validation issues before testing.')).toBeVisible();

  await page.getByLabel('Input Value').fill('ok');
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.getByLabel('Inactive').check();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);

  const execution = await latestExecutionFor(page, workflowId);
  expect(execution.status).toMatch(/SUCCESS|RUNNING/);
});

test('Node picker star is independent and dialog traps/restores focus', async ({ page }) => {
  await page.goto('/workflows');
  await page.getByLabel('Workflow name').fill('UX Picker Closure');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();

  const opener = page.getByRole('button', { name: 'Add first step' });
  await opener.click();
  await expect(page.getByLabel('Search nodes')).toBeFocused();
  await page.getByLabel('Search nodes').fill('HTTP');

  await page.getByRole('button', { name: 'Add HTTP Request to favorites' }).press('Enter');
  await expect(page.getByRole('dialog', { name: 'Add step' })).toBeVisible();
  await expect(page.locator('.node-body-title')).toHaveCount(0);

  await page.getByRole('button', { name: 'Close node picker' }).focus();
  await page.keyboard.press('Shift+Tab');
  await expect(page.getByRole('button', { name: 'Remove HTTP Request from favorites' })).toBeFocused();
  await page.keyboard.press('Tab');
  await expect(page.getByRole('button', { name: 'Close node picker' })).toBeFocused();

  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: 'Add step' })).toBeHidden();
  await expect(opener).toBeFocused();
});
