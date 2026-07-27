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
  const largeBlob = 'x'.repeat(21000);
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Failed Path',
    description: '',
    is_active: true,
    input_schema_json: JSON.stringify({
      type: 'object',
      required: ['source', 'route'],
      properties: {
        source: { type: 'string' },
        route: { type: 'string' },
      },
    }),
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'Source JSON',
        position: { x: 120, y: 170 },
        params: { json_template: JSON.stringify({ access_token: 'secret-token', route: '{{$trigger.route}}', blob: largeBlob }) },
      },
      {
        id: 'if_1',
        type: 'conditionIf',
        name: 'IF / ELSE Condition',
        position: { x: 430, y: 170 },
        params: { field: '{{json_1.transformed.route}}', operator: 'equals', value: 'fail' },
      },
      {
        id: 'bad_json',
        type: 'jsonTransform',
        name: 'Bad JSON',
        position: { x: 760, y: 80 },
        params: { json_template: '{bad json' },
      },
      {
        id: 'success_json',
        type: 'jsonTransform',
        name: 'Success JSON',
        position: { x: 760, y: 220 },
        params: { json_template: '{"ok":true}' },
      },
      {
        id: 'skipped_json',
        type: 'jsonTransform',
        name: 'Skipped JSON',
        position: { x: 760, y: 360 },
        params: { json_template: '{"skipped":true}' },
      },
    ]),
    edges_json: JSON.stringify([
      { id: 'e-json-if', source: 'json_1', target: 'if_1' },
      { id: 'e-if-bad', source: 'if_1', sourceHandle: 'true', target: 'bad_json' },
      { id: 'e-if-success', source: 'if_1', sourceHandle: 'true', target: 'success_json' },
      { id: 'e-if-skipped', source: 'if_1', sourceHandle: 'false', target: 'skipped_json' },
    ]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { route: 'fail', source: 'm4-required-input' } });
  const firstExecution = await waitForLatestExecution(page, workflow.id);
  expect(firstExecution.status).toBe('FAILED');

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText('FAILED');
  await expect(page.locator('.custom-node-card.status-failed')).toBeVisible();
  await expect(page.locator('.execution-edge-failed-path')).toHaveCount(2);
  await expect(page.locator('.execution-edge-success')).toHaveCount(1);
  await expect(page.locator('.execution-edge-skipped')).toHaveCount(1);

  await page.locator('.node-body-title', { hasText: 'Bad JSON' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' }).click();
  await expect(page.getByText('Open Parameters')).toBeVisible();
  await page.getByRole('button', { name: 'Open Parameters' }).click();
  await expect(page.locator('.panel-tabs').getByRole('tab', { name: 'Parameters' })).toHaveAttribute('aria-selected', 'true');

  await page.getByRole('button', { name: 'Copy debug bundle' }).click();
  await expect(page.locator('.debug-bundle-preview pre')).toHaveText(/\{/, { timeout: 5000 });
  const preview = JSON.parse(await page.locator('.debug-bundle-preview pre').textContent());
  expect(preview.truncated).toBe(true);
  expect(preview.preview).toContain('"selected_execution"');
  expect(preview.omitted_sections).toContain('debug_bundle_preview_tail');
  const copiedBundle = await page.evaluate(() => window.__goflowLastDebugBundle);
  expect(() => JSON.parse(copiedBundle)).not.toThrow();
  expect(copiedBundle.length).toBeGreaterThan(20000);
  await expect(page.locator('.debug-bundle-preview')).not.toContainText('secret-token');
  expect(copiedBundle).not.toContain('secret-token');

  const [replayResponse] = await Promise.all([
    page.waitForResponse((res) => res.url().includes(`/api/v1/executions/${firstExecution.id}/replay`) && res.request().method() === 'POST'),
    page.getByRole('button', { name: 'Retry selected execution' }).click(),
  ]);
  expect(replayResponse.status()).toBe(202);
  const replayed = await replayResponse.json();
  expect(replayed.execution_id).not.toBe(firstExecution.id);
  const replayedExecution = await (await page.request.get(`/api/v1/executions/${replayed.execution_id}`)).json();
  expect(replayedExecution.input.source).toBe('m4-required-input');
});

test('Milestone 4 selected execution stays isolated from later live events', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Execution Context Isolation',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'Source JSON',
        position: { x: 160, y: 120 },
        params: { json_template: '{"label":"{{$trigger.label}}"}' },
      },
      {
        id: 'delay_1',
        type: 'delaySleep',
        name: 'Delay / Sleep',
        position: { x: 500, y: 120 },
        params: { seconds: '1' },
      },
    ]),
    edges_json: JSON.stringify([{ id: 'e-json-delay', source: 'json_1', target: 'delay_1' }]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { label: 'A' } });
  const executionA = await waitForLatestExecution(page, workflow.id);
  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { label: 'B' } });
  await waitForLatestExecution(page, workflow.id);

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText('SUCCESS');
  await page.getByLabel('Execution selector').selectOption(executionA.id);
  await page.locator('.node-body-title', { hasText: 'Source JSON' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Output' }).click();
  await expect(page.locator('.json-tree code', { hasText: 'A' }).first()).toBeVisible();

  await page.getByRole('button', { name: 'Copy debug bundle' }).click();
  let bundle = JSON.parse(await page.evaluate(() => window.__goflowLastDebugBundle));
  expect(bundle.selected_execution.id).toBe(executionA.id);
  expect(JSON.stringify(bundle)).toContain('"label":"A"');
  expect(JSON.stringify(bundle)).not.toContain('"label":"B"');

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger?async=true`, { data: { label: 'C' } });
  await page.waitForTimeout(400);
  await expect(page.locator('.json-tree code', { hasText: 'A' }).first()).toBeVisible();
  await page.getByRole('button', { name: 'Copy debug bundle' }).click();
  bundle = JSON.parse(await page.evaluate(() => window.__goflowLastDebugBundle));
  expect(bundle.selected_execution.id).toBe(executionA.id);
  expect(JSON.stringify(bundle)).toContain('"label":"A"');
  expect(JSON.stringify(bundle)).not.toContain('"label":"C"');

  await page.getByLabel('Execution selector').selectOption('');
  await expect(page.locator('.json-tree code', { hasText: 'C' }).first()).toBeVisible({ timeout: 5000 });
  await page.getByRole('button', { name: 'Copy debug bundle' }).click();
  bundle = JSON.parse(await page.evaluate(() => window.__goflowLastDebugBundle));
  expect(JSON.stringify(bundle)).toContain('"label":"C"');
});

test('Milestone 4 editor can cancel a selected running execution', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Cancel',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'delay_1', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 200, y: 120 }, params: { seconds: '120' } },
    ]),
    edges_json: '[]',
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger?async=true`, { data: { source: 'm4-cancel' } });
  const running = await waitForLatestExecution(page, workflow.id, false);
  expect(['RUNNING', 'QUEUED']).toContain(running.status);

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByRole('button', { name: 'Cancel execution' })).toBeEnabled();
  const [cancelResponse] = await Promise.all([
    page.waitForResponse((res) => res.url().includes(`/api/v1/executions/${running.id}/cancel`) && res.request().method() === 'POST'),
    page.getByRole('button', { name: 'Cancel execution' }).click(),
  ]);
  expect(cancelResponse.status()).toBe(202);

  for (let i = 0; i < 20; i += 1) {
    const exec = await (await page.request.get(`/api/v1/executions/${running.id}`)).json();
    if (['CANCEL_REQUESTED', 'CANCELLED', 'INTERRUPTED'].includes(exec.status)) return;
    await page.waitForTimeout(250);
  }
  throw new Error('Execution was not cancelled');
});
