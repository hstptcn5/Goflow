<script setup>
import { computed, ref } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { nodeHelpMap } from './NodeHelpData';

const props = defineProps({
  selectedNode: Object,
});

const emit = defineEmits(['updateNodeParams', 'deleteNode', 'close']);
const workflowStore = useWorkflowStore();
const showHelp = ref(false);

const nodeDef = computed(() => {
  if (!props.selectedNode) return null;
  return workflowStore.nodeDefinitions.find((d) => d.type === props.selectedNode.type);
});

const helpData = computed(() => {
  if (!props.selectedNode) return null;
  return nodeHelpMap[props.selectedNode.type] || null;
});

const executionStore = useExecutionStore();

const nodeExecutionResult = computed(() => {
  if (!props.selectedNode) return null;
  // Lấy lượt chạy gần đây nhất
  const latestExec = executionStore.executionLogs[0];
  if (!latestExec) return null;

  try {
    const logs = JSON.parse(latestExec.logs_json || '[]');
    const stepLog = logs.find((log) => log.node_id === props.selectedNode.id);
    return stepLog || null;
  } catch (e) {
    return null;
  }
});

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
  if (props.selectedNode) {
    emit('deleteNode', props.selectedNode.id);
  }
}
</script>

<template>
  <aside class="properties-panel glass-panel" v-if="selectedNode">
    <div class="panel-header">
      <div class="header-left">
        <span class="node-type-badge">{{ selectedNode.type }}</span>
        <span class="node-id">#{{ selectedNode.id.substring(0, 8) }}</span>
      </div>
      <button class="btn-close" @click="emit('close')">✕</button>
    </div>

    <div class="panel-body">
      <div class="form-group">
        <label>Node Name</label>
        <input
          type="text"
          :value="selectedNode.name || nodeDef?.name"
          @input="handleNameChange($event.target.value)"
          class="form-input"
        />
      </div>

      <div class="divider"></div>
      <h4 class="section-title">Node Configuration</h4>

      <div v-if="nodeDef && nodeDef.params && nodeDef.params.length > 0">
        <div v-for="param in nodeDef.params" :key="param.name" class="form-group">
          <label>{{ param.label }} <span v-if="param.required" class="req">*</span></label>
          <span class="param-desc" v-if="param.description">{{ param.description }}</span>

          <!-- Text Input -->
          <input
            v-if="param.type === 'text'"
            type="text"
            :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
            @input="handleParamChange(param.name, $event.target.value)"
            class="form-input"
          />

          <!-- Select Input -->
          <select
            v-else-if="param.type === 'select'"
            :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
            @change="handleParamChange(param.name, $event.target.value)"
            class="form-select"
          >
            <option v-for="opt in param.options" :key="opt" :value="opt">{{ opt }}</option>
          </select>

          <!-- Textarea / JSON Input -->
          <textarea
            v-else-if="param.type === 'textarea' || param.type === 'json'"
            :value="selectedNode.params?.[param.name] ?? param.default ?? ''"
            @input="handleParamChange(param.name, $event.target.value)"
            class="form-textarea"
            rows="4"
          ></textarea>

          <!-- Credential Select -->
          <select
            v-else-if="param.type === 'credential'"
            :value="selectedNode.params?.[param.name] ?? ''"
            @change="handleParamChange(param.name, $event.target.value)"
            class="form-select"
          >
            <option value="">-- Select Credential Secret --</option>
            <option
              v-for="cred in workflowStore.credentials"
              :key="cred.id"
              :value="cred.id"
            >
              {{ cred.name }} ({{ cred.type }})
            </option>
          </select>
        </div>
      </div>
      <div v-else class="empty-params">
        No configurable parameters for this node.
      </div>

      <!-- Help Documentation Box -->
      <div v-if="helpData" class="help-section">
        <button class="btn btn-secondary btn-full btn-help-toggle" @click="showHelp = !showHelp">
          📖 {{ showHelp ? 'Hide Node Guide' : 'Show Node Guide' }}
        </button>
        <div class="help-content-box" v-if="showHelp">
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

      <!-- Live Execution Result Output -->
      <div v-if="nodeExecutionResult" class="execution-result-section">
        <div class="divider"></div>
        <h4 class="section-title">Latest Run Result</h4>
        <div class="exec-status-badge" :class="nodeExecutionResult.status.toLowerCase()">
          {{ nodeExecutionResult.status }} ({{ nodeExecutionResult.duration_ms }}ms)
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

      <div class="divider"></div>

      <button class="btn btn-danger btn-full" @click="handleDeleteNode">
        🗑️ Delete Node
      </button>
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
  box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
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
  background: rgba(139, 92, 246, 0.25);
  color: #a78bfa;
  border: 1px solid rgba(139, 92, 246, 0.4);
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
  color: #fff;
}

.panel-body {
  padding: 16px;
  overflow-y: auto;
  flex: 1;
}

.divider {
  height: 1px;
  background: var(--border-color);
  margin: 16px 0;
}

.section-title {
  font-size: 0.8rem;
  color: #94a3b8;
  text-transform: uppercase;
  margin-bottom: 12px;
  letter-spacing: 0.05em;
}

.req {
  color: #f87171;
}

.param-desc {
  font-size: 0.725rem;
  color: #94a3b8;
  margin-bottom: 4px;
}

.empty-params {
  font-size: 0.8rem;
  color: var(--text-secondary);
  font-style: italic;
  margin: 10px 0;
}

.btn-full {
  width: 100%;
  justify-content: center;
}

/* Help Documentation Styles */
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
  font-weight: 600;
  cursor: pointer;
  width: 100%;
  display: flex;
  align-items: center;
}
.btn-help-toggle:hover {
  background: #e2e8f0;
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

.help-code-block {
  background: #0f172a;
  color: #f8fafc;
  padding: 8px;
  border-radius: 4px;
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  margin: 0;
  line-height: 1.3;
}

/* Execution Result styles in PropertiesPanel */
.execution-result-section {
  margin-top: 16px;
}

.exec-status-badge {
  font-size: 0.725rem;
  font-weight: 700;
  padding: 4px 8px;
  border-radius: 4px;
  display: inline-block;
  margin-bottom: 8px;
  text-transform: uppercase;
}

.exec-status-badge.success {
  background: rgba(22, 163, 74, 0.15);
  color: #16a34a;
  border: 1px solid rgba(22, 163, 74, 0.3);
}

.exec-status-badge.failed {
  background: rgba(220, 38, 38, 0.15);
  color: #dc2626;
  border: 1px solid rgba(220, 38, 38, 0.3);
}

.exec-status-badge.running {
  background: rgba(217, 119, 6, 0.15);
  color: #d97706;
  border: 1px solid rgba(217, 119, 6, 0.3);
}

.exec-output-box, .exec-error-box {
  margin-top: 6px;
}

.exec-output-box label, .exec-error-box label {
  font-size: 0.75rem;
  font-weight: 700;
  color: #475569;
  display: block;
  margin-bottom: 4px;
}

.exec-error-box .error-code {
  background: #fef2f2;
  color: #dc2626;
  padding: 8px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  margin: 0;
  border: 1px solid #fecaca;
  white-space: pre-wrap;
}

.json-code {
  background: #0f172a;
  color: #f8fafc;
  padding: 8px;
  border-radius: 4px;
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  margin: 0;
  line-height: 1.3;
}
</style>
