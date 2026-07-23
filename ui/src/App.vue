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
import TemplateGallery from '@/components/TemplateGallery.vue';
import { api } from '@/services/api';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

const activeTab = ref('editor'); // 'editor' | 'executions'
const showWorkflowsModal = ref(false);
const showCredentialsModal = ref(false);
const showTemplateGallery = ref(false);
const initialLoading = ref(true);

const editorRef = ref(null);
let unsubscribeWS = null;

onMounted(async () => {
  try {
    // Connect WebSocket real-time updates
    wsClient.connect();
    unsubscribeWS = wsClient.subscribe((event) => {
      executionStore.handleWSEvent(event);
    });

    // Load initial data
    await workflowStore.fetchWorkflows();
    await workflowStore.fetchNodeDefinitions();

    // Pick first workflow if available; otherwise show the onboarding choices.
    if (workflowStore.workflows.length > 0) {
      await workflowStore.selectWorkflow(workflowStore.workflows[0].id);
    }
  } catch (err) {
    console.error('Failed to initialize app', err);
  } finally {
    initialLoading.value = false;
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

function handleExportWorkflow() {
  if (editorRef.value && editorRef.value.exportCanvas) {
    editorRef.value.exportCanvas();
  }
}

async function createBlankWorkflow() {
  try {
    const wf = await workflowStore.createWorkflow('Untitled Workflow', 'Created from onboarding');
    await workflowStore.selectWorkflow(wf.id);
  } catch (err) {
    alert(err.message);
  }
}

async function createWorkflowFromTemplate(template) {
  if (!template?.workflow) return;
  try {
    const data = template.workflow;
    const wf = await workflowStore.createWorkflow(
      data.name || template.title,
      data.description || template.summary || 'Workflow created from template'
    );
    const payload = {
      name: wf.name,
      description: wf.description,
      is_active: wf.is_active,
      nodes_json: JSON.stringify(data.nodes || []),
      edges_json: JSON.stringify(data.edges || []),
    };
    const updated = await api.updateWorkflow(wf.id, payload);
    workflowStore.currentWorkflow = updated;
    showTemplateGallery.value = false;
  } catch (err) {
    alert('Failed to create workflow from template: ' + err.message);
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
      @exportWorkflow="handleExportWorkflow"
    />

    <main class="main-content">
      <!-- Sleek Loading overlay during initial fetch -->
      <div v-if="initialLoading" class="loading-overlay">
        <div class="spinner"></div>
        <p>Loading Goflow Workspace...</p>
      </div>

      <WorkflowEditor
        v-else-if="activeTab === 'editor' && workflowStore.currentWorkflow"
        ref="editorRef"
      />

      <ExecutionViewer
        v-else-if="activeTab === 'executions' && workflowStore.currentWorkflow"
      />

      <div v-else class="no-workflow-state">
        <div class="empty-box glass-panel">
          <span class="empty-kicker">Welcome to Goflow</span>
          <h2>Choose how to start your first workflow.</h2>
          <p>Templates are the fastest way to learn how nodes connect. You can also create a blank workflow and drag nodes manually.</p>
          <div class="empty-actions">
            <button class="btn btn-primary" @click="showTemplateGallery = true">
              Browse Templates
            </button>
            <button class="btn btn-secondary" @click="createBlankWorkflow">
              Create Blank
            </button>
            <button class="btn btn-secondary" @click="showWorkflowsModal = true">
              Workflow Manager
            </button>
          </div>
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

    <TemplateGallery
      v-if="showTemplateGallery"
      title="Start From Template"
      action-label="Create Workflow"
      @close="showTemplateGallery = false"
      @select="createWorkflowFromTemplate"
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
  border-radius: 8px;
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  max-width: 560px;
}

.empty-box h2 {
  font-size: 1.5rem;
  color: #0f172a;
  line-height: 1.25;
}

.empty-box p {
  color: var(--text-secondary);
  font-size: 0.9rem;
  line-height: 1.45;
}

.empty-kicker {
  color: #2563eb;
  font-size: 0.75rem;
  font-weight: 800;
  text-transform: uppercase;
}

.empty-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: center;
}
.loading-overlay {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  color: var(--text-secondary);
  font-weight: 600;
  font-size: 0.9rem;
  gap: 16px;
  background: #ebf3fc;
}

.spinner {
  width: 36px;
  height: 36px;
  border: 4.5px solid rgba(37, 99, 235, 0.1);
  border-left-color: #2563eb;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
