<script setup>
import { onMounted, ref } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();
const name = ref('');
const type = ref('API_KEY');
const data = ref('');
const saving = ref(false);
const pageError = ref('');

onMounted(refresh);

async function refresh() {
  pageError.value = '';
  await workflowStore.fetchCredentials();
  if (workflowStore.error) pageError.value = workflowStore.error;
}

async function createCredential() {
  if (!name.value.trim() || !data.value.trim()) return;
  saving.value = true;
  pageError.value = '';
  try {
    await api.createCredential({ name: name.value.trim(), type: type.value, data: data.value });
    name.value = '';
    data.value = '';
    await workflowStore.fetchCredentials();
  } catch (err) {
    pageError.value = err.message;
  } finally {
    saving.value = false;
  }
}

async function deleteCredential(id) {
  if (!window.confirm('Delete this credential? The secret value will not be shown before deletion.')) return;
  pageError.value = '';
  try {
    await api.deleteCredential(id);
    await workflowStore.fetchCredentials();
  } catch (err) {
    pageError.value = err.message;
  }
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Credentials</h2>
        <p>Store encrypted credentials for workflow nodes. Plaintext secrets are never displayed after save.</p>
      </div>
      <button class="btn btn-secondary" type="button" @click="refresh">Retry</button>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Credential request failed"
      :message="pageError"
      action-label="Retry"
      @action="refresh"
    />

    <section class="section-panel">
      <h3>Create credential</h3>
      <div class="credential-form">
        <label>
          Name
          <input v-model="name" class="form-input" placeholder="Internal API key" />
        </label>
        <label>
          Type
          <select v-model="type" class="form-select">
            <option value="API_KEY">API Key</option>
            <option value="BEARER_TOKEN">Bearer Token</option>
            <option value="BASIC_AUTH">Basic Auth</option>
            <option value="TELEGRAM_BOT">Telegram Bot Token</option>
            <option value="OpenAI">OpenAI API Key</option>
            <option value="DeepSeek">DeepSeek API Key</option>
          </select>
        </label>
        <label>
          Secret value
          <input v-model="data" class="form-input" type="password" placeholder="Stored encrypted" />
        </label>
        <button class="btn btn-primary" type="button" :disabled="saving || !name.trim() || !data.trim()" @click="createCredential">
          Save credential
        </button>
      </div>
    </section>

    <StateBlock
      v-if="!workflowStore.loading && workflowStore.credentials.length === 0"
      title="No credentials saved"
      message="Create a credential when a node needs an API key, token, or password. Secret plaintext will not appear in this list."
    />

    <section v-else class="table-panel" aria-label="Credential list">
      <div class="table-row table-head">
        <span>Name</span>
        <span>Type</span>
        <span>Actions</span>
      </div>
      <div v-for="credential in workflowStore.credentials" :key="credential.id" class="table-row">
        <span>{{ credential.name }}</span>
        <span class="badge badge-muted">{{ credential.type }}</span>
        <button class="btn btn-danger" type="button" @click="deleteCredential(credential.id)">Delete</button>
      </div>
    </section>
  </div>
</template>
