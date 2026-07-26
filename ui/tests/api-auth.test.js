import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../src/services/api';

describe('API authentication state', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it('prompts for API key on 401 and retries with bearer token', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response('', { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }));
    vi.stubGlobal('fetch', fetchMock);
    vi.stubGlobal('prompt', vi.fn(() => 'admin-key'));

    const workflows = await api.getWorkflows();

    expect(workflows).toEqual([]);
    expect(localStorage.getItem('GOFLOW_API_KEY')).toBe('admin-key');
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1][1].headers.Authorization).toBe('Bearer admin-key');
  });
});
