<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { api } from '@/services/api';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();
const name = ref('');
const template = ref('zalo');
const provider = ref('zalo');
const kind = ref('BEARER_TOKEN');
const data = ref('');
const saving = ref(false);
const pageError = ref('');

const templates = [
  { id: 'zalo', label: 'Zalo OA', provider: 'zalo', kind: 'BEARER_TOKEN', secretLabel: 'OA access token' },
  { id: 'telegram', label: 'Telegram Bot', provider: 'telegram', kind: 'API_KEY', secretLabel: 'Bot token' },
  { id: 'openai', label: 'OpenAI', provider: 'openai', kind: 'API_KEY', secretLabel: 'API key' },
  { id: 'deepseek', label: 'DeepSeek', provider: 'deepseek', kind: 'API_KEY', secretLabel: 'API key' },
  { id: 'github', label: 'GitHub', provider: 'github', kind: 'BEARER_TOKEN', secretLabel: 'Personal access token' },
  { id: 'custom', label: 'Custom / other provider', provider: 'custom', kind: 'API_KEY', secretLabel: 'Secret value' },
];

const kinds = [
  { value: 'API_KEY', label: 'API Key' },
  { value: 'BEARER_TOKEN', label: 'Bearer / access token' },
  { value: 'BASIC_AUTH', label: 'Basic Auth payload' },
  { value: 'SERVICE_ACCOUNT', label: 'Service account payload' },
  { value: 'CUSTOM', label: 'Custom secret payload' },
];

const selectedTemplate = computed(() => templates.find((item) => item.id === template.value) || templates[templates.length - 1]);
const secretLabel = computed(() => template.value === 'custom' ? 'Secret value' : selectedTemplate.value.secretLabel);
const isCustom = computed(() => template.value === 'custom');

watch(template, (value) => {
  const picked = templates.find((item) => item.id === value);
  if (!picked) return;
  provider.value = picked.provider;
  kind.value = picked.kind;
});

onMounted(refresh);

async function refresh() {
  pageError.value = '';
  await workflowStore.fetchCredentials();
  if (workflowStore.error) pageError.value = workflowStore.error;
}

async function createCredential() {
  if (!name.value.trim() || !data.value.trim() || !provider.value.trim() || !kind.value) return;
  saving.value = true;
  pageError.value = '';
  try {
    await api.createCredential({
      name: name.value.trim(),
      provider: provider.value.trim().toLowerCase(),
      kind: kind.value,
      data: data.value,
    });
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

function credentialKind(credential) {
  return credential.kind || credential.type || 'CUSTOM';
}

function credentialProvider(credential) {
  return credential.provider || 'custom';
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Credentials</h2>
        <p>Store encrypted secrets by generic authentication kind. Provider is metadata/template information, not a new credential type.</p>
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
          <input v-model="name" class="form-input" placeholder="My Zalo OA token" />
        </label>

        <label>
          Provider template
          <select v-model="template" class="form-select">
            <option v-for="item in templates" :key="item.id" :value="item.id">{{ item.label }}</option>
          </select>
        </label>

        <label v-if="isCustom">
          Provider ID
          <input v-model="provider" class="form-input" placeholder="e.g. haravan, notion, internal-crm" />
          <small>Lowercase letters, numbers, dot, dash, and underscore. Adding a new provider does not require a new credential enum.</small>
        </label>

        <label v-if="isCustom">
          Authentication kind
          <select v-model="kind" class="form-select">
            <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>

        <label>
          {{ secretLabel }}
          <input v-model="data" class="form-input" type="password" placeholder="Stored encrypted" />
          <small v-if="kind === 'BASIC_AUTH' || kind === 'SERVICE_ACCOUNT' || kind === 'CUSTOM'">For complex credentials, store the provider payload as one encrypted value (for example JSON). Nodes decide how to interpret it.</small>
        </label>

        <button class="btn btn-primary" type="button" :disabled="saving || !name.trim() || !data.trim() || !provider.trim()" @click="createCredential">
          Save credential
        </button>
      </div>
    </section>

    <StateBlock
      v-if="!workflowStore.loading && workflowStore.credentials.length === 0"
      title="No credentials saved"
      message="Create a credential when a node needs an API key, token, account payload, or other secret. Secret plaintext will not appear in this list."
    />

    <section v-else class="table-panel" aria-label="Credential list">
      <div class="table-row table-head">
        <span>Name</span>
        <span>Provider</span>
        <span>Auth kind</span>
        <span>Actions</span>
      </div>
      <div v-for="credential in workflowStore.credentials" :key="credential.id" class="table-row">
        <span>{{ credential.name }}</span>
        <span class="badge badge-muted">{{ credentialProvider(credential) }}</span>
        <span class="badge badge-muted">{{ credentialKind(credential) }}</span>
        <button class="btn btn-danger" type="button" @click="deleteCredential(credential.id)">Delete</button>
      </div>
    </section>
  </div>
</template>
