<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';

const props = defineProps({
  visible: Boolean,
  currentNodes: Array,
  currentEdges: Array,
});

const emit = defineEmits(['close', 'loadWorkflow']);
const workflowStore = useWorkflowStore();

const selectedCredentialId = ref('');
const promptText = ref('');
const loading = ref(false);
const messagesListRef = ref(null);

const messages = ref([
  {
    id: 'welcome',
    sender: 'ai',
    type: 'text',
    text: '👋 Hello! I am Goflow AI Assistant. Tell me what workflow you want to create, or ask me to modify/explain the current canvas workflow!'
  }
]);

function isAICredential(cred) {
  const type = String(cred.type || '').toLowerCase();
  return type === 'openai' || type === 'deepseek' || type === 'api_key';
}

const aiCredentials = computed(() => {
  return workflowStore.credentials.filter(isAICredential);
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
    if (newVal.length > 0 && !selectedCredentialId.value) {
      selectedCredentialId.value = newVal[0].id;
    }
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
  return { nodes: serialNodes, edges: serialEdges };
}

async function handleSend() {
  const text = promptText.value.trim();
  if (!text || loading.value || !selectedCredentialId.value) return;

  // Add User message
  messages.value.push({
    id: `msg_user_${Date.now()}`,
    sender: 'user',
    type: 'text',
    text: text
  });
  promptText.value = '';
  loading.value = true;
  await scrollToBottom();

  // Map conversation history to LLM format
  const apiMessages = messages.value.map((m) => {
    let content = m.text || '';
    if (m.type === 'workflow' && m.workflow) {
      content = `${m.text || ''}\n\n[Sơ đồ thiết kế hiện tại]:\n${JSON.stringify(m.workflow)}`;
    }
    return {
      role: m.sender === 'user' ? 'user' : 'assistant',
      content: content
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
        text: response.text
      });
    } else if (response.type === 'workflow') {
      messages.value.push({
        id: `msg_ai_${Date.now()}`,
        sender: 'ai',
        type: 'workflow',
        workflow: response.workflow,
        validated: response.validated === true
      });
    }
  } catch (err) {
    messages.value.push({
      id: `msg_ai_err_${Date.now()}`,
      sender: 'ai',
      type: 'text',
      text: `❌ **Failed to communicate with AI**: ${err.message}`
    });
  } finally {
    loading.value = false;
    await scrollToBottom();
  }
}

function handleLoad(workflow) {
  if (workflow) {
    emit('loadWorkflow', workflow);
  }
}

async function scrollToBottom() {
  await nextTick();
  if (messagesListRef.value) {
    messagesListRef.value.scrollTop = messagesListRef.value.scrollHeight;
  }
}

function renderMarkdown(text) {
  if (!text) return '';
  // Simple markdown renderer for chat
  let html = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  html = html.replace(/\n/g, '<br>');
  // Bold: **text**
  html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
  // Italic: *text* or _text_
  html = html.replace(/\*(.*?)\*/g, '<em>$1</em>');
  // Bullet lists
  html = html.replace(/<br>\s*[-*]\s+/g, '<br>• ');
  if (html.startsWith('- ') || html.startsWith('* ')) {
    html = '• ' + html.substring(2);
  }
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
            <h3>AI Assistant</h3>
            <p class="subtitle">Understand and modify the workflow</p>
          </div>
        </div>
        <button class="btn-close" @click="emit('close')">✕</button>
      </div>

      <div class="chat-container">
        <!-- AI API Key configuration bar -->
        <div class="key-selector-bar">
          <label>Select AI Key:</label>
          <select v-model="selectedCredentialId" class="form-select select-sm" :disabled="loading">
            <option value="">-- Choose Credential --</option>
            <option v-for="cred in aiCredentials" :key="cred.id" :value="cred.id">
              {{ cred.name }} ({{ cred.type }})
            </option>
          </select>
          <div v-if="aiCredentials.length === 0" class="no-keys-error">
            ⚠️ Add an OpenAI/DeepSeek key in Credentials Manager first.
          </div>
        </div>

        <!-- Chat Helper Tip -->
        <div class="chat-helper-tip">
          💡 <strong>Mẹo:</strong> Nếu AI Assistant thiết kế sơ đồ, hãy nhớ bấm nút <strong>📥 Load Onto Canvas (Áp dụng)</strong> trong khung chat để nạp sơ đồ mới lên Canvas.
        </div>

        <!-- Messages list -->
        <div class="messages-list" ref="messagesListRef">
          <div 
            v-for="msg in messages" 
            :key="msg.id" 
            class="message-bubble-wrapper" 
            :class="msg.sender"
          >
            <div class="avatar">{{ msg.sender === 'user' ? '👤' : '🤖' }}</div>
            <div class="message-content">
              <!-- Text bubble -->
              <div 
                v-if="msg.type === 'text'" 
                class="text-bubble" 
                v-html="renderMarkdown(msg.text)"
              ></div>

              <!-- Workflow schema bubble -->
              <div v-else-if="msg.type === 'workflow'" style="display: flex; flex-direction: column; gap: 8px; width: 100%;">
                <!-- Conversational text part -->
                <div 
                  v-if="msg.text" 
                  class="text-bubble" 
                  style="border-top-left-radius: 2px; background: #ffffff; color: #1e293b; border: 1px solid var(--border-color); margin-bottom: 4px;"
                  v-html="renderMarkdown(msg.text)"
                ></div>

                <!-- Workflow card part -->
                <div class="workflow-bubble">
                  <div class="workflow-bubble-header">
                    <span class="success-icon">✨</span>
                    <div>
                      <strong>Sơ đồ workflow mới/sửa đổi</strong>
                      <p class="workflow-name-preview">Name: {{ msg.workflow.name || 'Unnamed Flow' }}</p>
                      <p v-if="msg.validated" class="workflow-validation">Validated by Goflow</p>
                    </div>
                  </div>
                  
                  <div class="pipeline-flow-preview">
                    <div class="sequence-chips">
                      <span 
                        v-for="(node, index) in msg.workflow.nodes" 
                        :key="node.id"
                        class="chip"
                      >
                        {{ node.name || node.type }}
                        <span v-if="index < msg.workflow.nodes.length - 1" class="arrow">→</span>
                      </span>
                    </div>
                  </div>

                  <button class="btn btn-success btn-sm btn-load" @click="handleLoad(msg.workflow)">
                    📥 Load Onto Canvas (Áp dụng)
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Loading bubble -->
          <div v-if="loading" class="message-bubble-wrapper ai loading">
            <div class="avatar">🤖</div>
            <div class="message-content">
              <div class="text-bubble loading-bubble">
                <div class="typing-indicator">
                  <span></span>
                  <span></span>
                  <span></span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Chat Input Bar -->
        <div class="chat-input-bar">
          <textarea
            v-model="promptText"
            class="form-textarea chat-input"
            placeholder="Ask AI to design, edit or explain workflow..."
            rows="2"
            @keydown.enter.prevent="handleSend"
            :disabled="loading || !selectedCredentialId"
          ></textarea>
          <button 
            class="btn btn-primary btn-send" 
            @click="handleSend"
            :disabled="loading || !selectedCredentialId || !promptText.trim()"
          >
            ✈️
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
  width: 450px;
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

.header-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-title .icon {
  font-size: 2rem;
}

.header-title h3 {
  font-size: 1.05rem;
  font-weight: 800;
  color: #0f172a;
  margin: 0;
}

.header-title .subtitle {
  font-size: 0.725rem;
  color: #64748b;
  margin: 2px 0 0 0;
}

.btn-close {
  background: transparent;
  border: none;
  font-size: 1.1rem;
  color: #64748b;
  cursor: pointer;
  padding: 4px;
}

.btn-close:hover {
  color: #0f172a;
}

/* Chat container and layout */
.chat-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
  background: #f1f5f9;
}

.chat-helper-tip {
  padding: 8px 16px;
  background: #f8fafc;
  border-bottom: 1px solid var(--border-color);
  font-size: 0.7rem;
  color: #475569;
  line-height: 1.4;
}

.key-selector-bar {
  padding: 10px 16px;
  background: #ffffff;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.key-selector-bar label {
  font-size: 0.725rem;
  font-weight: 700;
  color: #475569;
}

.select-sm {
  flex: 1;
  min-width: 150px;
  padding: 4px 8px;
  font-size: 0.75rem;
}

.no-keys-error {
  width: 100%;
  font-size: 0.675rem;
  color: #ef4444;
  font-weight: 600;
  margin-top: 2px;
}

/* Messages List */
.messages-list {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.message-bubble-wrapper {
  display: flex;
  gap: 10px;
  max-width: 85%;
}

.message-bubble-wrapper.user {
  align-self: flex-end;
  flex-direction: row-reverse;
}

.message-bubble-wrapper.ai {
  align-self: flex-start;
}

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

.message-content {
  flex: 1;
}

.text-bubble {
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 0.775rem;
  line-height: 1.45;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.user .text-bubble {
  background: #2563eb;
  color: #ffffff;
  border-top-right-radius: 2px;
}

.ai .text-bubble {
  background: #ffffff;
  color: #1e293b;
  border-top-left-radius: 2px;
  border: 1px solid var(--border-color);
}

/* Workflow Preview bubble */
.workflow-bubble {
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

.workflow-bubble-header {
  display: flex;
  gap: 10px;
  align-items: center;
}

.success-icon {
  font-size: 1.25rem;
}

.workflow-bubble-header strong {
  font-size: 0.775rem;
  color: #166534;
  display: block;
}

.workflow-name-preview {
  font-size: 0.675rem;
  color: #15803d;
  margin: 2px 0 0 0;
  font-weight: 600;
}

.workflow-validation {
  font-size: 0.65rem;
  color: #166534;
  margin: 3px 0 0 0;
  font-weight: 700;
}

.sequence-chips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.chip {
  font-size: 0.675rem;
  font-weight: 600;
  background: #f0fdf4;
  border: 1px solid #dcfce7;
  padding: 3px 6px;
  border-radius: 5px;
  color: #166534;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.arrow {
  color: #86efac;
  font-weight: 800;
}

.btn-load {
  padding: 8px;
  font-size: 0.725rem;
  font-weight: 700;
  background: #16a34a;
  box-shadow: 0 2px 6px rgba(22, 163, 74, 0.2);
}

.btn-load:hover {
  background: #15803d;
}

/* Chat Input Bar */
.chat-input-bar {
  padding: 14px 16px;
  background: #ffffff;
  border-top: 1px solid var(--border-color);
  display: flex;
  gap: 10px;
  align-items: flex-end;
}

.chat-input {
  flex: 1;
  resize: none;
  font-size: 0.775rem;
  padding: 8px 12px;
  line-height: 1.4;
  border-radius: 8px;
  border-color: var(--border-color);
}

.chat-input:focus {
  border-color: #2563eb;
}

.btn-send {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
}

/* Typing Indicator animation */
.typing-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 0;
}

.typing-indicator span {
  width: 6px;
  height: 6px;
  background: #64748b;
  border-radius: 50%;
  display: inline-block;
  animation: bounce 1.4s infinite ease-in-out both;
}

.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}
</style>
