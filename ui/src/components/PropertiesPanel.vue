<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { useWorkflowStore } from '@/stores/workflowStore';
import { useExecutionStore } from '@/stores/executionStore';
import { nodeHelpMap } from './NodeHelpData';
import { api } from '@/services/api';
import {
  buildJsonTree,
  classifyParams,
  credentialsForParam,
  expressionForPath,
  isCompleteExpression,
  orderedUpstreamNodes,
  parseSingleExpression,
  redactValue,
  resolveExpression,
  rowsForTable,
  safeJSONStringify,
  validateParamValue,
} from '@/utils/inspector';

const props = defineProps({
  selectedNode: Object,
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] },
  validationIssues: { type: Array, default: () => [] },
  selectedExecution: { type: Object, default: null },
  executionContextMode: { type: String, default: 'latest' },
  activeLiveExecutionId: { type: String, default: '' },
});

const emit = defineEmits(['updateNodeParams', 'deleteNode', 'close']);

const workflowStore = useWorkflowStore();
const executionStore = useExecutionStore();
const router = useRouter();

const tabs = ['parameters', 'input', 'output', 'logs'];
const activeTab = ref('parameters');
const activeOutputView = ref('json');
const activeInputView = ref('tree');
const showAdvanced = ref(false);
const showHelp = ref(false);
const mappingField = ref('');
const paramModes = ref({});
const selectedSourceId = ref('');
const treeSearch = ref('');
const conversionNotice = ref({});
const aiHelperPrompt = ref('');
const aiHelperLoading = ref(false);
const aiHelperError = ref(null);
const copiedMessage = ref('');

function isAICredential(cred) {
  const type = String(cred.type || '').toLowerCase();
  return type === 'openai' || type === 'deepseek' || type === 'api_key';
}

const aiCredentialId = computed(() => {
  const cred = workflowStore.credentials.find(isAICredential);
  return cred ? cred.id : null;
});

const nodeDef = computed(() => {
  if (!props.selectedNode) return null;
  return workflowStore.nodeDefinitions.find((d) => d.type === props.selectedNode.type);
});

const helpData = computed(() => {
  if (!props.selectedNode) return null;
  return nodeHelpMap[props.selectedNode.type] || null;
});

const latestExecution = computed(() => props.selectedExecution || executionStore.executionLogs[0] || null);

const executionLogs = computed(() => {
  if (!latestExecution.value) return [];
  if (Array.isArray(latestExecution.value.node_logs)) return latestExecution.value.node_logs;
  try {
    return JSON.parse(latestExecution.value.logs_json || '[]');
  } catch {
    return [];
  }
});

const nodeExecutionResult = computed(() => {
  if (!props.selectedNode) return null;
  if (props.executionContextMode === 'live' && props.activeLiveExecutionId) {
    const workflowId = workflowStore.currentWorkflow?.id || '';
    const realtimeEvent = executionStore.nodeEventsByWorkflow[workflowId]?.[props.activeLiveExecutionId]?.[props.selectedNode.id];
    if (realtimeEvent) return realtimeEvent;
  }
  return executionLogs.value.find((log) => log.node_id === props.selectedNode.id) || null;
});

const selectedNodeStatus = computed(() => {
  if (!props.selectedNode) return null;
  return nodeExecutionResult.value?.status || null;
});

const selectedNodeError = computed(() => redactValue(nodeExecutionResult.value?.error || ''));
const inspectorContextLabel = computed(() => {
  if (props.executionContextMode === 'live' && props.activeLiveExecutionId) return `Live execution ${props.activeLiveExecutionId}`;
  if (props.executionContextMode === 'selected' && latestExecution.value?.id) return `Selected execution ${latestExecution.value.id}`;
  if (latestExecution.value?.id) return `Latest execution ${latestExecution.value.id}`;
  return 'No execution selected';
});

const sourceNodes = computed(() => {
  const logByNode = new Map(executionLogs.value.map((log) => [log.node_id, log]));
  const sources = [];
  orderedUpstreamNodes(props.selectedNode?.id, props.nodes, props.edges).forEach((node) => {
    const nodeId = node.id;
    const log = logByNode.get(nodeId);
    if (log?.output !== undefined) {
      sources.push({
        id: nodeId,
        name: node?.data?.name || node?.label || nodeId,
        data: log.output,
        status: log.status,
        kind: props.edges.some((edge) => edge.source === nodeId && edge.target === props.selectedNode?.id) ? 'direct' : 'previous',
      });
    }
  });
  if (latestExecution.value?.input !== undefined) {
    sources.push({ id: '$trigger', name: 'Trigger payload', data: latestExecution.value.input, kind: 'trigger' });
  } else if (latestExecution.value?.input_json) {
    try {
      sources.push({ id: '$trigger', name: 'Trigger payload', data: JSON.parse(latestExecution.value.input_json), kind: 'trigger' });
    } catch {
      sources.push({ id: '$trigger', name: 'Trigger payload', data: {}, kind: 'trigger' });
    }
  }
  return sources.map((source) => ({ ...source, data: redactValue(source.data) }));
});

const activeSource = computed(() => sourceNodes.value.find((source) => source.id === selectedSourceId.value) || sourceNodes.value[0] || null);
const activeSourceData = computed(() => activeSource.value?.data || null);
const sourceTreeRows = computed(() => {
  const rows = buildJsonTree(activeSourceData.value || {});
  const query = treeSearch.value.trim().toLowerCase();
  if (!query) return rows;
  return rows.filter((row) => row.path.toLowerCase().includes(query) || String(row.value ?? '').toLowerCase().includes(query));
});
const sourceTable = computed(() => rowsForTable(activeSourceData.value));

const outputData = computed(() => redactValue(nodeExecutionResult.value?.output ?? null));
const outputTreeRows = computed(() => buildJsonTree(outputData.value || {}));
const outputTable = computed(() => rowsForTable(outputData.value));
const rawInputJSON = computed(() => safeJSONStringify(activeSourceData.value));
const rawOutputJSON = computed(() => safeJSONStringify(outputData.value));
const logsJSON = computed(() => safeJSONStringify(redactValue(nodeExecutionResult.value || {})));

const paramGroups = computed(() => classifyParams(nodeDef.value?.params || []));
const fieldErrors = computed(() => {
  const errors = {};
  (nodeDef.value?.params || []).forEach((param) => {
    const value = props.selectedNode?.params?.[param.name] ?? param.default ?? '';
    const message = validateParamValue(param, value, workflowStore.credentials);
    if (message) errors[param.name] = message;
  });
  props.validationIssues.forEach((issue) => {
    const paramName = issue.param || issue.paramName || issue.field;
    if (paramName && !errors[paramName]) errors[paramName] = issue.message;
  });
  return errors;
});

const expressionPreview = computed(() => {
  if (!mappingField.value || !props.selectedNode) return null;
  const value = props.selectedNode.params?.[mappingField.value] ?? '';
  return resolveExpression(value, sourceNodes.value);
});

function syncParamModes() {
  const next = {};
  (nodeDef.value?.params || []).forEach((param) => {
    if (supportsExpression(param)) {
      next[param.name] = isCompleteExpression(currentValue(param)) ? 'expression' : 'fixed';
    }
  });
  paramModes.value = next;
}

watch(
  () => props.selectedNode?.id,
  async () => {
    const hasValidationIssue = props.validationIssues.some((issue) => issue.type === 'missing_credential' || issue.type === 'missing_required_param' || issue.type === 'missing_required');
    activeTab.value = selectedNodeStatus.value === 'FAILED' ? 'logs' : hasValidationIssue ? 'parameters' : (tabs.includes(activeTab.value) ? activeTab.value : 'parameters');
    showHelp.value = false;
    aiHelperError.value = null;
    mappingField.value = '';
    syncParamModes();
    await nextTick();
    selectedSourceId.value = sourceNodes.value[0]?.id || '';
  }
);

watch(
  () => nodeDef.value?.type,
  () => syncParamModes()
);

watch(selectedNodeStatus, (status) => {
  if (status === 'FAILED') activeTab.value = 'logs';
}, { immediate: true });

watch(sourceNodes, () => {
  if (!sourceNodes.value.some((source) => source.id === selectedSourceId.value)) {
    selectedSourceId.value = sourceNodes.value[0]?.id || '';
  }
});

watch(
  () => workflowStore.currentWorkflow?.id,
  (newId) => {
    if (newId) executionStore.fetchExecutionHistory(newId);
  },
  { immediate: true }
);

function currentValue(param) {
  return props.selectedNode?.params?.[param.name] ?? param.default ?? '';
}

function supportsExpression(param) {
  return ['text', 'textarea', 'json', 'url', 'number', 'integer'].includes(param.type);
}

function isExpressionValue(param) {
  return Boolean(supportsExpression(param) && paramMode(param) === 'expression');
}

function paramMode(param) {
  if (parseSingleExpression(currentValue(param))) return 'expression';
  return paramModes.value[param.name] || 'fixed';
}

function expressionModeHint(param) {
  if (!supportsExpression(param) || paramMode(param) !== 'expression') return '';
  const value = currentValue(param);
  if (!String(value).trim()) return 'Choose a source value to create an expression.';
  if (!parseSingleExpression(value)) return 'Expression mode is active. Pick a source value to replace the current literal.';
  return '';
}

function inputTypeForParam(param) {
  if ((param.type === 'number' || param.type === 'integer') && paramMode(param) === 'fixed') return 'number';
  return 'text';
}

function conversionNoticeFor(param) {
  return conversionNotice.value[param.name] || '';
}

function clearConversionNotice(paramName) {
  if (!conversionNotice.value[paramName]) return;
  const next = { ...conversionNotice.value };
  delete next[paramName];
  conversionNotice.value = next;
}

function keepExpressionMode(param, message) {
  paramModes.value = { ...paramModes.value, [param.name]: 'expression' };
  mappingField.value = param.name;
  conversionNotice.value = { ...conversionNotice.value, [param.name]: message };
}

function canConvertJSONLiteral(param) {
  if (param.type !== 'json') return false;
  const value = currentValue(param);
  if (!parseSingleExpression(value)) return false;
  const preview = resolveExpression(value, sourceNodes.value);
  return Boolean(preview.ok && preview.value !== null && typeof preview.value === 'object');
}

function convertExpressionToJSONLiteral(param) {
  const value = currentValue(param);
  const preview = resolveExpression(value, sourceNodes.value);
  if (!preview.ok || preview.value === null || typeof preview.value !== 'object') {
    keepExpressionMode(param, 'This expression cannot be converted because the preview is unavailable.');
    return;
  }
  handleParamChange(param.name, JSON.stringify(preview.value, null, 2));
  paramModes.value = { ...paramModes.value, [param.name]: 'fixed' };
  mappingField.value = mappingField.value === param.name ? '' : mappingField.value;
  clearConversionNotice(param.name);
}

function switchParamMode(param, mode) {
  const value = currentValue(param);
  clearConversionNotice(param.name);
  if (mode === 'expression') {
    paramModes.value = { ...paramModes.value, [param.name]: 'expression' };
    mappingField.value = param.name;
    return;
  }
  if (parseSingleExpression(value)) {
    const preview = resolveExpression(value, sourceNodes.value);
    if (preview.ok && (preview.value == null || typeof preview.value !== 'object')) {
      handleParamChange(param.name, preview.value == null ? '' : String(preview.value));
      paramModes.value = { ...paramModes.value, [param.name]: 'fixed' };
      mappingField.value = '';
      return;
    }
    if (preview.ok && typeof preview.value === 'object') {
      keepExpressionMode(
        param,
        param.type === 'json'
          ? 'Object and array previews stay in Expression mode unless you convert them to a JSON literal.'
          : 'Object and array previews cannot become a fixed primitive value. Expression mode was kept.'
      );
      return;
    }
    keepExpressionMode(param, 'Preview is unavailable, so this field stayed in Expression mode.');
    return;
  }
  paramModes.value = { ...paramModes.value, [param.name]: 'fixed' };
  mappingField.value = mappingField.value === param.name ? '' : mappingField.value;
}

function handleParamChange(paramName, value) {
  if (!props.selectedNode) return;
  clearConversionNotice(paramName);
  const updatedParams = { ...props.selectedNode.params, [paramName]: value };
  emit('updateNodeParams', props.selectedNode.id, updatedParams);
}

function handleNameChange(newName) {
  if (!props.selectedNode) return;
  emit('updateNodeParams', props.selectedNode.id, props.selectedNode.params, newName);
}

function handleDeleteNode() {
  if (props.selectedNode) emit('deleteNode', props.selectedNode.id);
}

function insertExpression(row) {
  if (!mappingField.value || !activeSource.value || row.truncated) return;
  handleParamChange(mappingField.value, expressionForPath(activeSource.value.id, row.path));
  paramModes.value = { ...paramModes.value, [mappingField.value]: 'expression' };
  clearConversionNotice(mappingField.value);
}

function tabId(tab) {
  return `inspector-tab-${tab}`;
}

function panelId(tab) {
  return `inspector-panel-${tab}`;
}

async function focusActiveTab() {
  await nextTick();
  document.getElementById(tabId(activeTab.value))?.focus();
}

function handleTabKeydown(event) {
  const index = tabs.indexOf(activeTab.value);
  if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
    event.preventDefault();
    const delta = event.key === 'ArrowRight' ? 1 : -1;
    activeTab.value = tabs[(index + delta + tabs.length) % tabs.length];
    focusActiveTab();
  } else if (event.key === 'Home') {
    event.preventDefault();
    activeTab.value = tabs[0];
    focusActiveTab();
  } else if (event.key === 'End') {
    event.preventDefault();
    activeTab.value = tabs[tabs.length - 1];
    focusActiveTab();
  }
}

async function copyText(text, label = 'Copied') {
  copiedMessage.value = '';
  try {
    await navigator.clipboard.writeText(String(text ?? ''));
    copiedMessage.value = label;
  } catch {
    copiedMessage.value = 'Copy failed';
  }
}

function createCredential() {
  router.push('/credentials');
}

async function formatJSON(param) {
  const value = currentValue(param);
  try {
    const formatted = JSON.stringify(JSON.parse(value), null, 2);
    handleParamChange(param.name, formatted);
  } catch {
    // Inline validation already reports invalid JSON.
  }
}

function downloadOutput() {
  const blob = new Blob([rawOutputJSON.value.text], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${props.selectedNode?.id || 'node'}-output.json`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

async function runAIHelper() {
  if (!props.selectedNode || !aiHelperPrompt.value.trim() || aiHelperLoading.value) return;
  if (!aiCredentialId.value) {
    aiHelperError.value = 'No AI API key found. Add one in Credentials first.';
    return;
  }
  aiHelperLoading.value = true;
  aiHelperError.value = null;
  try {
    const updatedParams = await api.configureNodeParams(
      props.selectedNode.type,
      aiHelperPrompt.value,
      props.selectedNode.params || {},
      aiCredentialId.value
    );
    emit('updateNodeParams', props.selectedNode.id, updatedParams);
    aiHelperPrompt.value = '';
  } catch (err) {
    aiHelperError.value = `Failed: ${err.message}`;
  } finally {
    aiHelperLoading.value = false;
  }
}
</script>

<template>
  <aside v-if="selectedNode" class="properties-panel glass-panel" aria-label="Node inspector" data-testid="inspector-ready">
    <div class="panel-header">
      <div class="header-left">
        <span class="node-type-badge">{{ nodeDef?.name || selectedNode.type }}</span>
        <span class="node-id">#{{ selectedNode.id.substring(0, 8) }}</span>
      </div>
      <button class="btn-close" type="button" aria-label="Close node inspector" @click="emit('close')">x</button>
    </div>

    <div class="execution-context" :title="inspectorContextLabel">
      <strong>{{ selectedNodeStatus || 'Not run' }}</strong>
      <span>{{ inspectorContextLabel }}</span>
    </div>

    <div class="panel-tabs" role="tablist" aria-label="Inspector tabs" @keydown="handleTabKeydown">
      <button
        v-for="tab in tabs"
        :id="tabId(tab)"
        :key="tab"
        type="button"
        role="tab"
        class="tab-btn"
        :class="{ active: activeTab === tab }"
        :aria-selected="activeTab === tab"
        :aria-controls="panelId(tab)"
        :tabindex="activeTab === tab ? 0 : -1"
        @click="activeTab = tab"
      >
        {{ tab[0].toUpperCase() + tab.slice(1) }}
      </button>
    </div>

    <div class="panel-body">
      <div v-if="selectedNodeStatus === 'FAILED'" class="node-error-summary" role="alert">
        <div class="node-error-title">Node failed</div>
        <pre class="node-error-message">{{ selectedNodeError || 'No error details were reported for this node.' }}</pre>
      </div>

      <section v-if="activeTab === 'parameters'" :id="panelId('parameters')" class="inspector-tab" role="tabpanel" :aria-labelledby="tabId('parameters')">
        <div class="form-group">
          <label for="node-name">Node Name</label>
          <input
            id="node-name"
            type="text"
            :value="selectedNode.name || nodeDef?.name"
            class="form-input"
            aria-label="Node Name"
            @input="handleNameChange($event.target.value)"
          />
        </div>

        <template v-if="nodeDef?.params?.length">
          <section v-for="groupName in ['credential', 'resource', 'operation', 'required', 'common']" :key="groupName" class="param-group" v-show="paramGroups[groupName].length">
            <h4 class="section-title">{{ groupName === 'required' ? 'Required parameters' : groupName === 'common' ? 'Common parameters' : groupName }}</h4>
            <div v-for="param in paramGroups[groupName]" :key="param.name" class="form-group" :class="{ invalid: fieldErrors[param.name] }">
              <div class="field-heading">
                <label :for="`param-${param.name}`">{{ param.label || param.name }} <span v-if="param.required" class="req">*</span></label>
                <div v-if="supportsExpression(param)" class="mode-toggle" aria-label="Field value mode" :data-param-name="param.name">
                  <button type="button" :class="{ active: !isExpressionValue(param) }" @click="switchParamMode(param, 'fixed')">Fixed</button>
                  <button type="button" :class="{ active: isExpressionValue(param) }" @click="switchParamMode(param, 'expression')">Expression</button>
                </div>
              </div>
              <span v-if="param.description" class="param-desc">{{ param.description }}</span>

              <select
                v-if="param.type === 'credential'"
                :id="`param-${param.name}`"
                :value="currentValue(param)"
                class="form-select"
                :aria-label="param.label || param.name"
                @change="handleParamChange(param.name, $event.target.value)"
              >
                <option value="">-- Select Credential Secret --</option>
                <option v-for="cred in credentialsForParam(workflowStore.credentials, param, selectedNode.type)" :key="cred.id" :value="cred.id">
                  {{ cred.name }} ({{ cred.type }})
                </option>
              </select>
              <input
                v-else-if="param.type === 'password'"
                :id="`param-${param.name}`"
                type="password"
                autocomplete="new-password"
                :value="currentValue(param)"
                class="form-input"
                :aria-label="param.label || param.name"
                @input="handleParamChange(param.name, $event.target.value)"
              />
              <select
                v-else-if="param.type === 'select'"
                :id="`param-${param.name}`"
                :value="currentValue(param)"
                class="form-select"
                :aria-label="param.label || param.name"
                @change="handleParamChange(param.name, $event.target.value)"
              >
                <option v-for="opt in param.options" :key="opt" :value="opt">{{ opt }}</option>
              </select>
              <textarea
                v-else-if="param.type === 'textarea' || param.type === 'json'"
                :id="`param-${param.name}`"
                :value="currentValue(param)"
                class="form-textarea code-field"
                rows="5"
                :aria-label="param.label || param.name"
                spellcheck="false"
                @input="handleParamChange(param.name, $event.target.value)"
              ></textarea>
              <input
                v-else
                :id="`param-${param.name}`"
                :type="inputTypeForParam(param)"
                :value="currentValue(param)"
                class="form-input"
                :aria-label="param.label || param.name"
                @input="handleParamChange(param.name, $event.target.value)"
              />

              <div class="field-actions">
                <button v-if="param.type === 'credential' && fieldErrors[param.name]" type="button" class="mini-btn" @click="createCredential">Create credential</button>
                <button v-if="param.type === 'json'" type="button" class="mini-btn" @click="formatJSON(param)">Format JSON</button>
                <button v-if="supportsExpression(param)" type="button" class="mini-btn" @click="mappingField = param.name">Pick data</button>
              </div>
              <p v-if="fieldErrors[param.name]" class="field-error" role="alert">{{ fieldErrors[param.name] }}</p>
              <div v-if="mappingField === param.name" class="expression-preview">
                <strong>Expression preview</strong>
                <code>{{ currentValue(param) || 'No expression selected' }}</code>
                <p>Source: {{ expressionPreview?.sourceId || 'None' }} | {{ inspectorContextLabel }}</p>
                <pre v-if="expressionPreview?.ok">{{ safeJSONStringify(expressionPreview.value).text }}</pre>
                <p v-else class="field-error">{{ expressionPreview?.error || 'Choose a data path below.' }}</p>
                <button v-if="canConvertJSONLiteral(param)" type="button" class="mini-btn" @click="convertExpressionToJSONLiteral(param)">Convert to JSON literal</button>
              </div>
              <p v-if="expressionModeHint(param)" class="param-desc">{{ expressionModeHint(param) }}</p>
              <p v-if="conversionNoticeFor(param)" class="param-desc">{{ conversionNoticeFor(param) }}</p>
            </div>
          </section>

          <section v-if="paramGroups.advanced.length" class="param-group">
            <button type="button" class="advanced-toggle" @click="showAdvanced = !showAdvanced">
              {{ showAdvanced ? 'Hide advanced options' : 'Show advanced options' }}
            </button>
            <div v-if="showAdvanced">
              <div v-for="param in paramGroups.advanced" :key="param.name" class="form-group" :class="{ invalid: fieldErrors[param.name] }">
                <div class="field-heading">
                  <label :for="`param-${param.name}`">{{ param.label || param.name }}</label>
                  <div v-if="supportsExpression(param)" class="mode-toggle" aria-label="Field value mode" :data-param-name="param.name">
                    <button type="button" :class="{ active: !isExpressionValue(param) }" @click="switchParamMode(param, 'fixed')">Fixed</button>
                    <button type="button" :class="{ active: isExpressionValue(param) }" @click="switchParamMode(param, 'expression')">Expression</button>
                  </div>
                </div>
                <textarea
                  v-if="param.type === 'textarea' || param.type === 'json'"
                  :id="`param-${param.name}`"
                  :value="currentValue(param)"
                  class="form-textarea code-field"
                  rows="4"
                  :aria-label="param.label || param.name"
                  @input="handleParamChange(param.name, $event.target.value)"
                ></textarea>
                <input
                  v-else
                  :id="`param-${param.name}`"
                  :type="inputTypeForParam(param)"
                  :value="currentValue(param)"
                  class="form-input"
                  :aria-label="param.label || param.name"
                  @input="handleParamChange(param.name, $event.target.value)"
                />
                <div class="field-actions">
                  <button v-if="param.type === 'json'" type="button" class="mini-btn" @click="formatJSON(param)">Format JSON</button>
                  <button v-if="supportsExpression(param)" type="button" class="mini-btn" @click="mappingField = param.name">Pick data</button>
                </div>
                <p v-if="fieldErrors[param.name]" class="field-error" role="alert">{{ fieldErrors[param.name] }}</p>
                <div v-if="mappingField === param.name" class="expression-preview">
                  <strong>Expression preview</strong>
                  <code>{{ currentValue(param) || 'No expression selected' }}</code>
                  <p>Source: {{ expressionPreview?.sourceId || 'None' }} | {{ inspectorContextLabel }}</p>
                  <pre v-if="expressionPreview?.ok">{{ safeJSONStringify(expressionPreview.value).text }}</pre>
                  <p v-else class="field-error">{{ expressionPreview?.error || 'Choose a data path below.' }}</p>
                  <button v-if="canConvertJSONLiteral(param)" type="button" class="mini-btn" @click="convertExpressionToJSONLiteral(param)">Convert to JSON literal</button>
                </div>
                <p v-if="expressionModeHint(param)" class="param-desc">{{ expressionModeHint(param) }}</p>
                <p v-if="conversionNoticeFor(param)" class="param-desc">{{ conversionNoticeFor(param) }}</p>
              </div>
            </div>
          </section>
        </template>

        <div v-else class="empty-state">No configurable parameters for this node.</div>

        <section v-if="sourceNodes.length" class="data-picker" aria-label="Data picker">
          <div class="picker-header">
            <strong>Data picker</strong>
            <select v-model="selectedSourceId" class="form-select compact" aria-label="Source node selector">
              <option v-for="source in sourceNodes" :key="source.id" :value="source.id">{{ source.kind === 'direct' ? 'Direct input' : source.kind === 'previous' ? 'Previous node' : 'Trigger payload' }} - {{ source.name }}</option>
            </select>
          </div>
          <input v-model="treeSearch" class="form-input" type="search" aria-label="Search input data" placeholder="Search paths or values..." />
          <div class="json-tree">
            <button v-for="row in sourceTreeRows.slice(0, 120)" :key="`${row.path}-${row.key}`" type="button" class="tree-row" :style="{ paddingLeft: `${8 + (row.depth || 0) * 14}px` }" @click="insertExpression(row)">
              <span class="tree-path">{{ row.path || row.key }}</span>
              <span class="tree-type">{{ row.type }}</span>
              <code v-if="row.leaf">{{ String(row.value).slice(0, 80) }}</code>
            </button>
          </div>
          <p v-if="mappingField" class="mapping-hint">Click a value to insert an expression into {{ mappingField }}.</p>
        </section>

        <section class="ai-node-configurer">
          <label class="ai-configurer-title">AI Quick Config</label>
          <p class="ai-configurer-desc">Ask AI to fill this node's parameters.</p>
          <div class="ai-configurer-input-row">
            <input v-model="aiHelperPrompt" type="text" placeholder="Describe how to configure this node..." class="form-input ai-configurer-input" :disabled="aiHelperLoading" @keyup.enter="runAIHelper" />
            <button class="btn btn-primary ai-configurer-btn" type="button" :disabled="aiHelperLoading || !aiHelperPrompt.trim()" @click="runAIHelper">{{ aiHelperLoading ? '...' : 'AI' }}</button>
          </div>
          <p v-if="aiHelperError" class="field-error">{{ aiHelperError }}</p>
        </section>

        <section v-if="helpData" class="help-section">
          <button class="btn btn-secondary btn-full" type="button" @click="showHelp = !showHelp">{{ showHelp ? 'Hide Node Guide' : 'Show Node Guide' }}</button>
          <div v-if="showHelp" class="help-content-box">
            <h5>{{ helpData.title }}</h5>
            <p>{{ helpData.desc }}</p>
            <pre>{{ helpData.inputs }}</pre>
            <pre><code>{{ helpData.output }}</code></pre>
          </div>
        </section>

        <button class="btn btn-danger btn-full" type="button" @click="handleDeleteNode">Delete Node</button>
      </section>

      <section v-if="activeTab === 'input'" :id="panelId('input')" class="inspector-tab" role="tabpanel" :aria-labelledby="tabId('input')">
        <div class="view-toolbar">
          <strong>{{ activeSource?.name || 'No input data' }}</strong>
          <select v-model="selectedSourceId" class="form-select compact" aria-label="Source node selector">
            <option v-for="source in sourceNodes" :key="source.id" :value="source.id">{{ source.kind === 'direct' ? 'Direct input' : source.kind === 'previous' ? 'Previous node' : 'Trigger payload' }} - {{ source.name }}</option>
          </select>
        </div>
        <div v-if="!sourceNodes.length" class="empty-state">
          No input data is available. Run this workflow first, or connect an upstream node that produces output.
        </div>
        <template v-else>
          <div class="sub-tabs">
            <button type="button" :class="{ active: activeInputView === 'tree' }" @click="activeInputView = 'tree'">JSON tree</button>
            <button type="button" :class="{ active: activeInputView === 'table' }" @click="activeInputView = 'table'">Table</button>
            <button type="button" :class="{ active: activeInputView === 'raw' }" @click="activeInputView = 'raw'">Raw JSON</button>
          </div>
          <input v-model="treeSearch" class="form-input" type="search" aria-label="Search input data" placeholder="Search input..." />
          <div v-if="activeInputView === 'tree'" class="json-tree">
            <div v-for="row in sourceTreeRows.slice(0, 160)" :key="row.path" class="tree-row static" :style="{ paddingLeft: `${8 + (row.depth || 0) * 14}px` }">
              <span class="tree-path">{{ row.path || row.key }}</span>
              <button type="button" class="mini-btn" @click="copyText(row.path, 'Path copied')">Copy path</button>
              <button type="button" class="mini-btn" @click="copyText(row.value, 'Value copied')">Copy value</button>
              <code v-if="row.leaf">{{ String(row.value).slice(0, 120) }}</code>
            </div>
          </div>
          <table v-else-if="activeInputView === 'table' && sourceTable.columns.length" class="data-table">
            <thead><tr><th v-for="column in sourceTable.columns" :key="column">{{ column }}</th></tr></thead>
            <tbody><tr v-for="(row, index) in sourceTable.rows" :key="index"><td v-for="column in sourceTable.columns" :key="column">{{ row[column] }}</td></tr></tbody>
          </table>
          <div v-else-if="activeInputView === 'table'" class="empty-state">Table view is available for arrays of objects.</div>
          <pre v-else class="json-code"><code>{{ rawInputJSON.text }}</code></pre>
        </template>
      </section>

      <section v-if="activeTab === 'output'" :id="panelId('output')" class="inspector-tab" role="tabpanel" :aria-labelledby="tabId('output')">
        <div v-if="!nodeExecutionResult" class="empty-state">This node has not produced output in the selected execution.</div>
        <template v-else>
          <div class="run-meta">
            <span>Status: {{ nodeExecutionResult.status }}</span>
            <span>Duration: {{ nodeExecutionResult.duration_ms || 0 }}ms</span>
            <span>Attempts: {{ nodeExecutionResult.attempts || 1 }}</span>
          </div>
          <div class="sub-tabs">
            <button type="button" :class="{ active: activeOutputView === 'json' }" @click="activeOutputView = 'json'">JSON</button>
            <button type="button" :class="{ active: activeOutputView === 'table' }" @click="activeOutputView = 'table'">Table</button>
            <button type="button" :class="{ active: activeOutputView === 'raw' }" @click="activeOutputView = 'raw'">Raw</button>
            <button type="button" @click="copyText(rawOutputJSON.text, 'Output copied')">Copy output</button>
            <button type="button" @click="downloadOutput">Download JSON</button>
          </div>
          <div v-if="activeOutputView === 'json'" class="json-tree">
            <div v-for="row in outputTreeRows.slice(0, 160)" :key="row.path" class="tree-row static" :style="{ paddingLeft: `${8 + (row.depth || 0) * 14}px` }">
              <span class="tree-path">{{ row.path || row.key }}</span>
              <button type="button" class="mini-btn" @click="copyText(row.path, 'Path copied')">Copy path</button>
              <button type="button" class="mini-btn" @click="copyText(row.value, 'Value copied')">Copy value</button>
              <code v-if="row.leaf">{{ String(row.value).slice(0, 120) }}</code>
            </div>
          </div>
          <table v-else-if="activeOutputView === 'table' && outputTable.columns.length" class="data-table">
            <thead><tr><th v-for="column in outputTable.columns" :key="column">{{ column }}</th></tr></thead>
            <tbody><tr v-for="(row, index) in outputTable.rows" :key="index"><td v-for="column in outputTable.columns" :key="column">{{ row[column] }}</td></tr></tbody>
          </table>
          <div v-else-if="activeOutputView === 'table'" class="empty-state">Table view is available for arrays of objects.</div>
          <pre v-else class="json-code"><code>{{ rawOutputJSON.text }}</code></pre>
        </template>
      </section>

      <section v-if="activeTab === 'logs'" :id="panelId('logs')" class="inspector-tab" role="tabpanel" :aria-labelledby="tabId('logs')">
        <div v-if="!nodeExecutionResult" class="empty-state">No logs are available for this node in the selected execution.</div>
        <template v-else>
          <dl class="log-grid">
            <dt>Execution ID</dt><dd>{{ nodeExecutionResult.execution_id || latestExecution?.id || 'Latest execution' }}</dd>
            <dt>Status</dt><dd>{{ nodeExecutionResult.status }}</dd>
            <dt>Duration</dt><dd>{{ nodeExecutionResult.duration_ms || 0 }}ms</dd>
            <dt>Attempts</dt><dd>{{ nodeExecutionResult.attempts || 1 }}</dd>
            <dt>Started</dt><dd>{{ latestExecution?.started_at || nodeExecutionResult.timestamp || 'Unknown' }}</dd>
            <dt>Finished</dt><dd>{{ latestExecution?.finished_at || 'Unknown' }}</dd>
          </dl>
          <div v-if="nodeExecutionResult.error" class="node-error-summary">
            <div class="node-error-title">Error</div>
            <pre class="node-error-message">{{ redactValue(nodeExecutionResult.error) }}</pre>
            <div class="field-actions">
              <button type="button" class="mini-btn" @click="activeTab = 'parameters'">Open Parameters</button>
              <button type="button" class="mini-btn" @click="copyText(redactValue(nodeExecutionResult.error), 'Error copied')">Copy error</button>
            </div>
          </div>
          <p class="param-desc">Resolved parameters are shown only when the backend records redacted resolved parameters for this node.</p>
          <pre class="json-code"><code>{{ logsJSON.text }}</code></pre>
        </template>
      </section>

      <p v-if="copiedMessage" class="copy-status" role="status">{{ copiedMessage }}</p>
    </div>
  </aside>
</template>

<style scoped>
.properties-panel {
  width: min(440px, 34vw);
  min-width: 360px;
  height: calc(100% - var(--workflow-topbar-height));
  position: absolute;
  right: 0;
  top: var(--workflow-topbar-height);
  border-radius: 0;
  border-left: 1px solid var(--border-color);
  background: var(--bg-secondary);
  z-index: 50;
  display: flex;
  flex-direction: column;
  box-shadow: -10px 0 30px rgba(15, 23, 42, 0.14);
}

.panel-header,
.execution-context,
.panel-tabs,
.view-toolbar,
.picker-header,
.run-meta,
.sub-tabs,
.field-heading,
.field-actions,
.ai-configurer-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-header {
  padding: 12px 14px;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-tertiary);
}

.header-left {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.node-type-badge,
.node-id {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-type-badge {
  font-size: 0.78rem;
  font-weight: 800;
  color: #1e40af;
}

.node-id {
  font-size: 0.72rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.btn-close {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 1.05rem;
  padding: 4px 7px;
}

.execution-context {
  justify-content: space-between;
  padding: 8px 14px;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-color);
  font-size: 0.76rem;
}

.execution-context strong {
  color: var(--text-primary);
}

.panel-tabs {
  border-bottom: 1px solid var(--border-color);
  background: #f8fafc;
}

.tab-btn,
.sub-tabs button,
.mode-toggle button {
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: #64748b;
  cursor: pointer;
  font: inherit;
  font-size: 0.78rem;
  font-weight: 750;
  padding: 9px 8px;
}

.tab-btn {
  flex: 1;
}

.tab-btn.active,
.sub-tabs button.active,
.mode-toggle button.active {
  background: #fff;
  color: #2563eb;
  border-bottom-color: #2563eb;
}

.panel-body {
  padding: 14px;
  overflow-y: auto;
  flex: 1;
}

.inspector-tab {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.param-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-title {
  margin: 2px 0 0;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-size: 0.75rem;
}

.form-group.invalid .form-input,
.form-group.invalid .form-select,
.form-group.invalid .form-textarea {
  border-color: var(--color-danger);
}

.field-heading {
  justify-content: space-between;
  align-items: flex-start;
}

.mode-toggle {
  display: inline-flex;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.mode-toggle button {
  border: 0;
  padding: 4px 7px;
}

.req,
.field-error {
  color: #dc2626;
}

.param-desc,
.mapping-hint,
.copy-status {
  color: #64748b;
  font-size: 0.76rem;
  line-height: 1.4;
}

.field-error {
  font-size: 0.76rem;
  margin: 0;
}

.field-actions {
  flex-wrap: wrap;
}

.mini-btn,
.advanced-toggle,
.sub-tabs button {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: 0.74rem;
  padding: 5px 8px;
}

.advanced-toggle {
  width: fit-content;
}

.compact {
  min-width: 160px;
}

.code-field {
  font-family: var(--font-mono);
}

.expression-preview,
.data-picker,
.ai-node-configurer,
.help-section,
.empty-state,
.node-error-summary {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 10px;
  background: #f8fafc;
}

.expression-preview code {
  display: block;
  margin-top: 6px;
  color: #0f172a;
  font-family: var(--font-mono);
  word-break: break-all;
}

.expression-preview pre,
.json-code,
.help-content-box pre {
  max-height: 260px;
  overflow: auto;
  background: #0f172a;
  color: #f8fafc;
  padding: 9px;
  border-radius: 6px;
  font-family: var(--font-mono);
  font-size: 0.72rem;
  line-height: 1.35;
  white-space: pre-wrap;
}

.json-tree {
  display: flex;
  flex-direction: column;
  max-height: 280px;
  overflow: auto;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: #fff;
}

.tree-row {
  display: grid;
  grid-template-columns: minmax(110px, 1fr) auto minmax(0, 1fr);
  gap: 7px;
  align-items: center;
  min-height: 30px;
  border: 0;
  border-bottom: 1px solid #edf2f7;
  background: transparent;
  color: var(--color-text);
  text-align: left;
  font: inherit;
  font-size: 0.74rem;
  cursor: pointer;
}

.tree-row.static {
  grid-template-columns: minmax(120px, 1fr) auto auto minmax(0, 1fr);
  cursor: default;
}

.tree-row:hover {
  background: #eff6ff;
}

.tree-path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--font-mono);
}

.tree-type {
  color: #64748b;
}

.tree-row code {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #475569;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.75rem;
}

.data-table th,
.data-table td {
  border: 1px solid var(--color-border);
  padding: 6px;
  text-align: left;
  vertical-align: top;
}

.data-table th {
  background: #f1f5f9;
}

.node-error-summary {
  border-color: #fecaca;
  background: #fef2f2;
}

.node-error-title {
  color: #991b1b;
  font-size: 0.75rem;
  font-weight: 800;
  text-transform: uppercase;
  margin-bottom: 6px;
}

.node-error-message {
  color: #b91c1c;
  margin: 0;
  white-space: pre-wrap;
  font-family: var(--font-mono);
  font-size: 0.72rem;
}

.run-meta,
.log-grid {
  color: var(--color-text-secondary);
  font-size: 0.76rem;
}

.run-meta {
  flex-wrap: wrap;
}

.log-grid {
  display: grid;
  grid-template-columns: 95px minmax(0, 1fr);
  gap: 6px 10px;
}

.log-grid dt {
  font-weight: 800;
  color: var(--color-text);
}

.log-grid dd {
  min-width: 0;
  overflow-wrap: anywhere;
  margin: 0;
}

.btn-full {
  width: 100%;
  justify-content: center;
}

@media (max-width: 1100px) {
  .properties-panel {
    width: min(400px, 42vw);
    min-width: 330px;
  }
}
</style>
