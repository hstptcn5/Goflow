<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import { viNodeName } from '@/utils/operatorVi';
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

function localizedTemplateNodes(nodes = []) {
  return nodes.map((node) => ({
    ...node,
    name: viNodeName(node.type, node.name),
  }));
}

async function createFromTemplate(template) {
  if (!template?.workflow) return;
  creating.value = true;
  pageError.value = '';
  try {
    const data = template.workflow;
    const workflow = await workflowStore.createWorkflow(
      template.title || data.name || 'Workflow từ mẫu',
      template.summary || data.description || 'Workflow tạo từ mẫu'
    );
    const updated = await api.updateWorkflow(workflow.id, {
      name: workflow.name,
      description: workflow.description,
      is_active: workflow.is_active,
      nodes_json: JSON.stringify(localizedTemplateNodes(data.nodes || [])),
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
        <h2>Workflow</h2>
        <p>Tạo, mở, chạy thử và quản lý các workflow dùng trong Goflow.</p>
      </div>
      <div class="toolbar-actions">
        <button class="btn btn-secondary" type="button" @click="showTemplates = true">Tạo từ mẫu</button>
        <button class="btn btn-secondary" type="button" @click="refresh">Làm mới</button>
      </div>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Không tải được workflow"
      :message="pageError"
      action-label="Thử lại"
      @action="refresh"
    />

    <section class="section-panel">
      <h3>Tạo workflow</h3>
      <div class="inline-form">
        <input v-model="newName" class="form-input" placeholder="Tên workflow" aria-label="Tên workflow" />
        <input v-model="newDesc" class="form-input" placeholder="Mô tả" aria-label="Mô tả workflow" />
        <button class="btn btn-primary" type="button" :disabled="creating || !newName.trim()" @click="createBlankWorkflow">
          Tạo workflow trống
        </button>
      </div>
    </section>

    <StateBlock
      v-if="!workflowStore.loading && workflowStore.workflows.length === 0"
      title="Chưa có workflow"
      message="Bạn có thể bắt đầu từ một mẫu có sẵn hoặc tạo workflow trống."
      action-label="Tạo từ mẫu"
      @action="showTemplates = true"
    />

    <section v-else class="workflow-grid" aria-label="Danh sách workflow">
      <article v-for="workflow in workflowStore.workflows" :key="workflow.id" class="workflow-card">
        <div class="card-main">
          <div class="card-title-row">
            <h3>{{ workflow.name }}</h3>
            <span class="badge" :class="workflow.is_active ? 'badge-success' : 'badge-muted'">
              {{ workflow.is_active ? 'Đang bật' : 'Đang tắt' }}
            </span>
          </div>
          <p>{{ workflow.description || 'Chưa có mô tả' }}</p>
          <div class="meta-row">
            <span>{{ workflow.slug || workflow.id }}</span>
            <span v-if="workflow.updated_at">Cập nhật {{ workflow.updated_at }}</span>
          </div>
        </div>
        <div class="card-actions">
          <RouterLink class="btn btn-primary" :to="`/workflows/${workflow.id}`">Mở</RouterLink>
          <button class="btn btn-secondary" type="button" @click="runWorkflow(workflow)">Chạy</button>
        </div>
      </article>
    </section>

    <TemplateGallery
      v-if="showTemplates"
      title="Tạo workflow từ mẫu"
      action-label="Tạo workflow"
      @close="showTemplates = false"
      @select="createFromTemplate"
    />
  </div>
</template>
