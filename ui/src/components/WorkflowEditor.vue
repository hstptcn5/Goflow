<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { VueFlow, useVueFlow } from '@vue-flow/core';
import { Background } from '@vue-flow/background';
import { Controls } from '@vue-flow/controls';
import { MiniMap } from '@vue-flow/minimap';

import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';

import NodePalette from './NodePalette.vue';
import PropertiesPanel from './PropertiesPanel.vue';

import '@vue-flow/core/dist/style.css';
import '@vue-flow/core/dist/theme-default.css';
import '@vue-flow/controls/dist/style.css';
import '@vue-flow/minimap/dist/style.css';

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();

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
      type: 'default',
      position: n.position || { x: 250, y: 150 },
      label: n.name || n.type,
      data: { ...n },
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
    type: 'default',
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
    target: e.target,
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
        <Background pattern-color="#475569" :gap="20" :size="1.5" />
        <Controls />
        <MiniMap />
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
  background-color: #0f172a;
}

.canvas-area {
  flex: 1;
  height: 100%;
  position: relative;
}

.goflow-canvas {
  width: 100%;
  height: 100%;
  background-color: #0f172a !important;
}
</style>
