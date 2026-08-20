<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import { getNodeIconSVG } from './NodeIcons';
import { categoryLabel } from '@/utils/workflowEditor';

const props = defineProps({
  visible: Boolean,
});

const emit = defineEmits(['close', 'select']);

const workflowStore = useWorkflowStore();
const search = ref('');
const selectedIndex = ref(0);
const searchInput = ref(null);
const dialog = ref(null);
const previousFocus = ref(null);
const favoriteTypes = ref(readStoredList('goflow.favoriteNodes'));
const recentTypes = ref(readStoredList('goflow.recentNodes'));

function readStoredList(key) {
  try {
    return JSON.parse(localStorage.getItem(key) || '[]');
  } catch {
    return [];
  }
}

function writeStoredList(key, value) {
  localStorage.setItem(key, JSON.stringify(value.slice(0, 12)));
}

watch(
  () => props.visible,
  async (visible) => {
    if (visible) {
      previousFocus.value = document.activeElement;
      search.value = '';
      selectedIndex.value = 0;
      await nextTick();
      searchInput.value?.focus();
    }
  }
);

onMounted(async () => {
  if (!props.visible) return;
  previousFocus.value = document.activeElement;
  await nextTick();
  searchInput.value?.focus();
});

function categoryVi(value) {
  const source = categoryLabel(value);
  const labels = {
    Triggers: 'Kích hoạt',
    Actions: 'Hành động',
    Logic: 'Logic',
    AI: 'AI',
    Databases: 'Cơ sở dữ liệu',
    Communication: 'Giao tiếp',
    'Developer Tools': 'Công cụ lập trình',
  };
  return labels[source] || source;
}

const enrichedNodes = computed(() => {
  return workflowStore.nodeDefinitions.map((def) => {
    const credentialRequired = (def.params || []).some((param) => param.type === 'credential' && param.required);
    return {
      ...def,
      friendlyCategory: categoryVi(def.category),
      credentialRequired,
      experimental: def.experimental || def.type === 'goflowPlugin',
      triggerKind: def.category === 'TRIGGER' ? 'Kích hoạt' : 'Hành động',
    };
  });
});

const filteredNodes = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return enrichedNodes.value;
  return enrichedNodes.value.filter((def) => {
    return [def.name, def.type, def.description, def.category, def.friendlyCategory]
      .some((part) => String(part || '').toLowerCase().includes(q));
  });
});

const groupedNodes = computed(() => {
  const groups = [];
  const byType = new Map(enrichedNodes.value.map((def) => [def.type, def]));
  const favoriteNodes = favoriteTypes.value.map((type) => byType.get(type)).filter(Boolean);
  const recentNodes = recentTypes.value.map((type) => byType.get(type)).filter(Boolean);

  if (!search.value.trim()) {
    if (recentNodes.length) groups.push({ label: 'Gần đây', nodes: recentNodes });
    if (favoriteNodes.length) groups.push({ label: 'Yêu thích', nodes: favoriteNodes });
  }

  const order = ['Kích hoạt', 'Hành động', 'Logic', 'AI', 'Cơ sở dữ liệu', 'Giao tiếp', 'Công cụ lập trình'];
  order.forEach((label) => {
    const nodes = filteredNodes.value.filter((def) => def.friendlyCategory === label);
    if (nodes.length) groups.push({ label, nodes });
  });

  return groups;
});

const flatNodes = computed(() => groupedNodes.value.flatMap((group) => group.nodes));

watch(flatNodes, () => {
  selectedIndex.value = Math.min(selectedIndex.value, Math.max(0, flatNodes.value.length - 1));
});

function isFavorite(type) {
  return favoriteTypes.value.includes(type);
}

function toggleFavorite(def) {
  const next = isFavorite(def.type)
    ? favoriteTypes.value.filter((type) => type !== def.type)
    : [def.type, ...favoriteTypes.value];
  favoriteTypes.value = next;
  writeStoredList('goflow.favoriteNodes', next);
}

function rememberRecent(def) {
  const next = [def.type, ...recentTypes.value.filter((type) => type !== def.type)];
  recentTypes.value = next;
  writeStoredList('goflow.recentNodes', next);
}

function selectNode(def) {
  rememberRecent(def);
  restoreFocus();
  emit('select', def);
}

function restoreFocus() {
  const target = previousFocus.value;
  if (target && typeof target.focus === 'function') target.focus();
}

function closePicker() {
  restoreFocus();
  emit('close');
}

function focusableElements() {
  return Array.from(dialog.value?.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])') || [])
    .filter((el) => !el.disabled);
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    closePicker();
    return;
  }
  if (event.key === 'Tab') {
    const focusables = focusableElements();
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
    return;
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault();
    selectedIndex.value = Math.min(flatNodes.value.length - 1, selectedIndex.value + 1);
    return;
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault();
    selectedIndex.value = Math.max(0, selectedIndex.value - 1);
    return;
  }
  if (event.target?.classList?.contains('favorite-button')) return;
  if (event.key === 'Enter' && flatNodes.value[selectedIndex.value]) {
    event.preventDefault();
    selectNode(flatNodes.value[selectedIndex.value]);
  }
}
</script>

<template>
  <div v-if="visible" class="node-picker-backdrop" @click.self="closePicker">
    <section ref="dialog" class="node-picker" role="dialog" aria-modal="true" aria-labelledby="node-picker-title" @keydown="onKeydown">
      <header class="node-picker-header">
        <div>
          <h2 id="node-picker-title">Thêm bước</h2>
          <p>Tìm theo tên node, type kỹ thuật, mô tả hoặc nhóm chức năng.</p>
        </div>
        <button type="button" class="icon-button" aria-label="Đóng bộ chọn node" @click="closePicker">x</button>
      </header>

      <input ref="searchInput" v-model="search" class="node-picker-search" type="search" aria-label="Tìm node" placeholder="Tìm node..." />

      <div v-if="flatNodes.length === 0" class="node-picker-empty" role="status">Không có node phù hợp.</div>

      <div v-else class="node-picker-list" role="listbox" aria-label="Các node có thể thêm">
        <section v-for="group in groupedNodes" :key="group.label" class="node-picker-group">
          <h3>{{ group.label }}</h3>
          <div
            v-for="def in group.nodes"
            :key="`${group.label}-${def.type}`"
            class="node-picker-item"
            :class="{ highlighted: flatNodes[selectedIndex]?.type === def.type }"
            role="option"
            :aria-selected="flatNodes[selectedIndex]?.type === def.type"
            @mouseenter="selectedIndex = flatNodes.findIndex((item) => item.type === def.type)"
          >
            <button type="button" class="node-picker-select" @click="selectNode(def)">
              <span class="node-picker-icon" v-html="getNodeIconSVG(def.type)"></span>
              <span class="node-picker-main">
                <strong>{{ def.name }}</strong>
                <span>{{ def.description || def.type }}</span>
                <small>{{ def.friendlyCategory }} · {{ def.triggerKind }}</small>
              </span>
            </button>
            <span class="node-picker-badges">
              <span v-if="def.credentialRequired" class="badge badge-warning">Cần credential</span>
              <span v-if="def.experimental" class="badge badge-muted">Thử nghiệm</span>
              <button
                type="button"
                class="favorite-button"
                :aria-label="isFavorite(def.type) ? `Bỏ ${def.name} khỏi yêu thích` : `Thêm ${def.name} vào yêu thích`"
                @click="toggleFavorite(def)"
              >
                {{ isFavorite(def.type) ? 'Đã ghim' : 'Ghim' }}
              </button>
            </span>
          </div>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped>
.node-picker-backdrop { position: absolute; inset: var(--workflow-topbar-height) 0 0 0; z-index: 180; display: flex; align-items: flex-start; justify-content: center; padding: 44px 20px; background: rgba(15, 23, 42, 0.18); }
.node-picker { width: min(760px, calc(100vw - 56px)); max-height: min(720px, calc(100vh - 140px)); display: flex; flex-direction: column; overflow: hidden; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg); box-shadow: var(--shadow-lg); }
.node-picker-header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-4); padding: var(--space-4); border-bottom: 1px solid var(--color-border); }
.node-picker-header h2 { margin: 0; font-size: var(--font-size-xl); }
.node-picker-header p { margin: 4px 0 0; color: var(--color-text-muted); font-size: var(--font-size-sm); }
.node-picker-search { margin: var(--space-4); padding: 11px 13px; border: 1px solid var(--color-border-strong); border-radius: var(--radius-md); font: inherit; }
.node-picker-list { overflow: auto; padding: 0 var(--space-4) var(--space-4); }
.node-picker-group + .node-picker-group { margin-top: var(--space-4); }
.node-picker-group h3 { margin: 0 0 var(--space-2); font-size: var(--font-size-xs); color: var(--color-text-muted); text-transform: uppercase; }
.node-picker-item { width: 100%; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--space-3); align-items: center; padding: var(--space-3); border: 1px solid transparent; border-radius: var(--radius-md); background: transparent; text-align: left; cursor: pointer; }
.node-picker-select { min-width: 0; display: grid; grid-template-columns: auto minmax(0, 1fr); gap: var(--space-3); align-items: center; border: 0; background: transparent; color: inherit; cursor: pointer; font: inherit; padding: 0; text-align: left; }
.node-picker-item:hover, .node-picker-item.highlighted { background: var(--color-surface-muted); border-color: var(--color-border); }
.node-picker-icon { color: var(--color-primary); display: inline-flex; }
.node-picker-main { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.node-picker-main strong { color: var(--color-text); }
.node-picker-main span, .node-picker-main small { color: var(--color-text-muted); font-size: var(--font-size-sm); }
.node-picker-badges { display: flex; align-items: center; gap: var(--space-2); }
.favorite-button, .icon-button { border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-surface); color: var(--color-text-secondary); cursor: pointer; font: inherit; padding: 6px 8px; }
.node-picker-empty { margin: 0 var(--space-4) var(--space-4); padding: var(--space-6); border: 1px dashed var(--color-border-strong); border-radius: var(--radius-md); color: var(--color-text-muted); text-align: center; }
</style>
