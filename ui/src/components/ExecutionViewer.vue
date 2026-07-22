<script setup>
import { ref, onMounted } from 'vue';
import { useExecutionStore } from '@/stores/executionStore';
import { useWorkflowStore } from '@/stores/workflowStore';

const executionStore = useExecutionStore();
const workflowStore = useWorkflowStore();
const selectedExec = ref(null);

onMounted(() => {
  if (workflowStore.currentWorkflow) {
    executionStore.fetchExecutionHistory(workflowStore.currentWorkflow.id);
  }
});

function parseLogs(logsJsonStr) {
  try {
    return JSON.parse(logsJsonStr);
  } catch {
    return [];
  }
}

function formatDate(dateStr) {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleString();
}
</script>

<template>
  <div class="execution-viewer">
    <div class="sidebar glass-panel">
      <div class="sidebar-header">
        <h4>📜 Execution History</h4>
        <button class="btn btn-secondary btn-sm" @click="executionStore.fetchExecutionHistory(workflowStore.currentWorkflow.id)">
          🔄 Refresh
        </button>
      </div>

      <div class="history-list">
        <div
          v-for="exec in executionStore.executionLogs"
          :key="exec.id"
          class="history-item"
          :class="{ active: selectedExec?.id === exec.id }"
          @click="selectedExec = exec"
        >
          <div class="item-status" :class="`status-${exec.status.toLowerCase()}`"></div>
          <div class="item-info">
            <div class="item-header">
              <span class="status-text">{{ exec.status }}</span>
              <span class="duration">{{ exec.duration_ms }}ms</span>
            </div>
            <span class="time">{{ formatDate(exec.started_at) }}</span>
          </div>
        </div>

        <div v-if="executionStore.executionLogs.length === 0" class="empty-state">
          No executions recorded yet. Click "Run Workflow" to execute!
        </div>
      </div>
    </div>

    <!-- Execution Timeline & JSON Log Inspector -->
    <div class="detail-panel glass-panel" v-if="selectedExec">
      <div class="detail-header">
        <div>
          <h3>Execution ID: #{{ selectedExec.id }}</h3>
          <span class="started-at">Started at: {{ formatDate(selectedExec.started_at) }}</span>
        </div>
        <span class="badge-status" :class="`status-${selectedExec.status.toLowerCase()}`">
          {{ selectedExec.status }} ({{ selectedExec.duration_ms }}ms)
        </span>
      </div>

      <div class="timeline">
        <h4>Execution Steps Timeline</h4>
        <div
          v-for="step in parseLogs(selectedExec.logs_json)"
          :key="step.node_id"
          class="timeline-item"
          :class="`status-${step.status.toLowerCase()}`"
        >
          <div class="step-header">
            <span class="node-id">Node: {{ step.node_id }}</span>
            <span class="step-duration">{{ step.duration_ms }}ms</span>
          </div>

          <div class="step-body" v-if="step.output">
            <details>
              <summary>Output Payload (JSON)</summary>
              <pre class="json-code">{{ JSON.stringify(step.output, null, 2) }}</pre>
            </details>
          </div>

          <div class="step-error" v-if="step.error">
            <span>❌ Error: {{ step.error }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="detail-panel glass-panel empty-detail" v-else>
      <p>👈 Select an execution item on the left to inspect node outputs and timeline.</p>
    </div>
  </div>
</template>

<style scoped>
.execution-viewer {
  display: flex;
  width: 100vw;
  height: calc(100vh - 60px);
  gap: 1px;
}

.sidebar {
  width: 320px;
  height: 100%;
  border-radius: 0;
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-color);
}

.history-list {
  padding: 12px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.history-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.history-item:hover {
  border-color: rgba(255, 255, 255, 0.2);
}

.history-item.active {
  border-color: var(--accent-cyan);
  background: rgba(6, 182, 212, 0.1);
}

.item-status {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.status-success { background: var(--accent-green); }
.status-failed { background: var(--accent-red); }
.status-running { background: var(--accent-amber); }

.item-info {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.item-header {
  display: flex;
  justify-content: space-between;
  font-size: 0.85rem;
  font-weight: 600;
}

.duration {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--accent-cyan);
}

.time {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.detail-panel {
  flex: 1;
  height: 100%;
  border-radius: 0;
  padding: 24px;
  overflow-y: auto;
}

.empty-detail {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-color);
}

.badge-status {
  padding: 6px 14px;
  border-radius: 20px;
  font-weight: 700;
  font-size: 0.85rem;
}
.badge-status.status-success {
  background: rgba(16, 185, 129, 0.2);
  color: var(--accent-green);
  border: 1px solid var(--accent-green);
}
.badge-status.status-failed {
  background: rgba(239, 68, 68, 0.2);
  color: var(--accent-red);
  border: 1px solid var(--accent-red);
}

.timeline {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.timeline-item {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 14px;
}

.timeline-item.status-success {
  border-left: 4px solid var(--accent-green);
}

.timeline-item.status-failed {
  border-left: 4px solid var(--accent-red);
}

.step-header {
  display: flex;
  justify-content: space-between;
  font-weight: 600;
  font-size: 0.875rem;
  margin-bottom: 8px;
}

.json-code {
  background: var(--bg-primary);
  padding: 10px;
  border-radius: 6px;
  font-family: var(--font-mono);
  font-size: 0.775rem;
  color: var(--accent-cyan);
  overflow-x: auto;
  margin-top: 8px;
}

.step-error {
  color: var(--accent-red);
  font-size: 0.825rem;
  font-weight: 500;
  margin-top: 6px;
}
</style>
