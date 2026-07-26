export const categoryLabels = {
  TRIGGER: 'Triggers',
  ACTION: 'Actions',
  LOGIC: 'Logic',
  'LOGIC & UTILITY': 'Logic',
  DATABASE: 'Databases',
  'AI & LLM': 'AI',
  COMMUNICATION: 'Communication',
  DEVELOPER: 'Developer Tools',
};

export function categoryLabel(category) {
  return categoryLabels[category] || category || 'Actions';
}

export function getDefaultParams(nodeDef) {
  const params = {};
  (nodeDef?.params || []).forEach((param) => {
    params[param.name] = param.default ?? '';
  });
  return params;
}

let fallbackCounter = 0;

export function generateGraphId(prefix) {
  if (globalThis.crypto?.randomUUID) {
    return `${prefix}_${globalThis.crypto.randomUUID()}`;
  }
  fallbackCounter += 1;
  return `${prefix}_${Date.now().toString(36)}_${fallbackCounter.toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

export function createWorkflowNode(nodeDef, position, id = generateGraphId('node')) {
  return {
    id,
    type: 'custom',
    position: position || { x: 260, y: 180 },
    label: nodeDef.name,
    data: {
      id,
      type: nodeDef.type,
      name: nodeDef.name,
      params: getDefaultParams(nodeDef),
    },
  };
}

export function createWorkflowEdge(source, target, sourceHandle = null) {
  return {
    id: generateGraphId('edge'),
    source,
    sourceHandle,
    target,
    targetHandle: null,
    animated: false,
    style: { stroke: 'var(--color-border-strong)', strokeWidth: 2 },
  };
}

export function cloneGraph(nodes, edges) {
  return {
    nodes: JSON.parse(JSON.stringify(nodes || [])),
    edges: JSON.parse(JSON.stringify(edges || [])),
  };
}

export function serializeGraph(nodes, edges) {
  return {
    nodes: (nodes || []).map((node) => ({
      id: node.id,
      type: node.data?.type || node.type,
      name: node.data?.name || node.label,
      position: node.position || { x: 0, y: 0 },
      params: node.data?.params || {},
    })),
    edges: (edges || []).map((edge) => ({
      id: edge.id,
      source: edge.source,
      sourceHandle: edge.sourceHandle || null,
      target: edge.target,
      targetHandle: edge.targetHandle || null,
    })),
  };
}

export function graphFingerprint(nodes, edges) {
  const graph = serializeGraph(nodes, edges);
  const normalized = {
    nodes: graph.nodes.map((node) => ({
      ...node,
      params: Object.keys(node.params || {}).sort().reduce((acc, key) => {
        acc[key] = node.params[key];
        return acc;
      }, {}),
    })).sort((a, b) => a.id.localeCompare(b.id)),
    edges: graph.edges.slice().sort((a, b) => a.id.localeCompare(b.id)),
  };
  return JSON.stringify(normalized);
}

const hardIssueTypes = new Set([
  'duplicate_node_id',
  'invalid_edge_reference',
  'graph_cycle',
  'unknown_node',
]);

export function isHardValidationIssue(issue) {
  return hardIssueTypes.has(issue?.type);
}

export function splitValidationIssues(issues) {
  return {
    hard: (issues || []).filter(isHardValidationIssue),
    soft: (issues || []).filter((issue) => !isHardValidationIssue(issue)),
  };
}

export function summarizeNodeOperation(node, nodeDef) {
  const params = node?.data?.params || {};
  switch (node?.data?.type) {
    case 'httpRequest':
      return `${params.method || 'GET'} ${params.url || 'URL not set'}`;
    case 'telegramBot':
    case 'discordBot':
    case 'slackBot':
      return 'Send message';
    case 'conditionIf':
      return `${params.left || 'value'} ${params.operator || 'equals'} ${params.right || 'target'}`;
    case 'delaySleep':
      return `Wait ${params.seconds || '1'} seconds`;
    case 'manualTrigger':
      return 'Manual start';
    case 'cronTrigger':
      return params.schedule || 'Cron schedule';
    default:
      return nodeDef?.description || categoryLabel(nodeDef?.category);
  }
}

export function validateWorkflowGraph(nodes, edges, nodeDefinitions) {
  const definitionsByType = new Map((nodeDefinitions || []).map((def) => [def.type, def]));
  const issues = [];
  const ids = new Set();
  const duplicateIds = new Set();

  (nodes || []).forEach((node) => {
    if (ids.has(node.id)) duplicateIds.add(node.id);
    ids.add(node.id);
  });

  duplicateIds.forEach((id) => {
    issues.push({
      type: 'duplicate_node_id',
      nodeId: id,
      message: `Duplicate node ID: ${id}`,
    });
  });

  (nodes || []).forEach((node) => {
    const nodeType = node.data?.type || node.type;
    const def = definitionsByType.get(nodeType);
    if (!def) {
      issues.push({
        type: 'unknown_node',
        nodeId: node.id,
        message: `${node.data?.name || node.id} uses an unknown node type.`,
      });
      return;
    }

    (def.params || []).forEach((param) => {
      const value = node.data?.params?.[param.name];
      const missing = value === undefined || value === null || String(value).trim() === '';
      if (param.type === 'credential' && (param.required || missing)) {
        if (missing) {
          issues.push({
            type: 'missing_credential',
            nodeId: node.id,
            param: param.name,
            message: `${node.data?.name || def.name} needs a credential.`,
          });
        }
        return;
      }
      if (param.required && missing) {
        issues.push({
          type: 'missing_required',
          nodeId: node.id,
          param: param.name,
          message: `${node.data?.name || def.name} is missing ${param.label || param.name}.`,
        });
      }
      if (typeof value === 'string' && value.includes('{{')) {
        const expressionIssue = validateExpressionReference(value, node.id, nodes, edges);
        if (expressionIssue) {
          issues.push({
            ...expressionIssue,
            param: param.name,
            message: `${node.data?.name || def.name} has an invalid expression in ${param.label || param.name}: ${expressionIssue.message}`,
          });
        }
      }
    });
  });

  (edges || []).forEach((edge) => {
    if (!ids.has(edge.source)) {
      issues.push({
        type: 'invalid_edge_reference',
        edgeId: edge.id,
        message: `Edge ${edge.id} references missing source ${edge.source}.`,
      });
    }
    if (!ids.has(edge.target)) {
      issues.push({
        type: 'invalid_edge_reference',
        edgeId: edge.id,
        message: `Edge ${edge.id} references missing target ${edge.target}.`,
      });
    }
  });

  const adjacency = new Map();
  ids.forEach((id) => adjacency.set(id, []));
  (edges || []).forEach((edge) => {
    if (ids.has(edge.source) && ids.has(edge.target)) {
      adjacency.get(edge.source).push(edge.target);
    }
  });

  const visiting = new Set();
  const visited = new Set();
  let cycleNode = null;
  function visit(id) {
    if (visiting.has(id)) {
      cycleNode = id;
      return true;
    }
    if (visited.has(id)) return false;
    visiting.add(id);
    for (const next of adjacency.get(id) || []) {
      if (visit(next)) return true;
    }
    visiting.delete(id);
    visited.add(id);
    return false;
  }
  for (const id of ids) {
    if (visit(id)) break;
  }
  if (cycleNode) {
    issues.push({
      type: 'graph_cycle',
      nodeId: cycleNode,
      message: `Workflow contains a cycle near ${cycleNode}.`,
    });
  }

  return issues;
}

function validateExpressionReference(value, nodeId, nodes, edges) {
  const trimmed = String(value || '').trim();
  const match = trimmed.match(/^\{\{\s*([^{}]+?)\s*\}\}$/);
  if (!match) {
    if (trimmed.startsWith('{{') && trimmed.endsWith('}}')) {
      return { type: 'invalid_expression', nodeId, message: 'expression syntax is invalid.' };
    }
    return null;
  }
  const sourceId = match[1].trim().split('.')[0];
  if (!sourceId) return { type: 'invalid_expression', nodeId, message: 'expression source is missing.' };
  if (sourceId === '$trigger') return null;
  const ids = new Set((nodes || []).map((node) => node.id));
  if (!ids.has(sourceId)) {
    return { type: 'invalid_expression_reference', nodeId, message: `source "${sourceId}" does not exist.` };
  }
  const upstream = upstreamIds(nodeId, edges);
  if (!upstream.has(sourceId)) {
    return { type: 'invalid_expression_reference', nodeId, message: `source "${sourceId}" is not upstream of this node.` };
  }
  return null;
}

function upstreamIds(nodeId, edges) {
  const reverse = new Map();
  (edges || []).forEach((edge) => {
    if (!reverse.has(edge.target)) reverse.set(edge.target, []);
    reverse.get(edge.target).push(edge.source);
  });
  const found = new Set();
  const stack = [...(reverse.get(nodeId) || [])];
  while (stack.length) {
    const id = stack.pop();
    if (found.has(id)) continue;
    found.add(id);
    stack.push(...(reverse.get(id) || []));
  }
  return found;
}

export function validationStateForNode(nodeId, issues) {
  const nodeIssues = (issues || []).filter((issue) => issue.nodeId === nodeId);
  if (nodeIssues.some((issue) => issue.type === 'unknown_node')) return 'Unknown node';
  if (nodeIssues.some((issue) => issue.type === 'missing_credential')) return 'Missing credential';
  if (nodeIssues.length) return 'Missing required fields';
  return 'Configured';
}

export function autoLayoutGraph(nodes, edges) {
  const incoming = new Map();
  const outgoing = new Map();
  (nodes || []).forEach((node) => {
    incoming.set(node.id, 0);
    outgoing.set(node.id, []);
  });
  (edges || []).forEach((edge) => {
    if (!incoming.has(edge.target) || !outgoing.has(edge.source)) return;
    incoming.set(edge.target, incoming.get(edge.target) + 1);
    outgoing.get(edge.source).push(edge.target);
  });

  const queue = [...incoming.entries()].filter(([, count]) => count === 0).map(([id]) => id);
  const level = new Map(queue.map((id) => [id, 0]));
  while (queue.length) {
    const id = queue.shift();
    for (const target of outgoing.get(id) || []) {
      level.set(target, Math.max(level.get(target) || 0, (level.get(id) || 0) + 1));
      incoming.set(target, incoming.get(target) - 1);
      if (incoming.get(target) === 0) queue.push(target);
    }
  }

  const buckets = new Map();
  (nodes || []).forEach((node) => {
    const l = level.get(node.id) ?? 0;
    if (!buckets.has(l)) buckets.set(l, []);
    buckets.get(l).push(node);
  });

  return (nodes || []).map((node) => {
    const l = level.get(node.id) ?? 0;
    const bucket = buckets.get(l) || [];
    const index = bucket.findIndex((item) => item.id === node.id);
    return {
      ...node,
      position: {
        x: 120 + l * 260,
        y: 120 + index * 150,
      },
    };
  });
}
