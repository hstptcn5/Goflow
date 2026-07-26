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

export function createWorkflowNode(nodeDef, position, id = `node_${Date.now()}`) {
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
    id: `edge_${source}-${target}_${Date.now()}`,
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

