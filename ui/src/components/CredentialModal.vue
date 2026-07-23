<script setup>
import { ref, onMounted } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';

const emit = defineEmits(['close']);
const workflowStore = useWorkflowStore();

const name = ref('');
const type = ref('TELEGRAM_BOT');
const data = ref('');
const saving = ref(false);

onMounted(() => {
  workflowStore.fetchCredentials();
});

async function handleCreate() {
  if (!name.value.trim() || !data.value.trim()) return;
  saving.value = true;

  try {
    await api.createCredential({
      name: name.value,
      type: type.value,
      data: data.value,
    });
    name.value = '';
    data.value = '';
    await workflowStore.fetchCredentials();
  } catch (err) {
    alert(err.message);
  } finally {
    saving.value = false;
  }
}

async function handleDelete(id) {
  if (confirm('Delete this credential secret?')) {
    await api.deleteCredential(id);
    await workflowStore.fetchCredentials();
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-card glass-panel">
      <div class="modal-header">
        <h3>🔑 Encrypted Credentials (AES-256-GCM)</h3>
        <button class="btn-icon" @click="emit('close')">✕</button>
      </div>

      <div class="modal-body">
        <div class="create-box">
          <h4>Add New Secret Credential</h4>
          <div class="form-group">
            <label>Credential Name</label>
            <input v-model="name" type="text" placeholder="e.g. My Telegram Bot Token" class="form-input" />
          </div>
          <div class="form-group">
            <label>Credential Type</label>
            <select v-model="type" class="form-select">
              <option value="TELEGRAM_BOT">Telegram Bot Token</option>
              <option value="API_KEY">API Key</option>
              <option value="BEARER_TOKEN">Bearer Token</option>
              <option value="BASIC_AUTH">Basic Auth</option>
              <option value="OpenAI">OpenAI API Key</option>
              <option value="DeepSeek">DeepSeek API Key</option>
            </select>
          </div>
          <div class="form-group">
            <label>Secret Value (Will be encrypted with AES-256)</label>
            <input v-model="data" type="password" placeholder="••••••••••••••••" class="form-input" />
          </div>
          <button class="btn btn-primary" :disabled="saving" @click="handleCreate">
            🔒 Encrypt & Save
          </button>
        </div>

        <div class="divider"></div>

        <h4>Saved Credentials ({{ workflowStore.credentials.length }})</h4>
        <div class="cred-list">
          <div v-for="cred in workflowStore.credentials" :key="cred.id" class="cred-item">
            <div class="cred-info">
              <span class="cred-name">{{ cred.name }}</span>
              <span class="cred-badge">{{ cred.type }}</span>
            </div>
            <button class="btn-icon danger" @click="handleDelete(cred.id)">🗑️</button>
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
  width: 520px;
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

.cred-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
}

.cred-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.cred-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cred-name {
  font-weight: 600;
  font-size: 0.875rem;
}

.cred-badge {
  font-size: 0.65rem;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(139, 92, 246, 0.2);
  color: var(--accent-purple);
  font-family: var(--font-mono);
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
}
.btn-icon.danger:hover {
  color: var(--accent-red);
}
</style>
