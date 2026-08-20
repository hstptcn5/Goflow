<script setup>
import { computed, ref } from 'vue';
import { workflowTemplates } from '@/templates/workflowTemplates';

const props = defineProps({
  title: {
    type: String,
    default: 'Thư viện mẫu workflow',
  },
  actionLabel: {
    type: String,
    default: 'Dùng mẫu này',
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
    const haystack = [item.title, item.category, item.difficulty, item.summary, ...(item.requirements || [])].join(' ').toLowerCase();
    return haystack.includes(q);
  });
});

function nodeCount(template) {
  return template.workflow?.nodes?.length || 0;
}
function edgeCount(template) {
  return template.workflow?.edges?.length || 0;
}
function categoryLabel(value) {
  const labels = {
    All: 'Tất cả', Plugins: 'Plugin', 'AI + Support': 'AI + Hỗ trợ', 'AI + Trust': 'AI + Kiểm duyệt', 'AI + Ops': 'AI + Vận hành',
    Monitoring: 'Giám sát', DevOps: 'DevOps', 'Backend Jobs': 'Tác vụ backend', 'Developer Tools': 'Công cụ lập trình',
    'Business Ops': 'Vận hành kinh doanh', CRM: 'CRM', 'Customer Success': 'Chăm sóc khách hàng', 'Content Ops': 'Vận hành nội dung',
    'API Automation': 'Tự động hóa API', AI: 'AI', Testing: 'Kiểm thử',
  };
  return labels[value] || value;
}
function difficultyLabel(value) {
  return ({ Beginner: 'Cơ bản', Intermediate: 'Trung bình', Advanced: 'Nâng cao' })[value] || value;
}
</script>

<template>
  <div class="template-backdrop" @click.self="emit('close')">
    <div class="template-modal glass-panel">
      <div class="template-header">
        <div>
          <h3>{{ title }}</h3>
          <p>Bắt đầu từ workflow có sẵn, sau đó điền credential, URL và đích nhận tin nhắn của bạn.</p>
        </div>
        <button class="btn-icon" aria-label="Đóng thư viện mẫu" @click="emit('close')">x</button>
      </div>

      <div class="template-controls">
        <input v-model="query" class="form-input" type="text" placeholder="Tìm mẫu, node hoặc tình huống sử dụng..." />
        <select v-model="selectedCategory" class="form-select">
          <option v-for="category in categories" :key="category" :value="category">{{ categoryLabel(category) }}</option>
        </select>
      </div>

      <div class="template-grid">
        <article v-for="template in filteredTemplates" :key="template.id" class="template-card">
          <div class="template-card-top">
            <span class="template-category">{{ categoryLabel(template.category) }}</span>
            <span class="template-difficulty">{{ difficultyLabel(template.difficulty) }}</span>
          </div>
          <h4>{{ template.title }}</h4>
          <p class="template-summary">{{ template.summary }}</p>
          <div class="template-meta">
            <span>{{ nodeCount(template) }} node</span>
            <span>{{ edgeCount(template) }} dây nối</span>
          </div>
          <div class="template-reqs">
            <span v-for="requirement in template.requirements" :key="requirement" class="template-req">{{ requirement }}</span>
          </div>
          <button class="btn btn-primary template-use-btn" @click="emit('select', template)">{{ actionLabel }}</button>
        </article>
      </div>

      <div v-if="filteredTemplates.length === 0" class="template-empty">Không có mẫu nào phù hợp với tìm kiếm hiện tại.</div>
    </div>
  </div>
</template>

<style scoped>
.template-backdrop { position: fixed; inset: 0; z-index: 260; display: flex; align-items: center; justify-content: center; background: rgba(15, 23, 42, 0.56); backdrop-filter: blur(6px); }
.template-modal { width: min(980px, calc(100vw - 40px)); max-height: min(760px, calc(100vh - 40px)); display: flex; flex-direction: column; background: #ffffff; border-radius: 10px; overflow: hidden; }
.template-header { padding: 18px 20px; border-bottom: 1px solid var(--border-color); display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
.template-header h3 { margin: 0; font-size: 1.05rem; color: #0f172a; }
.template-header p { margin: 4px 0 0; font-size: 0.85rem; color: #64748b; }
.template-controls { padding: 14px 20px; display: grid; grid-template-columns: 1fr 220px; gap: 12px; border-bottom: 1px solid var(--border-color); background: #f8fafc; }
.template-grid { padding: 18px 20px 20px; overflow-y: auto; display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; }
.template-card { border: 1px solid #dbe3ef; border-radius: 9px; padding: 14px; background: #ffffff; display: flex; flex-direction: column; gap: 10px; }
.template-card-top { display: flex; justify-content: space-between; gap: 8px; }
.template-category, .template-difficulty, .template-req { font-size: 0.7rem; border-radius: 999px; background: #f1f5f9; color: #475569; padding: 3px 8px; }
.template-card h4 { margin: 0; color: #0f172a; }
.template-summary { margin: 0; color: #64748b; font-size: 0.82rem; line-height: 1.45; }
.template-meta, .template-reqs { display: flex; flex-wrap: wrap; gap: 7px; color: #64748b; font-size: 0.72rem; }
.template-use-btn { margin-top: auto; }
.template-empty { padding: 30px; text-align: center; color: #64748b; }
</style>
