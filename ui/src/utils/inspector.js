const SECRET_KEY_PATTERN = /(password|api[_-]?key|authorization|cookie|private[_-]?key|access[_-]?token|refresh[_-]?token|secret|bearer)/i;
const MAX_TREE_CHILDREN = 80;
const MAX_RAW_CHARS = 20000;

export function redactValue(value, key = '') {
  if (SECRET_KEY_PATTERN.test(String(key))) return '[REDACTED]';
  if (typeof value === 'string' && /(bearer\s+[a-z0-9._-]+|sk-[a-z0-9._-]+)/i.test(value)) return '[REDACTED]';
  if (Array.isArray(value)) return value.map((item) => redactValue(item));
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([childKey, childValue]) => [childKey, redactValue(childValue, childKey)]));
  }
  return value;
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
  const parts = match[1].trim().split('.');
  const sourceId = parts.shift();
  return sourceId ? { sourceId, path: parts.join('.') } : null;
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

export function classifyParams(params = []) {
  const groups = {
    credential: [],
    resource: [],
    operation: [],
    required: [],
    common: [],
    advanced: [],
  };
  params.forEach((param) => {
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
  if ((param.type === 'number' || param.type === 'integer') && !empty && Number.isNaN(Number(value))) return 'Enter a valid number.';
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
  if (typeof value === 'string' && value.includes('{{')) {
    const preview = parseSingleExpression(value);
    if (!preview && value.trim().startsWith('{{') && value.trim().endsWith('}}')) return 'Expression syntax is invalid.';
  }
  return '';
}

export function credentialsForParam(credentials, param, nodeType = '') {
  const hint = String(param.credential_type || param.credentialType || param.type_hint || nodeType || '').toLowerCase();
  if (!hint) return credentials;
  return credentials.filter((cred) => {
    const type = String(cred.type || '').toLowerCase();
    return hint.includes(type) || type.includes(hint) || credentialAliasMatch(type, hint);
  });
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
