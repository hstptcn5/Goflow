<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import { VueFlow, useVueFlow, Handle, Position } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';

import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { api } from '@/services/api';

import NodePalette from './NodePalette.vue';
import NodePicker from './NodePicker.vue';
import PropertiesPanel from './PropertiesPanel.vue';
import AIAssistantDrawer from './AIAssistantDrawer.vue';
import TemplateGallery from './TemplateGallery.vue';
import { getNodeIconSVG, getNavIconSVG } from './NodeIcons';
import {
  autoLayoutGraph,
  cloneGraph,
  createWorkflowEdge,
  createWorkflowNode,
  getDefaultParams,
  summarizeNodeOperation,
  validateWorkflowGraph,
  validationStateForNode,
} from '@/utils/workflowEditor';

import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const router = useRouter();
const showAIDrawer = ref(false);
const showTemplateGallery = ref(false);
const showNodePicker = ref(false);
const showPalette = ref(false);
const runError = ref('');
const triggering = ref(false);
const validationAttempted = ref(false);
const pickerContext = ref({ sourceNodeId: null, position: { x: 260, y: 180 } });
const undoStack = ref([]);
const redoStack = ref([]);
const clipboardNode = ref(null);
const selectedEdgeId = ref(null);
const dragStartSnapshot = ref(null);
const historyLimit = 60;

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

const validationIssues = computed(() => validateWorkflowGraph(nodes.value, edges.value, workflowStore.nodeDefinitions));
const validationIssuesByNode = computed(() => {
  return validationIssues.value.reduce((acc, issue) => {
    if (!issue.nodeId) return acc;
    acc[issue.nodeId] = acc[issue.nodeId] || [];
    acc[issue.nodeId].push(issue);
    return acc;
  }, {});
});

const canUndo = computed(() => undoStack.value.length > 0);
const canRedo = computed(() => redoStack.value.length > 0);

function nodeDefForType(type) {
  return workflowStore.nodeDefinitions.find((def) => def.type === type);
}

function nodeValidationState(id) {
  return validationStateForNode(id, validationIssues.value);
}

function nodeOperationSummary(data) {
  return summarizeNodeOperation({ data }, nodeDefForType(data?.type));
}

const { onConnect } = useVueFlow();

onMounted(() => {
  loadCurrentWorkflow();
  window.addEventListener('keydown', handleEditorShortcut);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleEditorShortcut);
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
    selectedNodeId.value = null;
    selectedEdgeId.value = null;
    resetHistory();
  } catch (err) {
    console.error('Failed to parse nodes/edges JSON', err);
  }
}

onConnect((connection) => {
  if (edges.value.some((edge) => edge.source === connection.source && edge.target === connection.target && (edge.sourceHandle || null) === (connection.sourceHandle || null))) {
    return;
  }
  pushHistory();
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
  markGraphDirty();
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

  pushHistory();
  const newNode = decorateNode(createWorkflowNode(nodeDef, {
    x: Math.max(20, event.offsetX - 120),
    y: Math.max(20, event.offsetY - 40),
  }, nodeId));

  nodes.value.push(newNode);
  selectedNodeId.value = nodeId;
  markGraphDirty();
}

function decorateNode(node) {
  return {
    ...node,
    label: node.data?.name || node.label,
    data: {
      ...node.data,
      categoryClass: getNodeCategory(node.data?.type),
      icon: getNodeIcon(node.data?.type),
    },
  };
}

function markGraphDirty() {
  runError.value = '';
  validationAttempted.value = validationAttempted.value && validationIssues.value.length > 0;
  workflowStore.markDirty();
}

function resetHistory() {
  undoStack.value = [];
  redoStack.value = [];
}

function pushHistory() {
  undoStack.value.push(cloneGraph(nodes.value, edges.value));
  if (undoStack.value.length > historyLimit) undoStack.value.shift();
  redoStack.value = [];
}

function restoreSnapshot(snapshot) {
  nodes.value = snapshot.nodes.map(decorateNode);
  edges.value = snapshot.edges.map((edge) => ({ ...edge, animated: false, style: edge.style || { stroke: 'var(--color-border-strong)', strokeWidth: 2 } }));
  selectedNodeId.value = null;
  selectedEdgeId.value = null;
  workflowStore.markDirty();
}

function undo() {
  if (!canUndo.value) return;
  redoStack.value.push(cloneGraph(nodes.value, edges.value));
  restoreSnapshot(undoStack.value.pop());
}

function redo() {
  if (!canRedo.value) return;
  undoStack.value.push(cloneGraph(nodes.value, edges.value));
  restoreSnapshot(redoStack.value.pop());
}

function openNodePicker(context = {}) {
  pickerContext.value = {
    sourceNodeId: context.sourceNodeId || null,
    position: context.position || nextNodePosition(context.sourceNodeId),
  };
  showNodePicker.value = true;
}

function closeNodePicker() {
  showNodePicker.value = false;
}

function nextNodePosition(sourceNodeId) {
  const source = sourceNodeId ? nodes.value.find((node) => node.id === sourceNodeId) : null;
  if (source) {
    return { x: source.position.x + 260, y: source.position.y };
  }
  return { x: 180 + (nodes.value.length % 4) * 220, y: 140 + Math.floor(nodes.value.length / 4) * 150 };
}

function addNodeFromPicker(nodeDef) {
  pushHistory();
  const nodeId = `node_${Date.now()}`;
  const newNode = decorateNode(createWorkflowNode(nodeDef, pickerContext.value.position, nodeId));
  nodes.value.push(newNode);

  if (pickerContext.value.sourceNodeId) {
    const duplicate = edges.value.some((edge) => edge.source === pickerContext.value.sourceNodeId && edge.target === nodeId);
    if (!duplicate) {
      edges.value.push(createWorkflowEdge(pickerContext.value.sourceNodeId, nodeId));
    }
  }

  selectedNodeId.value = nodeId;
  selectedEdgeId.value = null;
  closeNodePicker();
  markGraphDirty();
}

function onNodeClick(event) {
  if (event && event.node) {
    selectedNodeId.value = event.node.id;
    selectedEdgeId.value = null;
  }
}

function onPaneClick() {
  selectedNodeId.value = null;
  selectedEdgeId.value = null;
}

function onPaneDoubleClick(event) {
  const rect = event.currentTarget.getBoundingClientRect();
  openNodePicker({
    position: {
      x: Math.max(20, event.clientX - rect.left - 120),
      y: Math.max(20, event.clientY - rect.top - 40),
    },
  });
}

function onEdgeClick(event) {
  selectedEdgeId.value = event?.edge?.id || null;
  selectedNodeId.value = null;
}

function onNodeDragStart() {
  dragStartSnapshot.value = cloneGraph(nodes.value, edges.value);
}

function onNodeDragStop() {
  if (dragStartSnapshot.value) {
    undoStack.value.push(dragStartSnapshot.value);
    if (undoStack.value.length > historyLimit) undoStack.value.shift();
    redoStack.value = [];
    dragStartSnapshot.value = null;
  }
  markGraphDirty();
}

function handleUpdateNodeParams(nodeId, newParams, newName) {
  const n = nodes.value.find((item) => item.id === nodeId);
  if (n) {
    pushHistory();
    n.data.params = newParams;
    if (newName) {
      n.data.name = newName;
      n.label = newName;
    }
    markGraphDirty();
  }
}

function handleDeleteNode(nodeId) {
  pushHistory();
  nodes.value = nodes.value.filter((n) => n.id !== nodeId);
  edges.value = edges.value.filter((e) => e.source !== nodeId && e.target !== nodeId);
  if (selectedNodeId.value === nodeId) {
    selectedNodeId.value = null;
  }
  markGraphDirty();
}

function deleteSelection() {
  if (selectedNodeId.value) {
    handleDeleteNode(selectedNodeId.value);
    return;
  }
  if (selectedEdgeId.value) {
    pushHistory();
    edges.value = edges.value.filter((edge) => edge.id !== selectedEdgeId.value);
    selectedEdgeId.value = null;
    markGraphDirty();
  }
}

function duplicateSelectedNode() {
  const source = nodes.value.find((node) => node.id === selectedNodeId.value);
  if (!source) return;
  pushHistory();
  const nodeId = `node_${Date.now()}`;
  const duplicate = decorateNode({
    ...cloneGraph([source], []).nodes[0],
    id: nodeId,
    position: { x: source.position.x + 36, y: source.position.y + 36 },
    data: {
      ...source.data,
      id: nodeId,
      name: `${source.data.name || source.label || 'Node'} copy`,
      params: { ...(source.data.params || {}) },
    },
  });
  nodes.value.push(duplicate);
  selectedNodeId.value = nodeId;
  markGraphDirty();
}

function copySelectedNode() {
  const source = nodes.value.find((node) => node.id === selectedNodeId.value);
  if (!source) return;
  clipboardNode.value = cloneGraph([source], []).nodes[0];
}

function pasteNode() {
  if (!clipboardNode.value) return;
  pushHistory();
  const nodeId = `node_${Date.now()}`;
  const pasted = decorateNode({
    ...clipboardNode.value,
    id: nodeId,
    position: { x: clipboardNode.value.position.x + 44, y: clipboardNode.value.position.y + 44 },
    data: {
      ...clipboardNode.value.data,
      id: nodeId,
      params: { ...(clipboardNode.value.data?.params || {}) },
    },
  });
  nodes.value.push(pasted);
  selectedNodeId.value = nodeId;
  markGraphDirty();
}

function autoLayout() {
  pushHistory();
  nodes.value = autoLayoutGraph(nodes.value, edges.value).map(decorateNode);
  markGraphDirty();
}

function focusValidationIssue(issue) {
  if (!issue?.nodeId) return;
  selectedNodeId.value = issue.nodeId;
}

function validateBeforeAction() {
  validationAttempted.value = true;
  if (validationIssues.value.length === 0) return true;
  const firstNodeIssue = validationIssues.value.find((issue) => issue.nodeId);
  focusValidationIssue(firstNodeIssue);
  return false;
}

async function saveCanvas() {
  if (!validateBeforeAction()) {
    workflowStore.markDirty();
    return;
  }
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
  if (!validateBeforeAction()) {
    runError.value = 'Fix workflow validation issues before testing.';
    return;
  }
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
    const defaultParams = getDefaultParams(nodeDef);
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

  pushHistory();
  nodes.value = finalNodes;
  edges.value = mappedAIEdges;
  markGraphDirty();
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

defineExpose({
  addNodeFromPicker,
  autoLayout,
  canRedo,
  canUndo,
  closeNodePicker,
  duplicateSelectedNode,
  edges,
  nodes,
  openNodePicker,
  redo,
  runWorkflow,
  saveCanvas,
  selectedNodeId,
  undo,
  validationIssues,
});

function isTypingTarget(target) {
  const tag = target?.tagName?.toLowerCase();
  return tag === 'input' || tag === 'textarea' || tag === 'select' || target?.isContentEditable;
}

function handleEditorShortcut(event) {
  if (isTypingTarget(event.target)) return;
  const mod = event.ctrlKey || event.metaKey;
  if (event.key === 'Escape') {
    if (showNodePicker.value) closeNodePicker();
    else selectedNodeId.value = null;
    return;
  }
  if ((event.key === 'a' || event.key === 'A') && !mod) {
    event.preventDefault();
    openNodePicker();
    return;
  }
  if (mod && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    openNodePicker();
    return;
  }
  if (mod && event.key.toLowerCase() === 's') {
    event.preventDefault();
    saveCanvas();
    return;
  }
  if (mod && event.key.toLowerCase() === 'z' && event.shiftKey) {
    event.preventDefault();
    redo();
    return;
  }
  if (mod && event.key.toLowerCase() === 'z') {
    event.preventDefault();
    undo();
    return;
  }
  if (mod && event.key.toLowerCase() === 'y') {
    event.preventDefault();
    redo();
    return;
  }
  if (mod && event.key.toLowerCase() === 'd') {
    event.preventDefault();
    duplicateSelectedNode();
    return;
  }
  if (mod && event.key.toLowerCase() === 'c') {
    copySelectedNode();
    return;
  }
  if (mod && event.key.toLowerCase() === 'v') {
    event.preventDefault();
    pasteNode();
    return;
  }
  if (event.key === 'Delete' || event.key === 'Backspace') {
    event.preventDefault();
    deleteSelection();
  }
}
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
          <button type="button" @click="showPalette = !showPalette">{{ showPalette ? 'Hide node library' : 'Show node library' }}</button>
          <button type="button" :disabled="!canUndo" @click="undo">Undo</button>
          <button type="button" :disabled="!canRedo" @click="redo">Redo</button>
          <button type="button" @click="autoLayout">Auto-layout</button>
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

    <div v-if="validationAttempted && validationIssues.length" class="validation-summary" role="alert" aria-live="polite">
      <strong>{{ validationIssues.length }} issue{{ validationIssues.length === 1 ? '' : 's' }} need attention</strong>
      <button
        v-for="issue in validationIssues.slice(0, 4)"
        :key="`${issue.type}-${issue.nodeId || issue.edgeId || issue.message}`"
        type="button"
        @click="focusValidationIssue(issue)"
      >
        {{ issue.message }}
      </button>
    </div>

    <NodePalette v-if="showPalette" />

    <div class="canvas-area" @dragover="onDragOver" @drop="onDrop" @dblclick="onPaneDoubleClick">
      <!-- Floating Canvas Toolbar -->
      <div class="canvas-toolbar">
        <button class="btn-toolbar primary-toolbar" type="button" @click="openNodePicker()">
          Add step
        </button>
        <button class="btn-toolbar" type="button" :disabled="!canUndo" @click="undo">Undo</button>
        <button class="btn-toolbar" type="button" :disabled="!canRedo" @click="redo">Redo</button>
        <button class="btn-toolbar" type="button" :disabled="!selectedNodeId" @click="duplicateSelectedNode">Duplicate</button>
        <button class="btn-toolbar" type="button" @click="autoLayout">Auto-layout</button>
        <button class="btn-toolbar" @click="showTemplateGallery = true">
          Templates
        </button>
        <button class="btn-toolbar" @click="showAIDrawer = true" style="display: inline-flex; align-items: center; gap: 6px;">
          <span v-html="getNavIconSVG('ai')" style="display: flex;"></span> AI Assistant
        </button>
      </div>

      <div v-if="nodes.length === 0" class="canvas-empty-state">
        <div class="empty-kicker">No nodes yet</div>
        <h2>Add a first step or start from a template.</h2>
        <p>Use Add first step to search nodes without relying on the side palette.</p>
        <div class="empty-actions">
          <button class="btn btn-primary" @click="openNodePicker()">Add first step</button>
          <button class="btn btn-primary" @click="showTemplateGallery = true">Browse Templates</button>
          <button class="btn btn-secondary" @click="showAIDrawer = true">Use AI assistant</button>
        </div>
      </div>

      <VueFlow
        v-model:nodes="nodes"
        v-model:edges="edges"
        :fit-view-on-init="true"
        @node-click="onNodeClick"
        @edge-click="onEdgeClick"
        @pane-click="onPaneClick"
        @node-drag-start="onNodeDragStart"
        @node-drag-stop="onNodeDragStop"
        class="goflow-canvas"
      >
        <!-- Custom Node Design Template -->
        <template #node-custom="{ id, data, selected }">
          <div
            v-if="data"
            class="custom-node-card"
            :class="[data.categoryClass, getNodeStatusClass(id), { selected: selected }, { invalid: validationIssuesByNode[id]?.length }]"
            :aria-label="`${data.name || data.type}. ${nodeValidationState(id)}`"
          >
            <div class="node-accent-bar"></div>
            <div class="node-header">
              <span class="node-icon" v-html="getNodeIconSVG(data.type)"></span>
              <span class="node-type-label">{{ nodeDefForType(data.type)?.category || 'Unknown' }}</span>
            </div>
            <div class="node-body-title">
              {{ data.name || data.type || 'Unnamed Node' }}
            </div>
            <div class="node-operation-summary">{{ nodeOperationSummary(data) }}</div>
            <div class="node-state-row">
              <span class="node-config-state" :class="{ invalid: validationIssuesByNode[id]?.length }">
                {{ nodeValidationState(id) }}
              </span>
              <span v-if="executionStore.nodeStatuses[id]" class="node-execution-state">
                {{ executionStore.nodeStatuses[id] }}
              </span>
            </div>
            <button
              class="quick-add-button"
              type="button"
              :aria-label="`Add step after ${data.name || data.type}`"
              @click.stop="openNodePicker({ sourceNodeId: id })"
            >
              +
            </button>
            
            <!-- Conditional Node Handles -->
            <template v-if="data.type === 'conditionIf'">
              <Handle type="target" :position="Position.Top" />
              <Handle type="source" id="true" :position="Position.Bottom" style="left: 30%; background: #10b981; border-color: #ffffff;" />
              <span class="handle-label handle-label-true">true</span>
              <Handle type="source" id="false" :position="Position.Bottom" style="left: 70%; background: #ef4444; border-color: #ffffff;" />
              <span class="handle-label handle-label-false">false</span>
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

    <NodePicker
      :visible="showNodePicker"
      @close="closeNodePicker"
      @select="addNodeFromPicker"
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
  background-color: #f8fafc !important;
}

/* Custom Premium Nodes Styling */
.custom-node-card {
  background: #ffffff;
  border: 1.5px solid #cbd5e1;
  border-radius: 8px;
  padding: 11px 14px;
  min-width: 190px;
  max-width: 240px;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.07);
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
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2), 0 6px 18px rgba(15, 23, 42, 0.12) !important;
}

.custom-node-card.invalid {
  border-color: var(--color-danger) !important;
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

.category-trigger .node-accent-bar,
.category-logic .node-accent-bar,
.category-saas .node-accent-bar,
.category-db .node-accent-bar,
.category-ai .node-accent-bar,
.category-dev .node-accent-bar { background: #64748b; }

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
  font-size: 0.92rem;
  font-weight: 750;
  color: #0f172a;
  word-break: break-word;
  text-align: left;
}

.node-operation-summary {
  margin-top: 4px;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: 1.35;
}

.node-state-row {
  margin-top: 8px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.node-config-state,
.node-execution-state {
  display: inline-flex;
  align-items: center;
  padding: 3px 6px;
  border-radius: var(--radius-sm);
  background: var(--color-success-surface);
  color: var(--color-success);
  font-size: 0.68rem;
  font-weight: 750;
}

.node-config-state.invalid {
  background: var(--color-danger-surface);
  color: var(--color-danger);
}

.node-execution-state {
  background: var(--color-surface-muted);
  color: var(--color-text-secondary);
}

.quick-add-button {
  position: absolute;
  right: -15px;
  top: 50%;
  width: 28px;
  height: 28px;
  transform: translateY(-50%);
  border: 1px solid var(--color-border-strong);
  border-radius: 999px;
  background: var(--color-surface);
  color: var(--color-primary);
  cursor: pointer;
  font-weight: 800;
  box-shadow: var(--shadow-md);
}

.handle-label {
  position: absolute;
  bottom: -25px;
  font-size: 0.68rem;
  font-weight: 750;
  color: var(--color-text-secondary);
}

.handle-label-true {
  left: calc(30% - 12px);
}

.handle-label-false {
  left: calc(70% - 14px);
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
  flex-wrap: wrap;
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

.btn-toolbar:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
}

.primary-toolbar {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.validation-summary {
  position: absolute;
  top: calc(var(--workflow-topbar-height) + var(--space-3));
  left: 50%;
  transform: translateX(-50%);
  z-index: 120;
  width: min(720px, calc(100% - 56px));
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
  padding: var(--space-3);
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-md);
  background: var(--color-danger-surface);
  color: var(--color-danger);
  box-shadow: var(--shadow-md);
}

.validation-summary button {
  border: 1px solid var(--color-danger-border);
  background: var(--color-surface);
  color: var(--color-danger);
  border-radius: var(--radius-sm);
  padding: 5px 8px;
  cursor: pointer;
  font: inherit;
  font-size: var(--font-size-xs);
}

:deep(.vue-flow__handle) {
  width: 12px;
  height: 12px;
  border: 2px solid #ffffff;
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
