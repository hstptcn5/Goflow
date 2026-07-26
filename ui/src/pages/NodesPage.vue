<script setup>
import { computed } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();

const groupedNodes = computed(() => {
  return workflowStore.nodeDefinitions.reduce((groups, node) => {
    const category = node.category || 'Other';
    if (!groups[category]) groups[category] = [];
    groups[category].push(node);
    return groups;
  }, {});
});
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Nodes</h2>
        <p>Browse available node types and required parameters before adding them to a workflow.</p>
      </div>
    </section>

    <StateBlock
      v-if="workflowStore.nodeDefinitions.length === 0"
      title="Node definitions unavailable"
      message="The API did not return node definitions. Check that Goflow is running and retry from Workflows."
    />

    <section v-for="(items, category) in groupedNodes" :key="category" class="section-panel">
      <h3>{{ category }}</h3>
      <div class="node-list">
        <article v-for="node in items" :key="node.type" class="node-list-item">
          <div>
            <strong>{{ node.name }}</strong>
            <p>{{ node.description }}</p>
          </div>
          <span class="badge badge-muted">{{ node.type }}</span>
        </article>
      </div>
    </section>
  </div>
</template>
