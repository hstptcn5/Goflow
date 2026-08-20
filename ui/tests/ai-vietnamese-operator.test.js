import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../src/services/api';

describe('Vietnamese AI operator contract', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it('prepends Vietnamese instructions, keeps technical identifiers, and redacts secrets', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ type: 'text', text: 'Đã hiểu.' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }));
    vi.stubGlobal('fetch', fetchMock);

    await api.generateAIWorkflow(
      [{ role: 'user', content: 'Tạo HTTP request rồi gửi Telegram' }],
      'deepseek-cred',
      [{
        id: 'http_1',
        type: 'httpRequest',
        params: {
          url: 'https://example.com',
          credential_id: 'cred-real-id',
          headers: { Authorization: 'Bearer secret-token' },
        },
      }],
      []
    );

    const request = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(request.messages[0].role).toBe('system');
    expect(request.messages[0].content).toContain('phải viết bằng tiếng Việt');
    expect(request.messages[0].content).toContain('node type');
    expect(request.messages[0].content).toContain('parameter name');
    expect(request.current_nodes[0].type).toBe('httpRequest');
    expect(request.current_nodes[0].params.url).toBe('https://example.com');
    expect(request.current_nodes[0].params.credential_id).toBe('[REDACTED]');
    expect(request.current_nodes[0].params.headers.Authorization).toBe('[REDACTED]');
    expect(JSON.stringify(request)).not.toContain('cred-real-id');
    expect(JSON.stringify(request)).not.toContain('secret-token');
  });

  it('adds the same operator contract and redacts current params in AI Quick Config', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ method: 'GET', url: 'https://example.com' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }));
    vi.stubGlobal('fetch', fetchMock);

    await api.configureNodeParams('httpRequest', 'Dùng API ví dụ', {
      url: 'https://example.com',
      credential_id: 'cred-real-id',
    }, 'deepseek-cred');

    const request = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(request.node_type).toBe('httpRequest');
    expect(request.prompt).toContain('phải viết bằng tiếng Việt');
    expect(request.prompt).toContain('Dùng API ví dụ');
    expect(request.current_params.url).toBe('https://example.com');
    expect(request.current_params.credential_id).toBe('[REDACTED]');
  });
});
