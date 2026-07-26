<script setup>
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { wsClient } from '@/services/websocket';
import { getLogoSVG } from './NodeIcons';

const route = useRoute();
const workflowStore = useWorkflowStore();

const navItems = [
  { label: 'Workflows', to: '/workflows' },
  { label: 'Executions', to: '/executions' },
  { label: 'Credentials', to: '/credentials' },
  { label: 'Templates', to: '/templates' },
  { label: 'Nodes', to: '/nodes' },
  { label: 'Settings', to: '/settings' },
  { label: 'Help', to: '/help' },
];

const shellTitle = computed(() => {
  if (route.name === 'workflow-editor') return workflowStore.currentWorkflow?.name || 'Workflow';
  const item = navItems.find((entry) => route.path.startsWith(entry.to));
  return item?.label || 'Goflow';
});
</script>

<template>
  <div class="app-shell">
    <aside class="nav-rail">
      <RouterLink class="brand-link" to="/workflows" aria-label="Goflow Workflows">
        <span class="brand-mark" v-html="getLogoSVG()"></span>
        <span class="brand-name">Goflow</span>
      </RouterLink>

      <nav class="rail-nav" aria-label="Primary navigation">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          class="rail-link"
          :to="item.to"
          :aria-current="route.path.startsWith(item.to) ? 'page' : undefined"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <section class="shell-main">
      <header class="shell-header">
        <div>
          <div class="shell-kicker">Workspace</div>
          <h1>{{ shellTitle }}</h1>
        </div>
        <div class="shell-status">
          <span class="status-dot" :class="{ disconnected: wsClient.status.value !== 'connected' }"></span>
          <span>{{ wsClient.status.value === 'connected' ? 'Live updates connected' : 'Live updates disconnected' }}</span>
          <button
            v-if="wsClient.status.value !== 'connected'"
            type="button"
            class="status-action"
            @click="wsClient.connect()"
          >
            Reconnect
          </button>
        </div>
      </header>

      <main class="shell-content">
        <slot />
      </main>
    </section>
  </div>
</template>
