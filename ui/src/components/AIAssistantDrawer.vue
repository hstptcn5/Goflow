<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { api } from '@/services/api';
import { iterateAIAgent } from '@/services/aiAgent';
import { isReviewerCredential, reviewerCredentialProvider } from '@/utils/aiReviewer';
import {
  AI_WELCOME_MESSAGE,
  clearAIChatHistory,
  loadAIChatHistory,
  saveAIChatHistory,
} from '@/utils/aiChatHistory';

const props = defineProps({
  visible: Boolean,
  currentNodes: { type: Array, default: () => [] },
  currentEdges: { type: Array, default: () => [] },
  latestExecution: { type: Object, default: null },
});

const emit = defineEmits(['close', 'loadWorkflow']);
const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

const selectedCredentialId = ref('');
const promptText = ref('');
const loading = ref(false);
const messagesListRef = ref(null);
const assistantMode = ref('build');
const messages = ref([AI_WELCOME_MESSAGE]);
let restoringHistory = false;

const modes = [
  { id: 'build', label: 'Tạo / Chỉnh sửa' },
  { id: 'agent', label: 'Agent Lab' },
  { id: 'workflow', label: 'Đánh giá workflow' },
  { id: 'latest_run', label: 'Đánh giá lần chạy gần nhất' },
];

const workflowHistoryId = computed(() => String(workflowStore.currentWorkflow?.id || '').trim());
const aiCredentials = computed(() => workflowStore.credentials.filter(isReviewerCredential));
const selectedCredential = computed(() => aiCredentials.value.find((cred) => cred.id === selectedCredentialId.value) || null);
const selectedProvider = computed(() => reviewerCredentialProvider(selectedCredential.value));
const reviewExecution = computed(() => props.latestExecution || executionStore.executionLogs[0] || null);
const isReviewMode = computed(() => assistantMode.value === 'workflow' || assistantMode.value === 'latest_run');
const reviewNeedsRun = computed(() => assistantMode.value === 'latest_run');
const canSubmit = computed(() => {
  if (loading.value || !selectedCredentialId.value) return false;
  if (assistantMode.value === 'build') return Boolean(promptText.value.trim());
  if (assistantMode.value === 'agent') return Boolean(promptText.value.trim() && props.currentNodes.length);
  if (!props.currentNodes.length) return false;
  if (reviewNeedsRun.value && !reviewExecution.value) return false;
  return true;
});
const inputPlaceholder = computed(() => {
  if (assistantMode.value === 'build') return 'Yêu cầu AI tạo, chỉnh sửa hoặc giải thích workflow...';
  if (assistantMode.value === 'agent') return 'Mục tiêu cho Agent Lab, ví dụ: làm workflow này ổn định hơn và đơn giản hơn...';
  if (assistantMode.value === 'latest_run') return 'Tùy chọn: mô tả kết quả tốt mà bạn mong muốn...';
  return 'Tùy chọn: cho Reviewer biết bạn muốn tối ưu điều gì...';
});
const sendLabel = computed(() => {
  if (assistantMode.value === 'agent') return 'Chạy Agent';
  if (isReviewMode.value) return 'Đánh giá';
  return 'Gửi';
});

onMounted(() => {
  workflowStore.fetchCredentials();
});

watch(
  () => props.visible,
  async (visible) => {
    if (!visible) return;
    workflowStore.fetchCredentials();
    await scrollToBottom();
  }
);

watch(
  aiCredentials,
  (credentials) => {
    if (credentials.length > 0 && !credentials.some((cred) => cred.id === selectedCredentialId.value)) {
      selectedCredentialId.value = credentials[0].id;
    }
    if (credentials.length === 0) selectedCredentialId.value = '';
  },
  { immediate: true }
);

watch(
  workflowHistoryId,
  async (workflowId) => {
    restoringHistory = true;
    messages.value = loadAIChatHistory(workflowId);
    await nextTick();
    restoringHistory = false;
    await scrollToBottom();
  },
  { immediate: true }
);

watch(
  messages,
  (value) => {
    if (!restoringHistory && workflowHistoryId.value) {
      saveAIChatHistory(workflowHistoryId.value, value);
    }
  },
  { deep: true }
);

function getSerializableCanvasState() {
  return {
    name: workflowStore.currentWorkflow?.name || 'Workflow',
    nodes: props.currentNodes.map((node) => ({
      id: node.id,
      type: node.data?.type || node.type,
      name: node.data?.name || node.label,
      position: node.position,
      params: node.data?.params || {},
    })),
    edges: props.currentEdges.map((edge) => ({
      id: edge.id,
      source: edge.source,
      sourceHandle: edge.sourceHandle || null,
      target: edge.target,
      targetHandle: edge.targetHandle || null,
    })),
  };
}

function pushUserMessage(text, prefix = 'msg_user') {
  messages.value.push({
    id: `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
    sender: 'user',
    type: 'text',
    text,
  });
}

function pushAIError(err) {
  messages.value.push({
    id: `msg_ai_err_${Date.now()}`,
    sender: 'ai',
    type: 'text',
    text: `❌ **Không thể trao đổi với AI**: ${err.message}`,
  });
}

async function handleBuild() {
  const text = promptText.value.trim();
  if (!text) return;
  pushUserMessage(text);
  promptText.value = '';
  loading.value = true;
  await scrollToBottom();

  const apiMessages = messages.value
    .filter((message) => message.type === 'text' || message.type === 'workflow')
    .map((message) => {
      let content = message.text || '';
      if (message.type === 'workflow' && message.workflow) {
        content = `${content}\n\n[Sơ đồ workflow hiện tại]:\n${JSON.stringify(message.workflow)}`;
      }
      return { role: message.sender === 'user' ? 'user' : 'assistant', content };
    });
  const canvas = getSerializableCanvasState();

  try {
    const response = await api.generateAIWorkflow(apiMessages, selectedCredentialId.value, canvas.nodes, canvas.edges);
    if (response.type === 'workflow') {
      messages.value.push({
        id: `msg_ai_${Date.now()}`,
        sender: 'ai',
        type: 'workflow',
        text: response.text || '',
        workflow: response.workflow,
        validated: response.validated === true,
      });
    } else {
      messages.value.push({ id: `msg_ai_${Date.now()}`, sender: 'ai', type: 'text', text: response.text || '' });
    }
  } catch (err) {
    pushAIError(err);
  } finally {
    loading.value = false;
    await scrollToBottom();
  }
}

async function handleReview() {
  const focus = promptText.value.trim();
  const mode = assistantMode.value;
  const label = mode === 'latest_run' ? 'Đánh giá lần chạy gần nhất' : 'Đánh giá workflow';
  pushUserMessage(focus ? `${label}: ${focus}` : label, 'msg_user_review');
  promptText.value = '';
  loading.value = true;
  await scrollToBottom();

  try {
    const response = await api.reviewAIWorkflow(
      mode,
      selectedCredentialId.value,
      getSerializableCanvasState(),
      mode === 'latest_run' ? reviewExecution.value : null,
      focus
    );
    messages.value.push({ id: `msg_ai_review_${Date.now()}`, sender: 'ai', type: 'review', review: response });
  } catch (err) {
    pushAIError(err);
  } finally {
    loading.value = false;
    await scrollToBottom();
  }
}

async function handleAgent() {
  const goal = promptText.value.trim();
  if (!goal) return;
  pushUserMessage(`Agent Lab: ${goal}`, 'msg_user_agent');
  promptText.value = '';
  loading.value = true;
  await scrollToBottom();

  try {
    const response = await iterateAIAgent({
      goal,
      credentialId: selectedCredentialId.value,
      workflow: getSerializableCanvasState(),
      execution: reviewExecution.value,
      maxIterations: 2,
    });
    messages.value.push({ id: `msg_ai_agent_${Date.now()}`, sender: 'ai', type: 'agent', agent: response });
  } catch (err) {
    pushAIError(err);
  } finally {
    loading.value = false;
    await scrollToBottom();
  }
}

async function handleSend() {
  if (!canSubmit.value) return;
  if (assistantMode.value === 'build') return handleBuild();
  if (assistantMode.value === 'agent') return handleAgent();
  return handleReview();
}

function handleLoad(workflow) {
  if (workflow) emit('loadWorkflow', workflow);
}

function applyExplicitParamDeletions(deleteParams = {}) {
  for (const node of props.currentNodes) {
    const names = Array.isArray(deleteParams?.[node.id]) ? deleteParams[node.id] : [];
    if (!names.length || !node?.data?.params) continue;
    for (const name of names) delete node.data.params[name];
  }
}

function handleApplyReview(review) {
  if (!review?.proposed_workflow || !review?.proposal_validated) return;
  applyExplicitParamDeletions(review.proposal_delete_params || {});
  emit('loadWorkflow', review.proposed_workflow);
}

function clearHistory() {
  clearAIChatHistory(workflowHistoryId.value);
  messages.value = [AI_WELCOME_MESSAGE];
}

function scoreEntries(scores = {}) {
  const labels = {
    reliability: 'Độ tin cậy',
    security: 'Bảo mật',
    data_correctness: 'Dữ liệu',
    cost_efficiency: 'Chi phí',
    maintainability: 'Dễ bảo trì',
    output_quality: 'Chất lượng đầu ra',
  };
  return Object.entries(labels).map(([key, label]) => {
    const raw = scores?.[key];
    const available = typeof raw === 'number' && Number.isFinite(raw);
    return { key, label, value: available ? raw : 'N/A' };
  });
}

function severityLabel(value) {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'high') return 'CAO';
  if (normalized === 'low') return 'THẤP';
  return 'TRUNG BÌNH';
}

function agentStatusLabel(status) {
  if (status === 'passed') return 'Safe test: PASS';
  if (status === 'blocked') return 'Safe test: BLOCKED';
  if (status === 'failed') return 'Safe test: FAILED';
  return 'Safe test: chưa chạy';
}

async function scrollToBottom() {
  await nextTick();
  if (messagesListRef.value) messagesListRef.value.scrollTop = messagesListRef.value.scrollHeight;
}

function renderMarkdown(text) {
  if (!text) return '';
  let html = String(text).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  html = html.replace(/\n/g, '<br>');
  html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*(.*?)\*/g, '<em>$1</em>');
  html = html.replace(/<br>\s*[-*]\s+/g, '<br>• ');
  if (html.startsWith('- ') || html.startsWith('* ')) html = `• ${html.slice(2)}`;
  return html;
}
</script>

<template>
  <div v-show="visible" class="ai-assistant-backdrop" @click.self="emit('close')">
    <aside class="ai-assistant-drawer glass-panel">
      <header class="drawer-header">
        <div class="header-title">
          <span class="icon">🤖</span>
          <div>
            <h3>Trợ lý AI</h3>
            <p>Tạo, đánh giá và Agent Lab với DeepSeek / OpenAI</p>
          </div>
        </div>
        <div class="header-actions">
          <button type="button" class="btn-clear" :disabled="loading || messages.length <= 1" @click="clearHistory">Xóa lịch sử</button>
          <button type="button" class="btn-close" aria-label="Đóng trợ lý AI" @click="emit('close')">✕</button>
        </div>
      </header>

      <div class="mode-selector" role="tablist" aria-label="Chế độ trợ lý AI">
        <button
          v-for="mode in modes"
          :key="mode.id"
          type="button"
          class="mode-button"
          :class="{ active: assistantMode === mode.id }"
          :aria-selected="assistantMode === mode.id"
          @click="assistantMode = mode.id"
        >{{ mode.label }}</button>
      </div>

      <div class="key-selector-bar">
        <label>Bộ não AI:</label>
        <select v-model="selectedCredentialId" class="form-select select-sm" :disabled="loading">
          <option value="">-- Chọn credential OpenAI / DeepSeek --</option>
          <option v-for="cred in aiCredentials" :key="cred.id" :value="cred.id">
            {{ cred.name }} · {{ reviewerCredentialProvider(cred) }}
          </option>
        </select>
        <span v-if="selectedProvider" class="provider-badge">{{ selectedProvider }}</span>
      </div>

      <div v-if="assistantMode === 'agent'" class="safety-note agent-note">
        <strong>Agent Lab có giới hạn.</strong>
        Agent được đọc node registry, workflow và execution gần nhất, tự đề xuất và lặp lại khi safe test thất bại. Nó chỉ tự test-run graph deterministic; HTTP, Python, DB, file, email, plugin và các node có side effect sẽ bị chặn tự chạy. Proposal không tự Save hoặc Activate.
      </div>
      <div v-else-if="isReviewMode" class="safety-note">
        <strong>Đánh giá có xác nhận của người dùng.</strong> Reviewer chỉ phân tích và đề xuất; không tự lưu hoặc chạy.
        <span v-if="reviewNeedsRun && !reviewExecution"> Hãy chạy workflow ít nhất một lần trước.</span>
      </div>
      <div v-else class="helper-tip">AI chỉ thay đổi canvas sau khi bạn bấm nút nạp proposal.</div>

      <div ref="messagesListRef" class="messages-list">
        <div v-for="msg in messages" :key="msg.id" class="message-row" :class="msg.sender">
          <div class="avatar">{{ msg.sender === 'user' ? '👤' : '🤖' }}</div>
          <div class="message-content">
            <div v-if="msg.type === 'text'" class="text-bubble" v-html="renderMarkdown(msg.text)"></div>

            <div v-else-if="msg.type === 'workflow'" class="workflow-card">
              <p v-if="msg.text" v-html="renderMarkdown(msg.text)"></p>
              <strong>✨ Sơ đồ workflow mới / đã chỉnh sửa</strong>
              <p>{{ msg.workflow?.name || 'Workflow chưa đặt tên' }}</p>
              <div class="sequence-chips">
                <span v-for="node in msg.workflow?.nodes || []" :key="node.id" class="chip">{{ node.name || node.type }}</span>
              </div>
              <span v-if="msg.validated" class="validated-badge">Đã được Goflow kiểm tra</span>
              <button type="button" class="btn btn-success btn-sm" @click="handleLoad(msg.workflow)">Nạp lên canvas</button>
            </div>

            <div v-else-if="msg.type === 'agent'" class="agent-card">
              <div class="card-title-row">
                <strong>🧪 Agent Lab</strong>
                <span class="agent-status" :class="`status-${msg.agent?.test_status || 'not-run'}`">{{ agentStatusLabel(msg.agent?.test_status) }}</span>
              </div>
              <p>{{ msg.agent?.summary }}</p>
              <p v-if="msg.agent?.expected_improvement"><strong>Kỳ vọng:</strong> {{ msg.agent.expected_improvement }}</p>
              <p><strong>Số vòng:</strong> {{ msg.agent?.iterations || 0 }}</p>
              <ul v-if="msg.agent?.test_blocked_reasons?.length" class="blocked-list">
                <li v-for="reason in msg.agent.test_blocked_reasons" :key="reason">{{ reason }}</li>
              </ul>
              <details v-if="msg.agent?.test_execution">
                <summary>Xem bằng chứng safe test</summary>
                <pre>{{ JSON.stringify(msg.agent.test_execution, null, 2) }}</pre>
              </details>
              <button
                v-if="msg.agent?.proposed_workflow && msg.agent?.proposal_validated"
                type="button"
                class="btn btn-success btn-sm"
                @click="handleLoad(msg.agent.proposed_workflow)"
              >Nạp proposal lên canvas</button>
            </div>

            <div v-else-if="msg.type === 'review'" class="review-card">
              <div class="card-title-row">
                <div>
                  <strong>{{ msg.review?.mode === 'latest_run' ? 'Đánh giá lần chạy gần nhất' : 'Đánh giá workflow' }}</strong>
                  <p>{{ msg.review?.provider }} · {{ msg.review?.model }}</p>
                </div>
                <span>{{ msg.review?.findings?.length || 0 }} phát hiện</span>
              </div>
              <p>{{ msg.review?.summary }}</p>
              <div class="score-grid">
                <div v-for="score in scoreEntries(msg.review?.scores)" :key="score.key" class="score-item">
                  <span>{{ score.label }}:</span><strong>{{ score.value }}</strong>
                </div>
              </div>
              <div v-for="finding in msg.review?.findings || []" :key="finding.id" class="finding-card">
                <div class="finding-heading"><strong>{{ finding.title }}</strong><span>{{ severityLabel(finding.severity) }}</span></div>
                <p>{{ finding.why }}</p>
                <p><strong>Tác động:</strong> {{ finding.impact }}</p>
                <p><strong>Đề xuất:</strong> {{ finding.suggested_change }}</p>
                <p v-if="finding.evidence"><strong>Bằng chứng:</strong> {{ finding.evidence }}</p>
              </div>
              <button
                v-if="msg.review?.proposed_workflow && msg.review?.proposal_validated"
                type="button"
                class="btn btn-success btn-sm"
                @click="handleApplyReview(msg.review)"
              >Áp dụng đề xuất lên canvas</button>
            </div>
          </div>
        </div>
        <div v-if="loading" class="loading-row">🤖 Đang xử lý…</div>
      </div>

      <footer class="composer">
        <textarea
          v-model="promptText"
          class="prompt-input"
          :placeholder="inputPlaceholder"
          :disabled="loading"
          rows="3"
          @keydown.ctrl.enter.prevent="handleSend"
        ></textarea>
        <button type="button" class="btn btn-primary btn-send" :disabled="!canSubmit" @click="handleSend">{{ sendLabel }}</button>
      </footer>
      <div class="persistence-note">Lịch sử được lưu cục bộ theo workflow trên trình duyệt này.</div>
    </aside>
  </div>
</template>

<style scoped>
.ai-assistant-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1200;
  background: rgb(15 23 42 / 0.28);
  display: flex;
  justify-content: flex-end;
}
.ai-assistant-drawer {
  width: min(620px, 96vw);
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--color-surface, #fff);
  border-left: 1px solid var(--color-border, #dbe4ee);
  box-shadow: -12px 0 32px rgb(15 23 42 / 0.16);
}
.drawer-header, .card-title-row, .key-selector-bar, .composer, .header-actions {
  display: flex;
  align-items: center;
}
.drawer-header {
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid var(--color-border, #e2e8f0);
}
.header-title { display: flex; gap: 10px; align-items: center; }
.header-title h3, .header-title p, .card-title-row p { margin: 0; }
.header-title p { color: #64748b; font-size: 12px; }
.header-actions { gap: 8px; }
.btn-clear, .btn-close {
  border: 1px solid #cbd5e1;
  background: #fff;
  border-radius: 8px;
  padding: 6px 9px;
  cursor: pointer;
}
.btn-clear:disabled { opacity: .45; cursor: default; }
.mode-selector { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; padding: 10px 12px; }
.mode-button { border: 1px solid #cbd5e1; background: #f8fafc; border-radius: 8px; padding: 8px 6px; font-size: 12px; cursor: pointer; }
.mode-button.active { background: #e0e7ff; border-color: #818cf8; font-weight: 700; }
.key-selector-bar { gap: 8px; padding: 0 12px 10px; }
.key-selector-bar select { flex: 1; min-width: 0; }
.provider-badge, .validated-badge, .agent-status { border-radius: 999px; padding: 3px 8px; font-size: 11px; font-weight: 700; }
.provider-badge { background: #eef2ff; }
.validated-badge, .status-passed { background: #dcfce7; color: #166534; }
.status-blocked { background: #fef3c7; color: #92400e; }
.status-failed { background: #fee2e2; color: #991b1b; }
.safety-note, .helper-tip { margin: 0 12px 10px; padding: 9px 10px; border-radius: 8px; font-size: 12px; background: #f8fafc; border: 1px solid #e2e8f0; }
.agent-note { background: #f5f3ff; border-color: #ddd6fe; }
.messages-list { flex: 1; overflow-y: auto; padding: 14px; display: flex; flex-direction: column; gap: 12px; }
.message-row { display: flex; gap: 8px; align-items: flex-start; }
.message-row.user { flex-direction: row-reverse; }
.avatar { width: 30px; height: 30px; display: grid; place-items: center; border-radius: 50%; background: #f1f5f9; flex: 0 0 auto; }
.message-content { max-width: 88%; min-width: 0; }
.text-bubble, .workflow-card, .review-card, .agent-card { border: 1px solid #e2e8f0; border-radius: 12px; padding: 11px 12px; background: #fff; overflow-wrap: anywhere; }
.user .text-bubble { background: #eef2ff; border-color: #c7d2fe; }
.workflow-card, .review-card, .agent-card { display: grid; gap: 9px; }
.card-title-row { justify-content: space-between; gap: 12px; }
.sequence-chips { display: flex; flex-wrap: wrap; gap: 6px; }
.chip { background: #f1f5f9; border-radius: 999px; padding: 4px 8px; font-size: 12px; }
.score-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
.score-item { display: flex; justify-content: space-between; gap: 8px; background: #f8fafc; padding: 6px 8px; border-radius: 7px; font-size: 12px; }
.finding-card { border-top: 1px solid #e2e8f0; padding-top: 8px; }
.finding-heading { display: flex; justify-content: space-between; gap: 8px; }
.finding-card p { margin: 5px 0; font-size: 12px; }
.blocked-list { margin: 0; padding-left: 18px; font-size: 12px; }
details pre { white-space: pre-wrap; max-height: 220px; overflow: auto; font-size: 11px; background: #0f172a; color: #e2e8f0; padding: 8px; border-radius: 8px; }
.loading-row { color: #64748b; font-size: 12px; }
.composer { gap: 8px; padding: 12px; border-top: 1px solid #e2e8f0; align-items: stretch; }
.prompt-input { flex: 1; resize: vertical; min-height: 66px; border: 1px solid #cbd5e1; border-radius: 9px; padding: 9px; }
.btn-send { min-width: 94px; }
.persistence-note { padding: 0 12px 10px; color: #64748b; font-size: 11px; }
@media (max-width: 640px) {
  .mode-selector { grid-template-columns: repeat(2, 1fr); }
  .score-grid { grid-template-columns: 1fr; }
}
</style>
