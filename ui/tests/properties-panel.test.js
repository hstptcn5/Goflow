import { describe, expect, it } from 'vitest';
import PropertiesPanel from '../src/components/PropertiesPanel.vue';
import { useExecutionStore } from '../src/stores/executionStore';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { mountWithApp, nextFrame } from './mount';

const nodeDefs = [
  {
    type: 'jsonTransform',
    name: 'JSON Transform',
    category: 'ACTION',
    description: 'Create JSON',
    params: [{ name: 'json_template', label: 'JSON Structure', type: 'json', required: true, default: '' }],
  },
  {
    type: 'httpRequest',
    name: 'HTTP Request',
    category: 'ACTION',
    description: 'Send request',
    params: [
      { name: 'method', label: 'HTTP Method', type: 'select', default: 'GET', options: ['GET', 'POST'] },
      { name: 'url', label: 'Target URL', type: 'text', required: true, default: '' },
      { name: 'headers', label: 'Headers (JSON)', type: 'json', default: '{}' },
    ],
  },
  {
    type: 'telegramBot',
    name: 'Telegram Bot',
    category: 'COMMUNICATION',
    description: 'Send message',
    params: [{ name: 'credential_id', label: 'Credential Token', type: 'credential', required: true, default: '' }],
  },
];

describe('PropertiesPanel', () => {
  it('renders four inspector tabs with inline validation and credential action', async () => {
    const selectedNode = { id: 'telegram_1', type: 'telegramBot', name: 'Telegram Bot', params: { credential_id: '' } };
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode, validationIssues: [] } });
    const store = useWorkflowStore();
    store.nodeDefinitions = nodeDefs;
    await nextFrame();

    expect(root.textContent).toContain('Parameters');
    expect(root.textContent).toContain('Input');
    expect(root.textContent).toContain('Output');
    expect(root.textContent).toContain('Logs');
    expect(root.textContent).toContain('Select a credential before testing or activating this workflow.');
    expect(root.textContent).toContain('Create credential');
  });

  it('inserts a data picker expression and previews resolved value', async () => {
    const selectedNode = { id: 'http_1', type: 'httpRequest', name: 'HTTP Request', params: { method: 'GET', url: '', headers: '{}' } };
    const nodes = [
      { id: 'json_1', data: { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: {} } },
      { id: 'http_1', data: selectedNode },
    ];
    const edges = [{ id: 'e1', source: 'json_1', target: 'http_1' }];
    let updated = null;
    const { root } = await mountWithApp(PropertiesPanel, {
      props: {
        selectedNode,
        nodes,
        edges,
        onUpdateNodeParams: (_id, params) => { updated = params; selectedNode.params = params; },
      },
    });
    const store = useWorkflowStore();
    const executionStore = useExecutionStore();
    store.nodeDefinitions = nodeDefs;
    executionStore.executionLogs = [{
      id: 'exec_1',
      logs_json: JSON.stringify([
        { node_id: 'json_1', status: 'SUCCESS', output: { transformed: { user: { email: 'dev@example.com' } }, password: 'secret' }, attempts: 1, duration_ms: 3 },
      ]),
    }];
    await nextFrame();

    root.querySelector('[data-param-name="url"] button:nth-child(2)').click();
    await nextFrame();
    const emailRow = Array.from(root.querySelectorAll('.tree-row')).find((row) => row.textContent.includes('transformed.user.email'));
    emailRow.click();
    await nextFrame();

    expect(updated.url).toBe('{{json_1.transformed.user.email}}');
    expect(root.textContent).toContain('dev@example.com');
    expect(root.textContent).not.toContain('secret');
  });

  it('shows output and logs for the selected execution without leaking secrets', async () => {
    const selectedNode = { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: { json_template: '{}' } };
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode } });
    const store = useWorkflowStore();
    const executionStore = useExecutionStore();
    store.nodeDefinitions = nodeDefs;
    executionStore.executionLogs = [{
      id: 'exec_1',
      started_at: '2026-07-26T01:00:00Z',
      finished_at: '2026-07-26T01:00:01Z',
      logs_json: JSON.stringify([
        { node_id: 'json_1', status: 'SUCCESS', output: { result: [{ id: 1, access_token: 'secret-token' }] }, attempts: 2, duration_ms: 10 },
      ]),
    }];
    await nextFrame();

    root.querySelectorAll('.tab-btn')[2].click();
    await nextFrame();
    expect(root.textContent).toContain('Status: SUCCESS');
    expect(root.textContent).toContain('Attempts: 2');
    expect(root.textContent).not.toContain('secret-token');

    root.querySelectorAll('.tab-btn')[3].click();
    await nextFrame();
    expect(root.textContent).toContain('Execution ID');
    expect(root.textContent).not.toContain('secret-token');
  });

  it('uses undoable parameter updates through the editor integration', async () => {
    const selectedNode = { id: 'http_1', type: 'httpRequest', name: 'HTTP Request', params: { method: 'GET', url: '', headers: '{}' } };
    let calls = 0;
    const { root } = await mountWithApp(PropertiesPanel, {
      props: {
        selectedNode,
        onUpdateNodeParams: () => { calls += 1; },
      },
    });
    const store = useWorkflowStore();
    store.nodeDefinitions = nodeDefs;
    await nextFrame();

    root.querySelector('[aria-label="Target URL"]').value = 'https://example.com';
    root.querySelector('[aria-label="Target URL"]').dispatchEvent(new Event('input'));
    await nextFrame();
    expect(calls).toBe(1);
  });
});
