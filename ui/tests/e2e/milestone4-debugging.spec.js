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
  await expect(page.locator('.execution-context strong')).toHaveText('FAILED');
  await expect(page.locator('.execution-context')).toContainText(`Latest execution ${firstExecution.id}`);
  await expect(page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' })).toHaveAttribute('aria-selected', 'true');
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
  await expect(page.locator('.execution-context strong')).toHaveText('SUCCESS');
  await expect(page.locator('.execution-context')).toContainText(`Selected execution ${executionA.id}`);
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

test('Milestone 4 live execution is scoped to the current workflow', async ({ page }) => {
  const workflowA = await createWorkflow(page, {
    name: 'UX M4 Live Scope A',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'delay_1', type: 'delaySleep', name: 'Delay A', position: { x: 200, y: 120 }, params: { seconds: '1' } },
    ]),
    edges_json: '[]',
  });
  const workflowB = await createWorkflow(page, {
    name: 'UX M4 Live Scope B',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'delay_1', type: 'delaySleep', name: 'Delay B', position: { x: 200, y: 120 }, params: { seconds: '120' } },
    ]),
    edges_json: '[]',
  });

  await page.goto(`/workflows/${workflowA.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText('No executions yet');
  await page.request.post(`/api/v1/workflows/${workflowB.id}/trigger?async=true`, { data: { source: 'workflow-b' } });
  await page.waitForTimeout(500);
  await expect(page.getByLabel('Execution selector')).toContainText('No executions yet');
  await expect(page.locator('.node-execution-state')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Cancel execution' })).toBeDisabled();

  const runA = await page.request.post(`/api/v1/workflows/${workflowA.id}/trigger?async=true`, { data: { source: 'workflow-a' } });
  expect(runA.ok()).toBeTruthy();
  const runABody = await runA.json();
  await expect(page.getByLabel('Execution selector')).toContainText(`Live execution ${runABody.execution_id.slice(0, 8)}`);
  await expect(page.locator('.node-execution-state')).toContainText(/RUNNING|SUCCESS/);
});

test('Milestone 4 cancel targets only the displayed running execution', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M4 Exact Cancel',
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

  const currentWorkflow = await (await page.request.get(`/api/v1/workflows/${workflow.id}`)).json();
  const quickNodes = JSON.stringify([
    { id: 'json_1', type: 'jsonTransform', name: 'Quick JSON', position: { x: 200, y: 120 }, params: { json_template: '{"ok":true}' } },
  ]);
  const update = await page.request.put(`/api/v1/workflows/${workflow.id}`, {
    data: { ...currentWorkflow, nodes_json: quickNodes, edges_json: '[]' },
  });
  expect(update.ok()).toBeTruthy();
  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'terminal' } });
  const terminal = await waitForLatestExecution(page, workflow.id);
  expect(terminal.status).toBe('SUCCESS');

  await page.goto(`/workflows/${workflow.id}`);
  await expect(page.getByLabel('Execution selector')).toContainText('SUCCESS');
  await expect(page.getByRole('button', { name: 'Cancel execution' })).toBeDisabled();
  await page.getByLabel('Execution selector').selectOption(running.id);
  await expect(page.getByRole('button', { name: `Cancel ${running.id.slice(0, 8)}` })).toBeEnabled();
  const [cancelResponse] = await Promise.all([
    page.waitForResponse((res) => res.url().includes(`/api/v1/executions/${running.id}/cancel`) && res.request().method() === 'POST'),
    page.getByRole('button', { name: `Cancel ${running.id.slice(0, 8)}` }).click(),
  ]);
  expect(cancelResponse.status()).toBe(202);

  let cancelled = false;
  for (let i = 0; i < 20; i += 1) {
    const exec = await (await page.request.get(`/api/v1/executions/${running.id}`)).json();
    if (['CANCEL_REQUESTED', 'CANCELLED', 'INTERRUPTED'].includes(exec.status)) {
      cancelled = true;
      break;
    }
    await page.waitForTimeout(250);
  }
  if (!cancelled) throw new Error('Execution was not cancelled');
  const terminalAfterCancel = await (await page.request.get(`/api/v1/executions/${terminal.id}`)).json();
  expect(terminalAfterCancel.status).toBe('SUCCESS');
});
