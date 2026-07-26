import { describe, expect, it } from 'vitest';
import {
  buildJsonTree,
  expressionForPath,
  redactValue,
  resolveExpression,
  rowsForTable,
  safeJSONStringify,
  validateParamValue,
} from '../src/utils/inspector';

describe('inspector utilities', () => {
  it('builds JSON tree rows and table rows without exposing secret fields', () => {
    const data = {
      user: { email: 'dev@example.com', password: 'secret' },
      items: [{ id: 1, name: 'A' }],
      authorization: 'Bearer raw-token',
    };
    const tree = buildJsonTree(data);
    expect(tree.some((row) => row.path === 'user.email' && row.value === 'dev@example.com')).toBe(true);
    expect(JSON.stringify(tree)).not.toContain('secret');
    expect(JSON.stringify(redactValue(data))).not.toContain('raw-token');

    const table = rowsForTable(data.items);
    expect(table.columns).toEqual(['id', 'name']);
    expect(table.rows[0].name).toBe('A');
  });

  it('creates and previews runtime-compatible placeholder expressions', () => {
    const expression = expressionForPath('json_1', 'transformed.user.email');
    expect(expression).toBe('{{json_1.transformed.user.email}}');
    const preview = resolveExpression(expression, [
      { id: 'json_1', data: { transformed: { user: { email: 'dev@example.com' } } } },
    ]);
    expect(preview).toEqual(expect.objectContaining({ ok: true, value: 'dev@example.com' }));
  });

  it('reports missing expression references and invalid JSON parameters', () => {
    const preview = resolveExpression('{{missing.value}}', []);
    expect(preview.ok).toBe(false);
    expect(preview.error).toContain('missing');
    expect(validateParamValue({ name: 'payload', label: 'Payload', type: 'json' }, '{bad')).toBe('Enter valid JSON.');
  });

  it('truncates large raw payloads intentionally', () => {
    const result = safeJSONStringify({ data: 'x'.repeat(21000) });
    expect(result.truncated).toBe(true);
    expect(result.text).toContain('truncated');
  });
});
