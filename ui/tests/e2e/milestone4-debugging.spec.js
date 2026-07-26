import { expect, test } from '@playwright/test';

async function createWorkflow(page, data) {
  const res = await page.request.post('/api/v1/workflows', { data });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

async function waitForLatestExecution(page, workflowId, terminal = true) {
  for (let i = 0; i < 40; i += 1) {
    const res = await page.request.get(`/api/v1/workflows/${workflowId}/executions`);
    expect(res.ok()).toBeTruthy();
    const executions = await res.json();
    if (executions[0] && (!terminal || !['RUNNING', 'QUEUED'].includes(executions[0].status))) return executions[0];
    await page.waitForTimeout(250);
  }
  throw new Error('Execution did not reach expected state');
}

test('Milestone 4 editor selector overlays failed path and replays selected execution', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Failed Path',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'Source JSON',
        position: { x: 180, y: 120 },
        params: { json_template: '{"access_token":"secret-token","ok":true}' },
      },
      {
        id: 'bad_json',
        type: 'jsonTransform',
        name: 'Bad JSON',
        position: { x: 520, y: 120 },
        params: { json_template: '{bad json' },
      },
    ]),
    edges_json: JSON.stringify([{ id: 'e1', source: 'json_1', target: 'bad_json' }]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { access_token: 'secret-token', source: 'm4' } });
  const firstExecution = await waitForLatestExecution(page, workflow.id);
  expect(firstExecution.status).toBe('FAILED');

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText('FAILED');
  await expect(page.locator('.custom-node-card.status-failed')).toBeVisible();
  await expect(page.locator('.execution-edge-failed')).toHaveCount(1);

  await page.locator('.node-body-title', { hasText: 'Bad JSON' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' }).click();
  await expect(page.getByText('Open Parameters')).toBeVisible();
  await page.getByRole('button', { name: 'Open Parameters' }).click();
  await expect(page.locator('.panel-tabs').getByRole('tab', { name: 'Parameters' })).toHaveAttribute('aria-selected', 'true');

  await page.getByRole('button', { name: 'Copy debug bundle' }).click();
  await expect(page.locator('.debug-bundle-preview')).toContainText('"selected_execution"');
  await expect(page.locator('.debug-bundle-preview')).not.toContainText('secret-token');

  await page.getByRole('button', { name: 'Replay execution' }).click();
  const replayed = await waitForLatestExecution(page, workflow.id, false);
  expect(replayed.id).not.toBe(firstExecution.id);
});

test('Milestone 4 editor can cancel a selected running execution', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Cancel',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'delay_1', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 200, y: 120 }, params: { seconds: '5' } },
    ]),
    edges_json: '[]',
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger?async=true`, { data: { source: 'm4-cancel' } });
  const running = await waitForLatestExecution(page, workflow.id, false);
  expect(['RUNNING', 'QUEUED']).toContain(running.status);

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText(/RUNNING|QUEUED/);
  await page.getByRole('button', { name: 'Cancel execution' }).click();

  for (let i = 0; i < 20; i += 1) {
    const exec = await (await page.request.get(`/api/v1/executions/${running.id}`)).json();
    if (['CANCEL_REQUESTED', 'CANCELLED', 'INTERRUPTED'].includes(exec.status)) return;
    await page.waitForTimeout(250);
  }
  throw new Error('Execution was not cancelled');
});
