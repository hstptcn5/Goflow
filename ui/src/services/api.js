const API_BASE = '/api/v1';

export const api = {
  // Workflows
  async getWorkflows() {
    const res = await fetch(`${API_BASE}/workflows`);
    if (!res.ok) throw new Error('Failed to fetch workflows');
    return res.json();
  },

  async getWorkflow(id) {
    const res = await fetch(`${API_BASE}/workflows/${id}`);
    if (!res.ok) throw new Error('Failed to fetch workflow');
    return res.json();
  },

  async createWorkflow(data) {
    const res = await fetch(`${API_BASE}/workflows`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Failed to create workflow');
    return res.json();
  },

  async updateWorkflow(id, data) {
    const res = await fetch(`${API_BASE}/workflows/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Failed to update workflow');
    return res.json();
  },

  async deleteWorkflow(id) {
    const res = await fetch(`${API_BASE}/workflows/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed to delete workflow');
    return res.json();
  },

  async toggleWorkflowActive(id, isActive) {
    const res = await fetch(`${API_BASE}/workflows/${id}/toggle`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_active: isActive }),
    });
    if (!res.ok) throw new Error('Failed to toggle active status');
    return res.json();
  },

  async triggerWorkflow(id, payload = {}, async = false) {
    const res = await fetch(`${API_BASE}/workflows/${id}/trigger?async=${async}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('Failed to trigger workflow');
    return res.json();
  },

  // Executions
  async getExecutions(workflowId) {
    const res = await fetch(`${API_BASE}/workflows/${workflowId}/executions`);
    if (!res.ok) throw new Error('Failed to fetch execution history');
    return res.json();
  },

  async getExecutionDetail(id) {
    const res = await fetch(`${API_BASE}/executions/${id}`);
    if (!res.ok) throw new Error('Failed to fetch execution detail');
    return res.json();
  },

  // Credentials
  async getCredentials() {
    const res = await fetch(`${API_BASE}/credentials`);
    if (!res.ok) throw new Error('Failed to fetch credentials');
    return res.json();
  },

  async createCredential(data) {
    const res = await fetch(`${API_BASE}/credentials`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Failed to create credential');
    return res.json();
  },

  async deleteCredential(id) {
    const res = await fetch(`${API_BASE}/credentials/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed to delete credential');
    return res.json();
  },

  // Node Definitions
  async getNodeDefinitions() {
    const res = await fetch(`${API_BASE}/nodes/definitions`);
    if (!res.ok) throw new Error('Failed to fetch node definitions');
    return res.json();
  },
};
