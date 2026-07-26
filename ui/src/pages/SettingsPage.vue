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
        <h2>Settings</h2>
        <p>Configure local browser settings for this Goflow instance.</p>
      </div>
    </section>

    <section class="section-panel narrow-panel">
      <h3>API key</h3>
      <p class="muted-copy">Stored in this browser only and sent as a Bearer token to the Goflow REST API and WebSocket.</p>
      <input v-model="apiKey" class="form-input" type="password" placeholder="GOFLOW_API_KEY" aria-label="Goflow API key" />
      <div class="panel-actions">
        <button class="btn btn-primary" type="button" @click="saveApiKey">Save API key</button>
      </div>
    </section>

    <StateBlock v-if="saved" tone="success" title="Settings saved" message="Future API requests will use the updated key." />
  </div>
</template>
