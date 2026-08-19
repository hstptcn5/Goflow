<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { wsClient } from '@/services/websocket';
import { applianceApi } from '@/services/applianceApi';
import ApplianceApp from '@/components/ApplianceApp.vue';
import ApplianceActivationProbe from '@/components/ApplianceActivationProbe.vue';
import AppShell from '@/components/AppShell.vue';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const route = useRoute();
const initialLoading = ref(true);
const bootError = ref('');
const applianceBootstrap = ref(null);
let unsubscribeWS = null;

async function bootstrap() {
  initialLoading.value = true;
  bootError.value = '';
  applianceBootstrap.value = null;
  try {
    const appliance = await applianceApi.bootstrap();
    if (appliance) {
      applianceBootstrap.value = appliance;
      return;
    }
    wsClient.connect();
    unsubscribeWS = wsClient.subscribe((event) => {
      executionStore.handleWSEvent(event);
    });
    await Promise.all([
      workflowStore.fetchWorkflows(),
      workflowStore.fetchNodeDefinitions(),
      workflowStore.fetchCredentials(),
    ]);
  } catch (err) {
    bootError.value = err.message || 'Failed to initialize Goflow';
  } finally {
    initialLoading.value = false;
  }
}

watch(
  () => route.fullPath,
  () => {
    if (initialLoading.value || applianceBootstrap.value) return;
    workflowStore.fetchCredentials().catch(() => {
      // Keep the current page usable; the store already records the refresh error.
    });
  }
);

onMounted(bootstrap);

onUnmounted(() => {
  if (unsubscribeWS) unsubscribeWS();
  wsClient.disconnect();
});
</script>

<template>
  <template v-if="applianceBootstrap">
    <ApplianceApp :bootstrap="applianceBootstrap" />
    <ApplianceActivationProbe :bootstrap="applianceBootstrap" />
  </template>
  <AppShell v-else>
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
