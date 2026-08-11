<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { applianceApi } from '@/services/applianceApi';

const props = defineProps({
  bootstrap: { type: Object, required: true },
});

const token = props.bootstrap.token;
const pack = computed(() => props.bootstrap.pack || {});
const loading = ref(true);
const saving = ref(false);
const running = ref(false);
const testingKey = ref('');
const error = ref('');
const notice = ref('');
const setup = ref(null);
const status = ref(null);
const workflowStatus = ref(null);
const latest = ref(null);
const recent = ref([]);
const diagnostics = ref(null);
const configValues = ref({});
const credentialDrafts = ref({});
const credentialResults = ref({});
const runInputText = ref('{}');
const runError = ref('');
const sourceResults = ref({});
let pollTimer = null;
let pollFailures = 0;

const needsSetup = computed(() => status.value?.state === 'NEEDS_SETUP');
const latestExecution = computed(() => latest.value?.execution || workflowStatus.value?.latest_execution || null);
const executionRunning = computed(() => String(latestExecution.value?.status || '').toUpperCase() === 'RUNNING' || workflowStatus.value?.state === 'RUNNING');
const canComplete = computed(() => {
  if (setup.value?.can_complete !== true || Object.keys(configErrors.value).length > 0) return false;
  for (const field of setup.value?.config_schema || []) {
    if (!field.required || !field.test_kind) continue;
    const result = sourceResults.value[field.key];
    if (result?.status !== 'Valid' || result.testedValue !== String(configValues.value[field.key] ?? '')) return false;
  }
  for (const requirement of setup.value?.credential_requirements || []) {
    if (!requirement.required || !requirement.test_kind) continue;
    const result = credentialResults.value[requirement.key];
    if (result?.status !== 'Valid' || result.testedChat !== String(configValues.value.chat_id ?? '')) return false;
  }
  return true;
});

function fieldId(key) {
  return `pack-config-${key}`;
}

function credentialName(key) {
  return credentialDrafts.value[key]?.name || '';
}

function credentialValue(key) {
  return credentialDrafts.value[key]?.value || '';
}

function updateCredentialDraft(key, patch) {
  credentialDrafts.value[key] = { ...(credentialDrafts.value[key] || {}), ...patch };
  credentialResults.value[key] = { status: 'Not tested', message: '' };
}

function updateConfigValue(key, value) {
  if (String(configValues.value[key] ?? '') === String(value ?? '')) return;
  configValues.value[key] = value;
  const field = (setup.value?.config_schema || []).find((candidate) => candidate.key === key);
  if (field?.test_kind) sourceResults.value[key] = { status: 'Not tested', message: '' };
  if (key === 'chat_id') {
    for (const requirement of setup.value?.credential_requirements || []) {
      if (requirement.test_kind) credentialResults.value[requirement.key] = { status: 'Not tested', message: '' };
    }
  }
}

function sourceResult(key) {
  return sourceResults.value[key] || { status: 'Not tested', message: '' };
}

function credentialResult(key) {
  return credentialResults.value[key] || { status: 'Not tested', message: '' };
}

function fieldError(field) {
  if (!field.required) return '';
  const value = configValues.value[field.key];
  if (value === undefined || value === null || String(value).trim() === '') {
    return `${field.label || field.key} is required`;
  }
  if (field.type === 'url') {
    try {
      const parsed = new URL(String(value));
      if (!['http:', 'https:'].includes(parsed.protocol)) return 'Use an http or https URL';
    } catch {
      return 'Use a valid absolute URL';
    }
  }
  return '';
}

const configErrors = computed(() => {
  const errors = {};
  for (const field of setup.value?.config_schema || []) {
    const message = fieldError(field);
    if (message) errors[field.key] = message;
  }
  return errors;
});

function formatDate(value) {
  if (!value) return 'Not available';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return 'Not available';
  return parsed.toLocaleString();
}

function formatDuration(value) {
  if (!value) return '0 ms';
  if (value < 1000) return `${value} ms`;
  return `${(value / 1000).toFixed(1)} s`;
}

async function refresh() {
  error.value = '';
  const [setupData, statusData, workflowData, latestData, recentData] = await Promise.all([
    applianceApi.getSetup(),
    applianceApi.getStatus(),
    applianceApi.getWorkflowStatus(),
    applianceApi.getLatestExecution(),
    applianceApi.getRecentExecutions(10),
  ]);
  setup.value = setupData;
  status.value = statusData;
  workflowStatus.value = workflowData;
  latest.value = latestData;
  recent.value = recentData.executions || [];
  configValues.value = { ...(setupData.current_config_values || {}) };
  for (const req of setupData.credential_requirements || []) {
    if (!credentialDrafts.value[req.key]) credentialDrafts.value[req.key] = { name: req.label || req.key, value: '' };
  }
}

async function refreshRuntime() {
  const [statusData, workflowData, latestData, recentData] = await Promise.all([
    applianceApi.getStatus(),
    applianceApi.getWorkflowStatus(),
    applianceApi.getLatestExecution(),
    applianceApi.getRecentExecutions(10),
  ]);
  status.value = statusData;
  workflowStatus.value = workflowData;
  latest.value = latestData;
  recent.value = recentData.executions || [];
}

async function load() {
  loading.value = true;
  try {
    await refresh();
    if (executionRunning.value) startPolling();
  } catch (err) {
    error.value = err.message || 'Appliance status could not be loaded';
  } finally {
    loading.value = false;
  }
}

async function testSource(field) {
  const testedValue = String(configValues.value[field.key] ?? '');
  sourceResults.value[field.key] = { status: 'Testing', message: 'Checking the JSON endpoint', testedValue };
  error.value = '';
  try {
    await applianceApi.saveConfig(token, configValues.value);
    const result = await applianceApi.testSource(token, field.key);
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    setup.value = await applianceApi.getSetup();
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    const date = result.summary?.report_date ? `Report ${result.summary.report_date}; ` : '';
    sourceResults.value[field.key] = {
      status: 'Valid',
      message: `${date}${result.summary?.valid_fields || 0} required fields valid`,
      testedValue,
    };
  } catch (err) {
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    sourceResults.value[field.key] = { status: 'Invalid', message: err.message || 'Source test failed', category: err.category, testedValue };
  }
}

async function saveConfig() {
  if (Object.keys(configErrors.value).length > 0) return;
  saving.value = true;
  error.value = '';
  try {
    await applianceApi.saveConfig(token, configValues.value);
    notice.value = 'Configuration saved';
    await refresh();
  } catch (err) {
    error.value = err.message || 'Configuration could not be saved';
  } finally {
    saving.value = false;
  }
}

async function createCredential(req) {
  saving.value = true;
  error.value = '';
  try {
    await applianceApi.saveConfig(token, configValues.value);
    await applianceApi.createCredential(token, {
      key: req.key,
      name: credentialName(req.key) || req.label || req.key,
      value: credentialValue(req.key),
    });
    updateCredentialDraft(req.key, { value: '' });
    credentialResults.value[req.key] = { status: 'Not tested', message: 'Credential saved; test the bot and chat.' };
    await refresh();
  } catch (err) {
    error.value = err.message || 'Credential could not be saved';
  } finally {
    saving.value = false;
  }
}

async function testCredential(req) {
  testingKey.value = req.key;
  error.value = '';
  const testedChat = String(configValues.value.chat_id ?? '');
  credentialResults.value[req.key] = { status: 'Testing', message: 'Checking bot token and chat access', testedChat };
  try {
    await applianceApi.saveConfig(token, configValues.value);
    const result = await applianceApi.testCredential(token, req.key);
    if (String(configValues.value.chat_id ?? '') !== testedChat) return;
    credentialResults.value[req.key] = { status: 'Valid', message: result.message || 'Bot token and chat are valid.', testedChat };
  } catch (err) {
    if (String(configValues.value.chat_id ?? '') !== testedChat) return;
    credentialResults.value[req.key] = { status: 'Invalid', message: err.message || 'Telegram destination test failed', category: err.category, testedChat };
  } finally {
    testingKey.value = '';
  }
}

async function completeSetup() {
  if (!canComplete.value) return;
  saving.value = true;
  error.value = '';
  try {
    await applianceApi.completeSetup(token);
    notice.value = 'Setup completed';
    await refresh();
  } catch (err) {
    error.value = err.message || 'Setup could not be completed';
  } finally {
    saving.value = false;
  }
}

async function reopenSetup() {
  if (!window.confirm('Return to setup now?')) return;
  saving.value = true;
  try {
    await applianceApi.reopenSetup(token);
    notice.value = 'Setup reopened';
    await refresh();
    sourceResults.value = {};
    credentialResults.value = {};
  } catch (err) {
    error.value = err.message || 'Setup could not be reopened';
  } finally {
    saving.value = false;
  }
}

async function runNow() {
  if (running.value || executionRunning.value) return;
  runError.value = '';
  running.value = true;
  let input = {};
  try {
    input = JSON.parse(runInputText.value || '{}');
  } catch {
    runError.value = 'Run input must be valid JSON';
    running.value = false;
    return;
  }
  try {
    await applianceApi.runNow(token, input);
    runInputText.value = '{}';
    notice.value = 'Workflow run started';
    await refreshRuntime();
    startPolling();
  } catch (err) {
    if (err.category === 'already_running') {
      notice.value = 'A workflow run is already in progress';
      await refreshRuntime().catch(() => {});
      startPolling();
    } else {
      runError.value = err.message || 'Workflow could not be started';
    }
  } finally {
    running.value = false;
  }
}

function stopPolling() {
  if (pollTimer) clearTimeout(pollTimer);
  pollTimer = null;
  pollFailures = 0;
}

function schedulePoll(delay) {
  if (pollTimer) return;
  pollTimer = setTimeout(async () => {
    pollTimer = null;
    try {
      await refreshRuntime();
      pollFailures = 0;
      if (executionRunning.value) schedulePoll(1500);
      else running.value = false;
    } catch {
      pollFailures += 1;
      if (pollFailures <= 4) {
        schedulePoll(Math.min(1500 * (2 ** pollFailures), 6000));
      } else {
        error.value = 'Live status is temporarily unavailable. Use Diagnostics refresh before reporting the issue.';
        running.value = false;
      }
    }
  }, delay);
}

function startPolling() {
  schedulePoll(0);
}

async function loadDiagnostics() {
  try {
    diagnostics.value = await applianceApi.getDiagnostics();
  } catch (err) {
    error.value = err.message || 'Diagnostics could not be loaded';
  }
}

async function copyDiagnostics() {
  if (!diagnostics.value) await loadDiagnostics();
  await navigator.clipboard.writeText(JSON.stringify(diagnostics.value, null, 2));
  notice.value = 'Diagnostics copied';
}

function downloadDiagnostics() {
  const payload = JSON.stringify(diagnostics.value || {}, null, 2);
  const blob = new Blob([payload], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${pack.value.id || 'goflow-pack'}-diagnostics.json`;
  link.click();
  URL.revokeObjectURL(url);
}

onMounted(load);
onBeforeUnmount(stopPolling);
</script>

<template>
  <main class="appliance-shell">
    <header class="appliance-header">
      <div>
        <div class="shell-kicker">Pack appliance</div>
        <h1>{{ pack.name || 'Goflow Pack' }}</h1>
        <p>{{ pack.description || 'Managed workflow appliance' }}</p>
      </div>
      <div class="appliance-meta" aria-label="Pack identity">
        <span class="badge badge-muted">{{ pack.version || '0.0.0' }}</span>
        <span class="badge" :class="`badge-status-${String(status?.state || 'loading').toLowerCase()}`">
          {{ status?.state || 'LOADING' }}
        </span>
      </div>
    </header>

    <div v-if="loading" class="app-loading" aria-live="polite">
      <div class="spinner"></div>
      <span>Loading appliance</span>
    </div>

    <section v-else class="appliance-grid">
      <div class="appliance-main">
        <section v-if="needsSetup" class="appliance-panel" aria-labelledby="setup-title">
          <div class="panel-heading">
            <div>
              <div class="shell-kicker">First run</div>
              <h2 id="setup-title">Setup</h2>
            </div>
            <span class="badge badge-muted">Unsigned pack integrity</span>
          </div>

          <form class="setup-section" @submit.prevent="saveConfig">
            <h3>Configuration</h3>
            <div v-if="(setup?.config_schema || []).length === 0" class="muted-copy">No configuration is required.</div>
            <div v-for="field in setup?.config_schema || []" :key="field.key" class="form-group">
              <label :for="fieldId(field.key)">{{ field.label || field.key }}</label>
              <input
                v-if="field.type !== 'boolean'"
                :id="fieldId(field.key)"
                :value="configValues[field.key]"
                class="form-input"
                :type="field.type === 'password' ? 'password' : 'text'"
                :aria-invalid="Boolean(configErrors[field.key])"
                :aria-describedby="configErrors[field.key] ? `${fieldId(field.key)}-error` : undefined"
                @input="updateConfigValue(field.key, $event.target.value)"
              />
              <label v-else class="toggle-row">
                <input :checked="Boolean(configValues[field.key])" type="checkbox" @change="updateConfigValue(field.key, $event.target.checked)" />
                <span>{{ field.description || field.label || field.key }}</span>
              </label>
              <p v-if="field.description && field.type !== 'boolean'" class="field-help">{{ field.description }}</p>
              <p v-if="configErrors[field.key]" :id="`${fieldId(field.key)}-error`" class="field-error">{{ configErrors[field.key] }}</p>
              <div v-if="field.test_kind" class="test-row">
                <button class="btn btn-secondary" type="button" :disabled="saving || sourceResult(field.key).status === 'Testing' || Boolean(configErrors[field.key])" @click="testSource(field)">
                  {{ sourceResult(field.key).status === 'Testing' ? 'Testing source...' : 'Test source' }}
                </button>
                <span class="test-state" :data-state="sourceResult(field.key).status.toLowerCase().replace(' ', '-')" aria-live="polite">
                  <strong>{{ sourceResult(field.key).status }}</strong>
                  <span v-if="sourceResult(field.key).message">{{ sourceResult(field.key).message }}</span>
                </span>
              </div>
            </div>
            <button class="btn btn-secondary" type="submit" :disabled="saving || Object.keys(configErrors).length > 0">
              Save configuration
            </button>
          </form>

          <section class="setup-section" aria-labelledby="credentials-title">
            <h3 id="credentials-title">Credentials</h3>
            <div v-if="(setup?.credential_requirements || []).length === 0" class="muted-copy">No credentials are required.</div>
            <article v-for="req in setup?.credential_requirements || []" :key="req.key" class="credential-row">
              <div>
                <strong>{{ req.label || req.key }}</strong>
                <p>{{ req.description || req.type }}</p>
                <span class="badge" :class="req.assigned ? 'badge-success' : 'badge-muted'">
                  {{ req.assigned ? 'Assigned' : 'Required' }}
                </span>
              </div>
              <div class="credential-controls">
                <label>
                  Name
                  <input
                    class="form-input"
                    :value="credentialName(req.key)"
                    @input="updateCredentialDraft(req.key, { name: $event.target.value })"
                  />
                </label>
                <label>
                  Secret
                  <input
                    class="form-input"
                    type="password"
                    autocomplete="new-password"
                    :value="credentialValue(req.key)"
                    @input="updateCredentialDraft(req.key, { value: $event.target.value })"
                  />
                </label>
                <div class="toolbar-actions">
                  <button class="btn btn-secondary" type="button" :disabled="saving || !credentialValue(req.key) || Object.keys(configErrors).length > 0" @click="createCredential(req)">
                    {{ req.assigned ? 'Replace' : 'Create' }}
                  </button>
                  <button v-if="req.test_kind" class="btn btn-secondary" type="button" :disabled="testingKey === req.key || !req.assigned || Object.keys(configErrors).length > 0" @click="testCredential(req)">
                    {{ testingKey === req.key ? 'Testing Telegram...' : 'Test Telegram' }}
                  </button>
                </div>
                <div v-if="req.test_kind" class="test-state" :data-state="credentialResult(req.key).status.toLowerCase().replace(' ', '-')" aria-live="polite">
                  <strong>{{ credentialResult(req.key).status }}</strong>
                  <span v-if="credentialResult(req.key).message">{{ credentialResult(req.key).message }}</span>
                </div>
              </div>
            </article>
          </section>

          <div class="panel-actions">
            <button class="btn btn-primary" type="button" :disabled="saving || !canComplete || Object.keys(configErrors).length > 0" @click="completeSetup">
              Complete setup
            </button>
            <span v-if="setup?.missing?.length" class="field-help">{{ setup.missing.join(', ') }}</span>
            <span v-else-if="!canComplete" class="field-help">Test the current source and Telegram destination before completing setup.</span>
          </div>
        </section>

        <section v-else class="appliance-panel" aria-labelledby="dashboard-title">
          <div class="panel-heading">
            <div>
              <div class="shell-kicker">Dashboard</div>
              <h2 id="dashboard-title">Managed workflow</h2>
            </div>
            <button class="btn btn-secondary" type="button" @click="reopenSetup">Reconfigure</button>
          </div>

          <div class="status-strip">
            <div>
              <span>Server</span>
              <strong>{{ status?.server || 'ok' }}</strong>
            </div>
            <div>
              <span>Workflow</span>
              <strong>{{ workflowStatus?.workflow?.name || status?.workflow_id }}</strong>
            </div>
            <div>
              <span>State</span>
              <strong>{{ workflowStatus?.state || status?.state }}</strong>
            </div>
          </div>

          <form class="run-panel" @submit.prevent="runNow">
            <label for="run-input">Run input</label>
            <textarea id="run-input" v-model="runInputText" class="form-textarea" spellcheck="false"></textarea>
            <p v-if="runError" class="field-error">{{ runError }}</p>
            <button class="btn btn-primary" type="submit" :disabled="running || executionRunning || (workflowStatus?.state || status?.state) === 'NEEDS_SETUP'">
              {{ running || executionRunning ? 'Running...' : 'Run now' }}
            </button>
          </form>

          <section class="execution-summary" aria-labelledby="latest-title">
            <h3 id="latest-title">Latest execution</h3>
            <div v-if="latestExecution" class="summary-grid">
              <div><span>Status</span><strong>{{ latestExecution.status }}</strong></div>
              <div><span>Duration</span><strong>{{ formatDuration(latestExecution.duration_ms) }}</strong></div>
              <div><span>Started</span><strong>{{ formatDate(latestExecution.started_at) }}</strong></div>
              <div v-if="latestExecution.error_message"><span>Error</span><strong>{{ latestExecution.error_message }}</strong></div>
              <div v-if="latestExecution.error_category"><span>Category</span><strong>{{ latestExecution.error_category }}</strong></div>
            </div>
            <p v-else class="muted-copy">No executions yet.</p>
          </section>

          <section class="execution-summary" aria-labelledby="recent-title">
            <h3 id="recent-title">Recent executions</h3>
            <div class="table-panel">
              <div class="table-row table-head appliance-execution-row">
                <span>Status</span>
                <span>Started</span>
                <span>Duration</span>
                <span>Error</span>
              </div>
              <div v-for="exec in recent" :key="exec.id" class="table-row appliance-execution-row">
                <span class="badge" :class="`badge-status-${String(exec.status).toLowerCase()}`">{{ exec.status }}</span>
                <span>{{ formatDate(exec.started_at) }}</span>
                <span>{{ formatDuration(exec.duration_ms) }}</span>
                <span>{{ exec.error_message || '' }}</span>
              </div>
            </div>
          </section>
        </section>
      </div>

      <aside class="appliance-side">
        <section class="appliance-panel compact">
          <h2>Diagnostics</h2>
          <div class="toolbar-actions">
            <button class="btn btn-secondary" type="button" @click="loadDiagnostics">Refresh</button>
            <button class="btn btn-secondary" type="button" @click="copyDiagnostics">Copy</button>
            <button class="btn btn-secondary" type="button" :disabled="!diagnostics" @click="downloadDiagnostics">Download</button>
          </div>
          <pre v-if="diagnostics" class="diagnostics-box">{{ JSON.stringify(diagnostics, null, 2) }}</pre>
        </section>

        <section class="appliance-panel compact">
          <h2>Stop</h2>
          <p class="muted-copy">Close the terminal running this pack process, or press Ctrl+C in that terminal.</p>
          <RouterLink class="advanced-link" :to="`/workflows/${status?.workflow_id || ''}`">Advanced workflow UI</RouterLink>
        </section>
      </aside>
    </section>

    <div v-if="notice" class="appliance-toast" role="status">{{ notice }}</div>
    <div v-if="error" class="appliance-error" role="alert">{{ error }}</div>
  </main>
</template>
