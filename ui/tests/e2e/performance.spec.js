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
