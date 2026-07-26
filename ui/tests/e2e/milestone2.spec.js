import { expect, test } from '@playwright/test';

function visualSnapshotOptions(page) {
  return {
    animations: 'disabled',
    maxDiffPixelRatio: 0.02,
    mask: [
      page.locator('.shell-status'),
      page.locator('.node-id'),
    ],
  };
}

test('Milestone 2 editor usability flow', async ({ page }) => {
  await page.goto('/workflows');
  await page.getByLabel('Workflow name').fill('UX M2 Workflow');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();
  await expect(page).toHaveURL(/\/workflows\/.+/);

  await page.getByRole('button', { name: 'Add first step' }).click();
  await expect(page.getByRole('dialog', { name: 'Add step' })).toBeVisible();
  await page.getByLabel('Search nodes').fill('Cron');
  await page.getByRole('option', { name: /Cron Schedule/ }).first().click();
  await expect(page.locator('.node-body-title', { hasText: 'Cron Schedule' })).toBeVisible();

  await page.getByRole('button', { name: 'Close node inspector' }).click();
  await page.getByRole('button', { name: /Add step after Cron Schedule/ }).click();
  await page.getByLabel('Search nodes').fill('IF');
  await page.getByRole('option', { name: /IF \/ ELSE Condition/ }).first().click();
  await expect(page.getByText('Missing required fields')).toBeVisible();

  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page.locator('.validation-summary')).toContainText('need attention');

  await page.getByLabel('Input Value').fill('200');
  await expect(page.getByText('Unsaved changes')).toBeVisible();
  await page.getByRole('button', { name: 'Duplicate' }).click();
  await expect(page.getByText(/copy/)).toBeVisible();

  await page.getByRole('button', { name: 'Auto-layout' }).first().click();
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+Z' : 'Control+Z');
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+Shift+Z' : 'Control+Shift+Z');

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();
  await page.reload();
  await expect(page.locator('.node-body-title', { hasText: 'Cron Schedule' })).toBeVisible();
  await expect(page.getByText('IF / ELSE Condition', { exact: true })).toBeVisible();

  await page.getByLabel('Inactive').check();
  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);
  await expect(page.getByRole('heading', { name: 'Executions', level: 1 })).toBeVisible();
});

test('Milestone 2 keyboard picker and delete smoke', async ({ page }) => {
  await page.goto('/workflows');
  await page.getByLabel('Workflow name').fill('UX M2 Keyboard');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();
  await expect(page).toHaveURL(/\/workflows\/.+/);

  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K');
  await page.getByLabel('Search nodes').fill('Delay');
  await page.keyboard.press('Enter');
  await expect(page.locator('.node-body-title', { hasText: 'Delay / Sleep' })).toBeVisible();

  await page.locator('.node-body-title', { hasText: 'Delay / Sleep' }).click();
  await page.keyboard.press('Delete');
  await expect(page.getByText('No nodes yet')).toBeVisible();
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+Z' : 'Control+Z');
  await expect(page.locator('.node-body-title', { hasText: 'Delay / Sleep' })).toBeVisible();
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+S' : 'Control+S');
  await expect(page.getByText('Saved')).toBeVisible();
});

test('Milestone 2 visual regression baseline states at 1366 editor', async ({ page }) => {
  const snapshotOptions = visualSnapshotOptions(page);
  await page.setViewportSize({ width: 1366, height: 768 });
  await page.goto('/workflows');
  await page.getByLabel('Workflow name').fill('UX M2 Visual');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();
  await expect(page.getByText('No nodes yet')).toBeVisible();
  await expect(page).toHaveScreenshot('blank-canvas.png', snapshotOptions);

  await page.getByRole('button', { name: 'Add first step' }).click();
  await expect(page.getByRole('dialog', { name: 'Add step' })).toBeVisible();
  await expect(page).toHaveScreenshot('node-picker.png', snapshotOptions);
  await page.getByLabel('Search nodes').fill('zzzzzz');
  await expect(page.getByText('No nodes match this search.')).toBeVisible();
  await expect(page).toHaveScreenshot('node-picker-empty.png', snapshotOptions);

  await page.getByLabel('Search nodes').fill('Delay');
  await page.getByRole('option', { name: /Delay \/ Sleep/ }).first().click();
  await expect(page.getByText('Configured')).toBeVisible();
  await expect(page).toHaveScreenshot('configured-node.png', snapshotOptions);

  await page.getByRole('button', { name: 'Close node inspector' }).click();
  await page.getByRole('button', { name: /Add step after Delay \/ Sleep/ }).click();
  await page.getByLabel('Search nodes').fill('IF');
  await page.getByRole('option', { name: /IF \/ ELSE Condition/ }).first().click();
  await expect(page.getByText('Missing required fields')).toBeVisible();
  await expect(page.getByText('true')).toBeVisible();
  await expect(page.getByText('false')).toBeVisible();
  await expect(page).toHaveScreenshot('invalid-node-if-handles.png', snapshotOptions);
  await expect(page).toHaveScreenshot('editor-1366.png', snapshotOptions);
});
