const SECRET_KEY_PATTERN = /(password|passwd|api[_-]?key|apikey|authorization|cookie|set-cookie|private[_-]?key|access[_-]?token|refresh[_-]?token|bot[_-]?token|auth[_-]?token|client[_-]?secret|connection[_-]?string|credential|secret|token|webhook[_-]?url|bearer)/i;
const SECRET_VALUE_PATTERN = /(https:\/\/discord\.com\/api\/webhooks\/[^\s"'<>]+|https:\/\/hooks\.slack\.com\/services\/[^\s"'<>]+|bearer\s+[a-z0-9._-]+|(access_token|refresh_token|auth_token|bot_token|client_secret|api_key|apikey|password|passwd|secret|token|credential|webhook_url|connection_string)=[^\s&"'<>]+|sk-[a-z0-9._-]{12,}|ghp_[a-z0-9_]{12,}|github_pat_[a-z0-9_]{12,}|xoxb-[a-z0-9-]{12,}|-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----)/ig;
const MAX_TREE_CHILDREN = 80;
const MAX_RAW_CHARS = 20000;

export function redactValue(value, key = '') {
  if (SECRET_KEY_PATTERN.test(String(key))) return '[REDACTED]';
  if (typeof value === 'string') return redactString(value);
  if (Array.isArray(value)) return value.map((item) => redactValue(item));
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([childKey, childValue]) => [childKey, redactValue(childValue, childKey)]));
  }
  return value;
}

export function redactString(value) {
  const trimmed = String(value ?? '').trim();
  if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
    try {
      return JSON.stringify(redactValue(JSON.parse(trimmed)));
    } catch {
      // Fall through to string redaction.
    }
  }
  let redacted = String(value ?? '').replace(SECRET_VALUE_PATTERN, '[REDACTED]');
  try {
    const parsed = new URL(redacted);
    if (parsed.username || parsed.password) {
      parsed.username = '[REDACTED]';
      parsed.password = '[REDACTED]';
    }
    ['token', 'api_key', 'apikey', 'access_token', 'refresh_token', 'auth_token', 'bot_token', 'client_secret', 'password', 'secret', 'credential', 'webhook_url', 'connection_string'].forEach((name) => {
      if (parsed.searchParams.has(name)) parsed.searchParams.set(name, '[REDACTED]');
    });
    redacted = parsed.toString();
  } catch {
    // Not a URL.
  }
  SECRET_VALUE_PATTERN.lastIndex = 0;
  return SECRET_VALUE_PATTERN.test(redacted) ? '[REDACTED]' : redacted;
}

export function safeJSONStringify(value, space = 2, maxChars = MAX_RAW_CHARS) {
  let raw;
  try {
    raw = JSON.stringify(redactValue(value), null, space);
  } catch {
    return { text: 'Unable to stringify this payload.', truncated: false };
  }
  if (!raw) return { text: '', truncated: false };
  if (raw.length <= maxChars) return { text: raw, truncated: false };
  return { text: `${raw.slice(0, maxChars)}\n... truncated ${raw.length - maxChars} characters`, truncated: true };
}

export function getByPath(source, path) {
  if (!path) return source;
  return path.split('.').reduce((current, part) => {
    if (current == null) return undefined;
    if (Array.isArray(current)) {
      const index = Number(part);
      return Number.isInteger(index) ? current[index] : undefined;
    }
    if (typeof current === 'object') return current[part];
    return undefined;
  }, source);
}

export function expressionForPath(sourceId, path) {
  const trimmedPath = String(path || '').trim();
  const cleanSource = String(sourceId || '').trim();
  if (!cleanSource) return '';
  return `{{${cleanSource}${trimmedPath ? `.${trimmedPath}` : ''}}}`;
}

export function parseSingleExpression(expression) {
  const match = String(expression || '').trim().match(/^\{\{\s*([^{}]+?)\s*\}\}$/);
  if (!match) return null;
  return parseExpressionInner(match[1]);
}

export function isCompleteExpression(value) {
  return Boolean(parseSingleExpression(value));
}

function parseExpressionInner(inner) {
  const parts = String(inner || '').trim().split('.');
  const sourceId = parts.shift();
  return sourceId ? { sourceId, path: parts.join('.') } : null;
}

export function extractExpressions(value) {
  const text = String(value ?? '');
  const matches = [...text.matchAll(/\{\{\s*([^{}]+?)\s*\}\}/g)];
  return matches.map((match) => ({
    raw: match[0],
    ...parseExpressionInner(match[1]),
  })).filter((item) => item.sourceId);
}

export function validateExpressionSyntax(value) {
  const text = String(value ?? '').trim();
  if (!text.includes('{{') && !text.includes('}}')) return '';
  if (parseSingleExpression(text)) return '';
  const withoutValidPlaceholders = text.replace(/\{\{\s*([^{}]+?)\s*\}\}/g, '');
  if (withoutValidPlaceholders.includes('{{') || withoutValidPlaceholders.includes('}}')) return 'Expression syntax is invalid.';
  return '';
}

export function resolveExpression(expression, sources) {
  const parsed = parseSingleExpression(expression);
  if (!parsed) {
    if (String(expression || '').includes('{{')) {
      return { ok: false, error: 'Only one complete placeholder expression can be previewed here.' };
    }
    return { ok: true, value: expression ?? '' };
  }
  const source = sources.find((item) => item.id === parsed.sourceId);
  if (!source) return { ok: false, error: `Source "${parsed.sourceId}" is not available in the selected execution/sample.` };
  const value = getByPath(source.data, parsed.path);
  if (value === undefined) return { ok: false, error: `Path "${parsed.path || parsed.sourceId}" was not found.` };
  return { ok: true, value: redactValue(value), sourceId: parsed.sourceId, path: parsed.path };
}

export function buildJsonTree(value, path = '', depth = 0) {
  const redacted = redactValue(value);
  if (redacted == null || typeof redacted !== 'object') {
    return [{ path, key: path.split('.').pop() || '$', type: typeof redacted, value: redacted, leaf: true }];
  }
  const entries = Array.isArray(redacted)
    ? redacted.map((item, index) => [String(index), item])
    : Object.entries(redacted);
  const visible = entries.slice(0, MAX_TREE_CHILDREN);
  const rows = [];
  visible.forEach(([key, child]) => {
    const childPath = path ? `${path}.${key}` : key;
    const childIsObject = child && typeof child === 'object';
    rows.push({
      path: childPath,
      key,
      type: Array.isArray(child) ? 'array' : childIsObject ? 'object' : typeof child,
      value: childIsObject ? child : child,
      leaf: !childIsObject,
      depth,
      truncated: false,
    });
    if (childIsObject && depth < 2) {
      rows.push(...buildJsonTree(child, childPath, depth + 1));
    }
  });
  if (entries.length > visible.length) {
    rows.push({ path, key: `+${entries.length - visible.length} more`, type: 'truncated', value: '', leaf: true, depth, truncated: true });
  }
  return rows;
}

export function rowsForTable(value) {
  const data = redactValue(value);
  if (!Array.isArray(data) || data.length === 0) return { columns: [], rows: [] };
  if (!data.every((item) => item && typeof item === 'object' && !Array.isArray(item))) return { columns: [], rows: [] };
  const columns = Array.from(new Set(data.slice(0, 50).flatMap((item) => Object.keys(item)))).slice(0, 12);
  return { columns, rows: data.slice(0, 50) };
}

export function paramIsVisible(param, values = {}) {
  const rules = param?.visible_when;
  if (!rules || typeof rules !== 'object') return true;
  return Object.entries(rules).every(([key, allowed]) => {
    const candidates = Array.isArray(allowed) ? allowed : [allowed];
    return candidates.map((value) => String(value)).includes(String(values?.[key] ?? ''));
  });
}

export function classifyParams(params = [], values = {}) {
  const groups = {
    credential: [],
    resource: [],
    operation: [],
    required: [],
    common: [],
    advanced: [],
  };
  params.forEach((param) => {
    if (!paramIsVisible(param, values)) return;
    const name = String(param.name || '').toLowerCase();
    if (param.type === 'credential') groups.credential.push(param);
    else if (name.includes('resource')) groups.resource.push(param);
    else if (name.includes('operation') || name === 'method') groups.operation.push(param);
    else if (param.required) groups.required.push(param);
    else if (param.advanced || name.includes('timeout') || name.includes('headers') || name.includes('retry')) groups.advanced.push(param);
    else groups.common.push(param);
  });
  return groups;
}

export function validateParamValue(param, value, credentials = []) {
  const empty = value === undefined || value === null || value === '';
  if (param.required && empty) {
    return param.type === 'credential' ? 'Select a credential before testing or activating this workflow.' : `${param.label || param.name} is required.`;
  }
  if (param.type === 'credential' && value && !credentials.some((cred) => cred.id === value)) return 'Selected credential is not available.';
  if (typeof value === 'string' && value.includes('{{')) {
    const expressionError = validateExpressionSyntax(value);
    if (expressionError) return expressionError;
    if (isCompleteExpression(value)) return '';
  }
  if ((param.type === 'number' || param.type === 'integer') && !empty && Number.isNaN(Number(value))) return 'Enter a valid number.';
  if (param.type === 'integer' && !empty && !Number.isInteger(Number(value))) return 'Enter a valid integer.';
  if (param.type === 'select' && !empty && Array.isArray(param.options) && !param.options.includes(value)) return 'Choose a valid option.';
  if (param.type === 'json' && !empty) {
    try {
      JSON.parse(value);
    } catch {
      return 'Enter valid JSON.';
    }
  }
  if (param.type === 'url' && !empty) {
    try {
      new URL(value);
    } catch {
      return 'Enter a valid URL.';
    }
  }
  return '';
}

export function upstreamNodeIds(nodeId, edges = []) {
  const reverse = new Map();
  edges.forEach((edge) => {
    if (!reverse.has(edge.target)) reverse.set(edge.target, []);
    reverse.get(edge.target).push(edge.source);
  });
  const found = new Set();
  const stack = [...(reverse.get(nodeId) || [])];
  while (stack.length) {
    const id = stack.pop();
    if (!id || found.has(id) || id === nodeId) continue;
    found.add(id);
    stack.push(...(reverse.get(id) || []));
  }
  return found;
}

export function orderedUpstreamNodes(selectedNodeId, nodes = [], edges = []) {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const directIds = new Set(edges.filter((edge) => edge.target === selectedNodeId).map((edge) => edge.source));
  const allIds = upstreamNodeIds(selectedNodeId, edges);
  const direct = [];
  const previous = [];
  nodes.forEach((node) => {
    if (!allIds.has(node.id)) return;
    if (directIds.has(node.id)) direct.push(node);
    else previous.push(node);
  });
  return [...direct, ...previous].filter((node, index, all) => byId.has(node.id) && all.findIndex((item) => item.id === node.id) === index);
}

export function credentialsForParam(credentials, param, nodeType = '', nodeParams = {}) {
  const requestedKinds = normalizeStringList(param?.credential_kinds || param?.credentialKinds).map((value) => value.toUpperCase());
  const requestedProviders = normalizeStringList(param?.credential_providers || param?.credentialProviders).map((value) => value.toLowerCase());
  const selectedProvider = nodeType === 'aiExtract' ? String(nodeParams?.provider || '').trim().toLowerCase() : '';
  const effectiveProviders = selectedProvider && requestedProviders.includes(selectedProvider)
    ? [selectedProvider]
    : requestedProviders;

  if (requestedKinds.length || effectiveProviders.length) {
    return credentials.filter((cred) => {
      const kind = canonicalCredentialKind(cred);
      const provider = canonicalCredentialProvider(cred);
      if (requestedKinds.length && !requestedKinds.includes(kind)) return false;
      if (effectiveProviders.length && !effectiveProviders.includes(provider)) return false;
      return true;
    });
  }

  // Legacy nodes may not declare canonical metadata yet. Keep the old hint path so
  // existing workflows remain compatible while new nodes can be provider-agnostic.
  const hint = String(param?.credential_type || param?.credentialType || param?.type_hint || nodeType || '').toLowerCase();
  if (!hint) return credentials;
  return credentials.filter((cred) => {
    const type = String(cred.type || '').toLowerCase();
    const kind = canonicalCredentialKind(cred);
    const provider = canonicalCredentialProvider(cred);
    if (hint.includes('aiextract')) {
      if (kind !== 'API_KEY') return false;
      if (selectedProvider) return provider === selectedProvider;
      return provider === 'openai' || provider === 'deepseek';
    }
    return hint.includes(type) || type.includes(hint) || hint.includes(provider) || provider.includes(hint) || credentialAliasMatch(type, hint);
  });
}

function normalizeStringList(value) {
  if (Array.isArray(value)) return value.map((item) => String(item || '').trim()).filter(Boolean);
  if (typeof value === 'string' && value.trim()) return value.split(',').map((item) => item.trim()).filter(Boolean);
  return [];
}

function canonicalCredentialKind(cred) {
  const declared = String(cred?.kind || '').trim().toUpperCase();
  if (declared) return declared;
  switch (String(cred?.type || '').trim().toLowerCase()) {
    case 'openai':
    case 'deepseek':
    case 'telegram_bot':
    case 'api_key':
    case 'openai_api_key':
    case 'deepseek_api_key':
      return 'API_KEY';
    case 'bearer_token':
      return 'BEARER_TOKEN';
    case 'basic_auth':
      return 'BASIC_AUTH';
    case 'oauth2':
      return 'OAUTH2';
    case 'google_service_account':
      return 'SERVICE_ACCOUNT';
    default:
      return 'CUSTOM';
  }
}

function canonicalCredentialProvider(cred) {
  const declared = String(cred?.provider || '').trim().toLowerCase();
  if (declared) return declared;
  switch (String(cred?.type || '').trim().toLowerCase()) {
    case 'openai':
    case 'openai_api_key':
      return 'openai';
    case 'deepseek':
    case 'deepseek_api_key':
      return 'deepseek';
    case 'telegram_bot':
      return 'telegram';
    default:
      return 'custom';
  }
}

function credentialAliasMatch(type, hint) {
  if (hint.includes('telegram')) return type.includes('telegram');
  if (hint.includes('discord')) return type.includes('discord') || type.includes('webhook');
  if (hint.includes('slack')) return type.includes('slack') || type.includes('webhook');
  if (hint.includes('openai')) return type.includes('openai') || type.includes('api_key');
  if (hint.includes('deepseek')) return type.includes('deepseek') || type.includes('api_key');
  if (hint.includes('google')) return type.includes('google') || type.includes('oauth') || type.includes('service');
  if (hint.includes('ssh')) return type.includes('ssh');
  if (hint.includes('redis')) return type.includes('redis');
  if (hint.includes('postgres')) return type.includes('postgres');
  if (hint.includes('mysql')) return type.includes('mysql');
  if (hint.includes('mongo')) return type.includes('mongo');
  return false;
}
