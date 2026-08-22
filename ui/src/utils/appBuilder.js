export function fieldsFromJSONSchema(raw) {
  let schema = raw;
  if (typeof raw === 'string') {
    try { schema = JSON.parse(raw || '{}'); } catch { schema = {}; }
  }
  const required = new Set(Array.isArray(schema?.required) ? schema.required : []);
  return Object.entries(schema?.properties || {}).map(([key, property]) => ({
    key,
    label: property.title || humanize(key),
    description: property.description || '',
    type: fieldType(property),
    required: required.has(key),
    default: property.default,
    options: Array.isArray(property.enum) ? property.enum : undefined,
	optionsText: Array.isArray(property.enum) ? property.enum.join(', ') : '',
    placeholder: property.examples?.[0] === undefined ? '' : String(property.examples[0]),
  }));
}

export function fieldType(property = {}) {
  if (Array.isArray(property.enum)) return 'select';
  if (property.type === 'boolean') return 'boolean';
  if (property.type === 'integer') return 'integer';
  if (property.type === 'number') return 'number';
  if (property.type === 'array' && property.items?.type === 'number') return 'number_list';
  if (property.type === 'object' || property.type === 'array') return 'json';
  return property.format === 'textarea' ? 'textarea' : 'string';
}

export function defaultInputMode(nodes = []) {
  return nodes.some((node) => (node.data?.type || node.type) === 'webhookTrigger') ? 'webhook_body' : 'direct';
}

export function humanize(value) {
  return String(value || '').replaceAll('_', ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}
