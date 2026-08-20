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
  return value ? new Date(value).toLocaleString('vi-VN') : '';
}

function statusLabel(value) {
  const labels = { SUCCESS: 'THÀNH CÔNG', FAILED: 'THẤT BẠI', RUNNING: 'ĐANG CHẠY', QUEUED: 'ĐANG CHỜ', SKIPPED: 'BỎ QUA' };
  const normalized = String(value || '').toUpperCase();
  return labels[normalized] || value || 'KHÔNG XÁC ĐỊNH';
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Lịch sử chạy</h2>
        <p>Xem lịch sử thực thi theo từng workflow để kiểm tra lỗi, thời gian chạy và nguồn kích hoạt.</p>
      </div>
      <div class="toolbar-actions">
        <select v-model="selectedWorkflowId" class="form-select compact-select" aria-label="Workflow" @change="loadExecutions">
          <option disabled value="">Chọn workflow</option>
          <option v-for="workflow in workflowStore.workflows" :key="workflow.id" :value="workflow.id">
            {{ workflow.name }}
          </option>
        </select>
        <button class="btn btn-secondary" type="button" :disabled="!selectedWorkflowId" @click="loadExecutions">Làm mới</button>
      </div>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Không tải được lịch sử chạy"
      :message="pageError"
      action-label="Thử lại"
      @action="loadExecutions"
    />

    <StateBlock
      v-if="workflowStore.workflows.length === 0"
      title="Chưa có workflow"
      message="Hãy tạo một workflow trước khi xem lịch sử chạy."
      action-label="Đi tới Workflow"
      @action="router.push('/workflows')"
    />

    <StateBlock
      v-else-if="executionStore.executionLogs.length === 0"
      title="Chưa có lần chạy nào"
      :message="selectedWorkflow ? `${selectedWorkflow.name} chưa được chạy.` : 'Chọn một workflow để xem lịch sử.'"
      action-label="Mở workflow"
      @action="selectedWorkflowId && router.push(`/workflows/${selectedWorkflowId}`)"
    />

    <section v-else class="table-panel" aria-label="Danh sách lần chạy">
      <div class="table-row table-head execution-row">
        <span>Trạng thái</span>
        <span>Workflow</span>
        <span>Bắt đầu</span>
        <span>Thời gian</span>
        <span>Nguồn kích hoạt</span>
        <span>Lỗi</span>
        <span>Thao tác</span>
      </div>
      <div v-for="execution in executionStore.executionLogs" :key="execution.id" class="table-row execution-row">
        <span class="badge" :class="`badge-status-${String(execution.status || '').toLowerCase()}`">{{ statusLabel(execution.status) }}</span>
        <span>{{ selectedWorkflow?.name || execution.workflow_id }}</span>
        <span>{{ formatDate(execution.started_at) }}</span>
        <span>{{ execution.duration_ms }}ms</span>
        <span>{{ execution.trigger_source || 'không xác định' }}</span>
        <span class="error-summary">{{ execution.error_message || '-' }}</span>
        <RouterLink class="btn btn-secondary" :to="`/workflows/${execution.workflow_id}`">Mở</RouterLink>
      </div>
    </section>
  </div>
</template>
