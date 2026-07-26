<script setup>
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import StateBlock from '@/components/StateBlock.vue';

const router = useRouter();
const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const selectedWorkflowId = ref('');
const pageError = ref('');

const selectedWorkflow = computed(() => workflowStore.workflows.find((item) => item.id === selectedWorkflowId.value));

onMounted(async () => {
  if (workflowStore.workflows.length === 0) {
    await workflowStore.fetchWorkflows();
  }
  if (!selectedWorkflowId.value && workflowStore.workflows[0]) {
    selectedWorkflowId.value = workflowStore.workflows[0].id;
    await loadExecutions();
  }
});

async function loadExecutions() {
  if (!selectedWorkflowId.value) return;
  pageError.value = '';
  try {
    await executionStore.fetchExecutionHistory(selectedWorkflowId.value);
  } catch (err) {
    pageError.value = err.message;
  }
}

function formatDate(value) {
  return value ? new Date(value).toLocaleString() : '';
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Executions</h2>
        <p>Review workflow-specific execution history. A global execution endpoint is not available yet.</p>
      </div>
      <div class="toolbar-actions">
        <select v-model="selectedWorkflowId" class="form-select compact-select" aria-label="Workflow" @change="loadExecutions">
          <option disabled value="">Choose workflow</option>
          <option v-for="workflow in workflowStore.workflows" :key="workflow.id" :value="workflow.id">
            {{ workflow.name }}
          </option>
        </select>
        <button class="btn btn-secondary" type="button" :disabled="!selectedWorkflowId" @click="loadExecutions">Retry</button>
      </div>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Execution history failed"
      :message="pageError"
      action-label="Retry"
      @action="loadExecutions"
    />

    <StateBlock
      v-if="workflowStore.workflows.length === 0"
      title="No workflows available"
      message="Create a workflow before reviewing execution history."
      action-label="Go to workflows"
      @action="router.push('/workflows')"
    />

    <StateBlock
      v-else-if="executionStore.executionLogs.length === 0"
      title="No executions recorded"
      :message="selectedWorkflow ? `${selectedWorkflow.name} has not run yet.` : 'Choose a workflow to view its history.'"
      action-label="Open workflow"
      @action="selectedWorkflowId && router.push(`/workflows/${selectedWorkflowId}`)"
    />

    <section v-else class="table-panel" aria-label="Execution list">
      <div class="table-row table-head execution-row">
        <span>Status</span>
        <span>Workflow</span>
        <span>Start time</span>
        <span>Duration</span>
        <span>Trigger</span>
        <span>Error</span>
        <span>Action</span>
      </div>
      <div v-for="execution in executionStore.executionLogs" :key="execution.id" class="table-row execution-row">
        <span class="badge" :class="`badge-status-${String(execution.status || '').toLowerCase()}`">{{ execution.status }}</span>
        <span>{{ selectedWorkflow?.name || execution.workflow_id }}</span>
        <span>{{ formatDate(execution.started_at) }}</span>
        <span>{{ execution.duration_ms }}ms</span>
        <span>{{ execution.trigger_source || 'unknown' }}</span>
        <span class="error-summary">{{ execution.error_message || '-' }}</span>
        <RouterLink class="btn btn-secondary" :to="`/workflows/${execution.workflow_id}`">Open</RouterLink>
      </div>
    </section>
  </div>
</template>
