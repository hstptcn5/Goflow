const API_BASE = '/api/v1';

async function agentFetch(url, options = {}) {
  const token = typeof window !== 'undefined' ? window.localStorage.getItem('GOFLOW_API_KEY') : '';
  const headers = { ...(options.headers || {}) };
  if (token) headers.Authorization = `Bearer ${token}`;
  let response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    throw new Error(`Không thể kết nối Agent Lab: ${err.message}`);
  }
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Agent Lab trả HTTP ${response.status}`);
  }
  return response.json();
}

export async function iterateAIAgent({ goal, credentialId, workflow, execution = null, maxIterations = 2 }) {
  return agentFetch(`${API_BASE}/ai/agent/iterate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      goal,
      credential_id: credentialId,
      workflow,
      execution,
      max_iterations: maxIterations,
    }),
  });
}
