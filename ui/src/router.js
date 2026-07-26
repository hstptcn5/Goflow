import { createRouter, createWebHistory } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';

import WorkflowsPage from '@/pages/WorkflowsPage.vue';
import WorkflowPage from '@/pages/WorkflowPage.vue';
import ExecutionsPage from '@/pages/ExecutionsPage.vue';
import CredentialsPage from '@/pages/CredentialsPage.vue';
import TemplatesPage from '@/pages/TemplatesPage.vue';
import NodesPage from '@/pages/NodesPage.vue';
import SettingsPage from '@/pages/SettingsPage.vue';
import HelpPage from '@/pages/HelpPage.vue';

export const routes = [
  { path: '/', redirect: '/workflows' },
  { path: '/workflows', name: 'workflows', component: WorkflowsPage },
  { path: '/workflows/:id', name: 'workflow-editor', component: WorkflowPage, props: true },
  { path: '/executions', name: 'executions', component: ExecutionsPage },
  { path: '/credentials', name: 'credentials', component: CredentialsPage },
  { path: '/templates', name: 'templates', component: TemplatesPage },
  { path: '/nodes', name: 'nodes', component: NodesPage },
  { path: '/settings', name: 'settings', component: SettingsPage },
  { path: '/help', name: 'help', component: HelpPage },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to, from) => {
  const workflowStore = useWorkflowStore();
  if (workflowStore.isDirty && from.name === 'workflow-editor' && to.fullPath !== from.fullPath) {
    const ok = window.confirm('You have unsaved workflow changes. Leave without saving?');
    if (!ok) return false;
    workflowStore.markSaved();
  }
  return true;
});
