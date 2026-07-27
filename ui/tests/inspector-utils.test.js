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

  it('allows complete expressions in typed fields but validates invalid literals', () => {
    const expression = '{{json_1.value}}';
    [
      { name: 'amount', label: 'Amount', type: 'number' },
      { name: 'count', label: 'Count', type: 'integer' },
      { name: 'url', label: 'URL', type: 'url' },
      { name: 'payload', label: 'Payload', type: 'json' },
      { name: 'message', label: 'Message', type: 'text' },
    ].forEach((param) => {
      expect(validateParamValue(param, expression)).toBe('');
    });

    expect(validateParamValue({ name: 'amount', type: 'number' }, 'abc')).toBe('Enter a valid number.');
    expect(validateParamValue({ name: 'count', type: 'integer' }, '1.5')).toBe('Enter a valid integer.');
    expect(validateParamValue({ name: 'url', type: 'url' }, 'not-a-url')).toBe('Enter a valid URL.');
    expect(validateParamValue({ name: 'payload', type: 'json' }, '{bad')).toBe('Enter valid JSON.');
    expect(validateParamValue({ name: 'method', type: 'select', options: ['GET'] }, 'POST')).toBe('Choose a valid option.');
  });

  it('truncates large raw payloads intentionally', () => {
    const result = safeJSONStringify({ data: 'x'.repeat(21000) });
    expect(result.truncated).toBe(true);
    expect(result.text).toContain('truncated');
  });

  it('redacts debug bundle secret fixture shapes', () => {
    const value = {
      url_userinfo: 'https://user:password@example.com/path',
      discord: 'https://discord.com/api/webhooks/123456789/rawsecret',
      slack: 'https://hooks.slack.com/services/T000/B000/secret',
      query: 'https://example.com/callback?token=query-token&api_key=query-key&ok=1',
      token: 'plain-token',
      credential: 'credential-value',
      webhook_url: 'https://discord.com/api/webhooks/999/raw',
      connection_string: 'postgres://user:password@localhost/db',
      headers: {
        Authorization: 'Bearer bearer-secret-token',
        Cookie: 'sid=session-secret',
      },
      private_key: '-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----',
      nested_json: JSON.stringify({
        access_token: 'nested-token',
        array: ['sk-1234567890abcdef', { api_key: 'array-key' }],
      }),
      formats: [
        'Bearer abcdefghijklmnop',
        'ghp_abcdefghijklmnopqrstuvwxyz',
        'github_pat_abcdefghijklmnopqrstuvwxyz',
        'sk-abcdefghijklmnopqrstuvwxyz',
        'xoxb-123456789012-abcdef',
      ],
    };

    const redacted = JSON.stringify(redactValue(value));
    [
      'password@example.com',
      'rawsecret',
      'query-token',
      'query-key',
      'plain-token',
      'credential-value',
      'session-secret',
      'BEGIN PRIVATE KEY',
      'nested-token',
      'array-key',
      'abcdefghijklmnopqrstuvwxyz',
    ].forEach((secret) => {
      expect(redacted).not.toContain(secret);
    });
    expect(redacted).toContain('[REDACTED]');
  });
});
