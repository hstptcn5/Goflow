<script setup>
import { computed, ref, watch } from 'vue';

const props = defineProps({
  value: { type: [String, Object], default: '' },
  ariaLabel: { type: String, default: 'Key-value editor' },
});

const emit = defineEmits(['change']);
const rows = ref([]);
const rawMode = ref(false);
const rawValue = ref('');

function normalizeRows(value) {
  if (value == null || value === '') return [];
  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (trimmed.startsWith('{{') && trimmed.endsWith('}}')) {
      rawMode.value = true;
      rawValue.value = value;
      return [];
    }
    try {
      value = JSON.parse(value);
    } catch {
      rawMode.value = true;
      rawValue.value = value;
      return [];
    }
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    rawMode.value = true;
    rawValue.value = typeof value === 'string' ? value : JSON.stringify(value ?? '');
    return [];
  }
  rawMode.value = false;
  rawValue.value = '';
  return Object.entries(value).map(([key, item]) => ({ key, value: Array.isArray(item) ? item.join(', ') : String(item ?? '') }));
}

watch(
  () => props.value,
  (value) => {
    rows.value = normalizeRows(value);
  },
  { immediate: true, deep: true }
);

function emitRows() {
  const output = {};
  rows.value.forEach((row) => {
    const key = String(row.key || '').trim();
    if (!key) return;
    const text = String(row.value ?? '');
    output[key] = text.includes(',') ? text.split(',').map((item) => item.trim()) : text;
  });
  emit('change', JSON.stringify(output, null, 2));
}

function addRow() {
  rows.value = [...rows.value, { key: '', value: '' }];
}

function removeRow(index) {
  rows.value = rows.value.filter((_, rowIndex) => rowIndex !== index);
  emitRows();
}

function switchToRows() {
  try {
    const parsed = JSON.parse(rawValue.value || '{}');
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return;
    rows.value = Object.entries(parsed).map(([key, value]) => ({ key, value: Array.isArray(value) ? value.join(', ') : String(value ?? '') }));
    rawMode.value = false;
    emitRows();
  } catch {
    // Keep raw mode for expressions or intentionally non-object values.
  }
}

const canUseRows = computed(() => {
  const trimmed = rawValue.value.trim();
  if (!trimmed || trimmed.startsWith('{{')) return false;
  try {
    const parsed = JSON.parse(trimmed);
    return Boolean(parsed && typeof parsed === 'object' && !Array.isArray(parsed));
  } catch {
    return false;
  }
});
</script>

<template>
  <div class="kv-editor" :aria-label="ariaLabel">
    <template v-if="rawMode">
      <textarea
        class="form-textarea code-field"
        rows="4"
        :value="rawValue"
        spellcheck="false"
        @input="rawValue = $event.target.value; emit('change', rawValue)"
      ></textarea>
      <button v-if="canUseRows" type="button" class="mini-btn" @click="switchToRows">Edit as rows</button>
    </template>
    <template v-else>
      <div v-for="(row, index) in rows" :key="index" class="kv-row">
        <input v-model="row.key" class="form-input" type="text" placeholder="Key" @input="emitRows" />
        <input v-model="row.value" class="form-input" type="text" placeholder="Value" @input="emitRows" />
        <button type="button" class="mini-btn danger" aria-label="Remove row" @click="removeRow(index)">×</button>
      </div>
      <button type="button" class="mini-btn" @click="addRow">Add row</button>
    </template>
  </div>
</template>

<style scoped>
.kv-editor { display: grid; gap: 8px; }
.kv-row { display: grid; grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr) auto; gap: 6px; align-items: center; }
.mini-btn.danger { color: #b91c1c; }
</style>
