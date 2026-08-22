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
  if (mode === 'auto') {
    if (Array.isArray(value) && value.every((item) => item && typeof item === 'object' && !Array.isArray(item))) mode = 'table';
    else if (value && typeof value === 'object' && !Array.isArray(value) && Object.values(value).every(isScalar)) mode = 'cards';
    else mode = 'json';
  }
  if (mode === 'table' && Array.isArray(value)) {
    const columns = [...new Set(value.flatMap((row) => Object.keys(row || {})))];
    return { mode, columns, rows: value };
  }
  if (mode === 'cards' && value && typeof value === 'object' && !Array.isArray(value)) {
    return { mode, cards: Object.entries(value).map(([key, item]) => ({ key, value: item })) };
  }
  return { mode: 'json', json: JSON.stringify(value ?? null, null, 2) };
}

function isScalar(value) { return value === null || ['string', 'number', 'boolean'].includes(typeof value); }
