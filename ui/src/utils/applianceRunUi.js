export function initialRunValues(fields = []) {
  return Object.fromEntries(fields.map((field) => [field.key, field.default ?? (field.type === 'boolean' ? false : '')]));
}

export function coerceRunValue(field, raw) {
  if (field.type === 'boolean') return Boolean(raw);
  if (field.type === 'number' || field.type === 'integer') {
    if (raw === '') return null;
    const value = Number(raw);
    if (!Number.isFinite(value) || (field.type === 'integer' && !Number.isInteger(value))) throw new Error('invalid_number');
    return value;
  }
  if (field.type === 'json') {
    if (raw === '') return null;
    if (typeof raw !== 'string') return raw;
    return JSON.parse(raw);
  }
  if (field.type === 'number_list') {
    if (Array.isArray(raw)) return finiteNumbers(raw);
    const text = String(raw || '').trim();
    if (!text) return [];
    if (text.startsWith('[')) return finiteNumbers(JSON.parse(text));
    return finiteNumbers(text.split(',').map((item) => Number(item.trim())));
  }
  return raw;
}

function finiteNumbers(value) {
  if (!Array.isArray(value) || value.some((item) => !Number.isFinite(Number(item)))) throw new Error('invalid_number_list');
  return value.map(Number);
}

export function buildRunInput(fields = [], values = {}, inputMode = 'direct') {
  const input = {};
  const errors = {};
  for (const field of fields) {
    const raw = values[field.key];
    if (field.required && (raw === '' || raw === null || raw === undefined)) {
      errors[field.key] = `${field.label || field.key} là bắt buộc`;
      continue;
    }
    try { input[field.key] = coerceRunValue(field, raw); }
    catch { errors[field.key] = `${field.label || field.key} không đúng định dạng`; }
  }
  return { input: inputMode === 'webhook_body' ? { body: input } : input, errors };
}

export function outputView(value, requested = 'auto') {
  let mode = requested || 'auto';
  let displayValue = value;
  let detailsJson = '';

  // AI and extractor nodes commonly return a useful `data` object plus
  // provider/debug metadata. In auto mode, promote that object to the primary
  // result while preserving the complete envelope in collapsed details.
  if (mode === 'auto' && isObject(value) && isObject(value.data)) {
    displayValue = value.data;
    detailsJson = JSON.stringify(value, null, 2);
    mode = Object.values(displayValue).every(isScalar) ? 'cards' : 'structured';
  } else if (mode === 'auto') {
    if (Array.isArray(value) && value.every((item) => isObject(item))) mode = 'table';
    else if (isObject(value) && Object.values(value).every(isScalar)) mode = 'cards';
    else mode = 'json';
  }

  if (mode === 'table' && Array.isArray(displayValue)) {
    const columns = [...new Set(displayValue.flatMap((row) => Object.keys(row || {})))];
    return { mode, columns, rows: displayValue, detailsJson };
  }
  if (mode === 'cards' && isObject(displayValue)) {
    return { mode, cards: Object.entries(displayValue).map(([key, item]) => ({ key, value: item })), detailsJson };
  }
  if (mode === 'structured' && isObject(displayValue)) {
    const fields = Object.entries(displayValue).map(([key, item]) => ({
      key,
      value: item,
      kind: Array.isArray(item) ? 'list' : (isObject(item) ? 'json' : 'scalar'),
    }));
    return { mode, fields, detailsJson };
  }
  return { mode: 'json', json: JSON.stringify(displayValue ?? null, null, 2), detailsJson };
}

function isObject(value) { return Boolean(value) && typeof value === 'object' && !Array.isArray(value); }
function isScalar(value) { return value === null || ['string', 'number', 'boolean'].includes(typeof value); }
