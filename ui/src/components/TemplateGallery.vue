<script setup>
import { computed, ref } from 'vue';
import { workflowTemplates } from '@/templates/workflowTemplates';

const props = defineProps({
  title: {
    type: String,
    default: 'Template Gallery',
  },
  actionLabel: {
    type: String,
    default: 'Use Template',
  },
});

const emit = defineEmits(['close', 'select']);

const query = ref('');
const selectedCategory = ref('All');

const categories = computed(() => {
  const unique = new Set(workflowTemplates.map((item) => item.category));
  return ['All', ...Array.from(unique).sort()];
});

const filteredTemplates = computed(() => {
  const q = query.value.trim().toLowerCase();
  return workflowTemplates.filter((item) => {
    const inCategory = selectedCategory.value === 'All' || item.category === selectedCategory.value;
    if (!inCategory) return false;
    if (!q) return true;
    const haystack = [
      item.title,
      item.category,
      item.difficulty,
      item.summary,
      ...(item.requirements || []),
    ].join(' ').toLowerCase();
    return haystack.includes(q);
  });
});

function nodeCount(template) {
  return template.workflow?.nodes?.length || 0;
}

function edgeCount(template) {
  return template.workflow?.edges?.length || 0;
}
</script>

<template>
  <div class="template-backdrop" @click.self="emit('close')">
    <div class="template-modal glass-panel">
      <div class="template-header">
        <div>
          <h3>{{ title }}</h3>
          <p>Start from a working workflow, then fill in credentials, URLs, and message targets.</p>
        </div>
        <button class="btn-icon" @click="emit('close')">x</button>
      </div>

      <div class="template-controls">
        <input
          v-model="query"
          class="form-input"
          type="text"
          placeholder="Search templates, nodes, or use cases..."
        />
        <select v-model="selectedCategory" class="form-select">
          <option v-for="category in categories" :key="category" :value="category">
            {{ category }}
          </option>
        </select>
      </div>

      <div class="template-grid">
        <article
          v-for="template in filteredTemplates"
          :key="template.id"
          class="template-card"
        >
          <div class="template-card-top">
            <span class="template-category">{{ template.category }}</span>
            <span class="template-difficulty">{{ template.difficulty }}</span>
          </div>

          <h4>{{ template.title }}</h4>
          <p class="template-summary">{{ template.summary }}</p>

          <div class="template-meta">
            <span>{{ nodeCount(template) }} nodes</span>
            <span>{{ edgeCount(template) }} edges</span>
          </div>

          <div class="template-reqs">
            <span
              v-for="requirement in template.requirements"
              :key="requirement"
              class="template-req"
            >
              {{ requirement }}
            </span>
          </div>

          <button class="btn btn-primary template-use-btn" @click="emit('select', template)">
            {{ actionLabel }}
          </button>
        </article>
      </div>

      <div v-if="filteredTemplates.length === 0" class="template-empty">
        No templates match the current search.
      </div>
    </div>
  </div>
</template>

<style scoped>
.template-backdrop {
  position: fixed;
  inset: 0;
  z-index: 260;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.56);
  backdrop-filter: blur(6px);
}

.template-modal {
  width: min(980px, calc(100vw - 40px));
  max-height: min(760px, calc(100vh - 40px));
  display: flex;
  flex-direction: column;
  background: #ffffff;
  border-radius: 10px;
  overflow: hidden;
}

.template-header {
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
}

.template-header h3 {
  margin: 0;
  font-size: 1.05rem;
  color: #0f172a;
}

.template-header p {
  margin: 4px 0 0;
  font-size: 0.85rem;
  color: #64748b;
}

.template-controls {
  padding: 14px 20px;
  display: grid;
  grid-template-columns: 1fr 220px;
  gap: 12px;
  border-bottom: 1px solid var(--border-color);
  background: #f8fafc;
}

.template-grid {
  padding: 18px 20px 20px;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 14px;
}

.template-card {
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  padding: 14px;
  background: #ffffff;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 250px;
}

.template-card:hover {
  border-color: #2563eb;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.12);
}

.template-card-top,
.template-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.template-category,
.template-difficulty,
.template-meta span {
  font-size: 0.72rem;
  font-weight: 700;
  color: #475569;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  padding: 3px 8px;
}

.template-card h4 {
  margin: 0;
  font-size: 0.98rem;
  color: #0f172a;
}

.template-summary {
  color: #475569;
  line-height: 1.42;
  font-size: 0.84rem;
  min-height: 58px;
}

.template-reqs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: auto;
}

.template-req {
  font-size: 0.7rem;
  color: #075985;
  background: #e0f2fe;
  border: 1px solid #bae6fd;
  border-radius: 999px;
  padding: 3px 7px;
}

.template-use-btn {
  width: 100%;
  justify-content: center;
}

.template-empty {
  padding: 24px;
  text-align: center;
  color: #64748b;
}

.btn-icon {
  border: 1px solid #cbd5e1;
  background: #ffffff;
  border-radius: 6px;
  color: #334155;
  height: 30px;
  min-width: 30px;
  cursor: pointer;
}

@media (max-width: 720px) {
  .template-controls {
    grid-template-columns: 1fr;
  }

  .template-grid {
    grid-template-columns: 1fr;
  }
}
</style>

