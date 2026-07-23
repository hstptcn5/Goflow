<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { VueFlow, useVueFlow, Handle, Position } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';

import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';

import NodePalette from './NodePalette.vue';
import PropertiesPanel from './PropertiesPanel.vue';

import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

function getNodeIcon(type) {
  const icons = {
    webhookTrigger: '🔗',
    cronTrigger: '⏰',
    manualTrigger: '⚡',
    httpRequest: '🌐',
    telegramBot: '📢',
    jsonTransform: '🔄',
    conditionIf: '🌿',
    emailSMTP: '📧',
    delaySleep: '⏳',
    openAIGPT: '🤖',
    deepseekAI: '🧠',
    discordBot: '💬',
    slackBot: '🗣️',
    jsCodeRunner: '⚙️',
    subWorkflow: '📁',
    postgresQuery: '🐘',
    redisCommand: '🔑',
    googleSheets: '📊',
    mysqlQuery: '🐬',
    mongodbCommand: '🍃',
    googleDrive: '💾',
    gmailREST: '✉️',
    notionPage: '📓',
    sshRunner: '💻',
    gitCommand: '🚀',
    githubWebhook: '🐙',
    goflowPlugin: '🔌',
  };
  return icons[type] || '⚙️';
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
      type: 'customNode',
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
      target: e.target,
      animated: true,
      style: { stroke: '#38bdf8', strokeWidth: 3 },
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
    target: connection.target,
    animated: true,
    style: { stroke: '#38bdf8', strokeWidth: 3 },
  };
  edges.value.push(newEdge);
  workflowStore.isDirty = true;
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
    type: 'customNode',
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
  workflowStore.isDirty = true;
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
    workflowStore.isDirty = true;
  }
}

function handleDeleteNode(nodeId) {
  nodes.value = nodes.value.filter((n) => n.id !== nodeId);
  edges.value = edges.value.filter((e) => e.source !== nodeId && e.target !== nodeId);
  if (selectedNodeId.value === nodeId) {
    selectedNodeId.value = null;
  }
  workflowStore.isDirty = true;
}

function saveCanvas() {
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

  workflowStore.saveCurrentWorkflow(serializableNodes, serializableEdges);
}

defineExpose({ saveCanvas });
</script>

<template>
  <div class="workflow-editor-container">
    <NodePalette />

    <div class="canvas-area" @dragover="onDragOver" @drop="onDrop">
      <VueFlow
        v-model:nodes="nodes"
        v-model:edges="edges"
        :fit-view-on-init="true"
        @node-click="onNodeClick"
        @pane-click="onPaneClick"
        class="goflow-canvas"
      >
        <!-- Custom Node Design Template -->
        <template #node-customNode="{ id, data }">
          <div class="custom-node-card" :class="[data.categoryClass, getNodeStatusClass(id)]">
            <div class="node-accent-bar"></div>
            <div class="node-header">
              <span class="node-icon">{{ data.icon }}</span>
              <span class="node-type-label">{{ data.type }}</span>
            </div>
            <div class="node-body-title">
              {{ data.name || data.type }}
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
  </div>
</template>

<style scoped>
.workflow-editor-container {
  display: flex;
  width: 100vw;
  height: calc(100vh - 60px);
  position: relative;
  overflow: hidden;
  background-color: #f1f5f9;
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
</style>
