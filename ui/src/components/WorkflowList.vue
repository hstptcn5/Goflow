<script setup>
import { ref, onMounted } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';

const emit = defineEmits(['close', 'selectWorkflow']);
const workflowStore = useWorkflowStore();

const newName = ref('');
const newDesc = ref('');
const creating = ref(false);

onMounted(() => {
  workflowStore.fetchWorkflows();
});

async function handleCreate() {
  if (!newName.value.trim()) return;
  creating.value = true;
  try {
    const wf = await workflowStore.createWorkflow(newName.value, newDesc.value);
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

async function handleDelete(id) {
  if (confirm('Are you sure you want to delete this workflow?')) {
    await workflowStore.deleteWorkflow(id);
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-card glass-panel">
      <div class="modal-header">
        <h3>📋 Workflows Manager</h3>
        <button class="btn-icon" @click="emit('close')">✕</button>
      </div>

      <div class="modal-body">
        <!-- Form Tạo mới -->
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
          <button class="btn btn-primary" :disabled="creating" @click="handleCreate">
            + Create Workflow
          </button>
        </div>

        <div class="divider"></div>

        <!-- Danh sách Workflows -->
        <h4>All Workflows ({{ workflowStore.workflows.length }})</h4>
        <div class="wf-list">
          <div
            v-for="wf in workflowStore.workflows"
            :key="wf.id"
            class="wf-item"
            :class="{ active: workflowStore.currentWorkflow?.id === wf.id }"
            @click="emit('selectWorkflow', wf.id); emit('close');"
          >
            <div class="wf-info">
              <div class="wf-title-row">
                <span class="wf-title">{{ wf.name }}</span>
                <span class="badge" :class="wf.is_active ? 'badge-green' : 'badge-gray'">
                  {{ wf.is_active ? 'Active' : 'Inactive' }}
                </span>
              </div>
              <span class="wf-desc">{{ wf.description || 'No description' }}</span>
            </div>

            <button class="btn-icon danger" @click.stop="handleDelete(wf.id)">
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-card {
  width: 540px;
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
  font-size: 1rem;
}
.btn-icon.danger:hover {
  color: var(--accent-red);
}
</style>
