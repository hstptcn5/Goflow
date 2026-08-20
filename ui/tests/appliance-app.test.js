import { describe, expect, it, vi } from 'vitest';
import App from '../src/App.vue';
import ApplianceApp from '../src/components/ApplianceApp.vue';
import { mountWithApp, nextFrame } from './mount';

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

function json(data, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
}

function applianceFetch(overrides = {}) {
  const state = {
    configSaved: false,
    credentialAssigned: false,
    completed: false,
    runStarted: false,
    schedule: {
      configured: false,
      revision: 0,
      enabled: false,
      kind: 'daily',
      local_time: '09:00',
      timezone: 'UTC',
      state: 'DISABLED',
    },
    diagnostics: {
      schema_version: 1,
      app: { name: 'Goflow', version: '0.5.0-test', platform: 'windows/amd64' },
      pack: { id: bootstrap.pack.id, version: bootstrap.pack.version },
      setup: { state: 'READY', category: 'ready' },
      recent_executions: [{ status: 'SUCCESS', duration_ms: 42 }],
      privacy: { local_only: true, credential_ids_hidden: true, secrets_hidden: true },
    },
    ...overrides,
  };
  return vi.fn(async (url, options = {}) => {
    const path = String(url);
    if (path === '/api/appliance/bootstrap') return json(bootstrap);
    if (path === '/api/appliance/setup') {
      return json({
        state: state.completed ? 'READY' : 'NEEDS_SETUP',
        missing: state.completed ? [] : ['config.source_url', 'credential.telegram'],
        can_complete: state.configSaved && state.credentialAssigned,
        current_config_values: {
          source_url: state.configSaved ? 'https://example.test/feed.json' : '',
          chat_id: '@dailyops',
          ai_provider: 'none',
        },
        config_schema: [
          { key: 'source_url', label: 'Source URL', type: 'url', required: true, test_kind: 'http_json_contract', description: 'Feed URL' },
          { key: 'chat_id', label: 'Telegram chat ID', type: 'string', required: true },
          { key: 'ai_provider', label: 'AI commentary', type: 'select', required: true, options: ['none', 'openai', 'deepseek'], description: 'Optional AI note' },
        ],
        credential_requirements: [
          { key: 'telegram', label: 'Telegram', type: 'TELEGRAM_BOT', required: true, test_kind: 'telegram_get_me', assigned: state.credentialAssigned },
          { key: 'openai', label: 'OpenAI API key', type: 'OPENAI_API_KEY', required: false, assigned: false },
        ],
        attention_category: state.attentionCategory,
      });
    }
    if (path === '/api/appliance/status') {
      return json({
        pack: bootstrap.pack,
        workflow_id: 'wf-1',
        server: 'ok',
        state: state.completed ? 'READY' : 'NEEDS_SETUP',
        missing: state.completed ? [] : ['config.source_url', 'credential.telegram'],
        can_complete: state.configSaved && state.credentialAssigned,
      });
    }
    if (path === '/api/appliance/workflow/status') {
      return json({
        workflow_id: 'wf-1',
        server: 'ok',
        state: state.runStarted ? 'RUNNING' : state.completed ? 'READY' : 'NEEDS_SETUP',
        workflow: { id: 'wf-1', name: 'Daily Digest', is_active: true, risk: 'low' },
        latest_execution: state.runStarted ? { id: 'exec-1', status: 'SUCCESS', duration_ms: 42, started_at: '2026-08-09T00:00:00Z' } : null,
      });
    }
    if (path === '/api/appliance/executions/latest') {
      return json({
        execution: state.runStarted ? { id: 'exec-1', status: 'SUCCESS', duration_ms: 42, started_at: '2026-08-09T00:00:00Z' } : null,
      });
    }
    if (path.startsWith('/api/appliance/executions')) {
      return json({
        limit: 10,
        executions: state.runStarted ? [{ id: 'exec-1', status: 'SUCCESS', duration_ms: 42, started_at: '2026-08-09T00:00:00Z' }] : [],
      });
    }
    if (path === '/api/appliance/schedule' && (!options.method || options.method === 'GET')) {
      return json(state.schedule);
    }
    if (path === '/api/appliance/schedule' && options.method === 'PUT') {
      const request = JSON.parse(options.body);
      if (request.expected_revision !== state.schedule.revision) {
        return json({ category: 'revision_conflict', error: 'The schedule changed in another session.' }, 409);
      }
      state.schedule = {
        ...state.schedule,
        configured: true,
        revision: state.schedule.revision + 1,
        enabled: request.enabled,
        local_time: request.local_time,
        timezone: request.timezone,
        state: request.enabled ? 'OK' : 'DISABLED',
        next_run_at: request.enabled ? '2026-08-09T01:05:00Z' : null,
      };
      return json(state.schedule);
    }
    if (path === '/api/appliance/setup/config' && options.method === 'POST') {
      state.configSaved = true;
      return json({ values: { source_url: 'https://example.test/feed.json' } });
    }
    if (path === '/api/appliance/setup/source/test' && options.method === 'POST') {
      if (state.sourceTestPromise) return state.sourceTestPromise;
      if (state.sourceTestError) return json({ status: 'INVALID', category: state.sourceTestError.category, message: state.sourceTestError.message }, 502);
      return json({ status: 'VALID', summary: { report_date: '2026-08-09', valid_fields: 7 } });
    }
    if (path === '/api/appliance/setup/credentials/create' && options.method === 'POST') {
      state.credentialAssigned = true;
      return json({ credentials: { telegram: { credential_type: 'TELEGRAM_BOT', assigned: true } } }, 201);
    }
    if (path === '/api/appliance/setup/credentials/test' && options.method === 'POST') {
      if (state.telegramTestError) return json({ status: 'INVALID', category: state.telegramTestError.category, message: state.telegramTestError.message }, 502);
      return json({ status: 'VALID', message: 'Bot token is valid and the configured chat is accessible.' });
    }
    if (path === '/api/appliance/setup/complete' && options.method === 'POST') {
      state.completed = true;
      return json({ state: 'READY', setup_complete: true });
    }
    if (path === '/api/appliance/workflow/run' && options.method === 'POST') {
      state.runStarted = true;
      return json({ execution_id: 'exec-1', workflow_id: 'wf-1', status: 'RUNNING' }, 202);
    }
    if (path === '/api/appliance/diagnostics') return json(state.diagnostics);
    return json({});
  });
}

describe('appliance UI', () => {
  it('shows the runtime version and public beta status', async () => {
    vi.stubGlobal('fetch', applianceFetch());
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    expect(root.textContent).toContain('Goflow 0.5.0-test');
    expect(root.textContent).toContain('Unsigned public beta');
  });

  it('renders select config and marks optional credentials clearly', async () => {
    vi.stubGlobal('fetch', applianceFetch());
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    const provider = root.querySelector('#pack-config-ai_provider');
    expect(provider?.tagName).toBe('SELECT');
    expect([...provider.options].map((option) => option.value)).toEqual(['none', 'openai', 'deepseek']);
    expect(provider.value).toBe('none');

    const openai = [...root.querySelectorAll('.credential-row')].find((row) => row.textContent.includes('OpenAI API key'));
    expect(openai?.textContent).toContain('Optional');
    expect(openai?.textContent).not.toContain('Required');
  });

  it('falls back to generic workspace when appliance bootstrap is absent', async () => {
    vi.stubGlobal('fetch', vi.fn(async (url) => {
      if (String(url) === '/api/appliance/bootstrap') return new Response('', { status: 404 });
      if (String(url).includes('/nodes/definitions')) return json([]);
      if (String(url).includes('/workflows')) return json([]);
      return json({});
    }));
    const { root } = await mountWithApp(App, { route: '/workflows' });
    await nextFrame();

    expect(root.textContent).toContain('Workspace');
    expect(root.textContent).toContain('Workflows');
    expect(root.textContent).not.toContain('Pack appliance');
  });

  it('completes setup without keeping submitted secret in the DOM', async () => {
    vi.stubGlobal('fetch', applianceFetch());
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    const sourceUrl = root.querySelector('#pack-config-source_url');
    sourceUrl.value = 'https://example.test/feed.json';
    sourceUrl.dispatchEvent(new Event('input'));
    await nextFrame();
    root.querySelector('form button[type="submit"]').click();
    await nextFrame();

    const secretInput = root.querySelector('input[type="password"]');
    secretInput.value = '123:secret-canary';
    secretInput.dispatchEvent(new Event('input'));
    await nextFrame();
    root.querySelector('.credential-controls .btn-secondary').click();
    await nextFrame();
    await nextFrame();

    expect(root.textContent).toContain('Credential saved');
    expect(root.textContent).not.toContain('secret-canary');
    expect(root.querySelector('input[type="password"]')?.value).toBe('');
  });

  it('keeps scheduling off by default and saves a local daily schedule', async () => {
    const fetchMock = applianceFetch({ configSaved: true, credentialAssigned: true });
    vi.stubGlobal('fetch', fetchMock);
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    const enabled = root.querySelector('#schedule-title').closest('form').querySelector('input[type="checkbox"]');
    expect(enabled.checked).toBe(false);
    enabled.click();
    const time = root.querySelector('input[type="time"]');
    time.value = '08:05';
    time.dispatchEvent(new Event('input'));
    const timezone = root.querySelector('input[list="appliance-timezones"]');
    timezone.value = 'Asia/Bangkok';
    timezone.dispatchEvent(new Event('input'));
    root.querySelector('#schedule-title').closest('form').querySelector('button[type="submit"]').click();
    await nextFrame();
    await nextFrame();

    const request = fetchMock.mock.calls.find(([url, options]) => String(url) === '/api/appliance/schedule' && options.method === 'PUT');
    expect(JSON.parse(request[1].body)).toEqual({
      expected_revision: 0,
      enabled: true,
      local_time: '08:05',
      timezone: 'Asia/Bangkok',
    });
    expect(root.textContent).toContain('Schedule saved');
  });

  it('explains migration review without asking for the assigned credential again', async () => {
    vi.stubGlobal('fetch', applianceFetch({
      configSaved: true,
      credentialAssigned: true,
      attentionCategory: 'config',
    }));
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    expect(root.textContent).toContain('migrated non-secret setup fields');
    expect(root.textContent).toContain('Assigned');
    expect(root.textContent).not.toContain('credential-');
  });

  it('shows schedule, next run, last scheduled result, and manual state on the dashboard', async () => {
    vi.stubGlobal('fetch', applianceFetch({
      configSaved: true,
      credentialAssigned: true,
      completed: true,
      schedule: {
        configured: true,
        revision: 3,
        enabled: true,
        kind: 'daily',
        local_time: '08:05',
        timezone: 'Asia/Bangkok',
        state: 'OK',
        next_run_at: '2026-08-10T01:05:00Z',
        last_scheduled_for: '2026-08-09T01:05:00Z',
        last_result: { id: 'scheduled-1', status: 'SUCCESS', trigger_source: 'schedule' },
      },
    }));
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    const scheduleSection = root.querySelector('#schedule-status-title').closest('section');
    expect(scheduleSection.textContent).toContain('Enabled');
    expect(scheduleSection.textContent).toContain('Asia/Bangkok');
    expect(scheduleSection.textContent).toContain('SUCCESS');
    expect(scheduleSection.textContent).toContain('Ready');
  });

  it('runs the workflow and renders latest execution plus redacted diagnostics', async () => {
    vi.stubGlobal('fetch', applianceFetch({ configSaved: true, credentialAssigned: true, completed: true }));
    const writeText = vi.fn(async () => {});
    vi.stubGlobal('navigator', {
      clipboard: { writeText },
    });
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    expect(root.querySelector('.run-panel textarea')).toBeNull();
    expect(root.textContent).not.toContain('Run input');
    expect(root.textContent).toContain('Example Appliance');

    root.querySelector('.run-panel button[type="submit"]').click();
    await nextFrame();
    await nextFrame();

    expect(root.textContent).toContain('Daily Digest');
    expect(root.textContent).toContain('Latest execution');
    expect(root.textContent).toContain('SUCCESS');

    root.querySelector('.appliance-side .btn-secondary').click();
    await nextFrame();
    expect(root.textContent).toContain('Local diagnostics');
    expect(root.textContent).toContain('"secrets_hidden": true');
    expect(root.textContent).not.toContain('secret-canary');

    const copy = [...root.querySelectorAll('button')].find((button) => button.textContent.includes('Copy diagnostics'));
    copy.click();
    await nextFrame();
    expect(writeText).toHaveBeenCalledOnce();
    expect(writeText.mock.calls[0][0]).toContain('"local_only": true');
    expect(writeText.mock.calls[0][0]).not.toContain('secret-canary');
  });

  it('requires tests for current source and Telegram values before completion', async () => {
    const fetchMock = applianceFetch({ configSaved: true, credentialAssigned: true });
    vi.stubGlobal('fetch', fetchMock);
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    const complete = root.querySelector('.panel-actions .btn-primary');
    expect(complete.disabled).toBe(true);
    expect(root.textContent).toContain('Not tested');

    root.querySelector('.test-row button').click();
    await nextFrame();
    await nextFrame();
    expect(root.textContent).toContain('7 required fields valid');

    root.querySelector('.credential-controls .toolbar-actions button:last-child').click();
    await nextFrame();
    await nextFrame();
    expect(root.textContent).toContain('Bot token is valid and the configured chat is accessible.');
    expect(complete.disabled).toBe(false);

    const chat = root.querySelector('#pack-config-chat_id');
    chat.value = '@changed_chat';
    chat.dispatchEvent(new Event('input'));
    await nextFrame();
    expect(root.querySelector('.credential-row .test-state strong').textContent).toContain('Not tested');
    expect(complete.disabled).toBe(true);

    const source = root.querySelector('#pack-config-source_url');
    source.value = 'https://example.test/changed.json';
    source.dispatchEvent(new Event('input'));
    await nextFrame();
    expect(complete.disabled).toBe(true);
    expect(root.querySelector('.test-row .test-state strong').textContent).toContain('Not tested');
  });

  it('shows Testing until the source request resolves', async () => {
    let resolveSource;
    const sourceTestPromise = new Promise((resolve) => {
      resolveSource = resolve;
    });
    vi.stubGlobal('fetch', applianceFetch({ configSaved: true, credentialAssigned: true, sourceTestPromise }));
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    root.querySelector('.test-row button').click();
    await nextFrame();
    expect(root.querySelector('.test-row .test-state strong').textContent).toContain('Testing');

    resolveSource(json({ status: 'VALID', summary: { report_date: '2026-08-09', valid_fields: 7 } }));
    await nextFrame();
    await nextFrame();
    expect(root.querySelector('.test-row .test-state strong').textContent).toContain('Valid');
  });

  it('shows friendly categorized source and Telegram failures without secrets', async () => {
    vi.stubGlobal('fetch', applianceFetch({
      configSaved: true,
      credentialAssigned: true,
      sourceTestError: { category: 'source_invalid', message: 'The source configuration or response is invalid.' },
      telegramTestError: { category: 'telegram_chat_not_found', message: 'Telegram could not access the configured destination.' },
    }));
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    root.querySelector('.test-row button').click();
    await nextFrame();
    await nextFrame();
    expect(root.textContent).toContain('The source configuration or response is invalid.');

    root.querySelector('.credential-controls .toolbar-actions button:last-child').click();
    await nextFrame();
    await nextFrame();
    expect(root.textContent).toContain('Telegram could not access the configured destination.');
    expect(root.textContent).not.toContain('secret-canary');
  });

  it('coalesces a double click and polls until the execution is terminal', async () => {
    let runCalls = 0;
    let runStatus = 'SUCCESS';
    let workflowPolls = 0;
    const baseFetch = applianceFetch({ configSaved: true, credentialAssigned: true, completed: true });
    vi.stubGlobal('fetch', vi.fn(async (url, options = {}) => {
      const path = String(url);
      if (path === '/api/appliance/workflow/run' && options.method === 'POST') {
        runCalls += 1;
        runStatus = 'RUNNING';
        return json({ execution_id: 'exec-live', workflow_id: 'wf-1', status: 'RUNNING' }, 202);
      }
      if (path === '/api/appliance/workflow/status') {
        workflowPolls += 1;
        if (workflowPolls >= 4) runStatus = 'SUCCESS';
        return json({ workflow_id: 'wf-1', server: 'ok', state: runStatus === 'RUNNING' ? 'RUNNING' : 'READY', workflow: { id: 'wf-1', name: 'Daily Digest', is_active: true }, latest_execution: { id: 'exec-live', status: runStatus, duration_ms: runStatus === 'SUCCESS' ? 80 : 0 } });
      }
      if (path === '/api/appliance/executions/latest') return json({ execution: { id: 'exec-live', status: runStatus, duration_ms: runStatus === 'SUCCESS' ? 80 : 0 } });
      if (path.startsWith('/api/appliance/executions')) return json({ executions: [{ id: 'exec-live', status: runStatus, duration_ms: runStatus === 'SUCCESS' ? 80 : 0 }], limit: 10 });
      return baseFetch(url, options);
    }));
    const { root, unmount } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();
    const run = root.querySelector('.run-panel button[type="submit"]');
    run.click();
    run.click();
    await nextFrame();
    await nextFrame();
    expect(runCalls).toBe(1);
    expect(run.textContent).toContain('Running');
    expect(run.disabled).toBe(true);

    await new Promise((resolve) => setTimeout(resolve, 1700));
    await nextFrame();
    expect(root.textContent).toContain('SUCCESS');
    expect(run.disabled).toBe(false);
    unmount();
  });

  it('treats already_running as tracked work instead of a generic error', async () => {
    let active = false;
    let statusReads = 0;
    const baseFetch = applianceFetch({ configSaved: true, credentialAssigned: true, completed: true });
    vi.stubGlobal('fetch', vi.fn(async (url, options = {}) => {
      const path = String(url);
      if (path === '/api/appliance/workflow/run' && options.method === 'POST') {
        active = true;
        return json({ category: 'already_running', error: 'This workflow is already running.' }, 409);
      }
      if (path === '/api/appliance/workflow/status' && active) {
        statusReads += 1;
        const executionStatus = statusReads > 1 ? 'SUCCESS' : 'RUNNING';
        if (executionStatus === 'SUCCESS') active = false;
        return json({ workflow_id: 'wf-1', server: 'ok', state: executionStatus === 'RUNNING' ? 'RUNNING' : 'READY', workflow: { id: 'wf-1', name: 'Daily Digest', is_active: true }, latest_execution: { id: 'exec-active', status: executionStatus } });
      }
      if (path === '/api/appliance/executions/latest' && active) return json({ execution: { id: 'exec-active', status: 'RUNNING' } });
      if (path.startsWith('/api/appliance/executions') && active) return json({ executions: [{ id: 'exec-active', status: 'RUNNING' }], limit: 10 });
      return baseFetch(url, options);
    }));
    const { root, unmount } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();
    root.querySelector('.run-panel button[type="submit"]').click();
    await nextFrame();
    await nextFrame();
    expect(root.textContent).toContain('A report run is already in progress');
    expect(root.textContent).not.toContain('Appliance request failed');
    unmount();
  });
});