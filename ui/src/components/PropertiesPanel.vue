<script setup>
import { computed, ref, watch } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { nodeHelpMap } from './NodeHelpData';
import { api } from '@/services/api';

const props = defineProps({
  selectedNode: Object,
});

const emit = defineEmits(['updateNodeParams', 'deleteNode', 'close']);

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

const showHelp = ref(false);
const activeSidebarTab = ref('config');
const aiHelperPrompt = ref('');
const aiHelperLoading = ref(false);
const aiHelperError = ref(null);

function isAICredential(cred) {
  const type = String(cred.type || '').toLowerCase();
  return type === 'openai' || type === 'deepseek' || type === 'api_key';
}

const aiCredentialId = computed(() => {
  const cred = workflowStore.credentials.find(isAICredential);
  return cred ? cred.id : null;
});

const nodeDef = computed(() => {
  if (!props.selectedNode) return null;
  return workflowStore.nodeDefinitions.find((d) => d.type === props.selectedNode.type);
});

const helpData = computed(() => {
  if (!props.selectedNode) return null;
  return nodeHelpMap[props.selectedNode.type] || null;
});

const nodeExecutionResult = computed(() => {
  if (!props.selectedNode) return null;

  const realtimeEvent = executionStore.nodeEvents[props.selectedNode.id];
  if (realtimeEvent) return realtimeEvent;

  const latestExec = executionStore.executionLogs[0];
  if (!latestExec) return null;

  try {
    const logs = JSON.parse(latestExec.logs_json || '[]');
    return logs.find((log) => log.node_id === props.selectedNode.id) || null;
  } catch {
    return null;
  }
});

const selectedNodeStatus = computed(() => {
  if (!props.selectedNode) return null;
  return executionStore.nodeStatuses[props.selectedNode.id] || nodeExecutionResult.value?.status || null;
});

const selectedNodeError = computed(() => nodeExecutionResult.value?.error || null);

watch(
  () => props.selectedNode?.id,
  () => {
    const status = props.selectedNode ? executionStore.nodeStatuses[props.selectedNode.id] : null;
    activeSidebarTab.value = status === 'FAILED' ? 'output' : 'config';
    showHelp.value = false;
    aiHelperError.value = null;
  }
);

watch(
  () => workflowStore.currentWorkflow?.id,
  (newId) => {
    if (newId) executionStore.fetchExecutionHistory(newId);
  },
  { immediate: true }
);

function handleParamChange(paramName, value) {
  if (!props.selectedNode) return;
  const updatedParams = { ...props.selectedNode.params, [paramName]: value };
  emit('updateNodeParams', props.selectedNode.id, updatedParams);
}

function handleNameChange(newName) {
  if (!props.selectedNode) return;
  emit('updateNodeParams', props.selectedNode.id, props.selectedNode.params, newName);
}

function handleDeleteNode() {
  if (props.selectedNode) emit('deleteNode', props.selectedNode.id);
}

async function runAIHelper() {
  if (!props.selectedNode || !aiHelperPrompt.value.trim() || aiHelperLoading.value) return;

  if (!aiCredentialId.value) {
    aiHelperError.value = 'No AI API key found. Add one in Credentials first.';
    return;
  }

  aiHelperLoading.value = true;
  aiHelperError.value = null;

  try {
    const updatedParams = await api.configureNodeParams(
      props.selectedNode.type,
      aiHelperPrompt.value,
      props.selectedNode.params || {},
      aiCredentialId.value
    );
    emit('updateNodeParams', props.selectedNode.id, updatedParams);
    aiHelperPrompt.value = '';
  } catch (err) {
    aiHelperError.value = `Failed: ${err.message}`;
  } finally {
    aiHelperLoading.value = false;
  }
}
</script>

<template>
  <aside v-if="selectedNode" class="properties-panel glass-panel">
    <div class="panel-header">
      <div class="header-left">
        <span class="node-type-badge">{{ selectedNode.type }}</span>
        <span class="node-id">#{{ selectedNode.id.substring(0, 8) }}</span>
      </div>
      <button class="btn-close" @click="emit('close')">x</button>
    </div>

    <div class="panel-tabs">
      <button
        class="tab-btn"
        :class="{ active: activeSidebarTab === 'config' }"
        @click="activeSidebarTab = 'config'"
      >
        Config
      </button>
      <button
        class="tab-btn"
        :class="{ active: activeSidebarTab === 'output' }"
        @click="activeSidebarTab = 'output'"
      >
        Live Output
      </button>
    </div>

    <div class="panel-body">
      <div v-if="selectedNodeStatus === 'FAILED'" class="node-error-summary">
        <div class="node-error-title">Node failed</div>
        <pre class="node-error-message">{{ selectedNodeError || 'No error details were reported for this node.' }}</pre>
      </div>

      <div v-if="activeSidebarTab === 'config'">
        <div class="form-group">
          <label>Node Name</label>
          <input
            type="text"
            :value="selectedNode.name || nodeDef?.name"
            class="form-input"
            @input="handleNameChange($event.target.value)"
          />
        </div>

        <div class="divider"></div>
        <h4 class="section-title">Node Configuration</h4>

        <div v-if="nodeDef?.params?.length">
          <div v-for="param in nodeDef.params" :key="param.name" class="form-group">
            <label>{{ param.label }} <span v-if="param.required" class="req">*</span></label>
            <span v-if="param.description" class="param-desc">{{ param.description }}</span>

            <input
              v-if="param.type === 'text'"
              type="text"
              :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
              class="form-input"
              @input="handleParamChange(param.name, $event.target.value)"
            />

            <input
              v-else-if="param.type === 'password'"
              type="password"
              :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
              class="form-input"
              @input="handleParamChange(param.name, $event.target.value)"
            />

            <select
              v-else-if="param.type === 'select'"
              :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
              class="form-select"
              @change="handleParamChange(param.name, $event.target.value)"
            >
              <option v-for="opt in param.options" :key="opt" :value="opt">{{ opt }}</option>
            </select>

            <textarea
              v-else-if="param.type === 'textarea' || param.type === 'json'"
              :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
              class="form-textarea"
              rows="4"
              @input="handleParamChange(param.name, $event.target.value)"
            ></textarea>

            <select
              v-else-if="param.type === 'credential'"
              :value="selectedNode.params?.[param.name] ?? ''"
              class="form-select"
              @change="handleParamChange(param.name, $event.target.value)"
            >
              <option value="">-- Select Credential Secret --</option>
              <option v-for="cred in workflowStore.credentials" :key="cred.id" :value="cred.id">
                {{ cred.name }} ({{ cred.type }})
              </option>
            </select>

            <input
              v-else
              type="text"
              :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
              class="form-input"
              @input="handleParamChange(param.name, $event.target.value)"
            />
          </div>
        </div>

        <div v-else class="empty-params">No configurable parameters for this node.</div>

        <div class="divider"></div>

        <div class="ai-node-configurer">
          <label class="ai-configurer-title">AI Quick Config</label>
          <p class="ai-configurer-desc">
            Ask AI to fill this node's parameters, for example: set the URL to fetch London weather.
          </p>
          <div class="ai-configurer-input-row">
            <input
              v-model="aiHelperPrompt"
              type="text"
              placeholder="Describe how to configure this node..."
              class="form-input ai-configurer-input"
              :disabled="aiHelperLoading"
              @keyup.enter="runAIHelper"
            />
            <button
              class="btn btn-primary ai-configurer-btn"
              :disabled="aiHelperLoading || !aiHelperPrompt.trim()"
              @click="runAIHelper"
            >
              {{ aiHelperLoading ? '...' : 'AI' }}
            </button>
          </div>
          <p v-if="aiHelperError" class="ai-configurer-error">{{ aiHelperError }}</p>
        </div>

        <div v-if="helpData" class="help-section">
          <button class="btn btn-secondary btn-full btn-help-toggle" @click="showHelp = !showHelp">
            {{ showHelp ? 'Hide Node Guide' : 'Show Node Guide' }}
          </button>
          <div v-if="showHelp" class="help-content-box">
            <h5 class="help-node-title">{{ helpData.title }}</h5>
            <p class="help-desc">{{ helpData.desc }}</p>
            <div class="help-sub-sec">
              <span class="help-sub-title">Inputs:</span>
              <pre class="help-pre-text">{{ helpData.inputs }}</pre>
            </div>
            <div class="help-sub-sec">
              <span class="help-sub-title">Output Reference:</span>
              <pre class="help-code-block"><code>{{ helpData.output }}</code></pre>
            </div>
          </div>
        </div>

        <div class="divider"></div>
        <button class="btn btn-danger btn-full" @click="handleDeleteNode">Delete Node</button>
      </div>

      <div v-if="activeSidebarTab === 'output'">
        <div v-if="nodeExecutionResult">
          <h4 class="section-title">Latest Run Result</h4>
          <div class="exec-status-badge" :class="String(nodeExecutionResult.status || '').toLowerCase()">
            {{ nodeExecutionResult.status }} ({{ nodeExecutionResult.duration_ms || 0 }}ms)
          </div>

          <div v-if="nodeExecutionResult.output" class="exec-output-box">
            <label>Output Payload:</label>
            <pre class="json-code"><code>{{ JSON.stringify(nodeExecutionResult.output, null, 2) }}</code></pre>
          </div>

          <div v-if="nodeExecutionResult.error" class="exec-error-box">
            <label>Error Details:</label>
            <pre class="error-code"><code>{{ nodeExecutionResult.error }}</code></pre>
          </div>
        </div>

        <div v-else class="empty-output">
          <span class="empty-icon">Output</span>
          <p>No execution data available.</p>
          <p class="empty-sub">Run the workflow or trigger nodes to inspect live outputs.</p>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.properties-panel {
  width: 320px;
  height: calc(100vh - 60px);
  position: absolute;
  right: 0;
  top: 0;
  border-radius: 0;
  border-left: 1px solid var(--border-color);
  background: var(--bg-secondary);
  z-index: 100;
  display: flex;
  flex-direction: column;
  box-shadow: -10px 0 30px rgba(15, 23, 42, 0.16);
}

.panel-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-tertiary);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-type-badge {
  font-size: 0.75rem;
  font-weight: 700;
  padding: 3px 8px;
  border-radius: 4px;
  background: #ede9fe;
  color: #6d28d9;
  border: 1px solid #ddd6fe;
  font-family: var(--font-mono);
}

.node-id {
  font-size: 0.75rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.btn-close {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 1.1rem;
}

.btn-close:hover {
  color: #0f172a;
}

.panel-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-color);
  background: #f8fafc;
}

.tab-btn {
  flex: 1;
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  padding: 10px 8px;
  color: #64748b;
  cursor: pointer;
  font-weight: 700;
}

.tab-btn.active {
  color: #2563eb;
  border-bottom-color: #2563eb;
  background: #ffffff;
}

.panel-body {
  padding: 16px;
  overflow-y: auto;
  flex: 1;
}

.node-error-summary,
.exec-error-box {
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-left: 4px solid #dc2626;
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 14px;
}

.node-error-title {
  color: #991b1b;
  font-size: 0.75rem;
  font-weight: 800;
  text-transform: uppercase;
  margin-bottom: 6px;
}

.node-error-message,
.error-code {
  color: #b91c1c;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  line-height: 1.35;
  white-space: pre-wrap;
  margin: 0;
}

.divider {
  height: 1px;
  background: var(--border-color);
  margin: 16px 0;
}

.section-title {
  font-size: 0.8rem;
  color: #64748b;
  text-transform: uppercase;
  margin-bottom: 12px;
  letter-spacing: 0.05em;
}

.req {
  color: #dc2626;
}

.param-desc {
  font-size: 0.725rem;
  color: #64748b;
  margin-bottom: 4px;
}

.empty-params,
.empty-output {
  font-size: 0.82rem;
  color: var(--text-secondary);
  margin: 10px 0;
}

.btn-full {
  width: 100%;
  justify-content: center;
}

.ai-node-configurer {
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.ai-configurer-title {
  display: block;
  color: #0f172a;
  font-weight: 800;
  font-size: 0.82rem;
  margin-bottom: 4px;
}

.ai-configurer-desc {
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.4;
  margin-bottom: 10px;
}

.ai-configurer-input-row {
  display: flex;
  gap: 8px;
}

.ai-configurer-input {
  min-width: 0;
}

.ai-configurer-btn {
  padding-left: 12px;
  padding-right: 12px;
}

.ai-configurer-error {
  color: #b91c1c;
  font-size: 0.75rem;
  margin-top: 8px;
}

.help-section {
  margin-top: 16px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-primary);
}

.btn-help-toggle {
  border-radius: 0;
  border: none;
  font-size: 0.8rem;
  padding: 8px 12px;
  background: #f1f5f9;
  color: #475569;
}

.help-content-box {
  padding: 12px;
  border-top: 1px solid var(--border-color);
  font-size: 0.75rem;
  color: var(--text-primary);
  background: #ffffff;
}

.help-node-title {
  font-size: 0.85rem;
  font-weight: 700;
  margin: 0 0 6px 0;
  color: #0f172a;
}

.help-desc {
  margin: 0 0 10px 0;
  line-height: 1.4;
  color: #475569;
}

.help-sub-sec {
  margin-top: 8px;
}

.help-sub-title {
  font-weight: 700;
  color: #334155;
  display: block;
  margin-bottom: 2px;
}

.help-pre-text {
  font-family: inherit;
  white-space: pre-wrap;
  margin: 0;
  color: #475569;
  background: #f8fafc;
  padding: 6px;
  border-radius: 4px;
  border: 1px solid #e2e8f0;
}

.help-code-block,
.json-code {
  background: #0f172a;
  color: #f8fafc;
  padding: 8px;
  border-radius: 4px;
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  margin: 6px 0 0;
  line-height: 1.3;
}

.exec-status-badge {
  font-size: 0.725rem;
  font-weight: 700;
  padding: 4px 8px;
  border-radius: 4px;
  display: inline-block;
  margin-bottom: 8px;
  text-transform: uppercase;
  background: #e2e8f0;
  color: #334155;
}

.exec-status-badge.success {
  background: #dcfce7;
  color: #166534;
}

.exec-status-badge.failed {
  background: #fee2e2;
  color: #991b1b;
}

.exec-output-box label,
.exec-error-box label {
  font-size: 0.75rem;
  color: #475569;
  font-weight: 700;
}

.empty-icon {
  display: inline-block;
  font-weight: 800;
  color: #2563eb;
  margin-bottom: 6px;
}

.empty-sub {
  color: #64748b;
  font-size: 0.75rem;
  margin-top: 4px;
}
</style>
