<script setup>
import { computed } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';

const props = defineProps({
  selectedNode: Object,
});

const emit = defineEmits(['updateNodeParams', 'deleteNode', 'close']);
const workflowStore = useWorkflowStore();

const nodeDef = computed(() => {
  if (!props.selectedNode) return null;
  return workflowStore.nodeDefinitions.find((d) => d.type === props.selectedNode.type);
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
</style>
