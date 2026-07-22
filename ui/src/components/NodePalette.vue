<script setup>
import { onMounted } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';

const workflowStore = useWorkflowStore();

onMounted(() => {
  workflowStore.fetchNodeDefinitions();
});

function onDragStart(event, nodeDef) {
  event.dataTransfer.setData('application/goflow-node', JSON.stringify(nodeDef));
  event.dataTransfer.effectAllowed = 'move';
}
</script>

<template>
  <aside class="node-palette glass-panel">
    <div class="palette-header">
      <span class="icon">🧩</span>
      <span class="title">Node Library</span>
    </div>

    <div class="palette-scroll">
      <!-- Group Triggers -->
      <div class="group-title">Triggers</div>
      <div class="nodes-grid">
        <div
          v-for="def in workflowStore.nodeDefinitions.filter(d => d.category === 'TRIGGER')"
          :key="def.type"
          class="palette-item item-trigger"
          draggable="true"
          @dragstart="onDragStart($event, def)"
        >
          <div class="item-icon">🔗</div>
          <div class="item-info">
            <span class="item-name">{{ def.name }}</span>
            <span class="item-desc">{{ def.description }}</span>
          </div>
        </div>
      </div>

      <!-- Group Actions -->
      <div class="group-title">Actions</div>
      <div class="nodes-grid">
        <div
          v-for="def in workflowStore.nodeDefinitions.filter(d => d.category === 'ACTION')"
          :key="def.type"
          class="palette-item item-action"
          draggable="true"
          @dragstart="onDragStart($event, def)"
        >
          <div class="item-icon">⚡</div>
          <div class="item-info">
            <span class="item-name">{{ def.name }}</span>
            <span class="item-desc">{{ def.description }}</span>
          </div>
        </div>
      </div>

      <!-- Group Logic -->
      <div class="group-title">Logic & Flow</div>
      <div class="nodes-grid">
        <div
          v-for="def in workflowStore.nodeDefinitions.filter(d => d.category === 'LOGIC')"
          :key="def.type"
          class="palette-item item-logic"
          draggable="true"
          @dragstart="onDragStart($event, def)"
        >
          <div class="item-icon">🔀</div>
          <div class="item-info">
            <span class="item-name">{{ def.name }}</span>
            <span class="item-desc">{{ def.description }}</span>
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.node-palette {
  width: 250px;
  height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  border-radius: 0;
  background: #ffffff;
  border-right: 1px solid var(--border-color);
  user-select: none;
}

.palette-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  font-size: 0.95rem;
  color: #0f172a;
  border-bottom: 1px solid var(--border-color);
  background: #f8fafc;
}

.palette-scroll {
  padding: 14px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.group-title {
  font-size: 0.725rem;
  font-weight: 800;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.nodes-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.palette-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: #ffffff;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  cursor: grab;
  transition: all 0.15s ease;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
}

.palette-item:hover {
  transform: translateY(-2px);
  border-color: #2563eb;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.15);
}

.palette-item:active {
  cursor: grabbing;
}

.item-icon {
  font-size: 1.1rem;
}

.item-info {
  display: flex;
  flex-direction: column;
}

.item-name {
  font-size: 0.85rem;
  font-weight: 700;
  color: #0f172a;
}

.item-desc {
  font-size: 0.725rem;
  color: #64748b;
  line-height: 1.25;
}
</style>
