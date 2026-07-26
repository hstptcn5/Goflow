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
      search.value = '';
      selectedIndex.value = 0;
      await nextTick();
      searchInput.value?.focus();
    }
  }
);

onMounted(async () => {
  if (!props.visible) return;
  await nextTick();
  searchInput.value?.focus();
});

const enrichedNodes = computed(() => {
  return workflowStore.nodeDefinitions.map((def) => {
    const credentialRequired = (def.params || []).some((param) => param.type === 'credential' && param.required);
    return {
      ...def,
      friendlyCategory: categoryLabel(def.category),
      credentialRequired,
      experimental: def.experimental || def.type === 'goflowPlugin',
      triggerKind: def.category === 'TRIGGER' ? 'Trigger' : 'Action',
    };
  });
});

const filteredNodes = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return enrichedNodes.value;
  return enrichedNodes.value.filter((def) => {
    return [
      def.name,
      def.type,
      def.description,
      def.category,
      def.friendlyCategory,
    ].some((part) => String(part || '').toLowerCase().includes(q));
  });
});

const groupedNodes = computed(() => {
  const groups = [];
  const byType = new Map(enrichedNodes.value.map((def) => [def.type, def]));
  const favoriteNodes = favoriteTypes.value.map((type) => byType.get(type)).filter(Boolean);
  const recentNodes = recentTypes.value.map((type) => byType.get(type)).filter(Boolean);

  if (!search.value.trim()) {
    if (recentNodes.length) groups.push({ label: 'Recent', nodes: recentNodes });
    if (favoriteNodes.length) groups.push({ label: 'Favorites', nodes: favoriteNodes });
  }

  const order = ['Triggers', 'Actions', 'Logic', 'AI', 'Databases', 'Communication', 'Developer Tools'];
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

function toggleFavorite(def, event) {
  event.stopPropagation();
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
  emit('select', def);
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    emit('close');
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
  if (event.key === 'Enter' && flatNodes.value[selectedIndex.value]) {
    event.preventDefault();
    selectNode(flatNodes.value[selectedIndex.value]);
  }
}
</script>

<template>
  <div v-if="visible" class="node-picker-backdrop" @click.self="emit('close')">
    <section
      class="node-picker"
      role="dialog"
      aria-modal="true"
      aria-labelledby="node-picker-title"
      @keydown="onKeydown"
    >
      <header class="node-picker-header">
        <div>
          <h2 id="node-picker-title">Add step</h2>
          <p>Search by node name, type, description, or category.</p>
        </div>
        <button type="button" class="icon-button" aria-label="Close node picker" @click="emit('close')">x</button>
      </header>

      <input
        ref="searchInput"
        v-model="search"
        class="node-picker-search"
        type="search"
        aria-label="Search nodes"
        placeholder="Search nodes..."
      />

      <div v-if="flatNodes.length === 0" class="node-picker-empty" role="status">
        No nodes match this search.
      </div>

      <div v-else class="node-picker-list" role="listbox" aria-label="Available nodes">
        <section v-for="group in groupedNodes" :key="group.label" class="node-picker-group">
          <h3>{{ group.label }}</h3>
          <button
            v-for="def in group.nodes"
            :key="`${group.label}-${def.type}`"
            type="button"
            class="node-picker-item"
            :class="{ highlighted: flatNodes[selectedIndex]?.type === def.type }"
            role="option"
            :aria-selected="flatNodes[selectedIndex]?.type === def.type"
            @click="selectNode(def)"
            @mouseenter="selectedIndex = flatNodes.findIndex((item) => item.type === def.type)"
          >
            <span class="node-picker-icon" v-html="getNodeIconSVG(def.type)"></span>
            <span class="node-picker-main">
              <strong>{{ def.name }}</strong>
              <span>{{ def.description || def.type }}</span>
              <small>{{ def.friendlyCategory }} · {{ def.triggerKind }}</small>
            </span>
            <span class="node-picker-badges">
              <span v-if="def.credentialRequired" class="badge badge-warning">Credential</span>
              <span v-if="def.experimental" class="badge badge-muted">Experimental</span>
              <button
                type="button"
                class="favorite-button"
                :aria-label="isFavorite(def.type) ? `Remove ${def.name} from favorites` : `Add ${def.name} to favorites`"
                @click="toggleFavorite(def, $event)"
              >
                {{ isFavorite(def.type) ? 'Starred' : 'Star' }}
              </button>
            </span>
          </button>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped>
.node-picker-backdrop {
  position: absolute;
  inset: var(--workflow-topbar-height) 0 0 0;
  z-index: 180;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 44px 20px;
  background: rgba(15, 23, 42, 0.18);
}

.node-picker {
  width: min(760px, calc(100vw - 56px));
  max-height: min(720px, calc(100vh - 140px));
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}

.node-picker-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  border-bottom: 1px solid var(--color-border);
}

.node-picker-header h2 {
  margin: 0;
  font-size: var(--font-size-xl);
}

.node-picker-header p {
  margin: 4px 0 0;
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
}

.node-picker-search {
  margin: var(--space-4);
  padding: 11px 13px;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-md);
  font: inherit;
}

.node-picker-list {
  overflow: auto;
  padding: 0 var(--space-4) var(--space-4);
}

.node-picker-group + .node-picker-group {
  margin-top: var(--space-4);
}

.node-picker-group h3 {
  margin: 0 0 var(--space-2);
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
  text-transform: uppercase;
}

.node-picker-item {
  width: 100%;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: var(--space-3);
  align-items: center;
  padding: var(--space-3);
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.node-picker-item:hover,
.node-picker-item.highlighted {
  background: var(--color-surface-muted);
  border-color: var(--color-border);
}

.node-picker-icon {
  color: var(--color-primary);
  display: inline-flex;
}

.node-picker-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.node-picker-main strong {
  color: var(--color-text);
}

.node-picker-main span,
.node-picker-main small {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
}

.node-picker-badges {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.favorite-button,
.icon-button {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  padding: 6px 8px;
}

.node-picker-empty {
  margin: 0 var(--space-4) var(--space-4);
  padding: var(--space-6);
  border: 1px dashed var(--color-border-strong);
  border-radius: var(--radius-md);
  color: var(--color-text-muted);
  text-align: center;
}
</style>
