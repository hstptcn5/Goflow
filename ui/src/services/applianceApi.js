const APPLIANCE_BASE = '/api/appliance';

function jsonHeaders(token) {
  return {
    'Content-Type': 'application/json',
    'X-Goflow-Appliance-Token': token,
  };
}

async function readError(res, fallback) {
  const text = await res.text();
  return text.trim() || fallback;
}

async function request(path, options = {}) {
  const res = await fetch(`${APPLIANCE_BASE}${path}`, options);
  if (!res.ok) {
    throw new Error(await readError(res, `Appliance request failed with ${res.status}`));
  }
  return res.json();
}

export const applianceApi = {
  async bootstrap() {
    const res = await fetch(`${APPLIANCE_BASE}/bootstrap`);
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(await readError(res, 'Failed to load appliance bootstrap'));
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
