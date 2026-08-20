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
  { id: 'zalo', label: 'Zalo OA', provider: 'zalo', kind: 'BEARER_TOKEN', secretLabel: 'Access token OA' },
  { id: 'telegram', label: 'Telegram Bot', provider: 'telegram', kind: 'API_KEY', secretLabel: 'Bot token' },
  { id: 'openai', label: 'OpenAI', provider: 'openai', kind: 'API_KEY', secretLabel: 'Khóa API' },
  { id: 'deepseek', label: 'DeepSeek', provider: 'deepseek', kind: 'API_KEY', secretLabel: 'Khóa API' },
  { id: 'github', label: 'GitHub', provider: 'github', kind: 'BEARER_TOKEN', secretLabel: 'Personal access token' },
  { id: 'custom', label: 'Nhà cung cấp khác / tùy chỉnh', provider: 'custom', kind: 'API_KEY', secretLabel: 'Giá trị bí mật' },
];

const kinds = [
  { value: 'API_KEY', label: 'Khóa API' },
  { value: 'BEARER_TOKEN', label: 'Bearer / access token' },
  { value: 'BASIC_AUTH', label: 'Dữ liệu Basic Auth' },
  { value: 'SERVICE_ACCOUNT', label: 'Dữ liệu service account' },
  { value: 'CUSTOM', label: 'Secret tùy chỉnh' },
];

const selectedTemplate = computed(() => templates.find((item) => item.id === template.value) || templates[templates.length - 1]);
const secretLabel = computed(() => template.value === 'custom' ? 'Giá trị bí mật' : selectedTemplate.value.secretLabel);
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
  if (!window.confirm('Xóa credential này? Goflow sẽ không hiển thị lại secret trước khi xóa.')) return;
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
        <h2>Thông tin xác thực</h2>
        <p>Lưu API key, token và secret dưới dạng mã hóa. Provider chỉ là metadata để Goflow chọn đúng credential cho từng node hoặc Agent.</p>
      </div>
      <button class="btn btn-secondary" type="button" @click="refresh">Làm mới</button>
    </section>

    <StateBlock
      v-if="pageError"
      tone="danger"
      title="Không tải được credential"
      :message="pageError"
      action-label="Thử lại"
      @action="refresh"
    />

    <section class="section-panel">
      <h3>Tạo credential</h3>
      <div class="credential-form">
        <label>
          Tên
          <input v-model="name" class="form-input" placeholder="Ví dụ: DeepSeek Reviewer" />
        </label>

        <label>
          Nhà cung cấp
          <select v-model="template" class="form-select">
            <option v-for="item in templates" :key="item.id" :value="item.id">{{ item.label }}</option>
          </select>
        </label>

        <label v-if="isCustom">
          ID nhà cung cấp
          <input v-model="provider" class="form-input" placeholder="ví dụ: haravan, notion, internal-crm" />
          <small>Dùng chữ thường, số, dấu chấm, gạch ngang hoặc gạch dưới. Thêm provider mới không cần tạo kiểu credential mới.</small>
        </label>

        <label v-if="isCustom">
          Kiểu xác thực
          <select v-model="kind" class="form-select">
            <option v-for="item in kinds" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>

        <label>
          {{ secretLabel }}
          <input v-model="data" class="form-input" type="password" placeholder="Được lưu dưới dạng mã hóa" />
          <small v-if="kind === 'BASIC_AUTH' || kind === 'SERVICE_ACCOUNT' || kind === 'CUSTOM'">Với credential phức tạp, lưu toàn bộ payload của provider thành một giá trị mã hóa, ví dụ JSON. Node sẽ quyết định cách đọc payload đó.</small>
        </label>

        <button class="btn btn-primary" type="button" :disabled="saving || !name.trim() || !data.trim() || !provider.trim()" @click="createCredential">
          Lưu credential
        </button>
      </div>
    </section>

    <StateBlock
      v-if="!workflowStore.loading && workflowStore.credentials.length === 0"
      title="Chưa có credential"
      message="Tạo credential khi node hoặc Agent cần API key, token, service account hoặc secret khác. Secret dạng plaintext sẽ không xuất hiện trong danh sách này."
    />

    <section v-else class="table-panel" aria-label="Danh sách credential">
      <div class="table-row table-head credential-table-row">
        <span>Tên</span>
        <span>Nhà cung cấp</span>
        <span>Kiểu xác thực</span>
        <span>Thao tác</span>
      </div>
      <div v-for="credential in workflowStore.credentials" :key="credential.id" class="table-row credential-table-row">
        <span>{{ credential.name }}</span>
        <span class="badge badge-muted">{{ credentialProvider(credential) }}</span>
        <span class="badge badge-muted">{{ credentialKind(credential) }}</span>
        <button class="btn btn-danger" type="button" @click="deleteCredential(credential.id)">Xóa</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.credential-table-row {
  grid-template-columns: minmax(180px, 1fr) minmax(110px, 0.45fr) minmax(150px, 0.55fr) auto;
}
</style>
