<script setup>
defineProps({
  title: { type: String, required: true },
  message: { type: String, default: '' },
  actionLabel: { type: String, default: '' },
  tone: { type: String, default: 'default' },
});

const emit = defineEmits(['action']);
</script>

<template>
  <section class="state-block" :class="`state-${tone}`" role="status">
    <div class="state-title">{{ title }}</div>
    <p v-if="message" class="state-message">{{ message }}</p>
    <button v-if="actionLabel" class="btn btn-secondary" type="button" @click="emit('action')">
      {{ actionLabel }}
    </button>
    <slot />
  </section>
</template>
