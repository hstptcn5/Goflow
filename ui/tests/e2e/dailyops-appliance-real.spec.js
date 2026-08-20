import { expect, test } from '@playwright/test';

const sourceURL = process.env.GOFLOW_DAILYOPS_SOURCE_URL;
const chatID = process.env.GOFLOW_DAILYOPS_CHAT_ID || '@dailyops_e2e';
const tokenParts = (process.env.GOFLOW_DAILYOPS_TOKEN_PARTS || '').split('|');
const scheduleControlURL = process.env.GOFLOW_DAILYOPS_SCHEDULE_CONTROL_URL;
const sourceControlURL = process.env.GOFLOW_DAILYOPS_SOURCE_CONTROL_URL;
const phase = process.env.GOFLOW_DAILYOPS_PHASE || 'setup';
const hasHarness = Boolean(sourceURL && process.env.GOFLOW_DAILYOPS_TOKEN_PARTS);

function fakeToken() {
  if (tokenParts.length !== 2 || !tokenParts[0] || !tokenParts[1]) {
    throw new Error('GOFLOW_DAILYOPS_TOKEN_PARTS must contain two token segments');
  }
  return `${tokenParts[0]}:${tokenParts[1]}`;
}

async function expectNoSecret(page) {
  await expect(page.locator('body')).not.toContainText(tokenParts[1]);
}

async function executionSnapshot(page) {
  const response = await page.request.get('/api/appliance/executions?limit=10');
  expect(response.ok()).toBeTruthy();
  const body = await response.json();
  return body.executions || [];
}

async function waitForNewSuccess(page, previousExecutions) {
  const previousIDs = new Set(previousExecutions.map((execution) => execution.id));
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const executions = await executionSnapshot(page);
    const succeeded = executions.find((execution) => !previousIDs.has(execution.id) && execution.status === 'SUCCESS');
    if (succeeded) {
      await page.reload();
      await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram', level: 2 })).toBeVisible();
      await expect(page.getByLabel('Lần chạy gần nhất').getByText('THÀNH CÔNG')).toBeVisible();
      return succeeded;
    }
    await page.waitForTimeout(500);
  }
  throw new Error('A new successful DailyOps execution was not recorded');
}

async function controlPost(page, baseURL, path, data) {
  const response = await page.request.post(`${baseURL}${path}`, data === undefined ? {} : { data });
  expect(response.ok()).toBeTruthy();
  return response;
}

test('DailyOps appliance completes real setup and execution with mocked Telegram', async ({ page }) => {
  test.skip(!hasHarness, 'DailyOps appliance harness is required');
  test.skip(phase !== 'setup', 'setup phase only');

  await page.goto('/workflows');
  await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram', level: 1 })).toBeVisible();
  await expect(page.getByText('Gói chưa ký số')).toBeVisible();

  await page.getByLabel('Source URL').fill(sourceURL);
  await page.getByLabel('Telegram chat ID').fill(chatID);

  await page.getByLabel('Source URL').fill(sourceURL.replace('/dailyops.json', '/html'));
  await page.getByRole('button', { name: 'Kiểm tra nguồn' }).click();
  await expect(page.getByText('The source configuration or response is invalid.')).toBeVisible();
  await expect(page.getByText('Không hợp lệ', { exact: true })).toBeVisible();

  await page.getByLabel('Source URL').fill(sourceURL.replace('/dailyops.json', '/missing'));
  await page.getByRole('button', { name: 'Kiểm tra nguồn' }).click();
  await expect(page.getByText('The source data does not match the required contract.')).toBeVisible();

  await page.getByLabel('Source URL').fill(sourceURL);
  await page.getByRole('button', { name: 'Kiểm tra nguồn' }).click();
  await expect(page.getByText('7 trường bắt buộc hợp lệ')).toBeVisible();
  await expect(page.getByText('Hợp lệ', { exact: true }).first()).toBeVisible();

  await page.getByLabel('Telegram chat ID').fill('@inaccessible_dailyops_e2e');
  await page.locator('input[type="password"]').fill(`000000:invalid-${tokenParts[1]}`);
  await page.getByRole('button', { name: 'Tạo' }).click();
  await page.getByRole('button', { name: 'Kiểm tra Telegram' }).click();
  await expect(page.getByText('Telegram rejected the configured bot credential.')).toBeVisible();
  await expectNoSecret(page);

  await page.locator('input[type="password"]').fill(fakeToken());
  await page.getByRole('button', { name: 'Thay thế' }).click();
  await expect(page.getByText('Đã lưu thông tin xác thực')).toBeVisible();
  await expectNoSecret(page);

  await page.getByRole('button', { name: 'Kiểm tra Telegram' }).click();
  await expect(page.getByText('Telegram could not access the configured destination.')).toBeVisible();

  await page.getByLabel('Telegram chat ID').fill(chatID);
  await page.getByRole('button', { name: 'Kiểm tra Telegram' }).click();
  await expect(page.getByText('Bot token is valid and the configured chat is accessible.')).toBeVisible();
  await page.getByLabel('Bật lịch chạy mỗi ngày').check();
  await page.getByLabel('Giờ chạy').fill('08:05');
  await page.getByLabel('Múi giờ').fill('Asia/Bangkok');
  await page.getByRole('button', { name: 'Hoàn tất thiết lập' }).click();

  await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram', level: 2 })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Lịch mỗi ngày' })).toBeVisible();
  await expect(page.getByText('Đã bật', { exact: true })).toBeVisible();
  await expect(page.getByText('Asia/Bangkok', { exact: true })).toBeVisible();
  expect(await page.getByText('Run input').count()).toBe(0);
  const beforeManual = await executionSnapshot(page);
  await page.getByRole('button', { name: 'Chạy ngay' }).click();
  const manual = await waitForNewSuccess(page, beforeManual);
  expect(manual.trigger_source).toBe('ui');
  const afterManual = await executionSnapshot(page);
  const beforeManualIDs = new Set(beforeManual.map((item) => item.id));
  expect(afterManual.filter((execution) => !beforeManualIDs.has(execution.id))).toHaveLength(1);

  const previousExecutions = afterManual;
  const bootstrapResponse = await page.request.get('/api/appliance/bootstrap');
  const bootstrap = await bootstrapResponse.json();
  await controlPost(page, sourceControlURL, '/__control/hold');
  const tick = await controlPost(page, scheduleControlURL, '/tick', { now: '2026-08-09T01:05:00Z' });
  const tickBody = await tick.json();
  expect(tickBody.Triggered).toBe(true);
  const duplicateResponse = await page.request.post('/api/appliance/workflow/run', {
    headers: {
      Origin: new URL(page.url()).origin,
      'Content-Type': 'application/json',
      'X-Goflow-Appliance-Token': bootstrap.token,
    },
    data: { input: {} },
  });
  expect(duplicateResponse.status()).toBe(409);
  expect((await duplicateResponse.json()).category).toBe('already_running');
  const activeStatus = await page.request.get('/api/appliance/workflow/status');
  expect(activeStatus.ok()).toBeTruthy();
  expect((await activeStatus.json()).state).toBe('RUNNING');
  await controlPost(page, sourceControlURL, '/__control/release');
  const succeeded = await waitForNewSuccess(page, previousExecutions);
  const previousIDs = new Set(previousExecutions.map((execution) => execution.id));
  const newExecutions = (await executionSnapshot(page)).filter((execution) => !previousIDs.has(execution.id));
  expect(newExecutions).toHaveLength(1);
  expect(newExecutions[0].id).toBe(succeeded.id);
  expect(newExecutions[0].trigger_source).toBe('schedule');
  await expect(page.getByLabel('Các lần chạy gần đây').getByText('THÀNH CÔNG').first()).toBeVisible();
  await page.getByRole('button', { name: 'Làm mới' }).click();
  await expect(page.getByText('"secrets_hidden": true')).toBeVisible();
  await expect(page.getByText('"credential_ids_hidden": true')).toBeVisible();
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Tải chẩn đoán' }).click(),
  ]);
  expect(download.suggestedFilename()).toContain('diagnostics.json');
  const stream = await download.createReadStream();
  const chunks = [];
  for await (const chunk of stream) chunks.push(chunk);
  const exportedDiagnostics = Buffer.concat(chunks).toString('utf8');
  expect(exportedDiagnostics).toContain('"secrets_hidden": true');
  expect(exportedDiagnostics).not.toContain(tokenParts[1]);
  expect(exportedDiagnostics).not.toContain(sourceURL);
  expect(exportedDiagnostics).not.toContain(scheduleControlURL);
  await expectNoSecret(page);
});

test('DailyOps appliance restart preserves setup without exposing decrypted secrets', async ({ page }) => {
  test.skip(!hasHarness, 'DailyOps appliance harness is required');
  test.skip(phase !== 'persist', 'persistence phase only');

  await page.goto('/workflows');
  await expect(page.getByRole('heading', { name: 'DailyOps REST to Telegram', level: 2 })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cấu hình lại' })).toBeVisible();
  await expect(page.getByLabel('Lần chạy gần nhất').getByText('THÀNH CÔNG')).toBeVisible();
  await expect(page.getByLabel('Các lần chạy gần đây').getByText('THÀNH CÔNG').first()).toBeVisible();

  const status = await page.request.get('/api/appliance/status');
  expect(status.ok()).toBeTruthy();
  const statusBody = await status.json();
  expect(statusBody.state).toBe('READY');
  expect(statusBody.setup_complete).toBe(true);

  const setup = await page.request.get('/api/appliance/setup');
  expect(setup.ok()).toBeTruthy();
  const body = await setup.json();
  expect(body.setup_complete).toBe(true);
  expect(body.decrypted_values_returned).toBe(false);
  expect(body.credential_requirements.find((requirement) => requirement.key === 'telegram')?.assigned).toBe(true);
  expect(JSON.stringify(body)).not.toContain(tokenParts[1]);
  await expect(page.locator('input[type="password"]')).toHaveCount(0);

  const schedule = await page.request.get('/api/appliance/schedule');
  expect(schedule.ok()).toBeTruthy();
  const scheduleBody = await schedule.json();
  expect(scheduleBody.enabled).toBe(true);
  expect(scheduleBody.local_time).toBe('08:05');
  expect(scheduleBody.timezone).toBe('Asia/Bangkok');
  expect(scheduleBody.next_run_at).toBe('2026-08-10T01:05:00Z');

  const previousExecutions = await executionSnapshot(page);
  const tick = await controlPost(page, scheduleControlURL, '/tick', { now: '2026-08-10T01:05:00Z' });
  expect((await tick.json()).Triggered).toBe(true);
  const succeeded = await waitForNewSuccess(page, previousExecutions);
  expect(succeeded.trigger_source).toBe('schedule');
  await expect(page.getByLabel('Các lần chạy gần đây').getByText('THÀNH CÔNG')).toHaveCount(previousExecutions.length + 1);
  await expectNoSecret(page);
});
