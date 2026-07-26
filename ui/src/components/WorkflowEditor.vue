<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { VueFlow, useVueFlow, Handle, Position } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';

import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { api } from '@/services/api';

import NodePalette from './NodePalette.vue';
import PropertiesPanel from './PropertiesPanel.vue';
import AIAssistantDrawer from './AIAssistantDrawer.vue';
import TemplateGallery from './TemplateGallery.vue';
import { getNodeIconSVG, getNavIconSVG } from './NodeIcons';

import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const router = useRouter();
const showAIDrawer = ref(false);
const showTemplateGallery = ref(false);
const runError = ref('');
const triggering = ref(false);

function getNodeIcon(type) {
  const icons = {
    webhookTrigger: 'Webhook',
    cronTrigger: 'Cron',
    manualTrigger: 'Manual',
    httpRequest: 'HTTP',
    telegramBot: 'Telegram',
    jsonTransform: 'JSON',
    conditionIf: 'IF',
    emailSMTP: 'SMTP',
    delaySleep: 'Delay',
    openAIGPT: 'OpenAI',
    deepseekAI: 'DeepSeek',
    discordBot: 'Discord',
    slackBot: 'Slack',
    jsCodeRunner: 'JS',
    subWorkflow: 'Subflow',
    postgresQuery: 'Postgres',
    redisCommand: 'Redis',
    googleSheets: 'Sheets',
    mysqlQuery: 'MySQL',
    mongodbCommand: 'MongoDB',
    googleDrive: 'Drive',
    gmailREST: 'Gmail',
    notionPage: 'Notion',
    sshRunner: 'SSH',
    gitCommand: 'Git',
    githubWebhook: 'GitHub',
    goflowPlugin: 'Plugin',
  };
  return icons[type] || 'Node';
}
function getNodeCategory(type) {
  const triggerNodes = ['webhookTrigger', 'cronTrigger', 'manualTrigger', 'githubWebhook'];
  const databaseNodes = ['postgresQuery', 'mysqlQuery', 'mongodbCommand', 'redisCommand'];
  const saasNodes = ['googleSheets', 'googleDrive', 'gmailREST', 'notionPage', 'emailSMTP', 'telegramBot', 'discordBot', 'slackBot'];
  const aiNodes = ['openAIGPT', 'deepseekAI'];
  const devNodes = ['sshRunner', 'gitCommand'];
  
  if (triggerNodes.includes(type)) return 'category-trigger';
  if (databaseNodes.includes(type)) return 'category-db';
  if (saasNodes.includes(type)) return 'category-saas';
  if (aiNodes.includes(type)) return 'category-ai';
  if (devNodes.includes(type)) return 'category-dev';
  return 'category-logic';
}

function getNodeStatusClass(nodeId) {
  const status = executionStore.nodeStatuses[nodeId];
  if (!status) return '';
  return `status-${status.toLowerCase()}`;
}

const nodes = ref([]);
const edges = ref([]);
const selectedNodeId = ref(null);

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null;
  const found = nodes.value.find((item) => item.id === selectedNodeId.value);
  return found ? found.data : null;
});

const { onConnect } = useVueFlow();

onMounted(() => {
  loadCurrentWorkflow();
});

watch(
  () => workflowStore.currentWorkflow,
  () => {
    loadCurrentWorkflow();
  }
);

function loadCurrentWorkflow() {
  if (!workflowStore.currentWorkflow) return;

  try {
    const rawNodes = typeof workflowStore.currentWorkflow.nodes_json === 'string'
      ? JSON.parse(workflowStore.currentWorkflow.nodes_json || '[]')
      : workflowStore.currentWorkflow.nodes_json;

    const rawEdges = typeof workflowStore.currentWorkflow.edges_json === 'string'
      ? JSON.parse(workflowStore.currentWorkflow.edges_json || '[]')
      : workflowStore.currentWorkflow.edges_json;

    nodes.value = rawNodes.map((n) => ({
      id: n.id,
      type: 'custom',
      position: n.position || { x: 250, y: 150 },
      label: n.name || n.type,
      data: { 
        ...n,
        categoryClass: getNodeCategory(n.type),
        icon: getNodeIcon(n.type)
      },
    }));

    edges.value = rawEdges.map((e) => ({
      id: e.id,
      source: e.source,
      sourceHandle: e.sourceHandle || null,
      target: e.target,
      targetHandle: e.targetHandle || null,
      animated: false,
      style: { stroke: 'var(--color-border-strong)', strokeWidth: 2 },
    }));
  } catch (err) {
    console.error('Failed to parse nodes/edges JSON', err);
  }
}

onConnect((connection) => {
  const edgeId = `edge_${connection.source}-${connection.target}_${Date.now()}`;
  const newEdge = {
    id: edgeId,
    source: connection.source,
    sourceHandle: connection.sourceHandle || null,
    target: connection.target,
    targetHandle: connection.targetHandle || null,
    animated: false,
    style: { stroke: 'var(--color-border-strong)', strokeWidth: 2 },
  };
  edges.value.push(newEdge);
  workflowStore.markDirty();
});

function onDragOver(event) {
  event.preventDefault();
  event.dataTransfer.dropEffect = 'move';
}

function onDrop(event) {
  event.preventDefault();
  const rawDef = event.dataTransfer.getData('application/goflow-node');
  if (!rawDef) return;

  const nodeDef = JSON.parse(rawDef);
  const nodeId = `node_${Date.now()}`;

  const defaultParams = {};
  if (nodeDef.params) {
    nodeDef.params.forEach((p) => {
      defaultParams[p.name] = p.default ?? '';
    });
  }

  const newNode = {
    id: nodeId,
    type: 'custom',
    position: {
      x: Math.max(20, event.offsetX - 120),
      y: Math.max(20, event.offsetY - 40),
    },
    label: nodeDef.name,
    data: {
      id: nodeId,
      type: nodeDef.type,
      name: nodeDef.name,
      params: defaultParams,
      categoryClass: getNodeCategory(nodeDef.type),
      icon: getNodeIcon(nodeDef.type)
    },
  };

  nodes.value.push(newNode);
  selectedNodeId.value = nodeId;
  workflowStore.markDirty();
}

function addDelayStep() {
  const nodeId = `node_${Date.now()}`;
  nodes.value.push({
    id: nodeId,
    type: 'custom',
    position: { x: 260 + nodes.value.length * 220, y: 180 },
    label: 'Delay / Sleep',
    data: {
      id: nodeId,
      type: 'delaySleep',
      name: 'Delay / Sleep',
      params: { seconds: '1' },
      categoryClass: getNodeCategory('delaySleep'),
      icon: getNodeIcon('delaySleep'),
    },
  });
  selectedNodeId.value = nodeId;
  workflowStore.markDirty();
}

function onNodeClick(event) {
  if (event && event.node) {
    selectedNodeId.value = event.node.id;
  }
}

function onPaneClick() {
  selectedNodeId.value = null;
}

function handleUpdateNodeParams(nodeId, newParams, newName) {
  const n = nodes.value.find((item) => item.id === nodeId);
  if (n) {
    n.data.params = newParams;
    if (newName) {
      n.data.name = newName;
      n.label = newName;
    }
    workflowStore.markDirty();
  }
}

function handleDeleteNode(nodeId) {
  nodes.value = nodes.value.filter((n) => n.id !== nodeId);
  edges.value = edges.value.filter((e) => e.source !== nodeId && e.target !== nodeId);
  if (selectedNodeId.value === nodeId) {
    selectedNodeId.value = null;
  }
  workflowStore.markDirty();
}

async function saveCanvas() {
  const serializableNodes = nodes.value.map((n) => ({
    id: n.id,
    type: n.data.type,
    name: n.data.name,
    position: n.position,
    params: n.data.params,
  }));

  const serializableEdges = edges.value.map((e) => ({
    id: e.id,
    source: e.source,
    sourceHandle: e.sourceHandle || null,
    target: e.target,
    targetHandle: e.targetHandle || null,
  }));

  await workflowStore.saveCurrentWorkflow(serializableNodes, serializableEdges);
}

async function runWorkflow() {
  if (!workflowStore.currentWorkflow) return;
  triggering.value = true;
  runError.value = '';
  executionStore.resetNodeStatuses();
  try {
    const execution = await api.triggerWorkflow(workflowStore.currentWorkflow.id, {}, false);
    await executionStore.fetchExecutionHistory(workflowStore.currentWorkflow.id);
    await router.push('/executions');
    return execution;
  } catch (err) {
    runError.value = err.message;
  } finally {
    triggering.value = false;
  }
}

function handleLoadAIWorkflow(aiWorkflow) {
  if (!aiWorkflow) return;

  const mappedAINodes = (aiWorkflow.nodes || []).map((n) => {
    const nodeDef = workflowStore.nodeDefinitions.find((d) => d.type === n.type);
    const defaultParams = {};
    if (nodeDef && nodeDef.params) {
      nodeDef.params.forEach((p) => {
        defaultParams[p.name] = p.default ?? '';
      });
    }
    return {
      id: n.id,
      type: 'custom',
      position: n.position || { x: 250, y: 150 },
      label: n.name || (nodeDef ? nodeDef.name : n.type),
      data: {
        id: n.id,
        type: n.type,
        name: n.name || (nodeDef ? nodeDef.name : n.type),
        params: { ...defaultParams, ...n.params },
        categoryClass: getNodeCategory(n.type),
        icon: getNodeIcon(n.type),
      },
    };
  });

  const existingNodes = [...nodes.value];
  const finalNodes = [];

  mappedAINodes.forEach((aiNode) => {
    const existing = existingNodes.find((ex) => ex.id === aiNode.id);
    if (existing) {
      // Smart merge parameters
      const mergedParams = { ...existing.data.params };
      Object.keys(aiNode.data.params).forEach((key) => {
        const aiVal = aiNode.data.params[key];
        // Only overwrite if the AI returned a non-empty, non-null value
        if (aiVal !== '' && aiVal !== null && aiVal !== undefined) {
          mergedParams[key] = aiVal;
        }
      });

      finalNodes.push({
        ...existing,
        position: aiNode.position || existing.position,
        label: aiNode.label || existing.label,
        data: {
          ...existing.data,
          name: aiNode.data.name || existing.data.name,
          params: mergedParams,
        }
      });
    } else {
      finalNodes.push(aiNode);
    }
  });

  const mappedAIEdges = (aiWorkflow.edges || []).map((e) => ({
    id: e.id || `edge_${e.source}-${e.target}_${Date.now()}`,
    source: e.source,
    sourceHandle: e.sourceHandle || null,
    target: e.target,
    targetHandle: e.targetHandle || null,
    animated: false,
    style: { stroke: 'var(--color-border-strong)', strokeWidth: 2 },
  }));

  nodes.value = finalNodes;
  edges.value = mappedAIEdges;
  workflowStore.markDirty();
  showAIDrawer.value = false;

  // Select the first new/updated node
  if (mappedAINodes.length > 0) {
    selectedNodeId.value = mappedAINodes[0].id;
  }
}

function handleLoadTemplate(template) {
  if (!template?.workflow) return;
  handleLoadAIWorkflow(template.workflow);
  showTemplateGallery.value = false;
}

function exportCanvas() {
  if (!workflowStore.currentWorkflow) return;

  const exportData = {
    name: workflowStore.currentWorkflow.name,
    description: workflowStore.currentWorkflow.description,
    nodes: nodes.value.map((n) => ({
      id: n.id,
      type: n.data?.type || n.type,
      name: n.data?.name || n.label,
      position: n.position,
      params: n.data?.params || {},
    })),
    edges: edges.value.map((e) => ({
      id: e.id,
      source: e.source,
      sourceHandle: e.sourceHandle || null,
      target: e.target,
      targetHandle: e.targetHandle || null,
    })),
  };

  const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  const fileName = (workflowStore.currentWorkflow.name || 'workflow').replace(/\s+/g, '_');
  a.download = `${fileName}_workflow.json`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

defineExpose({ saveCanvas, exportCanvas });
</script>

<template>
  <div class="workflow-editor-container">
    <header class="workflow-topbar" aria-label="Workflow actions">
      <button class="btn btn-secondary" type="button" @click="router.push('/workflows')">Back</button>
      <div class="workflow-title-group">
        <strong>{{ workflowStore.currentWorkflow?.name || 'Workflow' }}</strong>
        <span class="save-state" :class="`save-${workflowStore.saveState}`">
          <template v-if="workflowStore.saveState === 'dirty'">Unsaved changes</template>
          <template v-else-if="workflowStore.saveState === 'saving'">Saving</template>
          <template v-else-if="workflowStore.saveState === 'failed'">Save failed</template>
          <template v-else>Saved</template>
        </span>
      </div>
      <label class="active-toggle">
        <input
          type="checkbox"
          :checked="workflowStore.currentWorkflow?.is_active"
          @change="workflowStore.toggleActive(workflowStore.currentWorkflow.id, $event.target.checked)"
        />
        <span>{{ workflowStore.currentWorkflow?.is_active ? 'Active' : 'Inactive' }}</span>
      </label>
      <button class="btn btn-primary" type="button" :disabled="triggering" @click="runWorkflow">
        {{ triggering ? 'Testing...' : 'Test Workflow' }}
      </button>
      <button class="btn btn-secondary" type="button" :disabled="workflowStore.saveState === 'saving'" @click="saveCanvas">
        Save
      </button>
      <details class="more-actions">
        <summary class="btn btn-secondary">More actions</summary>
        <div class="more-menu">
          <button type="button" @click="exportCanvas">Export</button>
          <RouterLink to="/workflows">Workflow settings</RouterLink>
          <RouterLink to="/workflows">Interface settings</RouterLink>
          <button class="danger" type="button" @click="workflowStore.deleteWorkflow(workflowStore.currentWorkflow.id); router.push('/workflows')">Delete</button>
        </div>
      </details>
    </header>

    <div v-if="workflowStore.saveState === 'failed'" class="inline-error" role="alert">
      {{ workflowStore.saveError || 'Save failed. Check the API connection and try again.' }}
    </div>
    <div v-if="runError" class="inline-error" role="alert">
      {{ runError }}
    </div>

    <NodePalette />

    <div class="canvas-area" @dragover="onDragOver" @drop="onDrop">
      <!-- Floating Canvas Toolbar -->
      <div class="canvas-toolbar">
        <button class="btn-toolbar" @click="showTemplateGallery = true">
          Templates
        </button>
        <button class="btn-toolbar" @click="showAIDrawer = true" style="display: inline-flex; align-items: center; gap: 6px;">
          <span v-html="getNavIconSVG('ai')" style="display: flex;"></span> AI Assistant
        </button>
      </div>

      <div v-if="nodes.length === 0" class="canvas-empty-state">
        <div class="empty-kicker">No nodes yet</div>
        <h2>Start from a template or ask AI to draft the workflow.</h2>
        <p>Templates give you a working node layout first. Replace placeholder credentials, URLs, chat IDs, and secrets before running.</p>
        <div class="empty-actions">
          <button class="btn btn-primary" @click="showTemplateGallery = true">Browse Templates</button>
          <button class="btn btn-secondary" @click="addDelayStep">Add Delay Step</button>
          <button class="btn btn-secondary" @click="showAIDrawer = true">Open AI Assistant</button>
        </div>
      </div>

      <VueFlow
        v-model:nodes="nodes"
        v-model:edges="edges"
        :fit-view-on-init="true"
        @node-click="onNodeClick"
        @pane-click="onPaneClick"
        class="goflow-canvas"
      >
        <!-- Custom Node Design Template -->
        <template #node-custom="{ id, data, selected }">
          <div v-if="data" class="custom-node-card" :class="[data.categoryClass, getNodeStatusClass(id), { selected: selected }]">
            <div class="node-accent-bar"></div>
            <div class="node-header">
              <span class="node-icon" v-html="getNodeIconSVG(data.type)"></span>
              <span class="node-type-label">{{ data.type || 'unknown' }}</span>
            </div>
            <div class="node-body-title">
              {{ data.name || data.type || 'Unnamed Node' }}
            </div>
            
            <!-- Conditional Node Handles -->
            <template v-if="data.type === 'conditionIf'">
              <Handle type="target" :position="Position.Top" />
              <Handle type="source" id="true" :position="Position.Bottom" style="left: 30%; background: #10b981; border-color: #ffffff;" />
              <Handle type="source" id="false" :position="Position.Bottom" style="left: 70%; background: #ef4444; border-color: #ffffff;" />
            </template>
            <!-- Standard Node Handles -->
            <template v-else>
              <Handle type="target" :position="Position.Top" />
              <Handle type="source" :position="Position.Bottom" />
            </template>
          </div>
        </template>

        <Background pattern-color="#bfdbfe" :gap="24" :size="1.5" />
        <Controls />
      </VueFlow>
    </div>

    <PropertiesPanel
      :selectedNode="selectedNode"
      @updateNodeParams="handleUpdateNodeParams"
      @deleteNode="handleDeleteNode"
      @close="selectedNodeId = null"
    />

    <!-- AI Assistant Drawer -->
    <AIAssistantDrawer
      :visible="showAIDrawer"
      :currentNodes="nodes"
      :currentEdges="edges"
      @close="showAIDrawer = false"
      @loadWorkflow="handleLoadAIWorkflow"
    />

    <TemplateGallery
      v-if="showTemplateGallery"
      title="Load Template Into Canvas"
      action-label="Load Template"
      @close="showTemplateGallery = false"
      @select="handleLoadTemplate"
    />
  </div>
</template>

<style scoped>
.workflow-editor-container {
  display: flex;
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  background-color: #f1f5f9;
  padding-top: var(--workflow-topbar-height);
}

.workflow-topbar {
  position: absolute;
  inset: 0 0 auto 0;
  height: var(--workflow-topbar-height);
  display: grid;
  grid-template-columns: auto minmax(160px, 1fr) auto auto auto auto;
  align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-4);
  background: var(--color-surface);
  border-bottom: 1px solid var(--color-border);
  z-index: var(--z-topbar);
}

.workflow-title-group {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.workflow-title-group strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.save-state {
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
}

.save-dirty,
.save-failed {
  color: var(--color-danger);
}

.save-saving {
  color: var(--color-warning);
}

.save-saved {
  color: var(--color-success);
}

.active-toggle {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.more-actions {
  position: relative;
}

.more-actions summary {
  list-style: none;
}

.more-actions summary::-webkit-details-marker {
  display: none;
}

.more-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  min-width: 190px;
  padding: var(--space-2);
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
  z-index: var(--z-menu);
}

.more-menu button,
.more-menu a {
  border: 0;
  background: transparent;
  color: var(--color-text);
  text-align: left;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  font: inherit;
  text-decoration: none;
  cursor: pointer;
}

.more-menu button:hover,
.more-menu a:hover {
  background: var(--color-surface-muted);
}

.more-menu .danger {
  color: var(--color-danger);
}

.inline-error {
  position: absolute;
  top: calc(var(--workflow-topbar-height) + var(--space-3));
  left: 50%;
  transform: translateX(-50%);
  z-index: var(--z-toast);
  max-width: min(640px, calc(100% - 48px));
  padding: 10px 14px;
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-md);
  background: var(--color-danger-surface);
  color: var(--color-danger);
  font-size: var(--font-size-sm);
  box-shadow: var(--shadow-md);
}

.canvas-area {
  flex: 1;
  height: 100%;
  position: relative;
}

.goflow-canvas {
  width: 100%;
  height: 100%;
  background-color: #ebf3fc !important; /* Soft premium light blue */
}

/* Custom Premium Nodes Styling */
.custom-node-card {
  background: #ffffff;
  border: 1.5px solid #cbd5e1;
  border-radius: 12px;
  padding: 10px 14px;
  min-width: 170px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  position: relative;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.custom-node-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.15);
}

.custom-node-card.selected {
  border-color: #7c3aed !important;
  box-shadow: 0 0 0 4px rgba(124, 58, 237, 0.25), 0 6px 18px rgba(124, 58, 237, 0.15) !important;
}

.node-accent-bar {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 5px;
  border-top-left-radius: 11px;
  border-bottom-left-radius: 11px;
  background: #64748b;
}

/* Accent Colors by Category */
.category-trigger .node-accent-bar { background: #f97316; }
.category-trigger { border-left: 1px solid #ffedd5; }

.category-logic .node-accent-bar { background: #3b82f6; }
.category-logic { border-left: 1px solid #dbeafe; }

.category-saas .node-accent-bar { background: #10b981; }
.category-saas { border-left: 1px solid #d1fae5; }

.category-db .node-accent-bar { background: #6366f1; }
.category-db { border-left: 1px solid #e0e7ff; }

.category-ai .node-accent-bar { background: #a855f7; }
.category-ai { border-left: 1px solid #f3e8ff; }

.category-dev .node-accent-bar { background: #64748b; }
.category-dev { border-left: 1px solid #f1f5f9; }

.node-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}

.node-icon {
  font-size: 0.95rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.node-type-label {
  font-size: 0.65rem;
  font-weight: 700;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.node-body-title {
  font-size: 0.8rem;
  font-weight: 600;
  color: #0f172a;
  word-break: break-word;
  text-align: left;
}

/* Execution Status Border Styles */
.custom-node-card.status-running {
  border-color: #d97706 !important; /* Warning Amber */
  box-shadow: 0 0 0 3px rgba(217, 119, 6, 0.25), 0 4px 12px rgba(0, 0, 0, 0.05) !important;
  animation: pulse-border 1.5s infinite alternate;
}

.custom-node-card.status-success {
  border-color: #16a34a !important; /* Green */
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.25), 0 4px 12px rgba(0, 0, 0, 0.05) !important;
}

.custom-node-card.status-failed {
  border-color: #dc2626 !important; /* Red */
  box-shadow: 0 0 0 3px rgba(220, 38, 38, 0.25), 0 4px 12px rgba(0, 0, 0, 0.05) !important;
}

@keyframes pulse-border {
  0% {
    box-shadow: 0 0 0 1px rgba(217, 119, 6, 0.15), 0 4px 12px rgba(0, 0, 0, 0.05);
  }
  100% {
    box-shadow: 0 0 0 5px rgba(217, 119, 6, 0.4), 0 4px 12px rgba(0, 0, 0, 0.05);
  }
}

/* Floating Canvas Toolbar styling */
.canvas-toolbar {
  position: absolute;
  top: 16px;
  left: 16px;
  z-index: 100;
  display: flex;
  gap: 8px;
  pointer-events: auto;
}

.btn-toolbar {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: #ffffff;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  font-size: 0.8rem;
  font-weight: 700;
  color: #0f172a;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  transition: all 0.15s ease;
}

.btn-toolbar:hover {
  border-color: #2563eb;
  color: #2563eb;
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.12);
}

.canvas-empty-state {
  position: absolute;
  z-index: 40;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: min(520px, calc(100% - 80px));
  background: #ffffff;
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  padding: 22px;
  box-shadow: 0 14px 35px rgba(15, 23, 42, 0.12);
}

.empty-kicker {
  color: #2563eb;
  font-size: 0.75rem;
  font-weight: 800;
  text-transform: uppercase;
  margin-bottom: 8px;
}

.canvas-empty-state h2 {
  margin: 0;
  font-size: 1.25rem;
  line-height: 1.25;
  color: #0f172a;
}

.canvas-empty-state p {
  margin: 10px 0 16px;
  color: #475569;
  line-height: 1.45;
  font-size: 0.9rem;
}

.empty-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .workflow-topbar {
    grid-template-columns: auto 1fr auto;
    height: auto;
    min-height: var(--workflow-topbar-height);
    flex-wrap: wrap;
  }

  .canvas-empty-state {
    width: calc(100% - 32px);
    padding: 18px;
  }

  .empty-actions .btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
