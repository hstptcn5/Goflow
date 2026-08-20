<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import { workflowTemplates } from '@/templates/workflowTemplates';
import StateBlock from '@/components/StateBlock.vue';

const router = useRouter();
const workflowStore = useWorkflowStore();
const creating = ref(false);
const pageError = ref('');

async function createFromTemplate(template) {
  creating.value = true;
  pageError.value = '';
  try {
    const data = template.workflow;
    const workflow = await workflowStore.createWorkflow(
      data.name || template.title,
      data.description || template.summary || 'Workflow tạo từ mẫu'
    );
    const updated = await api.updateWorkflow(workflow.id, {
      name: workflow.name,
      description: workflow.description,
      is_active: workflow.is_active,
      nodes_json: JSON.stringify(data.nodes || []),
      edges_json: JSON.stringify(data.edges || []),
    });
    workflowStore.currentWorkflow = updated;
    await router.push(`/workflows/${updated.id}`);
  } catch (err) {
    pageError.value = err.message;
  } finally {
    creating.value = false;
  }
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Mẫu workflow</h2>
        <p>Bắt đầu từ workflow mẫu, sau đó thay URL, ID và credential bằng cấu hình thật của bạn.</p>
      </div>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Không tạo được workflow từ mẫu"
      :message="pageError"
    />

    <section class="workflow-grid">
      <article v-for="template in workflowTemplates" :key="template.id" class="workflow-card">
        <div class="card-main">
          <div class="card-title-row">
            <h3>{{ template.title }}</h3>
            <span class="badge badge-muted">{{ template.category }}</span>
          </div>
          <p>{{ template.summary }}</p>
          <div class="meta-row">
            <span>Cần: {{ template.requirements?.join(', ') || 'Không có' }}</span>
            <span>Số node: {{ template.workflow?.nodes?.length || 0 }}</span>
          </div>
        </div>
        <button class="btn btn-primary" type="button" :disabled="creating" @click="createFromTemplate(template)">
          Tạo workflow
        </button>
      </article>
    </section>
  </div>
</template>
