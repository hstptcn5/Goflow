<script setup>
import { ref } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { api } from '@/services/api';
import { getNavIconSVG } from './NodeIcons';

const props = defineProps({
  activeTab: String,
});
const emit = defineEmits(['update:activeTab', 'openWorkflows', 'openCredentials', 'saveWorkflow', 'exportWorkflow']);

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const triggering = ref(false);

async function handleRunWorkflow() {
  if (!workflowStore.currentWorkflow) return;
  triggering.value = true;
  executionStore.resetNodeStatuses();

  try {
    const exec = await api.triggerWorkflow(workflowStore.currentWorkflow.id, {}, false);
    console.log('Execution finished:', exec);
    await executionStore.fetchExecutionHistory(workflowStore.currentWorkflow.id);
  } catch (err) {
    alert('Execution failed: ' + err.message);
  } finally {
    triggering.value = false;
  }
}
</script>

<template>
  <header class="navbar glass-panel">
    <div class="brand">
      <div class="logo-icon">⚡</div>
      <div class="brand-text">
        <span class="title">Goflow</span>
        <span class="tag">MVP</span>
      </div>
    </div>

    <div class="wf-title-section" v-if="workflowStore.currentWorkflow">
      <span class="wf-name">{{ workflowStore.currentWorkflow.name }}</span>
      <label class="switch">
        <input
          type="checkbox"
          :checked="workflowStore.currentWorkflow.is_active"
          @change="workflowStore.toggleActive(workflowStore.currentWorkflow.id, $event.target.checked)"
        />
        <span class="slider"></span>
      </label>
      <span class="status-label" :class="{ active: workflowStore.currentWorkflow.is_active }">
        {{ workflowStore.currentWorkflow.is_active ? 'Active' : 'Inactive' }}
      </span>
    </div>

    <div class="nav-controls">
      <button class="btn btn-secondary" @click="emit('openWorkflows')" style="display: inline-flex; align-items: center; gap: 6px;">
        <span v-html="getNavIconSVG('workflows')" style="display: flex;"></span> Workflows
      </button>
      <button class="btn btn-secondary" @click="emit('openCredentials')" style="display: inline-flex; align-items: center; gap: 6px;">
        <span v-html="getNavIconSVG('credentials')" style="display: flex;"></span> Credentials
      </button>

      <div class="divider"></div>

      <div class="tab-group" v-if="workflowStore.currentWorkflow">
        <button
          class="btn-tab"
          :class="{ active: activeTab === 'editor' }"
          @click="emit('update:activeTab', 'editor')"
          style="display: inline-flex; align-items: center; gap: 6px;"
        >
          <span v-html="getNavIconSVG('canvas')" style="display: flex;"></span> Canvas
        </button>
        <button
          class="btn-tab"
          :class="{ active: activeTab === 'executions' }"
          @click="emit('update:activeTab', 'executions')"
          style="display: inline-flex; align-items: center; gap: 6px;"
        >
          <span v-html="getNavIconSVG('history')" style="display: flex;"></span> Executions History
        </button>
      </div>

      <button
        v-if="workflowStore.currentWorkflow"
        class="btn btn-primary"
        :disabled="triggering"
        @click="handleRunWorkflow"
        style="display: inline-flex; align-items: center; gap: 6px;"
      >
        <template v-if="triggering">
          <span v-html="getNavIconSVG('loading')" style="display: flex;"></span> Executing...
        </template>
        <template v-else>
          <span v-html="getNavIconSVG('play')" style="display: flex;"></span> Run Workflow
        </template>
      </button>

      <button
        v-if="workflowStore.currentWorkflow"
        class="btn btn-success"
        @click="emit('saveWorkflow')"
        style="display: inline-flex; align-items: center; gap: 6px;"
      >
        <span v-html="getNavIconSVG('save')" style="display: flex;"></span> Save
      </button>

      <button
        v-if="workflowStore.currentWorkflow"
        class="btn btn-secondary"
        @click="emit('exportWorkflow')"
        style="margin-left: 8px; display: inline-flex; align-items: center; gap: 6px;"
      >
        <span v-html="getNavIconSVG('export')" style="display: flex;"></span> Export
      </button>
    </div>
  </header>
</template>

<style scoped>
.navbar {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  z-index: 100;
  border-radius: 0;
  background: #ffffff;
  border-bottom: 1px solid var(--border-color);
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  font-size: 1.5rem;
}

.brand-text {
  display: flex;
  align-items: center;
  gap: 6px;
}

.title {
  font-size: 1.25rem;
  font-weight: 800;
  color: #0f172a;
}

.tag {
  font-size: 0.65rem;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
  background: #dbeafe;
  color: #2563eb;
  border: 1px solid #bfdbfe;
}

.wf-title-section {
  display: flex;
  align-items: center;
  gap: 12px;
}

.wf-name {
  font-weight: 700;
  font-size: 1rem;
  color: #0f172a;
}

.status-label {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.status-label.active {
  color: var(--accent-green);
  font-weight: 700;
}

.nav-controls {
  display: flex;
  align-items: center;
  gap: 10px;
}

.divider {
  width: 1px;
  height: 24px;
  background: var(--border-color);
}

.tab-group {
  display: flex;
  background: #f1f5f9;
  padding: 3px;
  border-radius: 8px;
  border: 1px solid var(--border-color);
}

.btn-tab {
  background: transparent;
  border: none;
  color: #475569;
  padding: 6px 12px;
  font-size: 0.8rem;
  font-weight: 600;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-tab.active {
  background: #ffffff;
  color: #2563eb;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

/* Custom Switch Toggle */
.switch {
  position: relative;
  display: inline-block;
  width: 36px;
  height: 20px;
}
.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}
.slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background-color: #cbd5e1;
  transition: 0.3s;
  border-radius: 20px;
}
.slider:before {
  position: absolute;
  content: "";
  height: 14px; width: 14px;
  left: 3px; bottom: 3px;
  background-color: white;
  transition: 0.3s;
  border-radius: 50%;
}
input:checked + .slider {
  background-color: var(--accent-green);
}
input:checked + .slider:before {
  transform: translateX(16px);
}
</style>
