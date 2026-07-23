<script setup>
import { ref, onMounted } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import TemplateGallery from './TemplateGallery.vue';

const emit = defineEmits(['close', 'selectWorkflow']);
const workflowStore = useWorkflowStore();

const newName = ref('');
const newDesc = ref('');
const creating = ref(false);
const fileInputRef = ref(null);
const showTemplateGallery = ref(false);

const editingId = ref('');
const editName = ref('');
const editDesc = ref('');
const savingEdit = ref(false);

onMounted(() => {
  workflowStore.fetchWorkflows();
});

async function handleCreate() {
  if (!newName.value.trim()) return;
  creating.value = true;
  try {
    const wf = await workflowStore.createWorkflow(newName.value.trim(), newDesc.value.trim());
    newName.value = '';
    newDesc.value = '';
    emit('selectWorkflow', wf.id);
    emit('close');
  } catch (err) {
    alert(err.message);
  } finally {
    creating.value = false;
  }
}

function triggerFileInput() {
  fileInputRef.value.click();
}

async function handleImportJSON(event) {
  const file = event.target.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = async (e) => {
    try {
      const data = JSON.parse(e.target.result);
      if (!data.nodes || !data.edges) {
        alert('Invalid workflow JSON structure. Missing nodes or edges.');
        return;
      }

      creating.value = true;
      const wf = await workflowStore.createWorkflow(
        data.name || file.name.replace('.json', ''),
        data.description || 'Imported Goflow Workflow'
      );

      const payload = {
        name: wf.name,
        description: wf.description,
        is_active: wf.is_active,
        nodes_json: JSON.stringify(data.nodes),
        edges_json: JSON.stringify(data.edges),
      };

      const updated = await api.updateWorkflow(wf.id, payload);
      workflowStore.currentWorkflow = updated;

      emit('selectWorkflow', wf.id);
      emit('close');
    } catch (err) {
      alert('Failed to parse and import file: ' + err.message);
    } finally {
      creating.value = false;
      event.target.value = '';
    }
  };
  reader.readAsText(file);
}

async function createFromTemplate(template) {
  if (!template?.workflow) return;
  creating.value = true;
  try {
    const data = template.workflow;
    const wf = await workflowStore.createWorkflow(
      data.name || template.title,
      data.description || template.summary || 'Workflow created from template'
    );

    const payload = {
      name: wf.name,
      description: wf.description,
      is_active: wf.is_active,
      nodes_json: JSON.stringify(data.nodes || []),
      edges_json: JSON.stringify(data.edges || []),
    };

    const updated = await api.updateWorkflow(wf.id, payload);
    workflowStore.currentWorkflow = updated;
    showTemplateGallery.value = false;
    emit('selectWorkflow', wf.id);
    emit('close');
  } catch (err) {
    alert('Failed to create workflow from template: ' + err.message);
  } finally {
    creating.value = false;
  }
}

async function handleDelete(id) {
  if (confirm('Are you sure you want to delete this workflow?')) {
    await workflowStore.deleteWorkflow(id);
  }
}

function selectWorkflow(wf) {
  if (editingId.value === wf.id) return;
  emit('selectWorkflow', wf.id);
  emit('close');
}

function startEdit(wf) {
  editingId.value = wf.id;
  editName.value = wf.name || '';
  editDesc.value = wf.description || '';
}

function cancelEdit() {
  editingId.value = '';
  editName.value = '';
  editDesc.value = '';
}

async function saveEdit() {
  if (!editingId.value || !editName.value.trim()) return;
  savingEdit.value = true;
  try {
    await workflowStore.updateWorkflowMetadata(editingId.value, editName.value.trim(), editDesc.value.trim());
    cancelEdit();
  } catch (err) {
    alert(err.message);
  } finally {
    savingEdit.value = false;
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-card glass-panel">
      <div class="modal-header">
        <h3>Workflows Manager</h3>
        <button class="btn-icon" @click="emit('close')">x</button>
      </div>

      <div class="modal-body">
        <div class="create-box">
          <h4>Create New Workflow</h4>
          <div class="form-group">
            <input
              v-model="newName"
              type="text"
              placeholder="Workflow Name (e.g. Telegram Alert Bot)"
              class="form-input"
            />
          </div>
          <div class="form-group">
            <input
              v-model="newDesc"
              type="text"
              placeholder="Description (optional)"
              class="form-input"
            />
          </div>
          <div class="action-buttons-row">
            <button class="btn btn-primary" :disabled="creating || !newName.trim()" @click="handleCreate">
              + Create Workflow
            </button>
            <button class="btn btn-secondary" :disabled="creating" @click="triggerFileInput">
              Import JSON
            </button>
            <button class="btn btn-secondary" :disabled="creating" @click="showTemplateGallery = true">
              Browse Templates
            </button>
            <input
              ref="fileInputRef"
              type="file"
              accept=".json"
              style="display: none;"
              @change="handleImportJSON"
            />
          </div>
        </div>

        <div class="divider"></div>

        <h4>All Workflows ({{ workflowStore.workflows.length }})</h4>
        <div class="wf-list">
          <div
            v-for="wf in workflowStore.workflows"
            :key="wf.id"
            class="wf-item"
            :class="{ active: workflowStore.currentWorkflow?.id === wf.id }"
            @click="selectWorkflow(wf)"
          >
            <div class="wf-info">
              <template v-if="editingId === wf.id">
                <input
                  v-model="editName"
                  class="form-input edit-input"
                  placeholder="Workflow name"
                  @click.stop
                  @keyup.enter="saveEdit"
                />
                <input
                  v-model="editDesc"
                  class="form-input edit-input"
                  placeholder="Description"
                  @click.stop
                  @keyup.enter="saveEdit"
                />
                <div class="edit-actions" @click.stop>
                  <button class="btn btn-primary btn-xs" :disabled="savingEdit || !editName.trim()" @click="saveEdit">
                    Save
                  </button>
                  <button class="btn btn-secondary btn-xs" :disabled="savingEdit" @click="cancelEdit">
                    Cancel
                  </button>
                </div>
              </template>

              <template v-else>
                <div class="wf-title-row">
                  <span class="wf-title">{{ wf.name }}</span>
                  <span class="badge" :class="wf.is_active ? 'badge-green' : 'badge-gray'">
                    {{ wf.is_active ? 'Active' : 'Inactive' }}
                  </span>
                </div>
                <span class="wf-desc">{{ wf.description || 'No description' }}</span>
              </template>

              <span class="wf-id">
                ID: {{ wf.id }}
              </span>
            </div>

            <div class="wf-actions">
              <button v-if="editingId !== wf.id" class="btn-icon" title="Rename workflow" @click.stop="startEdit(wf)">
                Edit
              </button>
              <button class="btn-icon danger" title="Delete workflow" @click.stop="handleDelete(wf.id)">
                Delete
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <TemplateGallery
      v-if="showTemplateGallery"
      title="Create From Template"
      action-label="Create Workflow"
      @close="showTemplateGallery = false"
      @select="createFromTemplate"
    />
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-card {
  width: 560px;
  max-height: 80vh;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
}

.create-box {
  background: var(--bg-secondary);
  padding: 16px;
  border-radius: 10px;
  border: 1px solid var(--border-color);
  margin-bottom: 16px;
}

.create-box h4 {
  font-size: 0.875rem;
  margin-bottom: 12px;
  color: var(--text-secondary);
}

.action-buttons-row {
  display: flex;
  gap: 8px;
}

.action-buttons-row .btn {
  flex: 1;
}

.divider {
  height: 1px;
  background: var(--border-color);
  margin: 16px 0;
}

.wf-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 12px;
}

.wf-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.wf-item:hover {
  border-color: var(--accent-cyan);
  transform: translateX(2px);
}

.wf-item.active {
  border-color: var(--accent-purple);
  background: rgba(139, 92, 246, 0.1);
}

.wf-info {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.wf-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.wf-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.wf-desc {
  font-size: 0.775rem;
  color: var(--text-secondary);
}

.wf-id {
  font-size: 0.65rem;
  color: #94a3b8;
  font-family: var(--font-mono);
  display: block;
  margin-top: 4px;
  user-select: all;
}

.wf-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.edit-input {
  height: 32px;
  font-size: 0.75rem;
}

.edit-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

.btn-xs {
  padding: 4px 8px;
  font-size: 0.7rem;
}

.badge {
  font-size: 0.65rem;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.badge-green {
  background: rgba(16, 185, 129, 0.15);
  color: var(--accent-green);
}

.badge-gray {
  background: rgba(156, 163, 175, 0.15);
  color: var(--text-secondary);
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 0.75rem;
  font-weight: 700;
}

.btn-icon:hover {
  color: var(--accent-cyan);
}

.btn-icon.danger:hover {
  color: var(--accent-red);
}
</style>
