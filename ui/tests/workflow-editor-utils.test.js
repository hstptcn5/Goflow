import { describe, expect, it } from 'vitest';
import {
  autoLayoutGraph,
  buildExecutionPathState,
  createWorkflowEdge,
  createWorkflowNode,
  generateGraphId,
  graphFingerprint,
  splitValidationIssues,
  validateWorkflowGraph,
  validationStateForNode,
} from '../src/utils/workflowEditor';

const defs = [
  {
    type: 'manualTrigger',
    name: 'Manual Trigger',
    category: 'TRIGGER',
    description: 'Start manually',
    params: [],
  },
  {
    type: 'conditionIf',
    name: 'IF / ELSE Condition',
    category: 'LOGIC',
    description: 'Branch',
    params: [
      { name: 'left', label: 'Left value', type: 'text', required: true, default: '' },
      { name: 'operator', label: 'Operator', type: 'select', required: true, default: 'equals' },
    ],
  },
  {
    type: 'httpRequest',
    name: 'HTTP Request',
    category: 'ACTION',
    description: 'Send HTTP',
    params: [
      { name: 'url', label: 'URL', type: 'url', required: true, default: '' },
      { name: 'timeout', label: 'Timeout', type: 'number', default: '30' },
      { name: 'method', label: 'Method', type: 'select', default: 'GET', options: ['GET', 'POST'] },
      { name: 'headers', label: 'Headers', type: 'json', default: '{}' },
    ],
  },
  {
    type: 'telegramBot',
    name: 'Telegram Bot',
    category: 'COMMUNICATION',
    description: 'Send Telegram',
    params: [
      { name: 'credential_id', label: 'Credential', type: 'credential', required: true, default: '' },
    ],
  },
];

describe('workflow editor utilities', () => {
  it('creates idle edges without animation', () => {
    const edge = createWorkflowEdge('a', 'b');
    expect(edge.animated).toBe(false);
    expect(edge.style.strokeWidth).toBe(2);
  });

  it('generates unique node and edge IDs for rapid graph edits', () => {
    const ids = new Set();
    for (let i = 0; i < 500; i += 1) {
      ids.add(generateGraphId('node'));
      ids.add(generateGraphId('edge'));
    }
    expect(ids.size).toBe(1000);
  });

  it('validates missing fields, credentials, unknown nodes, bad edges, duplicate ids, and cycles', () => {
    const nodes = [
      createWorkflowNode(defs[1], { x: 0, y: 0 }, 'a'),
      createWorkflowNode(defs[3], { x: 0, y: 0 }, 'b'),
      { id: 'b', type: 'custom', data: { id: 'b', type: 'missingNode', name: 'Missing', params: {} } },
    ];
    const edges = [
      { id: 'e1', source: 'a', target: 'missing' },
      { id: 'e2', source: 'a', target: 'a' },
    ];
    const issues = validateWorkflowGraph(nodes, edges, defs);
    expect(issues.map((issue) => issue.type)).toEqual(expect.arrayContaining([
      'missing_required',
      'missing_credential',
      'unknown_node',
      'duplicate_node_id',
      'invalid_edge_reference',
      'graph_cycle',
    ]));
    expect(validationStateForNode('b', issues)).toBe('Unknown node');
    const split = splitValidationIssues(issues);
    expect(split.hard.map((issue) => issue.type)).toEqual(expect.arrayContaining([
      'duplicate_node_id',
      'invalid_edge_reference',
      'graph_cycle',
      'unknown_node',
    ]));
    expect(split.soft.map((issue) => issue.type)).toEqual(expect.arrayContaining([
      'missing_required',
      'missing_credential',
    ]));
  });

  it('auto-layout keeps IDs and moves connected nodes left to right', () => {
    const nodes = [
      createWorkflowNode(defs[0], { x: 300, y: 300 }, 'start'),
      createWorkflowNode(defs[1], { x: 10, y: 10 }, 'branch'),
    ];
    const laidOut = autoLayoutGraph(nodes, [{ id: 'e', source: 'start', target: 'branch' }]);
    expect(laidOut.map((node) => node.id)).toEqual(['start', 'branch']);
    expect(laidOut[1].position.x).toBeGreaterThan(laidOut[0].position.x);
  });

  it('fingerprints equivalent graph snapshots consistently', () => {
    const nodes = [createWorkflowNode(defs[0], { x: 10, y: 20 }, 'a')];
    expect(graphFingerprint(nodes, [])).toBe(graphFingerprint([{ ...nodes[0] }], []));
  });

  it('validates expression references against upstream runtime placeholder syntax', () => {
    const start = createWorkflowNode(defs[0], { x: 0, y: 0 }, 'start');
    const branch = createWorkflowNode(defs[1], { x: 260, y: 0 }, 'branch');
    branch.data.params.left = '{{missing.value}}';
    const missing = validateWorkflowGraph([start, branch], [{ id: 'e1', source: 'start', target: 'branch' }], defs);
    expect(missing.map((issue) => issue.type)).toContain('invalid_expression_reference');

    branch.data.params.left = '{{start.payload.email}}';
    const valid = validateWorkflowGraph([start, branch], [{ id: 'e1', source: 'start', target: 'branch' }], defs);
    expect(valid.map((issue) => issue.type)).not.toContain('invalid_expression_reference');
  });

  it('uses inspector field validation contract for pre-run soft errors', () => {
    const http = createWorkflowNode(defs[2], { x: 0, y: 0 }, 'http');
    http.data.params = { url: 'not-a-url', timeout: 'abc', method: 'PATCH', headers: '{bad' };
    const issues = validateWorkflowGraph([http], [], defs);
    expect(issues.map((issue) => issue.type)).toEqual(expect.arrayContaining([
      'invalid_url',
      'invalid_number',
      'invalid_select',
      'invalid_json',
    ]));
    expect(splitValidationIssues(issues).hard).toHaveLength(0);

    http.data.params = { url: '{{start.url}}', timeout: '{{start.timeout}}', method: 'GET', headers: '{{start.headers}}' };
    const start = createWorkflowNode(defs[0], { x: 0, y: 0 }, 'start');
    const valid = validateWorkflowGraph([start, http], [{ id: 'e', source: 'start', target: 'http' }], defs);
    expect(valid.map((issue) => issue.type)).not.toEqual(expect.arrayContaining(['invalid_url', 'invalid_number', 'invalid_json']));
  });

  it('marks only the true multi-hop failed path through branching executions', () => {
    const state = buildExecutionPathState([
      { id: 'a-b', source: 'A', target: 'B' },
      { id: 'b-c', source: 'B', target: 'C' },
      { id: 'a-x', source: 'A', target: 'X' },
      { id: 'b-y', source: 'B', target: 'Y' },
      { id: 'z-q', source: 'Z', target: 'Q' },
    ], [
      { node_id: 'A', status: 'SUCCESS' },
      { node_id: 'B', status: 'SUCCESS' },
      { node_id: 'C', status: 'FAILED' },
      { node_id: 'X', status: 'SKIPPED' },
      { node_id: 'Y', status: 'SUCCESS' },
      { node_id: 'Z', status: 'SUCCESS' },
    ]);

    expect(state.edgeStates['a-b']).toBe('failed-path');
    expect(state.edgeStates['b-c']).toBe('failed-path');
    expect(state.edgeStates['a-x']).toBe('skipped');
    expect(state.edgeStates['b-y']).toBe('success');
    expect(state.edgeStates['z-q']).toBe('not-run');
    expect(state.failedNodes.has('C')).toBe(true);
    expect(state.failedAncestryNodes.has('A')).toBe(true);
    expect(state.failedAncestryNodes.has('B')).toBe(true);
    expect(state.failedAncestryNodes.has('X')).toBe(false);
    expect(state.failedAncestryNodes.has('Y')).toBe(false);
  });
});
