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
  await page.goto(`/workflows/${workflow.id}`);
  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await page.locator('.panel-tabs').getByRole('button', { name: 'Output' }).click();
  await expect(page.getByText('Status: SUCCESS')).toBeVisible();
  await expect(page.getByText('Attempts: 1')).toBeVisible();
  await expect(page.getByText('transformed.user.email')).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-token');

  await page.locator('.panel-tabs').getByRole('button', { name: 'Logs' }).click();
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
  await page.getByRole('textbox', { name: 'Delay Duration (Seconds)' }).fill('2');
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
  await page.getByText('IF / ELSE Condition').click();
  await page.locator('.panel-tabs').getByRole('button', { name: 'Input' }).click();
  await expect(page.getByText('transformed.user.email')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-input-json-tree.png', snapshot);

  await page.locator('.panel-tabs').getByRole('button', { name: 'Parameters' }).click();
  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByLabel('Search input data').first().fill('email');
  await page.getByRole('button', { name: /transformed.user.email/ }).first().click();
  await expect(page.getByText('{{json_1.transformed.user.email}}')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-data-picker-expression-preview.png', snapshot);

  await page.locator('.node-body-title', { hasText: 'JSON Transform' }).click();
  await page.locator('.panel-tabs').getByRole('button', { name: 'Output' }).click();
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
  await page.getByText('Bad JSON').click();
  await page.locator('.panel-tabs').getByRole('button', { name: 'Logs' }).click();
  await expect(page.getByText('Node failed')).toBeVisible();
  await expect(page).toHaveScreenshot('m3-logs-error.png', snapshot);
  await expect(page).toHaveScreenshot('m3-inspector-1366.png', snapshot);
});
