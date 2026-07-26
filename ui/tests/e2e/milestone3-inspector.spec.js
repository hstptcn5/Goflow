import { expect, test } from '@playwright/test';

function visualOptions(page) {
  return {
    animations: 'disabled',
    maxDiffPixelRatio: 0.02,
    mask: [
      page.locator('.shell-status'),
      page.locator('.node-id'),
      page.locator('.execution-context'),
      page.locator('.log-grid dd'),
    ],
  };
}

async function createWorkflow(page, data) {
  const res = await page.request.post('/api/v1/workflows', { data });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

async function waitForLatestExecution(page, workflowId) {
  for (let i = 0; i < 30; i += 1) {
    const res = await page.request.get(`/api/v1/workflows/${workflowId}/executions`);
    expect(res.ok()).toBeTruthy();
    const executions = await res.json();
    if (executions[0] && !['RUNNING', 'QUEUED'].includes(executions[0].status)) return executions[0];
    await page.waitForTimeout(250);
  }
  throw new Error('Execution did not reach a terminal state');
}

const jsonNode = {
  id: 'json_1',
  type: 'jsonTransform',
  name: 'JSON Transform',
  position: { x: 360, y: 140 },
  params: { json_template: '{"user":{"email":"dev@example.com","name":"Dev User"},"items":[{"id":1,"name":"Alpha"}],"access_token":"secret-token"}' },
};

test('Milestone 3 data mapping inserts runtime expression and survives reload', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Mapping',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      jsonNode,
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 640, y: 140 }, params: { field: 'placeholder', operator: 'equals', value: 'dev@example.com' } },
    ]),
    edges_json: JSON.stringify([
      { id: 'edge_2', source: 'json_1', target: 'if_1' },
    ]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'm3-e2e' } });
  await page.goto(`/workflows/${workflow.id}`);
  await page.getByText('IF / ELSE Condition').click();

  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByLabel('Search input data').first().fill('email');
  await page.getByRole('button', { name: /transformed.user.email/ }).first().click();
  await expect(page.getByText('{{json_1.transformed.user.email}}')).toBeVisible();
  await expect(page.getByText('"dev@example.com"')).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-token');

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.reload();
  await page.getByText('IF / ELSE Condition').click();
  await expect(page.getByRole('textbox', { name: 'Input Value' })).toHaveValue('{{json_1.transformed.user.email}}');

  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Fixed' }).click();
  await expect(page.getByRole('textbox', { name: 'Input Value' })).toHaveValue('dev@example.com');
  await expect(page.locator('[data-param-name="field"]').getByRole('button', { name: 'Fixed' })).toHaveClass(/active/);
});

test('Milestone 3 output and logs tabs show execution context without secrets', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Output Logs',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      jsonNode,
    ]),
    edges_json: '[]',
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'm3-output' } });
  await waitForLatestExecution(page, workflow.id);
  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Output' }).click();
  await expect(page.getByText('Status: SUCCESS')).toBeVisible();
  await expect(page.getByText('Attempts: 1')).toBeVisible();
  await expect(page.getByText('transformed.user.email')).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-token');

  await page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' }).click();
  await expect(page.getByText('Execution ID')).toBeVisible();
  await expect(page.getByText('Duration', { exact: true })).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-token');
});

test('Milestone 3 invalid expression preview blocks Test Workflow until fixed', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Expression Failure',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 360, y: 140 }, params: { field: '{{missing.value}}', operator: 'equals', value: 'ok' } },
    ]),
    edges_json: '[]',
  });

  await page.goto(`/workflows/${workflow.id}`);
  await page.getByText('IF / ELSE Condition').click();
  await expect(page.getByText(/source "missing" does not exist/i)).toBeVisible();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page.getByText('Fix workflow validation issues before testing.')).toBeVisible();

  await page.getByRole('textbox', { name: 'Input Value' }).fill('{{$trigger.source}}');
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);
});

test('Milestone 3 invalid JSON can save as draft but blocks Test Workflow until fixed', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Invalid JSON Draft',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', position: { x: 360, y: 140 }, params: { json_template: '{bad' } },
    ]),
    edges_json: '[]',
  });

  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await expect(page.getByText('Enter valid JSON.')).toBeVisible();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page.getByText('Fix workflow validation issues before testing.')).toBeVisible();

  await page.getByRole('textbox', { name: 'JSON Structure' }).fill('{"ok":true}');
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);
});

test('Milestone 3 runtime expression parity resolves mapped values and records skipped branch', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Runtime Parity',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'JSON Transform',
        position: { x: 260, y: 140 },
        params: { json_template: '{"user":{"email":"dev@example.com"},"score":7,"enabled":true,"profile":{"role":"admin"}}' },
      },
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 560, y: 140 }, params: { field: 'placeholder', operator: 'equals', value: 'dev@example.com' } },
      { id: 'true_json', type: 'jsonTransform', name: 'True JSON', position: { x: 860, y: 70 }, params: { json_template: '{"branch":"true"}' } },
      { id: 'false_json', type: 'jsonTransform', name: 'False JSON', position: { x: 860, y: 220 }, params: { json_template: '{"branch":"false"}' } },
    ]),
    edges_json: JSON.stringify([
      { id: 'e1', source: 'json_1', target: 'if_1' },
      { id: 'e2', source: 'if_1', sourceHandle: 'true', target: 'true_json' },
      { id: 'e3', source: 'if_1', sourceHandle: 'false', target: 'false_json' },
    ]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'runtime-parity', count: 3 } });
  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'IF / ELSE Condition' }).click();
  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByLabel('Search input data').first().fill('email');
  await page.getByRole('button', { name: /transformed.user.email/ }).first().click();
  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);

  const execution = await waitForLatestExecution(page, workflow.id);
  const logs = execution.node_logs || JSON.parse(execution.logs_json || '[]');
  const byNode = Object.fromEntries(logs.map((log) => [log.node_id, log]));
  expect(byNode.if_1.output.evaluated).toContain("'dev@example.com' equals 'dev@example.com'");
  expect(byNode.if_1.output.target_handle).toBe('true');
  expect(byNode.true_json.status).toBe('SUCCESS');
  expect(byNode.false_json.status).toBe('SKIPPED');

  const saved = await (await page.request.get(`/api/v1/workflows/${workflow.id}`)).json();
  const nodes = JSON.parse(saved.nodes_json);
  nodes.find((node) => node.id === 'if_1').params = { field: '{{$trigger.source}}', operator: 'equals', value: 'runtime-parity' };
  await page.request.put(`/api/v1/workflows/${workflow.id}`, { data: { ...saved, nodes_json: JSON.stringify(nodes) } });
  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'runtime-parity' } });
  const triggerExecution = await waitForLatestExecution(page, workflow.id);
  const triggerLogs = triggerExecution.node_logs || JSON.parse(triggerExecution.logs_json || '[]');
  expect(triggerLogs.find((log) => log.node_id === 'if_1').output.target_handle).toBe('true');
});

test('Milestone 3 number expression remains editable and saves across reload', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Number Expression',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'JSON Transform',
        position: { x: 220, y: 120 },
        params: { json_template: '{"score":42}' },
      },
      { id: 'delay_1', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 520, y: 120 }, params: { seconds: '1' } },
    ]),
    edges_json: JSON.stringify([{ id: 'e1', source: 'json_1', target: 'delay_1' }]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'number-expression' } });
  await waitForLatestExecution(page, workflow.id);
  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'Delay / Sleep' }).click();
  const secondsInput = page.locator('[aria-label="Delay Duration (Seconds)"]');
  await expect(secondsInput).toHaveAttribute('type', 'number');

  await page.locator('[data-param-name="seconds"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByLabel('Search input data').first().fill('score');
  await page.getByRole('button', { name: /transformed.score/ }).first().click();
  await expect(secondsInput).toHaveAttribute('type', 'text');
  await expect(secondsInput).toHaveValue('{{json_1.transformed.score}}');

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.reload();
  await page.locator('.node-body-title', { hasText: 'Delay / Sleep' }).click();
  await expect(secondsInput).toHaveAttribute('type', 'text');
  await expect(secondsInput).toHaveValue('{{json_1.transformed.score}}');

  await page.locator('[data-param-name="seconds"]').getByRole('button', { name: 'Fixed' }).click();
  await expect(secondsInput).toHaveAttribute('type', 'number');
  await expect(secondsInput).toHaveValue('42');
});

test('Milestone 3 object and array expressions stay Expression unless JSON literal conversion is explicit', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Complex Expression Conversion',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      {
        id: 'json_1',
        type: 'jsonTransform',
        name: 'Source JSON',
        position: { x: 180, y: 120 },
        params: { json_template: '{"profile":{"role":"admin"},"items":[1,2]}' },
      },
      {
        id: 'json_obj',
        type: 'jsonTransform',
        name: 'Object Target',
        position: { x: 520, y: 70 },
        params: { json_template: '{{json_1.transformed.profile}}' },
      },
      {
        id: 'json_arr',
        type: 'jsonTransform',
        name: 'Array Target',
        position: { x: 520, y: 210 },
        params: { json_template: '{{json_1.transformed.items}}' },
      },
      {
        id: 'json_missing',
        type: 'jsonTransform',
        name: 'Missing Target',
        position: { x: 520, y: 350 },
        params: { json_template: '{{json_1.transformed.missing}}' },
      },
    ]),
    edges_json: JSON.stringify([
      { id: 'e1', source: 'json_1', target: 'json_obj' },
      { id: 'e2', source: 'json_1', target: 'json_arr' },
      { id: 'e3', source: 'json_1', target: 'json_missing' },
    ]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'complex-expression' } });
  await waitForLatestExecution(page, workflow.id);
  await page.goto(`/workflows/${workflow.id}`);

  await page.locator('.node-body-title', { hasText: 'Object Target' }).click();
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.profile}}');
  await expect(page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Expression' })).toHaveClass(/active/);
  await page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Fixed' }).click();
  await expect(page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Expression' })).toHaveClass(/active/);
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.profile}}');
  await expect(page.getByText('Object and array previews stay in Expression mode')).toBeVisible();
  await page.getByRole('button', { name: 'Convert to JSON literal' }).click();
  await expect(page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Fixed' })).toHaveClass(/active/);
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue(/"role": "admin"/);

  await page.getByRole('button', { name: 'Close node inspector' }).click();
  await page.locator('.node-body-title', { hasText: 'Array Target' }).click();
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.items}}');
  await page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Fixed' }).click();
  await expect(page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Expression' })).toHaveClass(/active/);
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.items}}');

  await page.getByRole('button', { name: 'Close node inspector' }).click();
  await page.locator('.node-body-title', { hasText: 'Missing Target' }).click();
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.missing}}');
  await page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Fixed' }).click();
  await expect(page.locator('[data-param-name="json_template"]').getByRole('button', { name: 'Expression' })).toHaveClass(/active/);
  await expect(page.getByRole('textbox', { name: 'JSON Structure' })).toHaveValue('{{json_1.transformed.missing}}');
  await expect(page.getByText('Preview is unavailable')).toBeVisible();
});

test('Milestone 3 data picker includes transitive upstream nodes only', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Transitive Mapping',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'json_a', type: 'jsonTransform', name: 'JSON A', position: { x: 140, y: 100 }, params: { json_template: '{"fromA":"a"}' } },
      { id: 'json_b', type: 'jsonTransform', name: 'JSON B', position: { x: 420, y: 100 }, params: { json_template: '{"fromB":"b"}' } },
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 700, y: 100 }, params: { field: 'x', operator: 'equals', value: 'x' } },
      { id: 'downstream', type: 'jsonTransform', name: 'Downstream', position: { x: 960, y: 100 }, params: { json_template: '{"down":"d"}' } },
      { id: 'unrelated', type: 'jsonTransform', name: 'Unrelated', position: { x: 140, y: 320 }, params: { json_template: '{"other":"o"}' } },
    ]),
    edges_json: JSON.stringify([
      { id: 'e1', source: 'json_a', target: 'json_b' },
      { id: 'e2', source: 'json_b', target: 'if_1' },
      { id: 'e3', source: 'if_1', target: 'downstream' },
    ]),
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'transitive' } });
  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'IF / ELSE Condition' }).click();
  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  const selector = page.getByLabel('Source node selector').first();
  await expect(selector).toContainText('JSON B');
  await expect(selector).toContainText('JSON A');
  await expect(selector).not.toContainText('Downstream');
  await expect(selector).not.toContainText('Unrelated');
});

test('Milestone 3 inspector consumes server-redacted execution DTO', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Server Redaction',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', position: { x: 360, y: 140 }, params: { json_template: '{"Authorization":"Bearer raw-token","access_token":"secret-token"}' } },
    ]),
    edges_json: '[]',
  });

  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { access_token: 'secret-token', headers: { Authorization: 'Bearer raw-token' } } });
  const execution = await waitForLatestExecution(page, workflow.id);
  const responseText = JSON.stringify(execution);
  expect(responseText).not.toContain('raw-token');
  expect(responseText).not.toContain('secret-token');

  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' }).click();
  await expect(page.locator('body')).not.toContainText('raw-token');
  await expect(page.locator('body')).not.toContainText('secret-token');
});

test('Milestone 3 dirty valid activation saves first', async ({ page }) => {
  const workflow = await createWorkflow(page, {
    name: 'UX M3 Activate Save',
    description: '',
    is_active: false,
    nodes_json: JSON.stringify([
      { id: 'delay_1', type: 'delaySleep', name: 'Delay / Sleep', position: { x: 120, y: 120 }, params: { seconds: '1' } },
    ]),
    edges_json: '[]',
  });

  await page.goto(`/workflows/${workflow.id}`);
  await page.getByText('Delay / Sleep').click();
  await page.locator('[aria-label="Delay Duration (Seconds)"]').fill('2');
  await expect(page.getByText('Unsaved changes')).toBeVisible();
  await page.getByLabel('Inactive').click();
  await expect(page.getByText('Active')).toBeVisible();
  await expect(page.getByText('Saved')).toBeVisible();

  const saved = await (await page.request.get(`/api/v1/workflows/${workflow.id}`)).json();
  const savedNodes = JSON.parse(saved.nodes_json);
  expect(savedNodes[0].params.seconds).toBe('2');
});

test('Milestone 3 inspector visual regression baseline states', async ({ page }) => {
  const snapshot = visualOptions(page);
  const hideExecutionDebugger = () => page.addStyleTag({
    content: `
      .execution-toolbar,.debug-bundle-preview,.node-execution-state,.node-execution-meta{display:none!important}
      .custom-node-card.status-running,.custom-node-card.status-success,.custom-node-card.status-failed{
        border-color: var(--color-border)!important;
        box-shadow: 0 2px 8px rgba(15,23,42,.07)!important;
        animation: none!important;
      }
      .vue-flow__edge-path{stroke:var(--color-border-strong)!important;stroke-width:2!important;stroke-dasharray:none!important}
    `,
  });
  await page.setViewportSize({ width: 1366, height: 768 });

  const validationWorkflow = await createWorkflow(page, {
    name: 'UX M3 Visual Validation',
    description: '',
    is_active: false,
    nodes_json: JSON.stringify([
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 220, y: 160 }, params: { field: '', operator: 'equals', value: '' } },
      { id: 'sheets_1', type: 'googleSheets', name: 'Google Sheets', position: { x: 520, y: 160 }, params: {} },
    ]),
    edges_json: '[]',
  });
  await page.goto(`/workflows/${validationWorkflow.id}`);
  await hideExecutionDebugger();
  await page.getByText('IF / ELSE Condition').click();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page.getByText('Input Value is required.')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-parameters-inline-required.png', snapshot);

  await page.getByRole('button', { name: 'Close node inspector' }).click();
  await page.locator('.node-body-title', { hasText: 'Google Sheets' }).click();
  await expect(page.getByText('Create credential')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-missing-credential.png', snapshot);

  const mappingWorkflow = await createWorkflow(page, {
    name: 'UX M3 Visual Mapping',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      jsonNode,
      { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', position: { x: 640, y: 140 }, params: { field: 'placeholder', operator: 'equals', value: 'dev@example.com' } },
    ]),
    edges_json: JSON.stringify([{ id: 'edge_1', source: 'json_1', target: 'if_1' }]),
  });
  await page.request.post(`/api/v1/workflows/${mappingWorkflow.id}/trigger`, { data: { source: 'visual' } });
  await page.goto(`/workflows/${mappingWorkflow.id}`);
  await hideExecutionDebugger();
  await page.getByText('IF / ELSE Condition').click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Input' }).click();
  await expect(page.getByText('transformed.user.email')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-input-json-tree.png', snapshot);

  await page.locator('.panel-tabs').getByRole('tab', { name: 'Parameters' }).click();
  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByLabel('Search input data').first().fill('email');
  await page.getByRole('button', { name: /transformed.user.email/ }).first().click();
  await expect(page.getByText('{{json_1.transformed.user.email}}')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-data-picker-expression-preview.png', snapshot);

  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Output' }).click();
  await expect(page.getByText('Status: SUCCESS')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-output-json.png', snapshot);

  const failedWorkflow = await createWorkflow(page, {
    name: 'UX M3 Visual Logs Error',
    description: '',
    is_active: true,
    nodes_json: JSON.stringify([
      { id: 'bad_json', type: 'jsonTransform', name: 'Bad JSON', position: { x: 220, y: 160 }, params: { json_template: '{bad json' } },
    ]),
    edges_json: '[]',
  });
  await page.request.post(`/api/v1/workflows/${failedWorkflow.id}/trigger`, { data: { source: 'visual-fail' } });
  await page.goto(`/workflows/${failedWorkflow.id}`);
  await hideExecutionDebugger();
  await page.getByText('Bad JSON').click();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Logs' }).click();
  await expect(page.getByText('Node failed')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-logs-error.png', snapshot);
  await expect(page).toHaveScreenshot('m3-inspector-1366.png', snapshot);
});
