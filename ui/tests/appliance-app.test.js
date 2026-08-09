import { describe, expect, it, vi } from 'vitest';
import App from '../src/App.vue';
import ApplianceApp from '../src/components/ApplianceApp.vue';
import { mountWithApp, nextFrame } from './mount';

const bootstrap = {
  token: 'session-token',
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
    diagnostics: {
      pack: bootstrap.pack,
      state: 'READY',
      latest_execution: { id: 'exec-1', status: 'SUCCESS', duration_ms: 42 },
      secrets_hidden: true,
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
        current_config_values: state.configSaved ? { source_url: 'https://example.test/feed.json' } : {},
        config_schema: [
          { key: 'source_url', label: 'Source URL', type: 'url', required: true, description: 'Feed URL' },
        ],
        credential_requirements: [
          { key: 'telegram', label: 'Telegram', type: 'TELEGRAM_BOT', required: true, test_kind: 'telegram_get_me', assigned: state.credentialAssigned },
        ],
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
    if (path === '/api/appliance/setup/config' && options.method === 'POST') {
      state.configSaved = true;
      return json({ values: { source_url: 'https://example.test/feed.json' } });
    }
    if (path === '/api/appliance/setup/credentials/create' && options.method === 'POST') {
      state.credentialAssigned = true;
      return json({ credentials: { telegram: { credential_type: 'TELEGRAM_BOT', assigned: true } } }, 201);
    }
    if (path === '/api/appliance/setup/credentials/test' && options.method === 'POST') {
      return json({ status: 'OK' });
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

  it('runs the workflow and renders latest execution plus redacted diagnostics', async () => {
    vi.stubGlobal('fetch', applianceFetch({ configSaved: true, credentialAssigned: true, completed: true }));
    vi.stubGlobal('navigator', {
      clipboard: { writeText: vi.fn(async () => {}) },
    });
    const { root } = await mountWithApp(ApplianceApp, { props: { bootstrap } });
    await nextFrame();

    root.querySelector('.run-panel button[type="submit"]').click();
    await nextFrame();
    await nextFrame();

    expect(root.textContent).toContain('Daily Digest');
    expect(root.textContent).toContain('Latest execution');
    expect(root.textContent).toContain('SUCCESS');

    root.querySelector('.appliance-side .btn-secondary').click();
    await nextFrame();
    expect(root.textContent).toContain('"secrets_hidden": true');
    expect(root.textContent).not.toContain('secret-canary');
  });
});
