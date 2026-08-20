<script setup>
import { ref } from 'vue';
import StateBlock from '@/components/StateBlock.vue';

const apiKey = ref(localStorage.getItem('GOFLOW_API_KEY') || '');
const saved = ref(false);

function saveApiKey() {
  if (apiKey.value.trim()) {
    localStorage.setItem('GOFLOW_API_KEY', apiKey.value.trim());
  } else {
    localStorage.removeItem('GOFLOW_API_KEY');
  }
  saved.value = true;
  window.setTimeout(() => {
    saved.value = false;
  }, 2000);
}
</script>

<template>
  <div class="page-stack">
    <section class="page-toolbar">
      <div>
        <h2>Cài đặt</h2>
        <p>Cấu hình các thiết lập cục bộ của trình duyệt cho instance Goflow này.</p>
      </div>
    </section>

    <section class="section-panel narrow-panel">
      <h3>Khóa API Goflow</h3>
      <p class="muted-copy">Chỉ được lưu trong trình duyệt này và gửi dưới dạng Bearer token tới REST API và WebSocket của Goflow.</p>
      <input v-model="apiKey" class="form-input" type="password" placeholder="GOFLOW_API_KEY" aria-label="Khóa API Goflow" />
      <div class="panel-actions">
        <button class="btn btn-primary" type="button" @click="saveApiKey">Lưu khóa API</button>
      </div>
    </section>

    <StateBlock v-if="saved" tone="success" title="Đã lưu cài đặt" message="Các yêu cầu API tiếp theo sẽ dùng khóa vừa cập nhật." />
  </div>
</template>
