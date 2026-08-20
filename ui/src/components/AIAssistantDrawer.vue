<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { api } from '@/services/api';
import { isReviewerCredential, reviewerCredentialProvider } from '@/utils/aiReviewer';

const props = defineProps({
  visible: Boolean,
  currentNodes: Array,
  currentEdges: Array,
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

const modes = [
  { id: 'build', label: 'Tạo / Chỉnh sửa' },
  { id: 'workflow', label: 'Đánh giá workflow' },
  { id: 'latest_run', label: 'Đánh giá lần chạy gần nhất' },
];

const messages = ref([
  {
    id: 'welcome',
    sender: 'ai',
    type: 'text',
    text: '👋 Tôi là Trợ lý AI của Goflow. Tôi có thể tạo, chỉnh sửa hoặc đánh giá workflow. Ở chế độ đánh giá, tôi chỉ đề xuất; không có thay đổi nào được áp dụng, lưu hoặc chạy cho tới khi bạn chủ động bấm nút.'
  }
]);

const aiCredentials = computed(() => workflowStore.credentials.filter(isReviewerCredential));
const selectedCredential = computed(() => aiCredentials.value.find((cred) => cred.id === selectedCredentialId.value) || null);
const selectedProvider = computed(() => reviewerCredentialProvider(selectedCredential.value));
const reviewExecution = computed(() => props.latestExecution || executionStore.executionLogs[0] || null);
const modeDefinition = computed(() => modes.find((mode) => mode.id === assistantMode.value) || modes[0]);
const isReviewMode = computed(() => assistantMode.value !== 'build');
const reviewNeedsRun = computed(() => assistantMode.value === 'latest_run');
const canSubmit = computed(() => {
  if (loading.value || !selectedCredentialId.value) return false;
  if (assistantMode.value === 'build') return Boolean(promptText.value.trim());
  if (!(props.currentNodes || []).length) return false;
  if (reviewNeedsRun.value && !reviewExecution.value) return false;
  return true;
});
const inputPlaceholder = computed(() => {
  if (assistantMode.value === 'build') return 'Yêu cầu AI tạo, chỉnh sửa hoặc giải thích workflow...';
  if (assistantMode.value === 'latest_run') return 'Tùy chọn: mô tả kết quả tốt mà bạn mong muốn...';
  return 'Tùy chọn: cho Reviewer biết bạn muốn tối ưu điều gì...';
});

onMounted(() => {
  workflowStore.fetchCredentials();
});

watch(
  () => props.visible,
  (isVis) => {
    if (isVis) {
      workflowStore.fetchCredentials();
      scrollToBottom();
    }
  }
);

watch(
  () => aiCredentials.value,
  (newVal) => {
    if (newVal.length > 0 && !newVal.some((cred) => cred.id === selectedCredentialId.value)) {
      selectedCredentialId.value = newVal[0].id;
    }
    if (newVal.length === 0) selectedCredentialId.value = '';
  },
  { immediate: true }
);

function getSerializableCanvasState() {
  const serialNodes = (props.currentNodes || []).map((n) => ({
    id: n.id,
    type: n.data?.type || n.type,
    name: n.data?.name || n.label,
    position: n.position,
    params: n.data?.params || {},
  }));
  const serialEdges = (props.currentEdges || []).map((e) => ({
    id: e.id,
    source: e.source,
    sourceHandle: e.sourceHandle || null,
    target: e.target,
    targetHandle: e.targetHandle || null,
  }));
  return {
    name: workflowStore.currentWorkflow?.name || 'Workflow',
    nodes: serialNodes,
    edges: serialEdges,
  };
}

async function handleBuild() {
  const text = promptText.value.trim();
  if (!text) return;

  messages.value.push({
    id: `msg_user_${Date.now()}`,
    sender: 'user',
    type: 'text',
    text,
  });
  promptText.value = '';
  loading.value = true;
  await scrollToBottom();

  const apiMessages = messages.value
    .filter((m) => m.type !== 'review')
    .map((m) => {
      let content = m.text || '';
      if (m.type === 'workflow' && m.workflow) {
        content = `${m.text || ''}\n\n[Sơ đồ workflow hiện tại]:\n${JSON.stringify(m.workflow)}`;
      }
      return {
        role: m.sender === 'user' ? 'user' : 'assistant',
        content,
      };
    });

  const canvasState = getSerializableCanvasState();

  try {
    const response = await api.generateAIWorkflow(
      apiMessages,
      selectedCredentialId.value,
      canvasState.nodes,
      canvasState.edges
    );

    if (response.type === 'text') {
      messages.value.push({
        id: `msg_ai_${Date.now()}`,
        sender: 'ai',
        type: 'text',
        text: response.text,
      });
    } else if (response.type === 'workflow') {
      messages.value.push({
        id: `msg_ai_${Date.now()}`,
        sender: 'ai',
        type: 'workflow',
        workflow: response.workflow,
        validated: response.validated === true,
      });
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
  messages.value.push({
    id: `msg_user_review_${Date.now()}`,
    sender: 'user',
    type: 'text',
    text: focus ? `${label}: ${focus}` : label,
  });
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
    messages.value.push({
      id: `msg_ai_review_${Date.now()}`,
      sender: 'ai',
      type: 'review',
      review: response,
    });
  } catch (err) {
    pushAIError(err);
  } finally {
    loading.value = false;
    await scrollToBottom();
  }
}

async function handleSend() {
  if (!canSubmit.value) return;
  if (assistantMode.value === 'build') await handleBuild();
  else await handleReview();
}

function pushAIError(err) {
  messages.value.push({
    id: `msg_ai_err_${Date.now()}`,
    sender: 'ai',
    type: 'text',
    text: `❌ **Không thể trao đổi với AI**: ${err.message}`,
  });
}

function handleLoad(workflow) {
  if (workflow) emit('loadWorkflow', workflow);
}

function applyExplicitParamDeletions(deleteParams = {}) {
  const requested = deleteParams || {};
  for (const node of props.currentNodes || []) {
    const names = Array.isArray(requested[node.id]) ? requested[node.id] : [];
    if (!names.length || !node?.data?.params) continue;
    for (const name of names) {
      delete node.data.params[name];
    }
  }
}

function handleApplyReview(review) {
  if (!review?.proposed_workflow || !review?.proposal_validated) return;
  applyExplicitParamDeletions(review.proposal_delete_params);
  emit('loadWorkflow', review.proposed_workflow);
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
    return { key, label, value: available ? raw : 'N/A', available };
  });
}

function severityLabel(value) {
  const normalized = String(value || '').toLowerCase();
  if (normalized === 'high') return 'CAO';
  if (normalized === 'low') return 'THẤP';
  return 'TRUNG BÌNH';
}

function deleteParamCount(deleteParams = {}) {
  return Object.values(deleteParams || {}).reduce((total, names) => total + (Array.isArray(names) ? names.length : 0), 0);
}

async function scrollToBottom() {
  await nextTick();
  if (messagesListRef.value) messagesListRef.value.scrollTop = messagesListRef.value.scrollHeight;
}

function renderMarkdown(text) {
  if (!text) return '';
  let html = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  html = html.replace(/\n/g, '<br>');
  html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*(.*?)\*/g, '<em>$1</em>');
  html = html.replace(/<br>\s*[-*]\s+/g, '<br>• ');
  if (html.startsWith('- ') || html.startsWith('* ')) html = '• ' + html.substring(2);
  return html;
}
</script>

<template>
  <div v-show="visible" class="ai-assistant-backdrop" @click.self="emit('close')">
    <aside class="ai-assistant-drawer glass-panel">
      <div class="drawer-header">
        <div class="header-title">
          <span class="icon">🤖</span>
          <div>
            <h3>Trợ lý AI</h3>
            <p class="subtitle">Tạo, đánh giá và cải thiện với xác nhận của người dùng</p>
          </div>
        </div>
        <button class="btn-close" aria-label="Đóng trợ lý AI" @click="emit('close')">✕</button>
      </div>

      <div class="chat-container">
        <div class="mode-selector" role="tablist" aria-label="Chế độ trợ lý AI">
          <button
            v-for="mode in modes"
            :key="mode.id"
            type="button"
            class="mode-button"
            :class="{ active: assistantMode === mode.id }"
            :aria-selected="assistantMode === mode.id"
            @click="assistantMode = mode.id"
          >
            {{ mode.label }}
          </button>
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
          <div v-if="aiCredentials.length === 0" class="no-keys-error">
            Hãy tạo credential API_KEY OpenAI hoặc DeepSeek trước. Reviewer không tự đoán generic API key thuộc nhà cung cấp nào.
          </div>
        </div>

        <div v-if="isReviewMode" class="review-safety-note">
          <strong>Đánh giá có xác nhận của người dùng.</strong> Reviewer chỉ trả findings và proposal đã được Goflow kiểm tra. Nó không thể tự lưu, kích hoạt, chạy hoặc áp dụng thay đổi.
          <span v-if="reviewNeedsRun && !reviewExecution">Hãy chạy workflow ít nhất một lần trước khi dùng “Đánh giá lần chạy gần nhất”.</span>
        </div>
        <div v-else class="chat-helper-tip">
          Nếu AI đề xuất workflow, canvas chỉ thay đổi sau khi bạn bấm <strong>Nạp lên canvas</strong>.
        </div>

        <div class="messages-list" ref="messagesListRef">
          <div
            v-for="msg in messages"
            :key="msg.id"
            class="message-bubble-wrapper"
            :class="msg.sender"
          >
            <div class="avatar">{{ msg.sender === 'user' ? '👤' : '🤖' }}</div>
            <div class="message-content">
              <div
                v-if="msg.type === 'text'"
                class="text-bubble"
                v-html="renderMarkdown(msg.text)"
              ></div>

              <div v-else-if="msg.type === 'workflow'" class="workflow-message">
                <div v-if="msg.text" class="text-bubble" v-html="renderMarkdown(msg.text)"></div>
                <div class="workflow-bubble">
                  <div class="workflow-bubble-header">
                    <span class="success-icon">✨</span>
                    <div>
                      <strong>Sơ đồ workflow mới / đã chỉnh sửa</strong>
                      <p class="workflow-name-preview">Tên: {{ msg.workflow.name || 'Workflow chưa đặt tên' }}</p>
                      <p v-if="msg.validated" class="workflow-validation">Đã được Goflow kiểm tra</p>
                    </div>
                  </div>
                  <div class="pipeline-flow-preview">
                    <div class="sequence-chips">
                      <span v-for="(node, index) in msg.workflow.nodes" :key="node.id" class="chip">
                        {{ node.name || node.type }}
                        <span v-if="index < msg.workflow.nodes.length - 1" class="arrow">→</span>
                      </span>
                    </div>
                  </div>
                  <button class="btn btn-success btn-sm btn-load" @click="handleLoad(msg.workflow)">
                    Nạp lên canvas
                  </button>
                </div>
              </div>

              <div v-else-if="msg.type === 'review'" class="review-card">
                <div class="review-header">
                  <div>
                    <strong>{{ msg.review.mode === 'latest_run' ? 'Đánh giá lần chạy gần nhất' : 'Đánh giá workflow' }}</strong>
                    <p>{{ msg.review.provider }} · {{ msg.review.model }}</p>
                  </div>
                  <span class="review-count">{{ msg.review.findings?.length || 0 }} phát hiện</span>
                </div>

                <p class="review-summary">{{ msg.review.summary }}</p>

                <div class="score-grid">
                  <div v-for="score in scoreEntries(msg.review.scores)" :key="score.key" class="score-item" :class="{ unavailable: !score.available }">
                    <span>{{ score.label }}</span>
                    <strong>{{ score.value }}</strong>
                  </div>
                </div>

                <div class="findings-list">
                  <article v-for="finding in msg.review.findings || []" :key="finding.id" class="finding-card">
                    <div class="finding-heading">
                      <span class="severity" :class="`severity-${finding.severity}`">{{ severityLabel(finding.severity) }}</span>
                      <strong>{{ finding.title }}</strong>
                    </div>
                    <p><b>Lý do:</b> {{ finding.why }}</p>
                    <p><b>Tác động:</b> {{ finding.impact }}</p>
                    <p><b>Đề xuất:</b> {{ finding.suggested_change }}</p>
                    <p v-if="finding.evidence"><b>Bằng chứng:</b> {{ finding.evidence }}</p>
                    <p v-if="typeof finding.confidence === 'number'"><b>Độ tin cậy:</b> {{ finding.confidence }}%</p>
                  </article>
                </div>

                <div v-if="msg.review.proposed_workflow && msg.review.proposal_validated" class="proposal-box">
                  <strong>Đề xuất cải thiện đã được kiểm tra</strong>
                  <p>{{ msg.review.proposal_summary }}</p>
                  <p v-if="msg.review.expected_improvement"><b>Kỳ vọng:</b> {{ msg.review.expected_improvement }}</p>
                  <p v-if="deleteParamCount(msg.review.proposal_delete_params)">
                    <b>Xóa có chủ đích:</b> {{ deleteParamCount(msg.review.proposal_delete_params) }} parameter cũ sẽ bị xóa khỏi canvas khi bạn bấm Áp dụng.
                  </p>
                  <div class="sequence-chips">
                    <span v-for="node in msg.review.proposed_workflow.nodes || []" :key="node.id" class="chip">
                      {{ node.name || node.type }}
                    </span>
                  </div>
                  <button class="btn btn-success btn-sm btn-load" @click="handleApplyReview(msg.review)">
                    Áp dụng đề xuất lên canvas
                  </button>
                  <p class="human-gate-copy">Chỉ canvas chưa lưu được cập nhật. Goflow không tự lưu, kích hoạt hay chạy workflow.</p>
                </div>

                <div v-else-if="msg.review.proposal_validation_issues?.length" class="proposal-invalid">
                  <strong>Proposal không được mở cho thao tác Áp dụng vì Goflow validation đã từ chối.</strong>
                  <p v-for="issue in msg.review.proposal_validation_issues" :key="issue">{{ issue }}</p>
                </div>
              </div>
            </div>
          </div>

          <div v-if="loading" class="message-bubble-wrapper ai loading">
            <div class="avatar">🤖</div>
            <div class="message-content">
              <div class="text-bubble loading-bubble">
                <div class="typing-indicator"><span></span><span></span><span></span></div>
              </div>
            </div>
          </div>
        </div>

        <div class="chat-input-bar">
          <textarea
            v-model="promptText"
            class="form-textarea chat-input"
            :placeholder="inputPlaceholder"
            rows="2"
            @keydown.enter.prevent="handleSend"
            :disabled="loading || !selectedCredentialId"
          ></textarea>
          <button
            class="btn btn-primary btn-send"
            @click="handleSend"
            :disabled="!canSubmit"
            :title="modeDefinition.label"
          >
            {{ isReviewMode ? 'Đánh giá' : '✈️' }}
          </button>
        </div>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.ai-assistant-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(15, 23, 42, 0.15);
  backdrop-filter: blur(4px);
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
}

.ai-assistant-drawer {
  width: 480px;
  height: 100%;
  border-radius: 0;
  background: rgba(255, 255, 255, 0.98);
  backdrop-filter: blur(12px);
  box-shadow: -10px 0 30px rgba(0, 0, 0, 0.1);
  display: flex;
  flex-direction: column;
  animation: slideIn 0.25s ease-out;
  border-left: 1px solid var(--border-color);
}

@keyframes slideIn {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.drawer-header {
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #f8fafc;
}

.header-title { display: flex; align-items: center; gap: 12px; }
.header-title .icon { font-size: 2rem; }
.header-title h3 { font-size: 1.05rem; font-weight: 800; color: #0f172a; margin: 0; }
.header-title .subtitle { font-size: 0.725rem; color: #64748b; margin: 2px 0 0 0; }

.btn-close {
  background: transparent;
  border: none;
  font-size: 1.1rem;
  color: #64748b;
  cursor: pointer;
  padding: 4px;
}
.btn-close:hover { color: #0f172a; }

.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
  background: #f1f5f9;
}

.mode-selector {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  background: #fff;
  border-bottom: 1px solid var(--border-color);
}

.mode-button {
  border: 0;
  border-right: 1px solid var(--border-color);
  background: #fff;
  color: #64748b;
  cursor: pointer;
  font: inherit;
  font-size: 0.7rem;
  font-weight: 750;
  padding: 9px 6px;
}
.mode-button:last-child { border-right: 0; }
.mode-button.active { background: #eff6ff; color: #1d4ed8; }

.chat-helper-tip,
.review-safety-note {
  padding: 8px 16px;
  background: #f8fafc;
  border-bottom: 1px solid var(--border-color);
  font-size: 0.7rem;
  color: #475569;
  line-height: 1.4;
}
.review-safety-note { background: #fffbeb; color: #713f12; }
.review-safety-note span { display: block; margin-top: 4px; font-weight: 700; }

.key-selector-bar {
  padding: 10px 16px;
  background: #ffffff;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.key-selector-bar label { font-size: 0.725rem; font-weight: 700; color: #475569; }
.select-sm { flex: 1; min-width: 180px; padding: 4px 8px; font-size: 0.75rem; }
.provider-badge { text-transform: uppercase; font-size: 0.62rem; font-weight: 800; color: #334155; }
.no-keys-error { width: 100%; font-size: 0.675rem; color: #ef4444; font-weight: 600; margin-top: 2px; }

.messages-list {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-bubble-wrapper { display: flex; gap: 10px; max-width: 92%; }
.message-bubble-wrapper.user { align-self: flex-end; flex-direction: row-reverse; }
.message-bubble-wrapper.ai { align-self: flex-start; }
.avatar {
  font-size: 1.25rem;
  width: 28px;
  height: 28px;
  background: #ffffff;
  border: 1px solid var(--border-color);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
.message-content { flex: 1; min-width: 0; }
.text-bubble {
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 0.775rem;
  line-height: 1.45;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
.user .text-bubble { background: #2563eb; color: #ffffff; border-top-right-radius: 2px; }
.ai .text-bubble { background: #ffffff; color: #1e293b; border-top-left-radius: 2px; border: 1px solid var(--border-color); }

.workflow-message { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.workflow-bubble,
.review-card {
  background: #ffffff;
  border: 1px solid #bbf7d0;
  padding: 14px;
  border-radius: 12px;
  border-top-left-radius: 2px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.02);
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}
.workflow-bubble-header,
.review-header,
.finding-heading { display: flex; gap: 10px; align-items: center; }
.review-header { justify-content: space-between; align-items: flex-start; }
.review-header strong { color: #0f172a; font-size: 0.82rem; }
.review-header p { margin: 2px 0 0; color: #64748b; font-size: 0.66rem; text-transform: uppercase; }
.review-count { font-size: 0.64rem; color: #475569; font-weight: 800; white-space: nowrap; }
.review-summary { margin: 0; color: #334155; font-size: 0.75rem; line-height: 1.45; }

.score-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
.score-item { border: 1px solid #e2e8f0; padding: 6px; background: #f8fafc; display: flex; justify-content: space-between; gap: 4px; font-size: 0.62rem; }
.score-item strong { color: #0f172a; }
.score-item.unavailable strong { color: #64748b; font-weight: 700; }

.findings-list { display: flex; flex-direction: column; gap: 8px; }
.finding-card { border: 1px solid #e2e8f0; padding: 9px; background: #f8fafc; }
.finding-heading strong { font-size: 0.72rem; color: #0f172a; }
.finding-card p { margin: 5px 0 0; color: #475569; font-size: 0.68rem; line-height: 1.4; }
.severity { font-size: 0.56rem; font-weight: 900; padding: 2px 4px; border: 1px solid currentColor; }
.severity-high { color: #b91c1c; }
.severity-medium { color: #a16207; }
.severity-low { color: #166534; }

.proposal-box { border: 1px solid #86efac; background: #f0fdf4; padding: 10px; display: flex; flex-direction: column; gap: 7px; }
.proposal-box > strong { font-size: 0.74rem; color: #166534; }
.proposal-box p { margin: 0; color: #166534; font-size: 0.68rem; line-height: 1.4; }
.human-gate-copy { opacity: 0.8; }
.proposal-invalid { border: 1px solid #fecaca; background: #fef2f2; padding: 9px; color: #991b1b; font-size: 0.68rem; }
.proposal-invalid p { margin: 4px 0 0; }

.success-icon { font-size: 1.25rem; }
.workflow-bubble-header strong { font-size: 0.775rem; color: #166534; display: block; }
.workflow-name-preview { font-size: 0.675rem; color: #15803d; margin: 2px 0 0 0; font-weight: 600; }
.workflow-validation { font-size: 0.65rem; color: #166534; margin: 3px 0 0 0; font-weight: 700; }
.sequence-chips { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
.chip { font-size: 0.675rem; font-weight: 600; background: #f0fdf4; border: 1px solid #dcfce7; padding: 3px 6px; border-radius: 5px; color: #166534; display: inline-flex; align-items: center; gap: 4px; }
.arrow { color: #86efac; font-weight: 800; }
.btn-load { padding: 8px; font-size: 0.725rem; font-weight: 700; background: #16a34a; box-shadow: 0 2px 6px rgba(22, 163, 74, 0.2); }
.btn-load:hover { background: #15803d; }

.chat-input-bar {
  padding: 14px 16px;
  background: #ffffff;
  border-top: 1px solid var(--border-color);
  display: flex;
  gap: 10px;
  align-items: flex-end;
}
.chat-input { flex: 1; resize: none; font-size: 0.775rem; padding: 8px 12px; line-height: 1.4; border-radius: 8px; border-color: var(--border-color); }
.chat-input:focus { border-color: #2563eb; }
.btn-send { min-width: 58px; height: 40px; border-radius: 8px; padding: 0 8px; display: flex; align-items: center; justify-content: center; font-size: 0.72rem; font-weight: 800; }

.typing-indicator { display: flex; align-items: center; gap: 4px; padding: 2px 0; }
.typing-indicator span { width: 6px; height: 6px; background: #64748b; border-radius: 50%; display: inline-block; animation: bounce 1.4s infinite ease-in-out both; }
.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }
@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}
</style>
