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
const interfaceEditingId = ref('');
const interfaceForm = ref(defaultInterfaceForm());
const savingInterface = ref(false);

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

function defaultInterfaceForm() {
  return {
    slug: '',
    input_schema_json: '{}',
    output_schema_json: '{}',
    expose_cli: true,
    expose_mcp: false,
    mcp_tool_name: '',
    mcp_description: '',
    risk_level: 'medium',
    requires_approval: false,
    max_concurrent_runs: 0,
    concurrency_policy: 'global',
  };
}

function openInterfaceEditor(wf) {
  interfaceEditingId.value = wf.id;
  interfaceForm.value = {
    slug: wf.slug || '',
    input_schema_json: wf.input_schema_json || '{}',
    output_schema_json: wf.output_schema_json || '{}',
    expose_cli: wf.expose_cli !== false,
    expose_mcp: !!wf.expose_mcp,
    mcp_tool_name: wf.mcp_tool_name || '',
    mcp_description: wf.mcp_description || wf.description || '',
    risk_level: wf.risk_level || 'medium',
    requires_approval: !!wf.requires_approval,
    max_concurrent_runs: wf.max_concurrent_runs || 0,
    concurrency_policy: wf.concurrency_policy || 'global',
  };
}

function closeInterfaceEditor() {
  interfaceEditingId.value = '';
  interfaceForm.value = defaultInterfaceForm();
}

async function saveInterface() {
  if (!interfaceEditingId.value) return;
  savingInterface.value = true;
  try {
    JSON.parse(interfaceForm.value.input_schema_json || '{}');
    JSON.parse(interfaceForm.value.output_schema_json || '{}');
    await workflowStore.updateWorkflowInterface(interfaceEditingId.value, {
      ...interfaceForm.value,
      max_concurrent_runs: Number(interfaceForm.value.max_concurrent_runs || 0),
    });
    closeInterfaceEditor();
  } catch (err) {
    alert(err.message);
  } finally {
    savingInterface.value = false;
  }
}

function workflowById(id) {
  return workflowStore.workflows.find((wf) => wf.id === id) || null;
}

function mcpReadiness(wf) {
  const blockers = [];
  if (!wf?.is_active) blockers.push('Workflow is inactive');
  if (!interfaceForm.value.expose_mcp) blockers.push('Expose to MCP is off');
  if (interfaceForm.value.requires_approval) blockers.push('Requires Approval blocks MCP alpha/beta runs');
  return {
    ready: blockers.length === 0,
    blockers,
  };
}

function copyMcpHttpSmokeCommand(wf) {
  if (!wf) return;
  const command = [
    'node scripts/mcp-http-smoke-test.mjs `',
    '  --url http://127.0.0.1:8080/mcp `',
    '  --api-key $scopedToken `',
    '  --origin http://127.0.0.1:8080 `',
    `  --workflow ${wf.id} \``,
    '  --input \'{\\"source\\":\\"mcp-http-smoke\\"}\'',
  ].join('\n');
  navigator.clipboard?.writeText(command).then(
    () => alert('HTTP MCP smoke command copied'),
    () => alert(command)
  );
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
                  <span v-if="wf.expose_mcp" class="badge badge-blue">MCP</span>
                </div>
                <span class="wf-desc">{{ wf.description || 'No description' }}</span>
              </template>

              <span class="wf-id">
                ID: {{ wf.id }}
              </span>

              <div v-if="interfaceEditingId === wf.id" class="interface-panel" @click.stop>
                <div
                  class="mcp-readiness"
                  :class="mcpReadiness(workflowById(wf.id) || wf).ready ? 'ready' : 'blocked'"
                >
                  <div class="mcp-readiness-main">
                    <strong>
                      MCP {{ mcpReadiness(workflowById(wf.id) || wf).ready ? 'ready' : 'blocked' }}
                    </strong>
                    <span>
                      HTTP MCP requires Active, Expose to MCP, and no approval gate.
                    </span>
                  </div>
                  <ul v-if="!mcpReadiness(workflowById(wf.id) || wf).ready" class="mcp-blockers">
                    <li v-for="blocker in mcpReadiness(workflowById(wf.id) || wf).blockers" :key="blocker">
                      {{ blocker }}
                    </li>
                  </ul>
                  <button
                    class="btn btn-secondary btn-xs"
                    type="button"
                    :disabled="!mcpReadiness(workflowById(wf.id) || wf).ready"
                    @click="copyMcpHttpSmokeCommand(workflowById(wf.id) || wf)"
                  >
                    Copy HTTP MCP Smoke Command
                  </button>
                </div>

                <div class="interface-grid">
                  <label>
                    Slug
                    <input v-model="interfaceForm.slug" class="form-input edit-input" placeholder="daily-release-check" />
                  </label>
                  <label>
                    MCP Tool Name
                    <input v-model="interfaceForm.mcp_tool_name" class="form-input edit-input" placeholder="daily_release_check" />
                  </label>
                  <label class="wide">
                    MCP Description
                    <input v-model="interfaceForm.mcp_description" class="form-input edit-input" placeholder="Describe what this workflow does for AI clients" />
                  </label>
                  <label>
                    Risk
                    <select v-model="interfaceForm.risk_level" class="form-input edit-input">
                      <option value="low">Low</option>
                      <option value="medium">Medium</option>
                      <option value="high">High</option>
                    </select>
                  </label>
                  <label>
                    Concurrency
                    <select v-model="interfaceForm.concurrency_policy" class="form-input edit-input">
                      <option value="global">Global</option>
                      <option value="allow">Allow</option>
                      <option value="reject">Reject</option>
                      <option value="queue">Queue</option>
                    </select>
                  </label>
                  <label>
                    Max Runs
                    <input v-model.number="interfaceForm.max_concurrent_runs" type="number" min="0" class="form-input edit-input" />
                  </label>
                </div>

                <div class="interface-toggles">
                  <label class="check-row">
                    <input v-model="interfaceForm.expose_cli" type="checkbox" />
                    Enable CLI
                  </label>
                  <label class="check-row">
                    <input v-model="interfaceForm.expose_mcp" type="checkbox" />
                    Expose to MCP
                  </label>
                  <label class="check-row">
                    <input v-model="interfaceForm.requires_approval" type="checkbox" />
                    Requires Approval
                  </label>
                </div>

                <label class="schema-label">
                  Input Schema JSON
                  <textarea v-model="interfaceForm.input_schema_json" class="form-input schema-textarea" spellcheck="false"></textarea>
                </label>
                <label class="schema-label">
                  Output Schema JSON
                  <textarea v-model="interfaceForm.output_schema_json" class="form-input schema-textarea" spellcheck="false"></textarea>
                </label>

                <div class="edit-actions">
                  <button class="btn btn-primary btn-xs" :disabled="savingInterface" @click="saveInterface">
                    Save Interface
                  </button>
                  <button class="btn btn-secondary btn-xs" :disabled="savingInterface" @click="closeInterfaceEditor">
                    Cancel
                  </button>
                </div>
              </div>
            </div>

            <div class="wf-actions">
              <button v-if="interfaceEditingId !== wf.id" class="btn-icon" title="Workflow interface settings" @click.stop="openInterfaceEditor(wf)">
                Interface
              </button>
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

.badge-blue {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
}

.interface-panel {
  margin-top: 10px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.45);
}

.mcp-readiness {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  margin-bottom: 10px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.55);
}

.mcp-readiness.ready {
  border-color: rgba(16, 185, 129, 0.45);
}

.mcp-readiness.blocked {
  border-color: rgba(239, 68, 68, 0.4);
}

.mcp-readiness-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 0.72rem;
  color: var(--text-secondary);
}

.mcp-readiness-main strong {
  color: var(--text-primary);
  font-size: 0.78rem;
}

.mcp-blockers {
  margin: 0;
  padding-left: 18px;
  font-size: 0.7rem;
  color: var(--accent-red);
}

.interface-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.interface-grid label,
.schema-label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 0.68rem;
  font-weight: 700;
  color: var(--text-secondary);
}

.interface-grid .wide {
  grid-column: 1 / -1;
}

.interface-toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 10px 0;
}

.check-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.72rem;
  color: var(--text-primary);
}

.schema-label {
  margin-top: 8px;
}

.schema-textarea {
  min-height: 74px;
  resize: vertical;
  font-family: var(--font-mono);
  font-size: 0.7rem;
  line-height: 1.45;
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
