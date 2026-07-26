import { describe, expect, it } from 'vitest';
import {
  autoLayoutGraph,
  createWorkflowEdge,
  createWorkflowNode,
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

  it('validates missing fields, credentials, unknown nodes, bad edges, duplicate ids, and cycles', () => {
    const nodes = [
      createWorkflowNode(defs[1], { x: 0, y: 0 }, 'a'),
      createWorkflowNode(defs[2], { x: 0, y: 0 }, 'b'),
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
});

