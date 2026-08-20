import { expect, test } from '@playwright/test';

const bootstrap = {
  token: 'session-token',
  app: { name: 'Goflow', version: '0.5.0-test', platform: 'windows/amd64', channel: 'UNSIGNED-PILOT-BETA' },
  pack: {
    id: 'example.appliance',
    name: 'Example Appliance',
    version: '0.1.0',
    description: 'Runs a managed workflow',
  },
};

async function fulfill(route, body, status = 200) {
  await route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
}

async function mockAppliance(page, initial = {}) {
  const state = { configSaved: false, credentialAssigned: false, completed: false, runStarted: false, ...initial };
  await page.route('**/api/appliance/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname === '/api/appliance/bootstrap') return fulfill(route, bootstrap);
    if (url.pathname === '/api/appliance/setup') {
      return fulfill(route, {
        state: state.completed ? 'READY' : 'NEEDS_SETUP',
        missing: state.completed ? [] : ['config.source_url', 'credential.telegram'],
        can_complete: state.configSaved && state.credentialAssigned,
        current_config_values: state.configSaved ? { source_url: 'https://example.test/feed.json' } : {},
        config_schema: [{ key: 'source_url', label: 'Source URL', type: 'url', required: true }],
        credential_requirements: [{ key: 'telegram', label: 'Telegram', type: 'TELEGRAM_BOT', required: true, test_kind: 'telegram_get_me', assigned: state.credentialAssigned }],
      });
    }
    if (url.pathname === '/api/appliance/status') {
      return fulfill(route, {
        pack: bootstrap.pack,
        workflow_id: 'wf-1',
        server: 'ok',
        state: state.completed ? 'READY' : 'NEEDS_SETUP',
        missing: state.completed ? [] : ['config.source_url', 'credential.telegram'],
        can_complete: state.configSaved && state.credentialAssigned,
      });
    }
    if (url.pathname === '/api/appliance/workflow/status') {
      return fulfill(route, {
        workflow_id: 'wf-1',
        server: 'ok',
        state: state.runStarted ? 'RUNNING' : state.completed ? 'READY' : 'NEEDS_SETUP',
        workflow: { id: 'wf-1', name: 'Daily Digest', is_active: true, risk: 'low' },
        latest_execution: state.runStarted ? { id: 'exec-1', status: 'SUCCESS', duration_ms: 88, started_at: '2026-08-09T00:00:00Z' } : null,
      });
    }
    if (url.pathname === '/api/appliance/executions/latest') {
      return fulfill(route, { execution: state.runStarted ? { id: 'exec-1', status: 'SUCCESS', duration_ms: 88, started_at: '2026-08-09T00:00:00Z' } : null });
    }
    if (url.pathname === '/api/appliance/executions') {
      return fulfill(route, { limit: 10, executions: state.runStarted ? [{ id: 'exec-1', status: 'SUCCESS', duration_ms: 88, started_at: '2026-08-09T00:00:00Z' }] : [] });
    }
    if (url.pathname === '/api/appliance/setup/config') {
      state.configSaved = true;
      return fulfill(route, { values: { source_url: 'https://example.test/feed.json' } });
    }
    if (url.pathname === '/api/appliance/setup/credentials/create') {
      state.credentialAssigned = true;
      return fulfill(route, { credentials: { telegram: { credential_type: 'TELEGRAM_BOT', assigned: true } } }, 201);
    }
    if (url.pathname === '/api/appliance/setup/credentials/test') return fulfill(route, { status: 'OK' });
    if (url.pathname === '/api/appliance/setup/complete') {
      state.completed = true;
      return fulfill(route, { state: 'READY', setup_complete: true });
    }
    if (url.pathname === '/api/appliance/workflow/run') {
      state.runStarted = true;
      return fulfill(route, { execution_id: 'exec-1', workflow_id: 'wf-1', status: 'RUNNING' }, 202);
    }
    if (url.pathname === '/api/appliance/diagnostics') {
      return fulfill(route, { pack: bootstrap.pack, state: 'READY', latest_execution: { id: 'exec-1', status: 'SUCCESS', duration_ms: 88 }, secrets_hidden: true });
    }
    return fulfill(route, {});
  });
  return state;
}

test('appliance first-run setup and run flow is Vietnamese and hides submitted secrets', async ({ page }) => {
  await mockAppliance(page);
  await page.goto('/workflows/wf-1');

  await expect(page.getByRole('heading', { name: 'Example Appliance', level: 1 })).toBeVisible();
  await expect(page.getByText('Gói chưa ký số')).toBeVisible();

  await page.getByLabel('Source URL').fill('https://example.test/feed.json');
  await page.getByRole('button', { name: 'Lưu cấu hình' }).click();
  await page.locator('input[type="password"]').fill('123:secret-canary');
  await page.getByRole('button', { name: 'Tạo' }).click();
  await expect(page.getByText('Đã lưu thông tin xác thực', { exact: true })).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-canary');

  await page.getByRole('button', { name: 'Kiểm tra Telegram' }).click();
  await expect(page.getByText('Hợp lệ', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Hoàn tất thiết lập' }).click();

  await expect(page.getByRole('heading', { name: 'Example Appliance', level: 2 })).toBeVisible();
  await page.getByRole('button', { name: 'Chạy ngay' }).click();
  await expect(page.getByLabel('Lần chạy gần nhất').getByText('THÀNH CÔNG')).toBeVisible();
  await page.getByRole('button', { name: 'Làm mới' }).click();
  await expect(page.getByText('"secrets_hidden": true')).toBeVisible();
  await expect(page.locator('body')).not.toContainText('secret-canary');
});

test('generic serve keeps the Vietnamese workspace when appliance bootstrap returns 404', async ({ page }) => {
  await page.route('**/api/appliance/bootstrap', (route) => route.fulfill({ status: 404, body: '' }));
  await page.goto('/workflows');

  await expect(page.locator('h1', { hasText: 'Workflow' })).toBeVisible();
  await expect(page.getByText('Ứng dụng Goflow')).toHaveCount(0);
});
