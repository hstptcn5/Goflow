<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { wsClient } from '@/services/websocket';

import Navbar from '@/components/Navbar.vue';
import WorkflowEditor from '@/components/WorkflowEditor.vue';
import ExecutionViewer from '@/components/ExecutionViewer.vue';
import WorkflowList from '@/components/WorkflowList.vue';
import CredentialModal from '@/components/CredentialModal.vue';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

const activeTab = ref('editor'); // 'editor' | 'executions'
const showWorkflowsModal = ref(false);
const showCredentialsModal = ref(false);

const editorRef = ref(null);
let unsubscribeWS = null;

onMounted(async () => {
  // Connect WebSocket real-time updates
  wsClient.connect();
  unsubscribeWS = wsClient.subscribe((event) => {
    executionStore.handleWSEvent(event);
  });

  // Load initial data
  await workflowStore.fetchWorkflows();
  await workflowStore.fetchNodeDefinitions();

  // Pick first workflow if available
  if (workflowStore.workflows.length > 0) {
    await workflowStore.selectWorkflow(workflowStore.workflows[0].id);
  } else {
    showWorkflowsModal.value = true;
  }
});

onUnmounted(() => {
  if (unsubscribeWS) unsubscribeWS();
  wsClient.disconnect();
});

function handleSaveWorkflow() {
  if (editorRef.value && editorRef.value.saveCanvas) {
    editorRef.value.saveCanvas();
  }
}
</script>

<template>
  <div class="app-layout">
    <Navbar
      v-model:activeTab="activeTab"
      @openWorkflows="showWorkflowsModal = true"
      @openCredentials="showCredentialsModal = true"
      @saveWorkflow="handleSaveWorkflow"
    />

    <main class="main-content">
      <WorkflowEditor
        v-if="activeTab === 'editor' && workflowStore.currentWorkflow"
        ref="editorRef"
      />

      <ExecutionViewer
        v-else-if="activeTab === 'executions' && workflowStore.currentWorkflow"
      />

      <div v-else class="no-workflow-state">
        <div class="empty-box glass-panel">
          <h2>👋 Welcome to Goflow</h2>
          <p>No workflow selected. Select or create a workflow to start automating!</p>
          <button class="btn btn-primary" @click="showWorkflowsModal = true">
            🚀 Open Workflows Manager
          </button>
        </div>
      </div>
    </main>

    <!-- Modals -->
    <WorkflowList
      v-if="showWorkflowsModal"
      @close="showWorkflowsModal = false"
      @selectWorkflow="workflowStore.selectWorkflow($event)"
    />

    <CredentialModal
      v-if="showCredentialsModal"
      @close="showCredentialsModal = false"
    />
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

.main-content {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.no-workflow-state {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.empty-box {
  padding: 40px;
  border-radius: 16px;
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  max-width: 480px;
}

.empty-box h2 {
  font-size: 1.5rem;
  background: linear-gradient(135deg, #fff, var(--text-secondary));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.empty-box p {
  color: var(--text-secondary);
  font-size: 0.9rem;
}
</style>
