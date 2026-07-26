<script setup>
import { onMounted, onUnmounted, ref } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { wsClient } from '@/services/websocket';
import AppShell from '@/components/AppShell.vue';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const initialLoading = ref(true);
const bootError = ref('');
let unsubscribeWS = null;

async function bootstrap() {
  initialLoading.value = true;
  bootError.value = '';
  try {
    wsClient.connect();
    unsubscribeWS = wsClient.subscribe((event) => {
      executionStore.handleWSEvent(event);
    });
    await Promise.all([
      workflowStore.fetchWorkflows(),
      workflowStore.fetchNodeDefinitions(),
    ]);
  } catch (err) {
    bootError.value = err.message || 'Failed to initialize Goflow';
  } finally {
    initialLoading.value = false;
  }
}

onMounted(bootstrap);

onUnmounted(() => {
  if (unsubscribeWS) unsubscribeWS();
  wsClient.disconnect();
});
</script>

<template>
  <AppShell>
    <div v-if="initialLoading" class="app-loading" aria-live="polite">
      <div class="spinner"></div>
      <span>Loading workspace</span>
    </div>

    <StateBlock
      v-else-if="bootError"
      tone="danger"
      title="Goflow is not reachable"
      :message="bootError"
      action-label="Retry"
      @action="bootstrap"
    />

    <RouterView v-else />
  </AppShell>
</template>
