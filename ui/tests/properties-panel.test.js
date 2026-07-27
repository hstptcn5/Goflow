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
      { name: 'url', label: 'Target URL', type: 'url', required: true, default: '' },
      { name: 'headers', label: 'Headers (JSON)', type: 'json', default: '{}' },
    ],
  },
  {
    type: 'conditionIf',
    name: 'IF / ELSE Condition',
    category: 'LOGIC',
    description: 'Branch',
    params: [
      { name: 'field', label: 'Input Value', type: 'text', required: true, default: '' },
      { name: 'operator', label: 'Operator', type: 'select', required: true, default: 'equals', options: ['equals', 'contains'] },
      { name: 'value', label: 'Compare Value', type: 'text', required: true, default: '' },
    ],
  },
  {
    type: 'telegramBot',
    name: 'Telegram Bot',
    category: 'COMMUNICATION',
    description: 'Send message',
    params: [{ name: 'credential_id', label: 'Credential Token', type: 'credential', required: true, default: '' }],
  },
  {
    type: 'delaySleep',
    name: 'Delay / Sleep',
    category: 'UTILITY',
    description: 'Wait',
    params: [{ name: 'seconds', label: 'Delay Duration (Seconds)', type: 'number', required: true, default: '1' }],
  },
  {
    type: 'integerCheck',
    name: 'Integer Check',
    category: 'UTILITY',
    description: 'Integer field test',
    params: [{ name: 'count', label: 'Count', type: 'integer', required: true, default: '1' }],
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

  it('uses the active execution result for header status instead of global realtime node status', async () => {
    const selectedNode = { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: { json_template: '{}' } };
    const { root } = await mountWithApp(PropertiesPanel, {
      props: {
        selectedNode,
        selectedExecution: {
          id: 'exec_failed',
          node_logs: [
            { node_id: 'json_1', status: 'FAILED', error: 'failed in selected execution', attempts: 1, duration_ms: 9 },
          ],
        },
        executionContextMode: 'selected',
      },
    });
    const store = useWorkflowStore();
    const executionStore = useExecutionStore();
    store.nodeDefinitions = nodeDefs;
    executionStore.nodeStatuses = { json_1: 'SUCCESS' };
    executionStore.nodeEventsByExecution = {
      exec_live: {
        json_1: { node_id: 'json_1', execution_id: 'exec_live', status: 'SUCCESS', output: { ok: true } },
      },
    };
    await nextFrame();

    expect(root.querySelector('.execution-context strong').textContent).toContain('FAILED');
    expect(root.querySelector('.execution-context').textContent).toContain('Selected execution exec_failed');
    expect(root.querySelector('[aria-selected="true"]').textContent).toContain('Logs');
    expect(root.textContent).toContain('failed in selected execution');
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

  it('keeps Fixed and Expression mode state consistent when converting back to a literal', async () => {
    const selectedNode = { id: 'http_1', type: 'httpRequest', name: 'HTTP Request', params: { method: 'GET', url: 'https://old.example', headers: '{}' } };
    const nodes = [
      { id: 'json_1', data: { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: {} } },
      { id: 'http_1', data: selectedNode },
    ];
    const edges = [{ id: 'e1', source: 'json_1', target: 'http_1' }];
    let updated = selectedNode.params;
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
      node_logs: [
        { node_id: 'json_1', status: 'SUCCESS', output: { transformed: { url: 'https://api.example.test/users' } }, attempts: 1, duration_ms: 3 },
      ],
    }];
    await nextFrame();

    const toggle = root.querySelector('[data-param-name="url"]');
    toggle.querySelectorAll('button')[1].click();
    await nextFrame();
    expect(root.textContent).toContain('Expression mode is active');
    const urlRow = Array.from(root.querySelectorAll('.tree-row')).find((row) => row.textContent.includes('transformed.url'));
    urlRow.click();
    await nextFrame();
    expect(updated.url).toBe('{{json_1.transformed.url}}');
    expect(toggle.querySelectorAll('button')[1].classList.contains('active')).toBe(true);

    toggle.querySelectorAll('button')[0].click();
    await nextFrame();
    expect(updated.url).toBe('https://api.example.test/users');
    expect(toggle.querySelectorAll('button')[0].classList.contains('active')).toBe(true);
  });

  it('renders number expressions as editable text and converts primitive previews back to fixed numbers', async () => {
    const selectedNode = { id: 'delay_1', type: 'delaySleep', name: 'Delay / Sleep', params: { seconds: '1' } };
    const nodes = [
      { id: 'json_1', data: { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: {} } },
      { id: 'delay_1', data: selectedNode },
    ];
    const edges = [{ id: 'e1', source: 'json_1', target: 'delay_1' }];
    let updated = selectedNode.params;
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
      node_logs: [
        { node_id: 'json_1', status: 'SUCCESS', output: { transformed: { score: 42 } }, attempts: 1, duration_ms: 3 },
      ],
    }];
    await nextFrame();

    const toggle = root.querySelector('[data-param-name="seconds"]');
    const input = root.querySelector('[aria-label="Delay Duration (Seconds)"]');
    expect(input.type).toBe('number');

    toggle.querySelectorAll('button')[1].click();
    await nextFrame();
    const scoreRow = Array.from(root.querySelectorAll('.tree-row')).find((row) => row.textContent.includes('transformed.score'));
    scoreRow.click();
    await nextFrame();

    const expressionInput = root.querySelector('[aria-label="Delay Duration (Seconds)"]');
    expect(updated.seconds).toBe('{{json_1.transformed.score}}');
    expect(expressionInput.type).toBe('text');
    expect(expressionInput.value).toBe('{{json_1.transformed.score}}');

    toggle.querySelectorAll('button')[0].click();
    await nextFrame();
    const fixedInput = root.querySelector('[aria-label="Delay Duration (Seconds)"]');
    expect(updated.seconds).toBe('42');
    expect(fixedInput.type).toBe('number');
    expect(fixedInput.value).toBe('42');
  });

  it('keeps object and array expressions in Expression mode unless JSON literal conversion is explicit', async () => {
    const selectedNode = { id: 'json_2', type: 'jsonTransform', name: 'JSON Transform', params: { json_template: '{{json_1.transformed.profile}}' } };
    const nodes = [
      { id: 'json_1', data: { id: 'json_1', type: 'jsonTransform', name: 'Source JSON', params: {} } },
      { id: 'json_2', data: selectedNode },
    ];
    const edges = [{ id: 'e1', source: 'json_1', target: 'json_2' }];
    let updated = selectedNode.params;
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
      node_logs: [
        { node_id: 'json_1', status: 'SUCCESS', output: { transformed: { profile: { role: 'admin' }, items: [1, 2] } } },
      ],
    }];
    await nextFrame();

    const toggle = root.querySelector('[data-param-name="json_template"]');
    expect(toggle.querySelectorAll('button')[1].classList.contains('active')).toBe(true);
    toggle.querySelectorAll('button')[0].click();
    await nextFrame();
    expect(updated.json_template).toBe('{{json_1.transformed.profile}}');
    expect(toggle.querySelectorAll('button')[1].classList.contains('active')).toBe(true);
    expect(root.textContent).toContain('Object and array previews stay in Expression mode');

    const convertButton = Array.from(root.querySelectorAll('button.mini-btn')).find((button) => button.textContent.includes('Convert to JSON literal'));
    convertButton.click();
    await nextFrame();
    expect(updated.json_template).toContain('"role": "admin"');
    expect(toggle.querySelectorAll('button')[0].classList.contains('active')).toBe(true);

    updated = { ...updated, json_template: '{{json_1.transformed.items}}' };
    selectedNode.params = updated;
    toggle.querySelectorAll('button')[1].click();
    await nextFrame();
    toggle.querySelectorAll('button')[0].click();
    await nextFrame();
    expect(updated.json_template).toBe('{{json_1.transformed.items}}');
    expect(toggle.querySelectorAll('button')[1].classList.contains('active')).toBe(true);
  });

  it('keeps Expression mode when preview is unavailable', async () => {
    const selectedNode = { id: 'integer_1', type: 'integerCheck', name: 'Integer Check', params: { count: '{{json_1.transformed.missing}}' } };
    const nodes = [
      { id: 'json_1', data: { id: 'json_1', type: 'jsonTransform', name: 'Source JSON', params: {} } },
      { id: 'integer_1', data: selectedNode },
    ];
    const edges = [{ id: 'e1', source: 'json_1', target: 'integer_1' }];
    let updated = selectedNode.params;
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
      node_logs: [
        { node_id: 'json_1', status: 'SUCCESS', output: { transformed: { score: 1 } } },
      ],
    }];
    await nextFrame();

    const toggle = root.querySelector('[data-param-name="count"]');
    toggle.querySelectorAll('button')[0].click();
    await nextFrame();
    expect(updated.count).toBe('{{json_1.transformed.missing}}');
    expect(toggle.querySelectorAll('button')[1].classList.contains('active')).toBe(true);
    expect(root.textContent).toContain('Preview is unavailable');

  });

  it('validates integer literals in Fixed mode', async () => {
    const selectedNode = { id: 'integer_1', type: 'integerCheck', name: 'Integer Check', params: { count: '1.5' } };
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode } });
    const store = useWorkflowStore();
    store.nodeDefinitions = nodeDefs;
    await nextFrame();

    const input = root.querySelector('[aria-label="Count"]');
    expect(input.type).toBe('number');
    expect(root.textContent).toContain('Enter a valid integer.');
  });

  it('lists transitive upstream sources but not downstream or unrelated nodes', async () => {
    const selectedNode = { id: 'if_1', type: 'conditionIf', name: 'IF / ELSE Condition', params: { field: '', operator: 'equals', value: 'ok' } };
    const nodes = [
      { id: 'json_a', data: { id: 'json_a', type: 'jsonTransform', name: 'JSON A', params: {} } },
      { id: 'json_b', data: { id: 'json_b', type: 'jsonTransform', name: 'JSON B', params: {} } },
      { id: 'if_1', data: selectedNode },
      { id: 'downstream', data: { id: 'downstream', type: 'jsonTransform', name: 'Downstream', params: {} } },
      { id: 'unrelated', data: { id: 'unrelated', type: 'jsonTransform', name: 'Unrelated', params: {} } },
    ];
    const edges = [
      { id: 'a-b', source: 'json_a', target: 'json_b' },
      { id: 'b-if', source: 'json_b', target: 'if_1' },
      { id: 'if-down', source: 'if_1', target: 'downstream' },
    ];
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode, nodes, edges } });
    const store = useWorkflowStore();
    const executionStore = useExecutionStore();
    store.nodeDefinitions = nodeDefs;
    executionStore.executionLogs = [{
      id: 'exec_1',
      node_logs: [
        { node_id: 'json_a', status: 'SUCCESS', output: { a: 1 } },
        { node_id: 'json_b', status: 'SUCCESS', output: { b: 2 } },
        { node_id: 'downstream', status: 'SUCCESS', output: { down: 3 } },
        { node_id: 'unrelated', status: 'SUCCESS', output: { unrelated: 4 } },
      ],
    }];
    await nextFrame();

    const options = Array.from(root.querySelectorAll('[aria-label="Source node selector"] option')).map((option) => option.textContent);
    expect(options.some((text) => text.includes('JSON B'))).toBe(true);
    expect(options.some((text) => text.includes('JSON A'))).toBe(true);
    expect(options.some((text) => text.includes('Downstream'))).toBe(false);
    expect(options.some((text) => text.includes('Unrelated'))).toBe(false);
  });

  it('supports semantic tab keyboard navigation and redacts rendered errors', async () => {
    const selectedNode = { id: 'json_1', type: 'jsonTransform', name: 'JSON Transform', params: { json_template: '{}' } };
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode } });
    const store = useWorkflowStore();
    const executionStore = useExecutionStore();
    store.nodeDefinitions = nodeDefs;
    executionStore.executionLogs = [{
      id: 'exec_1',
      node_logs: [
        { node_id: 'json_1', status: 'FAILED', error: 'Authorization: Bearer raw-token access_token=secret-token', attempts: 1 },
      ],
    }];
    await nextFrame();

    const tabs = root.querySelectorAll('[role="tab"]');
    expect(tabs).toHaveLength(4);
    tabs[0].click();
    await nextFrame();
    root.querySelector('.panel-tabs').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    await nextFrame();
    expect(root.querySelector('[aria-selected="true"]').textContent).toContain('Input');

    tabs[3].click();
    await nextFrame();
    expect(root.textContent).toContain('[REDACTED]');
    expect(root.textContent).not.toContain('raw-token');
    expect(root.textContent).not.toContain('secret-token');
  });
});
