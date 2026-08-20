<script setup>
import { onMounted, watch, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import WorkflowEditor from '@/components/WorkflowEditor.vue';
import StateBlock from '@/components/StateBlock.vue';

const route = useRoute();
const router = useRouter();
const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const loading = ref(false);
const pageError = ref('');

async function loadWorkflow(id) {
  loading.value = true;
  pageError.value = '';
  executionStore.clearExecutionHistory();
  executionStore.resetNodeStatuses();
  try {
    await workflowStore.selectWorkflow(id);
    if (workflowStore.error) pageError.value = workflowStore.error;
  } catch (err) {
    pageError.value = err.message;
  } finally {
    loading.value = false;
  }
}

onMounted(() => loadWorkflow(route.params.id));
watch(() => route.params.id, (id) => loadWorkflow(id));
</script>

<template>
  <div class="route-fill">
    <StateBlock v-if="loading" title="Đang tải workflow" message="Đang lấy định nghĩa workflow từ Goflow API." />
    <StateBlock
      v-else-if="pageError"
      tone="danger"
      title="Không mở được workflow"
      :message="pageError"
      action-label="Quay lại danh sách workflow"
      @action="router.push('/workflows')"
    />
    <WorkflowEditor v-else-if="workflowStore.currentWorkflow" />
  </div>
</template>
