<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { applianceApi } from '@/services/applianceApi';
import { applianceMessages, optionLabel, packLocaleCopy } from './applianceI18n';
import { buildRunInput, initialRunValues, outputView } from '@/utils/applianceRunUi';

const props = defineProps({ bootstrap: { type: Object, required: true } });
const token = props.bootstrap.token;
const pack = computed(() => props.bootstrap.pack || {});
const app = computed(() => props.bootstrap.app || {});
const runUI = computed(() => pack.value.run_ui || null);
const branding = computed(() => pack.value.branding || {});

function initialLocale() {
  const saved = typeof localStorage !== 'undefined' ? localStorage.getItem('goflow-appliance-locale') : '';
  if (saved === 'en' || saved === 'vi') return saved;
  const browser = typeof navigator !== 'undefined' ? String(navigator.language || '').toLowerCase() : '';
  if (browser.startsWith('vi')) return 'vi';
  return 'en';
}
const locale = ref(initialLocale());
const t = (key) => applianceMessages[locale.value]?.[key] || applianceMessages.en[key] || key;
const packCopy = computed(() => packLocaleCopy[pack.value.id]?.[locale.value] || {});
const packName = computed(() => packCopy.value.name || pack.value.name || 'Goflow Pack');
const packDescription = computed(() => packCopy.value.description || pack.value.description || 'Managed workflow appliance');
function fieldCopy(field) {
  const pair = packCopy.value.config?.[field.key];
  return { label: pair?.[0] || field.label || field.key, description: pair?.[1] || field.description || '' };
}
function credentialCopy(req) {
  const pair = packCopy.value.credentials?.[req.key];
  return { label: pair?.[0] || req.label || req.key, description: pair?.[1] || req.description || req.type };
}
function setLocale(value) {
  locale.value = value === 'vi' ? 'vi' : 'en';
}
watch(locale, (value) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('goflow-appliance-locale', value);
  if (typeof document !== 'undefined') document.documentElement.lang = value;
});

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
const runError = ref('');
const runValues = ref(initialRunValues(runUI.value?.input_fields || []));
const runErrors = ref({});
const sourceResults = ref({});
const schedule = ref(null);
const scheduleDraft = ref({ enabled: false, local_time: '09:00', timezone: 'UTC' });
const scheduleSaving = ref(false);
let pollTimer = null;
let pollFailures = 0;

const needsSetup = computed(() => status.value?.state === 'NEEDS_SETUP');
const latestExecution = computed(() => latest.value?.execution || workflowStatus.value?.latest_execution || null);
const executionRunning = computed(() => String(latestExecution.value?.status || '').toUpperCase() === 'RUNNING' || workflowStatus.value?.state === 'RUNNING');
const latestOutput = computed(() => latestExecution.value?.output);
const renderedOutput = computed(() => outputView(latestOutput.value, runUI.value?.output_mode));
const manualRunState = computed(() => {
  if (running.value || executionRunning.value) return locale.value === 'vi' ? 'Đang chạy' : 'Running';
  if (latestExecution.value?.trigger_source === 'ui') return latestExecution.value.status || 'Complete';
  return t('ready');
});
const scheduleError = computed(() => {
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(scheduleDraft.value.local_time || '')) return t('chooseTime');
  if (!String(scheduleDraft.value.timezone || '').trim()) return t('timezoneRequired');
  return '';
});
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

function fieldId(key) { return `pack-config-${key}`; }
function credentialName(key) { return credentialDrafts.value[key]?.name || ''; }
function credentialValue(key) { return credentialDrafts.value[key]?.value || ''; }
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
function sourceResult(key) { return sourceResults.value[key] || { status: 'Not tested', message: '' }; }
function credentialResult(key) { return credentialResults.value[key] || { status: 'Not tested', message: '' }; }
function statusText(value) {
  const map = { 'Not tested': 'notTested', Testing: 'testing', Valid: 'valid', Invalid: 'invalid' };
  return t(map[value] || value);
}
function fieldError(field) {
  if (!field.required) return '';
  const value = configValues.value[field.key];
  if (value === undefined || value === null || String(value).trim() === '') return locale.value === 'vi' ? `Cần nhập ${fieldCopy(field).label}` : `${fieldCopy(field).label} is required`;
  if (field.type === 'url') {
    try {
      const parsed = new URL(String(value));
      if (!['http:', 'https:'].includes(parsed.protocol)) return locale.value === 'vi' ? 'Chỉ dùng URL http hoặc https' : 'Use an http or https URL';
    } catch { return locale.value === 'vi' ? 'Hãy nhập URL đầy đủ và hợp lệ' : 'Use a valid absolute URL'; }
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
  if (!value) return t('notAvailable');
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return t('notAvailable');
  return parsed.toLocaleString(locale.value === 'vi' ? 'vi-VN' : undefined);
}
function formatDuration(value) {
  if (!value) return '0 ms';
  if (value < 1000) return `${value} ms`;
  return `${(value / 1000).toFixed(1)} s`;
}

async function refresh() {
  error.value = '';
  const [setupData, statusData, workflowData, latestData, recentData, scheduleData] = await Promise.all([
    applianceApi.getSetup(), applianceApi.getStatus(), applianceApi.getWorkflowStatus(), applianceApi.getLatestExecution(), applianceApi.getRecentExecutions(10), applianceApi.getSchedule(),
  ]);
  setup.value = setupData; status.value = statusData; workflowStatus.value = workflowData; latest.value = latestData; recent.value = recentData.executions || []; schedule.value = scheduleData;
  const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  scheduleDraft.value = { enabled: Boolean(scheduleData.enabled), local_time: scheduleData.local_time || '09:00', timezone: scheduleData.configured ? scheduleData.timezone : browserTimezone };
  configValues.value = { ...(setupData.current_config_values || {}) };
  for (const req of setupData.credential_requirements || []) {
    if (!credentialDrafts.value[req.key]) credentialDrafts.value[req.key] = { name: credentialCopy(req).label, value: '' };
  }
}
async function refreshRuntime() {
  const [statusData, workflowData, latestData, recentData, scheduleData] = await Promise.all([
    applianceApi.getStatus(), applianceApi.getWorkflowStatus(), applianceApi.getLatestExecution(), applianceApi.getRecentExecutions(10), applianceApi.getSchedule(),
  ]);
  status.value = statusData; workflowStatus.value = workflowData; latest.value = latestData; recent.value = recentData.executions || []; schedule.value = scheduleData;
}
async function load() {
  loading.value = true;
  try { await refresh(); if (executionRunning.value) startPolling(); }
  catch (err) { error.value = err.message || 'Appliance status could not be loaded'; }
  finally { loading.value = false; }
}
async function testSource(field) {
  const testedValue = String(configValues.value[field.key] ?? '');
  sourceResults.value[field.key] = { status: 'Testing', message: locale.value === 'vi' ? 'Đang kiểm tra endpoint JSON' : 'Checking the JSON endpoint', testedValue };
  error.value = '';
  try {
    await applianceApi.saveConfig(token, configValues.value);
    const result = await applianceApi.testSource(token, field.key);
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    setup.value = await applianceApi.getSetup();
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    const date = result.summary?.report_date ? `${locale.value === 'vi' ? 'Báo cáo' : 'Report'} ${result.summary.report_date}; ` : '';
    sourceResults.value[field.key] = { status: 'Valid', message: `${date}${result.summary?.valid_fields || 0} ${locale.value === 'vi' ? 'trường bắt buộc hợp lệ' : 'required fields valid'}`, testedValue };
  } catch (err) {
    if (String(configValues.value[field.key] ?? '') !== testedValue) return;
    sourceResults.value[field.key] = { status: 'Invalid', message: err.message || 'Source test failed', category: err.category, testedValue };
  }
}
async function saveConfig() {
  if (Object.keys(configErrors.value).length > 0) return;
  saving.value = true; error.value = '';
  try { await applianceApi.saveConfig(token, configValues.value); notice.value = locale.value === 'vi' ? 'Đã lưu cấu hình' : 'Configuration saved'; await refresh(); }
  catch (err) { error.value = err.message || 'Configuration could not be saved'; }
  finally { saving.value = false; }
}
async function createCredential(req) {
  saving.value = true; error.value = '';
  try {
    await applianceApi.saveConfig(token, configValues.value);
    await applianceApi.createCredential(token, { key: req.key, name: credentialName(req.key) || credentialCopy(req).label, value: credentialValue(req.key) });
    updateCredentialDraft(req.key, { value: '' });
    credentialResults.value[req.key] = { status: 'Not tested', message: locale.value === 'vi' ? 'Đã lưu thông tin xác thực.' : 'Credential saved; test the destination when a connection test is available.' };
    notice.value = locale.value === 'vi' ? 'Đã lưu thông tin xác thực' : 'Credential saved';
    await refresh();
  } catch (err) { error.value = err.message || 'Credential could not be saved'; }
  finally { saving.value = false; }
}
async function testCredential(req) {
  testingKey.value = req.key; error.value = '';
  const testedChat = String(configValues.value.chat_id ?? '');
  credentialResults.value[req.key] = { status: 'Testing', message: locale.value === 'vi' ? 'Đang kiểm tra bot và điểm nhận' : 'Checking bot token and chat access', testedChat };
  try {
    await applianceApi.saveConfig(token, configValues.value);
    const result = await applianceApi.testCredential(token, req.key);
    if (String(configValues.value.chat_id ?? '') !== testedChat) return;
    credentialResults.value[req.key] = { status: 'Valid', message: result.message || 'Bot token and chat are valid.', testedChat };
  } catch (err) {
    if (String(configValues.value.chat_id ?? '') !== testedChat) return;
    credentialResults.value[req.key] = { status: 'Invalid', message: err.message || 'Destination test failed', category: err.category, testedChat };
  } finally { testingKey.value = ''; }
}
async function completeSetup() {
  if (!canComplete.value) return;
  saving.value = true; error.value = '';
  try { if (!await saveSchedule(true)) return; await applianceApi.completeSetup(token); notice.value = locale.value === 'vi' ? 'Đã hoàn tất thiết lập' : 'Setup completed'; await refresh(); }
  catch (err) { error.value = err.message || 'Setup could not be completed'; }
  finally { saving.value = false; }
}
async function saveSchedule(quiet = false) {
  if (scheduleError.value) return false;
  scheduleSaving.value = true; error.value = '';
  try {
    const saved = await applianceApi.saveSchedule(token, { expected_revision: schedule.value?.revision || 0, enabled: Boolean(scheduleDraft.value.enabled), local_time: scheduleDraft.value.local_time, timezone: String(scheduleDraft.value.timezone || '').trim() });
    schedule.value = saved; scheduleDraft.value = { enabled: Boolean(saved.enabled), local_time: saved.local_time, timezone: saved.timezone };
    if (!quiet) notice.value = locale.value === 'vi' ? 'Đã lưu lịch' : 'Schedule saved';
    return true;
  } catch (err) {
    error.value = err.message || 'Schedule could not be saved';
    if (err.category === 'revision_conflict') schedule.value = await applianceApi.getSchedule().catch(() => schedule.value);
    return false;
  } finally { scheduleSaving.value = false; }
}
async function reopenSetup() {
  if (!window.confirm(t('returnSetup'))) return;
  saving.value = true;
  try { await applianceApi.reopenSetup(token); notice.value = locale.value === 'vi' ? 'Đã mở lại thiết lập' : 'Setup reopened'; await refresh(); sourceResults.value = {}; credentialResults.value = {}; }
  catch (err) { error.value = err.message || 'Setup could not be reopened'; }
  finally { saving.value = false; }
}
async function runNow() {
  if (running.value || executionRunning.value) return;
  runError.value = ''; running.value = true;
  const prepared = buildRunInput(runUI.value?.input_fields || [], runValues.value, runUI.value?.input_mode || 'direct');
  runErrors.value = prepared.errors;
  if (Object.keys(prepared.errors).length) { running.value = false; return; }
  try { await applianceApi.runNow(token, prepared.input); notice.value = locale.value === 'vi' ? 'Đã bắt đầu chạy quy trình' : 'Report run started'; await refreshRuntime(); startPolling(); }
  catch (err) {
    if (err.category === 'already_running') { notice.value = locale.value === 'vi' ? 'Quy trình đang chạy' : 'A report run is already in progress'; await refreshRuntime().catch(() => {}); startPolling(); }
    else runError.value = err.message || 'Report could not be started';
  } finally { running.value = false; }
}
function stopPolling() { if (pollTimer) clearTimeout(pollTimer); pollTimer = null; pollFailures = 0; }
function schedulePoll(delay) {
  if (pollTimer) return;
  pollTimer = setTimeout(async () => {
    pollTimer = null;
    try { await refreshRuntime(); pollFailures = 0; if (executionRunning.value) schedulePoll(1500); else running.value = false; }
    catch { pollFailures += 1; if (pollFailures <= 4) schedulePoll(Math.min(1500 * (2 ** pollFailures), 6000)); else { error.value = 'Live status is temporarily unavailable. Use Diagnostics refresh before reporting the issue.'; running.value = false; } }
  }, delay);
}
function startPolling() { schedulePoll(0); }
async function loadDiagnostics() { try { diagnostics.value = await applianceApi.getDiagnostics(); } catch (err) { error.value = err.message || 'Diagnostics could not be loaded'; } }
async function copyDiagnostics() { if (!diagnostics.value) await loadDiagnostics(); if (!diagnostics.value) return; await navigator.clipboard.writeText(JSON.stringify(diagnostics.value, null, 2)); notice.value = locale.value === 'vi' ? 'Đã sao chép chẩn đoán' : 'Diagnostics copied'; }
function downloadDiagnostics() {
  const payload = JSON.stringify(diagnostics.value || {}, null, 2); const blob = new Blob([payload], { type: 'application/json' }); const url = URL.createObjectURL(blob); const link = document.createElement('a');
  link.href = url; link.download = `${pack.value.id || 'goflow-pack'}-diagnostics.json`; link.click(); URL.revokeObjectURL(url); notice.value = locale.value === 'vi' ? 'Đã tải chẩn đoán' : 'Diagnostics downloaded';
}

onMounted(() => { if (typeof document !== 'undefined') document.documentElement.lang = locale.value; load(); });
onBeforeUnmount(stopPolling);
</script>

<template>
  <main class="appliance-shell" :style="branding.accent_color ? { '--app-accent': branding.accent_color } : undefined">
    <header class="appliance-header">
      <div>
        <div class="shell-kicker">{{ t('packAppliance') }}</div>
        <h1><span v-if="branding.icon" aria-hidden="true">{{ branding.icon }} </span>{{ packName }}</h1>
        <p>{{ packDescription }}</p>
      </div>
      <div class="appliance-meta" aria-label="Pack identity">
        <label class="locale-control" for="appliance-locale"><span>{{ t('language') }}</span><select id="appliance-locale" :value="locale" class="form-input" @change="setLocale($event.target.value)"><option value="en">English</option><option value="vi">Tiếng Việt</option></select></label>
        <span class="badge badge-muted">Goflow {{ app.version || 'development' }}</span>
        <span class="badge badge-warning">{{ t('unsignedBeta') }}</span>
        <span class="badge badge-muted">{{ pack.version || '0.0.0' }}</span>
        <span class="badge" :class="`badge-status-${String(status?.state || 'loading').toLowerCase()}`">{{ status?.state || 'LOADING' }}</span>
      </div>
    </header>

    <div v-if="loading" class="app-loading" aria-live="polite"><div class="spinner"></div><span>{{ t('loading') }}</span></div>

    <section v-else class="appliance-grid" :class="{ 'appliance-grid-focused': runUI && !needsSetup }">
      <div class="appliance-main">
        <section v-if="needsSetup" class="appliance-panel" aria-labelledby="setup-title">
          <div class="panel-heading"><div><div class="shell-kicker">{{ t('firstRun') }}</div><h2 id="setup-title">{{ t('setup') }}</h2></div><span class="badge badge-muted">{{ t('integrity') }}</span></div>
          <p v-if="setup?.attention_category === 'config'" class="attention-copy" role="status">{{ t('migratedConfig') }}</p>
          <p v-else-if="setup?.attention_category === 'user_review'" class="attention-copy" role="status">{{ t('userReview') }}</p>
          <p v-else-if="setup?.attention_category === 'revalidation'" class="attention-copy" role="status">{{ t('revalidation') }}</p>

          <form class="setup-section" @submit.prevent="saveConfig">
            <h3>{{ t('configuration') }}</h3>
            <div v-if="(setup?.config_schema || []).length === 0" class="muted-copy">{{ t('noConfig') }}</div>
            <div v-for="field in setup?.config_schema || []" :key="field.key" class="form-group">
              <label :for="fieldId(field.key)">{{ fieldCopy(field).label }}</label>
              <select v-if="field.type === 'select'" :id="fieldId(field.key)" :value="configValues[field.key]" class="form-input" :aria-invalid="Boolean(configErrors[field.key])" :aria-describedby="configErrors[field.key] ? `${fieldId(field.key)}-error` : undefined" @change="updateConfigValue(field.key, $event.target.value)">
                <option v-for="option in field.options || []" :key="option" :value="option">{{ optionLabel(locale, field.key, option) }}</option>
              </select>
              <input v-else-if="field.type !== 'boolean'" :id="fieldId(field.key)" :value="configValues[field.key]" class="form-input" type="text" :aria-invalid="Boolean(configErrors[field.key])" :aria-describedby="configErrors[field.key] ? `${fieldId(field.key)}-error` : undefined" @input="updateConfigValue(field.key, $event.target.value)" />
              <label v-else class="toggle-row"><input :checked="Boolean(configValues[field.key])" type="checkbox" @change="updateConfigValue(field.key, $event.target.checked)" /><span>{{ fieldCopy(field).description || fieldCopy(field).label }}</span></label>
              <p v-if="fieldCopy(field).description && field.type !== 'boolean'" class="field-help">{{ fieldCopy(field).description }}</p>
              <p v-if="configErrors[field.key]" :id="`${fieldId(field.key)}-error`" class="field-error">{{ configErrors[field.key] }}</p>
              <div v-if="field.test_kind" class="test-row">
                <button class="btn btn-secondary" type="button" :disabled="saving || sourceResult(field.key).status === 'Testing' || Boolean(configErrors[field.key])" @click="testSource(field)">{{ sourceResult(field.key).status === 'Testing' ? t('testingSource') : t('testSource') }}</button>
                <span class="test-state" :data-state="sourceResult(field.key).status.toLowerCase().replace(' ', '-')" aria-live="polite"><strong>{{ statusText(sourceResult(field.key).status) }}</strong><span v-if="sourceResult(field.key).message">{{ sourceResult(field.key).message }}</span></span>
              </div>
            </div>
            <button class="btn btn-secondary" type="submit" :disabled="saving || Object.keys(configErrors).length > 0">{{ t('saveConfiguration') }}</button>
          </form>

          <section class="setup-section" aria-labelledby="credentials-title">
            <h3 id="credentials-title">{{ t('credentials') }}</h3>
            <div v-if="(setup?.credential_requirements || []).length === 0" class="muted-copy">{{ t('noCredentials') }}</div>
            <article v-for="req in setup?.credential_requirements || []" :key="req.key" class="credential-row">
              <div><strong>{{ credentialCopy(req).label }}</strong><p>{{ credentialCopy(req).description }}</p><span class="badge" :class="req.assigned ? 'badge-success' : 'badge-muted'">{{ req.assigned ? t('assigned') : (req.required ? t('required') : t('optional')) }}</span></div>
              <div class="credential-controls">
                <label>{{ t('name') }}<input class="form-input" :value="credentialName(req.key)" @input="updateCredentialDraft(req.key, { name: $event.target.value })" /></label>
                <label>{{ t('secret') }}<input class="form-input" type="password" autocomplete="new-password" :value="credentialValue(req.key)" @input="updateCredentialDraft(req.key, { value: $event.target.value })" /></label>
                <div class="toolbar-actions">
                  <button class="btn btn-secondary" type="button" :disabled="saving || !credentialValue(req.key) || Object.keys(configErrors).length > 0" @click="createCredential(req)">{{ req.assigned ? t('replace') : t('create') }}</button>
                  <button v-if="req.test_kind" class="btn btn-secondary" type="button" :disabled="testingKey === req.key || !req.assigned || Object.keys(configErrors).length > 0" @click="testCredential(req)">{{ testingKey === req.key ? t('testingTelegram') : t('testTelegram') }}</button>
                </div>
                <div v-if="req.test_kind" class="test-state" :data-state="credentialResult(req.key).status.toLowerCase().replace(' ', '-')" aria-live="polite"><strong>{{ statusText(credentialResult(req.key).status) }}</strong><span v-if="credentialResult(req.key).message">{{ credentialResult(req.key).message }}</span></div>
              </div>
            </article>
          </section>

          <form class="setup-section" aria-labelledby="schedule-title" @submit.prevent="saveSchedule()">
            <div><h3 id="schedule-title">{{ t('scheduleTitle') }}</h3><p class="field-help">{{ t('scheduleHelp') }}</p></div>
            <label class="toggle-row"><input v-model="scheduleDraft.enabled" type="checkbox" /><span>{{ t('enableSchedule') }}</span></label>
            <div class="schedule-fields">
              <label>{{ t('localTime') }}<input v-model="scheduleDraft.local_time" class="form-input" type="time" aria-describedby="schedule-time-help" /></label>
              <label>{{ t('timezone') }}<input v-model="scheduleDraft.timezone" class="form-input" type="text" list="appliance-timezones" autocomplete="off" aria-describedby="schedule-time-help" /><datalist id="appliance-timezones"><option value="Asia/Bangkok" /><option value="Asia/Ho_Chi_Minh" /><option value="Asia/Singapore" /><option value="Europe/London" /><option value="America/New_York" /><option value="UTC" /></datalist></label>
            </div>
            <p id="schedule-time-help" class="field-help">{{ t('timezoneHelp') }}</p><p v-if="scheduleError" class="field-error" role="alert">{{ scheduleError }}</p>
            <button class="btn btn-secondary" type="submit" :disabled="scheduleSaving || Boolean(scheduleError)">{{ scheduleSaving ? t('savingSchedule') : t('saveSchedule') }}</button>
          </form>

          <div class="panel-actions"><button class="btn btn-primary" type="button" :disabled="saving || scheduleSaving || !canComplete || Boolean(scheduleError) || Object.keys(configErrors).length > 0" @click="completeSetup">{{ t('completeSetup') }}</button><span v-if="setup?.missing?.length" class="field-help">{{ setup.missing.join(', ') }}</span><span v-else-if="!canComplete" class="field-help">{{ t('testBeforeComplete') }}</span></div>
        </section>

        <section v-else class="appliance-panel" aria-labelledby="dashboard-title">
          <div class="panel-heading"><div><div class="shell-kicker">{{ t('dashboard') }}</div><h2 id="dashboard-title">{{ packName }}</h2></div><button class="btn btn-secondary" type="button" @click="reopenSetup">{{ t('reconfigure') }}</button></div>
          <div class="status-strip"><div><span>{{ t('server') }}</span><strong>{{ status?.server || 'ok' }}</strong></div><div><span>{{ t('report') }}</span><strong>{{ workflowStatus?.workflow?.name || status?.workflow_id }}</strong></div><div><span>{{ t('state') }}</span><strong>{{ workflowStatus?.state || status?.state }}</strong></div></div>
          <section class="execution-summary" aria-labelledby="schedule-status-title"><h3 id="schedule-status-title">{{ t('dailySchedule') }}</h3><div class="summary-grid schedule-summary"><div><span>{{ t('schedule') }}</span><strong>{{ schedule?.enabled ? t('enabled') : t('disabled') }}</strong></div><div><span>{{ t('timezone') }}</span><strong>{{ schedule?.timezone || 'UTC' }}</strong></div><div><span>{{ t('nextRun') }}</span><strong>{{ schedule?.enabled ? formatDate(schedule?.next_run_at) : t('notScheduled') }}</strong></div><div><span>{{ t('lastScheduledResult') }}</span><strong>{{ schedule?.last_result?.status || (schedule?.last_scheduled_for ? schedule?.state : t('notRunYet')) }}</strong></div><div><span>{{ t('manualRun') }}</span><strong>{{ manualRunState }}</strong></div><div v-if="schedule?.state === 'NEEDS_ATTENTION'"><span>{{ t('scheduleState') }}</span><strong>{{ schedule?.error_category || t('needsAttention') }}</strong></div></div></section>
          <form class="run-panel app-run-form" @submit.prevent="runNow">
            <h3 v-if="runUI">Dữ liệu đầu vào</h3>
            <div v-for="field in runUI?.input_fields || []" :key="field.key" class="form-group">
              <label :for="`run-${field.key}`">{{ field.label }}<span v-if="field.required"> *</span></label>
              <textarea v-if="field.type === 'textarea' || field.type === 'json'" :id="`run-${field.key}`" v-model="runValues[field.key]" class="form-input" rows="4" :placeholder="field.placeholder" />
              <select v-else-if="field.type === 'select'" :id="`run-${field.key}`" v-model="runValues[field.key]" class="form-input"><option v-for="option in field.options || []" :key="String(option)" :value="option">{{ option }}</option></select>
              <label v-else-if="field.type === 'boolean'" class="toggle-row"><input v-model="runValues[field.key]" type="checkbox" /><span>{{ field.description || field.label }}</span></label>
              <input v-else :id="`run-${field.key}`" v-model="runValues[field.key]" class="form-input" :type="['number','integer'].includes(field.type) ? 'number' : 'text'" :step="field.type === 'integer' ? '1' : 'any'" :placeholder="field.placeholder || (field.type === 'number_list' ? '10, 20, 30' : '')" />
              <p v-if="field.description && field.type !== 'boolean'" class="field-help">{{ field.description }}</p><p v-if="runErrors[field.key]" class="field-error">{{ runErrors[field.key] }}</p>
            </div>
            <p v-if="runError" class="field-error">{{ runError }}</p><button class="btn btn-primary" type="submit" :disabled="running || executionRunning || (workflowStatus?.state || status?.state) === 'NEEDS_SETUP'">{{ running || executionRunning ? t('running') : (runUI?.submit_label || t('runNow')) }}</button>
          </form>
          <section class="execution-summary" aria-labelledby="latest-title"><h3 id="latest-title">{{ t('latestExecution') }}</h3><div v-if="latestExecution" class="summary-grid"><div><span>{{ t('status') }}</span><strong>{{ latestExecution.status }}</strong></div><div><span>{{ t('duration') }}</span><strong>{{ formatDuration(latestExecution.duration_ms) }}</strong></div><div><span>{{ t('started') }}</span><strong>{{ formatDate(latestExecution.started_at) }}</strong></div><div v-if="latestExecution.error_message"><span>{{ t('error') }}</span><strong>{{ latestExecution.error_message }}</strong></div><div v-if="latestExecution.error_category"><span>{{ t('category') }}</span><strong>{{ latestExecution.error_category }}</strong></div></div><p v-else class="muted-copy">{{ t('noExecutions') }}</p></section>
          <section v-if="latestExecution && latestOutput !== undefined" class="execution-summary app-output" aria-labelledby="output-title">
            <div class="panel-heading output-heading"><div><div class="shell-kicker">{{ locale === 'vi' ? 'Kết quả mới nhất' : 'Latest result' }}</div><h3 id="output-title">{{ locale === 'vi' ? 'Kết quả' : 'Result' }}</h3></div><button class="btn btn-secondary" type="button" @click="navigator.clipboard.writeText(JSON.stringify(latestOutput, null, 2))">{{ locale === 'vi' ? 'Sao chép JSON' : 'Copy JSON' }}</button></div>
            <div v-if="renderedOutput.mode === 'cards'" class="output-cards"><div v-for="card in renderedOutput.cards" :key="card.key"><span>{{ card.key }}</span><strong>{{ card.value ?? 'null' }}</strong></div></div>
            <div v-else-if="renderedOutput.mode === 'structured'" class="output-structured">
              <article v-for="field in renderedOutput.fields" :key="field.key" class="output-field">
                <span class="output-label">{{ field.key }}</span>
                <strong v-if="field.kind === 'scalar'" class="output-value">{{ field.value ?? 'null' }}</strong>
                <ul v-else-if="field.kind === 'list'" class="output-list"><li v-for="(item, itemIndex) in field.value" :key="itemIndex">{{ typeof item === 'object' ? JSON.stringify(item) : item }}</li></ul>
                <pre v-else class="output-object">{{ JSON.stringify(field.value, null, 2) }}</pre>
              </article>
            </div>
            <div v-else-if="renderedOutput.mode === 'table'" class="table-panel output-table"><div class="table-row output-table-row table-head" :style="{ gridTemplateColumns: `repeat(${renderedOutput.columns.length}, minmax(120px, 1fr))` }"><span v-for="column in renderedOutput.columns" :key="column">{{ column }}</span></div><div v-for="(row, rowIndex) in renderedOutput.rows" :key="rowIndex" class="table-row output-table-row" :style="{ gridTemplateColumns: `repeat(${renderedOutput.columns.length}, minmax(120px, 1fr))` }"><span v-for="column in renderedOutput.columns" :key="column">{{ row[column] }}</span></div></div>
            <pre v-else class="app-output-json">{{ renderedOutput.json }}</pre>
            <details v-if="renderedOutput.detailsJson" class="output-technical"><summary>{{ locale === 'vi' ? 'Chi tiết kỹ thuật' : 'Technical details' }}</summary><pre class="app-output-json">{{ renderedOutput.detailsJson }}</pre></details>
          </section>
          <section class="execution-summary" aria-labelledby="recent-title"><h3 id="recent-title">{{ t('recentExecutions') }}</h3><div class="table-panel"><div class="table-row table-head appliance-execution-row"><span>{{ t('status') }}</span><span>{{ t('started') }}</span><span>{{ t('duration') }}</span><span>{{ t('error') }}</span></div><div v-for="exec in recent" :key="exec.id" class="table-row appliance-execution-row"><span class="badge" :class="`badge-status-${String(exec.status).toLowerCase()}`">{{ exec.status }}</span><span>{{ formatDate(exec.started_at) }}</span><span>{{ formatDuration(exec.duration_ms) }}</span><span>{{ exec.error_message || '' }}</span></div></div></section>
        </section>
      </div>

      <aside class="appliance-side">
        <section class="appliance-panel compact"><h2>{{ t('localDiagnostics') }}</h2><div class="toolbar-actions"><button class="btn btn-secondary" type="button" @click="loadDiagnostics">{{ t('refresh') }}</button><button class="btn btn-secondary" type="button" @click="copyDiagnostics">{{ t('copyDiagnostics') }}</button><button class="btn btn-secondary" type="button" :disabled="!diagnostics" @click="downloadDiagnostics">{{ t('downloadDiagnostics') }}</button></div><pre v-if="diagnostics" class="diagnostics-box">{{ JSON.stringify(diagnostics, null, 2) }}</pre></section>
        <section class="appliance-panel compact"><h2>{{ t('stop') }}</h2><p class="muted-copy">{{ t('stopHelp') }}</p><RouterLink class="advanced-link" :to="`/workflows/${status?.workflow_id || ''}`">{{ t('advanced') }}</RouterLink></section>
      </aside>
    </section>

    <div v-if="notice" class="appliance-toast" role="status">{{ notice }}</div><div v-if="error" class="appliance-error" role="alert">{{ error }}</div>
  </main>
</template>

<style scoped>
.appliance-grid{width:min(1280px,100%);margin:0 auto;align-items:start}
.appliance-grid-focused{grid-template-columns:minmax(0,1fr);width:min(1100px,100%)}
.appliance-grid-focused .appliance-side{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}
.app-run-form{display:grid;gap:12px;padding:20px;border:1px solid #cbd5e1;border-radius:10px;background:#f8fafc}
.app-run-form .btn-primary{min-height:44px;justify-content:center;background:var(--app-accent,#2563eb)}
.app-output{min-width:0;padding:20px;border:1px solid #cbd5e1;border-radius:10px;background:#fff}
.output-heading{margin-bottom:0}
.output-cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}
.output-cards>div{display:grid;gap:6px;padding:16px;border:1px solid #cbd5e1;border-radius:8px;background:#f8fafc}
.output-cards span,.output-label{font-size:12px;color:#64748b;font-weight:800;text-transform:uppercase;letter-spacing:.04em}
.output-cards strong{font-size:20px;overflow-wrap:anywhere}
.output-structured{display:grid;gap:12px}
.output-field{display:grid;gap:8px;padding:16px;border:1px solid #dbe3ef;border-radius:8px;background:#f8fafc}
.output-value{font-size:17px;line-height:1.55;overflow-wrap:anywhere}
.output-list{display:grid;gap:8px;margin:0;padding-left:22px;line-height:1.5}
.output-list li{padding-left:3px;overflow-wrap:anywhere}
.output-object{margin:0;padding:12px;border-radius:6px;background:#eef2f7;color:#0f172a;white-space:pre-wrap;overflow-wrap:anywhere}
.app-output-json{width:100%;min-width:0;margin:0;background:#0f172a;color:#e2e8f0;padding:16px;border-radius:8px;overflow:auto;max-height:360px;white-space:pre-wrap;overflow-wrap:anywhere;word-break:break-word}
.output-technical{margin-top:4px;border-top:1px solid #e2e8f0;padding-top:12px}
.output-technical summary{width:fit-content;cursor:pointer;color:#475569;font-size:13px;font-weight:700}
.output-technical[open] summary{margin-bottom:10px}
.output-table{max-width:100%;overflow:auto}
.output-table-row{min-width:max-content}
@media(max-width:900px){.appliance-grid-focused .appliance-side{grid-template-columns:1fr}.app-output,.app-run-form{padding:14px}}
</style>
