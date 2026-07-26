<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import TemplateGallery from '@/components/TemplateGallery.vue';
import StateBlock from '@/components/StateBlock.vue';

const router = useRouter();
const workflowStore = useWorkflowStore();
const newName = ref('');
const newDesc = ref('');
const creating = ref(false);
const showTemplates = ref(false);
const pageError = ref('');

async function refresh() {
  pageError.value = '';
  await workflowStore.fetchWorkflows();
  if (workflowStore.error) pageError.value = workflowStore.error;
}

async function createBlankWorkflow() {
  if (!newName.value.trim()) return;
  creating.value = true;
  pageError.value = '';
  try {
    const workflow = await workflowStore.createWorkflow(newName.value.trim(), newDesc.value.trim());
    newName.value = '';
    newDesc.value = '';
    await router.push(`/workflows/${workflow.id}`);
  } catch (err) {
    pageError.value = err.message;
  } finally {
    creating.value = false;
  }
}

async function createFromTemplate(template) {
  if (!template?.workflow) return;
  creating.value = true;
  pageError.value = '';
  try {
    const data = template.workflow;
    const workflow = await workflowStore.createWorkflow(
      data.name || template.title,
      data.description || template.summary || 'Workflow created from template'
    );
    const updated = await api.updateWorkflow(workflow.id, {
      name: workflow.name,
      description: workflow.description,
      is_active: workflow.is_active,
      nodes_json: JSON.stringify(data.nodes || []),
      edges_json: JSON.stringify(data.edges || []),
    });
    workflowStore.currentWorkflow = updated;
    showTemplates.value = false;
    await router.push(`/workflows/${updated.id}`);
  } catch (err) {
    pageError.value = err.message;
  } finally {
    creating.value = false;
  }
}

async function runWorkflow(workflow) {
  pageError.value = '';
  try {
    await api.triggerWorkflow(workflow.id, { source: 'ui-workflows-page' }, true);
  } catch (err) {
    pageError.value = err.message;
  }
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Workflows</h2>
        <p>Create, open, run, and organize workflow definitions.</p>
      </div>
      <div class="toolbar-actions">
        <button class="btn btn-secondary" type="button" @click="showTemplates = true">Create from template</button>
        <button class="btn btn-secondary" type="button" @click="refresh">Retry</button>
      </div>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Workflow request failed"
      :message="pageError"
      action-label="Retry"
      @action="refresh"
    />

    <section class="section-panel">
      <h3>Create workflow</h3>
      <div class="inline-form">
        <input v-model="newName" class="form-input" placeholder="Workflow name" aria-label="Workflow name" />
        <input v-model="newDesc" class="form-input" placeholder="Description" aria-label="Workflow description" />
        <button class="btn btn-primary" type="button" :disabled="creating || !newName.trim()" @click="createBlankWorkflow">
          Create blank workflow
        </button>
      </div>
    </section>

    <StateBlock
      v-if="!workflowStore.loading && workflowStore.workflows.length === 0"
      title="No workflows yet"
      message="Start from a template to learn the shape of a working workflow, or create a blank one."
      action-label="Create from template"
      @action="showTemplates = true"
    />

    <section v-else class="workflow-grid" aria-label="Workflow list">
      <article v-for="workflow in workflowStore.workflows" :key="workflow.id" class="workflow-card">
        <div class="card-main">
          <div class="card-title-row">
            <h3>{{ workflow.name }}</h3>
            <span class="badge" :class="workflow.is_active ? 'badge-success' : 'badge-muted'">
              {{ workflow.is_active ? 'Active' : 'Inactive' }}
            </span>
          </div>
          <p>{{ workflow.description || 'No description' }}</p>
          <div class="meta-row">
            <span>{{ workflow.slug || workflow.id }}</span>
            <span v-if="workflow.updated_at">Updated {{ workflow.updated_at }}</span>
          </div>
        </div>
        <div class="card-actions">
          <RouterLink class="btn btn-primary" :to="`/workflows/${workflow.id}`">Open</RouterLink>
          <button class="btn btn-secondary" type="button" @click="runWorkflow(workflow)">Run</button>
        </div>
      </article>
    </section>

    <TemplateGallery
      v-if="showTemplates"
      title="Create From Template"
      action-label="Create Workflow"
      @close="showTemplates = false"
      @select="createFromTemplate"
    />
  </div>
</template>
