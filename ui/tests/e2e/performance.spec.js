import { expect, test } from '@playwright/test';

test('Milestone 2 editor performance smoke with 10, 50, and 100 node graphs', async ({ page }) => {
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

    const start = Date.now();
    await page.goto(`/workflows/${body.id}`);
    await expect(page.getByRole('heading', { name: `UX Performance ${size}`, level: 1 })).toBeVisible();
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K');
    await page.getByLabel('Search nodes').fill('http');
    await expect(page.getByRole('option', { name: /HTTP Request/ }).first()).toBeVisible();
    await page.keyboard.press('Escape');
    samples.push({ size, pickerSearchMs: Date.now() - start });
  }

  await expect(page.getByRole('button', { name: 'Auto-layout' }).first()).toBeVisible();
  console.log(JSON.stringify({ editorPerformanceSmoke: samples }));
});
