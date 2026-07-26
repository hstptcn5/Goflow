import { expect, test } from '@playwright/test';

test('Milestone 2 editor performance smoke with separated measurements', async ({ page }) => {
  const samples = [];
  for (const size of [10, 50, 100]) {
    const workflow = await page.request.post('/api/v1/workflows', {
      data: {
        name: `UX Performance ${size}`,
        description: 'Performance smoke fixture',
        is_active: false,
        nodes_json: JSON.stringify(Array.from({ length: size }, (_, index) => ({
          id: `delay_${size}_${index}`,
          type: 'delaySleep',
          name: 'Delay / Sleep',
          position: { x: 120 + (index % 10) * 220, y: 120 + Math.floor(index / 10) * 140 },
          params: { seconds: '1' },
        }))),
        edges_json: JSON.stringify(Array.from({ length: size - 1 }, (_, index) => ({
          id: `edge_${size}_${index}`,
          source: `delay_${size}_${index}`,
          target: `delay_${size}_${index + 1}`,
        }))),
      },
    });
    expect(workflow.ok()).toBeTruthy();
    const body = await workflow.json();

    const readyStart = Date.now();
    await page.goto(`/workflows/${body.id}`);
    await expect(page.getByRole('heading', { name: `UX Performance ${size}`, level: 1 })).toBeVisible();
    const editorReadyMs = Date.now() - readyStart;

    const pickerOpenStart = Date.now();
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K');
    await expect(page.getByRole('dialog', { name: 'Add step' })).toBeVisible();
    const pickerOpenMs = Date.now() - pickerOpenStart;

    const pickerSearchStart = Date.now();
    await page.getByLabel('Search nodes').fill('http');
    await expect(page.getByRole('option', { name: /HTTP Request/ }).first()).toBeVisible();
    const pickerSearchMs = Date.now() - pickerSearchStart;
    await page.keyboard.press('Escape');

    const layoutStart = Date.now();
    await page.getByRole('button', { name: 'Auto-layout' }).first().click();
    await expect(page.getByText('Unsaved changes')).toBeVisible();
    const autoLayoutMs = Date.now() - layoutStart;

    expect(editorReadyMs).toBeLessThan(10_000);
    expect(pickerOpenMs).toBeLessThan(3_000);
    expect(pickerSearchMs).toBeLessThan(3_000);
    expect(autoLayoutMs).toBeLessThan(3_000);
    samples.push({ size, editorReadyMs, pickerOpenMs, pickerSearchMs, autoLayoutMs });
  }

  await expect(page.getByRole('button', { name: 'Auto-layout' }).first()).toBeVisible();
  console.log(JSON.stringify({ editorPerformanceSmokeSeparated: samples }));
});

test('Milestone 3 inspector performance smoke with data mapping measurements', async ({ page }) => {
  const upstreamCount = 12;
  const nodes = Array.from({ length: upstreamCount }, (_, index) => ({
    id: `json_${index}`,
    type: 'jsonTransform',
    name: `JSON Source ${index}`,
    position: { x: 80 + (index % 4) * 220, y: 80 + Math.floor(index / 4) * 150 },
    params: { json_template: JSON.stringify({ user: { email: `user${index}@example.com` }, rows: Array.from({ length: 40 }, (_, row) => ({ row, value: `value-${index}-${row}` })) }) },
  }));
  nodes.push({
    id: 'if_target',
    type: 'conditionIf',
    name: 'IF / ELSE Condition',
    position: { x: 980, y: 240 },
    params: { field: 'placeholder', operator: 'equals', value: 'user0@example.com' },
  });
  const edges = Array.from({ length: upstreamCount }, (_, index) => ({ id: `edge_${index}`, source: `json_${index}`, target: 'if_target' }));
  const workflowRes = await page.request.post('/api/v1/workflows', {
    data: {
      name: 'UX M3 Inspector Performance',
      description: 'Inspector performance smoke fixture',
      is_active: true,
      nodes_json: JSON.stringify(nodes),
      edges_json: JSON.stringify(edges),
    },
  });
  expect(workflowRes.ok()).toBeTruthy();
  const workflow = await workflowRes.json();
  await page.request.post(`/api/v1/workflows/${workflow.id}/trigger`, { data: { source: 'm3-perf' } });

  await page.goto(`/workflows/${workflow.id}`);
  const openStart = Date.now();
  await page.locator('.node-body-title', { hasText: 'IF / ELSE Condition' }).click();
  await expect(page.getByRole('complementary', { name: 'Node inspector' })).toBeVisible();
  const inspectorOpenMs = Date.now() - openStart;

  const treeStart = Date.now();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Input' }).click();
  await expect(page.getByText('user.email').first()).toBeVisible();
  const inputTreeMs = Date.now() - treeStart;

  const searchStart = Date.now();
  await page.getByLabel('Search input data').fill('user0@example.com');
  await expect(page.getByText('user0@example.com')).toBeVisible();
  const inputSearchMs = Date.now() - searchStart;

  const previewStart = Date.now();
  await page.locator('.panel-tabs').getByRole('tab', { name: 'Parameters' }).click();
  await page.locator('[data-param-name="field"]').getByRole('button', { name: 'Expression' }).click();
  await page.getByRole('button', { name: /user.email/ }).first().click();
  await expect(page.getByText('{{json_0.transformed.user.email}}')).toBeVisible();
  const expressionPreviewMs = Date.now() - previewStart;

  expect(inspectorOpenMs).toBeLessThan(3_000);
  expect(inputTreeMs).toBeLessThan(3_000);
  expect(inputSearchMs).toBeLessThan(3_000);
  expect(expressionPreviewMs).toBeLessThan(3_000);
  console.log(JSON.stringify({ inspectorPerformanceSmoke: { upstreamCount, inspectorOpenMs, inputTreeMs, inputSearchMs, expressionPreviewMs } }));
});
