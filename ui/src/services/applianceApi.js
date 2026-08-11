const APPLIANCE_BASE = '/api/appliance';

function jsonHeaders(token) {
  return {
    'Content-Type': 'application/json',
    'X-Goflow-Appliance-Token': token,
  };
}

async function readError(res, fallback) {
  const text = await res.text();
  const trimmed = text.trim();
  if (!trimmed) return { message: fallback, category: 'internal_error', payload: null };
  try {
    const payload = JSON.parse(trimmed);
    return {
      message: payload.message || payload.error || fallback,
      category: payload.category || 'internal_error',
      payload,
    };
  } catch {
    return { message: fallback, category: 'internal_error', payload: null };
  }
}

async function request(path, options = {}) {
  const res = await fetch(`${APPLIANCE_BASE}${path}`, options);
  if (!res.ok) {
    const details = await readError(res, `Appliance request failed with ${res.status}`);
    const error = new Error(details.message);
    error.category = details.category;
    error.status = res.status;
    error.payload = details.payload;
    throw error;
  }
  return res.json();
}

export const applianceApi = {
  async bootstrap() {
    const res = await fetch(`${APPLIANCE_BASE}/bootstrap`);
    if (res.status === 404) return null;
    if (!res.ok) {
      const details = await readError(res, 'Failed to load appliance bootstrap');
      throw new Error(details.message);
    }
    return res.json();
  },

  getSetup() {
    return request('/setup');
  },

  getStatus() {
    return request('/status');
  },

  getWorkflowStatus() {
    return request('/workflow/status');
  },

  getLatestExecution() {
    return request('/executions/latest');
  },

  getRecentExecutions(limit = 10) {
    return request(`/executions?limit=${encodeURIComponent(limit)}`);
  },

  getDiagnostics() {
    return request('/diagnostics');
  },

  saveConfig(token, values) {
    return request('/setup/config', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({ values }),
    });
  },

  testSource(token, key) {
    return request('/setup/source/test', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({ key }),
    });
  },

  createCredential(token, payload) {
    return request('/setup/credentials/create', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify(payload),
    });
  },

  testCredential(token, key) {
    return request('/setup/credentials/test', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({ key }),
    });
  },

  completeSetup(token) {
    return request('/setup/complete', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({}),
    });
  },

  reopenSetup(token) {
    return request('/setup/reopen', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({}),
    });
  },

  runNow(token, input = {}) {
    return request('/workflow/run', {
      method: 'POST',
      headers: jsonHeaders(token),
      body: JSON.stringify({ input }),
    });
  },
};
