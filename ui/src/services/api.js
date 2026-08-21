const API_BASE = '/api/v1';

const VI_AI_OPERATOR_INSTRUCTION = `Yêu cầu vận hành bắt buộc của Goflow:
- Mọi phần giải thích, phản hồi hội thoại, tên workflow và tên node dành cho người dùng phải viết bằng tiếng Việt tự nhiên.
- Giữ nguyên chính xác các identifier kỹ thuật như node type, node id, parameter name, expression path, model name và URL.
- Không dịch khóa JSON kỹ thuật của workflow schema.
- Nếu trả workflow JSON, trường name của workflow/node nên dùng tiếng Việt dễ hiểu nhưng type/params/edge identifiers phải giữ đúng contract Goflow.`;

function isSensitiveAIContextKey(key) {
  const normalized = String(key || '').trim().toLowerCase().replaceAll('-', '_');
  return [
    'api_key', 'apikey', 'authorization', 'password', 'passwd', 'secret', 'token',
    'credential', 'private_key', 'service_account', 'client_secret', 'connection_string',
    'webhook_url',
  ].some((part) => normalized.includes(part));
}

function redactAIContext(value) {
  if (Array.isArray(value)) return value.map(redactAIContext);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [
    key,
    isSensitiveAIContextKey(key) ? '[REDACTED]' : redactAIContext(item),
  ]));
}

async function customFetch(url, options = {}) {
  const token = localStorage.getItem('GOFLOW_API_KEY');
  if (token) {
    if (!options.headers) options.headers = {};
    options.headers['Authorization'] = `Bearer ${token}`;
  }

  let res;
  try {
    res = await fetch(url, options);
  } catch (err) {
    throw new Error(`Không thể kết nối tới Goflow. Hãy kiểm tra tiến trình Goflow còn chạy và thử lại. Chi tiết: ${err.message}`);
  }

  if (res.status === 401) {
    const userInput = prompt('Goflow API yêu cầu xác thực. Nhập GOFLOW_API_KEY:');
    if (userInput !== null) {
      localStorage.setItem('GOFLOW_API_KEY', userInput.trim());
      if (!options.headers) options.headers = {};
      options.headers['Authorization'] = `Bearer ${userInput.trim()}`;
      try {
        res = await fetch(url, options);
      } catch (err) {
        throw new Error(`Không thể kết nối sau khi nhập API key. Hãy kiểm tra Goflow còn chạy và thử lại. Chi tiết: ${err.message}`);
      }
    }
  }

  return res;
}

export const api = {
  async getWorkflows() {
    const res = await customFetch(`${API_BASE}/workflows`);
    if (!res.ok) throw new Error('Không tải được danh sách workflow');
    return res.json();
  },

  async getWorkflow(id) {
    const res = await customFetch(`${API_BASE}/workflows/${id}`);
    if (!res.ok) throw new Error('Không tải được workflow');
    return res.json();
  },

  async createWorkflow(data) {
    const res = await customFetch(`${API_BASE}/workflows`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Không tạo được workflow');
    return res.json();
  },

  async updateWorkflow(id, data) {
    const res = await customFetch(`${API_BASE}/workflows/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Không cập nhật được workflow');
    return res.json();
  },

  async getWorkflowInterface(id) {
    const res = await customFetch(`${API_BASE}/workflows/${id}/interface`);
    if (!res.ok) throw new Error('Không tải được cấu hình giao diện workflow');
    return res.json();
  },

  async updateWorkflowInterface(id, data) {
    const res = await customFetch(`${API_BASE}/workflows/${id}/interface`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || 'Không cập nhật được cấu hình giao diện workflow');
    }
    return res.json();
  },

  async deleteWorkflow(id) {
    const res = await customFetch(`${API_BASE}/workflows/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Không xóa được workflow');
    return res.json();
  },

  async toggleWorkflowActive(id, isActive) {
    const res = await customFetch(`${API_BASE}/workflows/${id}/toggle`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_active: isActive }),
    });
    if (!res.ok) throw new Error('Không thay đổi được trạng thái kích hoạt workflow');
    return res.json();
  },

  async triggerWorkflow(id, payload = {}, async = false) {
    const res = await customFetch(`${API_BASE}/workflows/${id}/trigger?async=${async}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Goflow-Trigger-Source': 'ui' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error('Không chạy được workflow');
    return res.json();
  },

  async getExecutions(workflowId) {
    const res = await customFetch(`${API_BASE}/workflows/${workflowId}/executions`);
    if (!res.ok) throw new Error('Không tải được lịch sử chạy');
    return res.json();
  },

  async getExecutionDetail(id) {
    const res = await customFetch(`${API_BASE}/executions/${id}`);
    if (!res.ok) throw new Error('Không tải được chi tiết lần chạy');
    return res.json();
  },

  async cancelExecution(id) {
    const res = await customFetch(`${API_BASE}/executions/${id}/cancel`, { method: 'POST' });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || 'Không hủy được lần chạy');
    }
    return res.json();
  },

  async replayExecution(id) {
    const res = await customFetch(`${API_BASE}/executions/${id}/replay`, { method: 'POST' });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || 'Không phát lại được lần chạy');
    }
    return res.json();
  },

  async getCredentials() {
    const res = await customFetch(`${API_BASE}/credentials`);
    if (!res.ok) throw new Error('Không tải được credential');
    return res.json();
  },

  async createCredential(data) {
    const res = await customFetch(`${API_BASE}/credentials`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Không tạo được credential');
    return res.json();
  },

  async deleteCredential(id) {
    const res = await customFetch(`${API_BASE}/credentials/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Không xóa được credential');
    return res.json();
  },

  async getNodeDefinitions() {
    const res = await customFetch(`${API_BASE}/nodes/definitions`);
    if (!res.ok) throw new Error('Không tải được định nghĩa node');
    return res.json();
  },

  async generateAIWorkflow(messages, credentialId, currentNodes = [], currentEdges = []) {
    const localizedMessages = [
      { role: 'system', content: VI_AI_OPERATOR_INSTRUCTION },
      ...(messages || []),
    ];
    const res = await customFetch(`${API_BASE}/ai/generate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        messages: localizedMessages,
        credential_id: credentialId,
        current_nodes: redactAIContext(currentNodes),
        current_edges: redactAIContext(currentEdges),
      }),
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'AI không tạo được workflow');
    }
    return res.json();
  },

  async configureNodeParams(nodeType, prompt, currentParams, credentialId) {
    const localizedPrompt = `${VI_AI_OPERATOR_INSTRUCTION}\n\nYêu cầu cấu hình node của người dùng:\n${prompt}`;
    const res = await customFetch(`${API_BASE}/ai/configure-node`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        node_type: nodeType,
        prompt: localizedPrompt,
        current_params: redactAIContext(currentParams),
        credential_id: credentialId,
      }),
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'AI không cấu hình được node');
    }
    return res.json();
  },

  async importCurl(command) {
    const res = await customFetch(`${API_BASE}/http/import-curl`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });
    if (!res.ok) throw new Error((await res.text()) || 'Không import được cURL');
    return res.json();
  },

  async assistCode(payload) {
    const res = await customFetch(`${API_BASE}/ai/code`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error((await res.text()) || 'AI không tạo được code');
    return res.json();
  },

  async promoteCodeNode(manifest) {
    const res = await customFetch(`${API_BASE}/custom-nodes/promote`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(manifest),
    });
    if (!res.ok) throw new Error((await res.text()) || 'Không tạo được reusable node');
    return res.json();
  },

  async reviewAIWorkflow(mode, credentialId, workflow, execution = null, focus = '') {
    const res = await customFetch(`${API_BASE}/ai/review`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        mode,
        credential_id: credentialId,
        workflow,
        execution,
        focus,
      }),
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'AI không đánh giá được workflow');
    }
    return res.json();
  },
};
