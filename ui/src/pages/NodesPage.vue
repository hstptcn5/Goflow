<script setup>
import { computed } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';
import StateBlock from '@/components/StateBlock.vue';

const workflowStore = useWorkflowStore();

const groupedNodes = computed(() => {
  return workflowStore.nodeDefinitions.reduce((groups, node) => {
    const category = node.category || 'OTHER';
    if (!groups[category]) groups[category] = [];
    groups[category].push(node);
    return groups;
  }, {});
});

function categoryLabel(category) {
  const labels = {
    TRIGGER: 'Kích hoạt',
    ACTION: 'Hành động',
    TRANSFORM: 'Biến đổi dữ liệu',
    CONDITION: 'Điều kiện',
    COMMUNICATION: 'Gửi / Nhận dữ liệu',
    DATABASE: 'Cơ sở dữ liệu',
    AI: 'Trí tuệ nhân tạo',
    UTILITY: 'Tiện ích',
    OTHER: 'Khác',
  };
  return labels[String(category || '').toUpperCase()] || category;
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Danh sách node</h2>
        <p>Xem các loại node hiện có, công dụng và tham số trước khi đưa chúng vào workflow.</p>
      </div>
    </section>

    <StateBlock
      v-if="workflowStore.nodeDefinitions.length === 0"
      title="Chưa tải được định nghĩa node"
      message="API chưa trả về danh sách node. Hãy kiểm tra Goflow đang chạy và thử lại từ trang Workflow."
    />

    <section v-for="(items, category) in groupedNodes" :key="category" class="section-panel">
      <h3>{{ categoryLabel(category) }}</h3>
      <div class="node-list">
        <article v-for="node in items" :key="node.type" class="node-list-item">
          <div>
            <strong>{{ node.name }}</strong>
            <p>{{ node.description }}</p>
          </div>
          <span class="badge badge-muted" :title="'ID kỹ thuật của node'">{{ node.type }}</span>
        </article>
      </div>
    </section>
  </div>
</template>
