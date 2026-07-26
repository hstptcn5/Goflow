import { expect, test } from '@playwright/test';

test('Milestone 1 workflow create edit save run navigation smoke', async ({ page }) => {
  await page.goto('/workflows');
  await expect(page.getByRole('navigation', { name: 'Primary navigation' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Workflows', level: 1 })).toBeVisible();

  await page.getByLabel('Workflow name').fill('UX Smoke Workflow');
  await page.getByLabel('Workflow description').fill('Created by Playwright');
  await page.getByRole('button', { name: 'Create blank workflow' }).click();

  await expect(page).toHaveURL(/\/workflows\/.+/);
  await expect(page.getByText('Saved')).toBeVisible();

  await page.getByRole('button', { name: 'Add first step' }).click();
  await page.getByLabel('Search nodes').fill('Delay');
  await page.getByRole('option', { name: /Delay \/ Sleep/ }).first().click();
  await expect(page.getByText('Unsaved changes')).toBeVisible();

  await page.getByRole('button', { name: 'Save' }).click();
  await expect(page.getByText('Saved')).toBeVisible();

  await page.getByLabel('Inactive').check();
  await expect(page.getByText('Active')).toBeVisible();

  await page.getByRole('button', { name: 'Test Workflow' }).click();
  await expect(page).toHaveURL(/\/executions/);
  await expect(page.getByRole('heading', { name: 'Executions', level: 1 })).toBeVisible();

  await page.getByRole('link', { name: 'Credentials' }).click();
  await expect(page.getByRole('heading', { name: 'Credentials', level: 1 })).toBeVisible();

  await page.getByRole('link', { name: 'Workflows', exact: true }).click();
  await expect(page.getByText('UX Smoke Workflow')).toBeVisible();
});
