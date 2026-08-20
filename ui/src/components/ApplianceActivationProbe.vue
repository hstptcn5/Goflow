<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { applianceApi } from '@/services/applianceApi';

const props = defineProps({ bootstrap: { type: Object, required: true } });
const visible = ref(false);
const execution = ref(null);
const busy = ref(false);
let timer = null;

function locale() {
  const saved = typeof localStorage !== 'undefined' ? localStorage.getItem('goflow-appliance-locale') : '';
  if (saved === 'vi' || saved === 'en') return saved;
  return String(navigator.language || '').toLowerCase().startsWith('vi') ? 'vi' : 'en';
}

function storageKey() {
  return `goflow-activation-complete:${props.bootstrap.pack?.id || 'pack'}`;
}

function successfulManualRun(value) {
  const status = String(value?.status || '').toUpperCase();
  return value?.trigger_source === 'ui' && ['SUCCESS', 'SUCCEEDED', 'COMPLETED'].includes(status);
}

async function checkActivation() {
  if (localStorage.getItem(storageKey())) return;
  try {
    const latest = await applianceApi.getLatestExecution();
    const value = latest?.execution || null;
    if (successfulManualRun(value)) {
      execution.value = value;
      visible.value = true;
      return;
    }
  } catch {
    // Activation feedback must never interfere with the appliance itself.
  }
  timer = window.setTimeout(checkActivation, 1800);
}

function dismiss() {
  localStorage.setItem(storageKey(), '1');
  visible.value = false;
  if (timer) window.clearTimeout(timer);
}

async function useRealData() {
  const vi = locale() === 'vi';
  if (!window.confirm(vi ? 'Mở lại thiết lập để thay nguồn dữ liệu mẫu bằng dữ liệu thật?' : 'Reopen setup to replace sample data with your real source?')) return;
  busy.value = true;
  try {
    await applianceApi.reopenSetup(props.bootstrap.token);
    localStorage.setItem(storageKey(), '1');
    window.location.reload();
  } finally {
    busy.value = false;
  }
}

onMounted(checkActivation);
onBeforeUnmount(() => { if (timer) window.clearTimeout(timer); });
</script>

<template>
  <aside v-if="visible" class="activation-card" aria-live="polite">
    <button type="button" class="activation-close" :aria-label="locale() === 'vi' ? 'Đóng' : 'Dismiss'" @click="dismiss">×</button>
    <div class="activation-kicker">✓ {{ locale() === 'vi' ? 'Lần chạy đầu thành công' : 'First run completed' }}</div>
    <h2>{{ locale() === 'vi' ? 'Quy trình đầu tiên của bạn đã chạy thành công.' : 'Your first automation is working.' }}</h2>
    <p>{{ locale() === 'vi' ? 'Bây giờ bạn có thể thay dữ liệu mẫu bằng nguồn thật, sau đó bật lịch chạy khi đã sẵn sàng.' : 'Next, replace the sample with your real data source, then enable the schedule when you are ready.' }}</p>
    <div class="activation-meta" v-if="execution?.started_at">{{ locale() === 'vi' ? 'Lần chạy' : 'Run' }} · {{ new Date(execution.started_at).toLocaleString(locale() === 'vi' ? 'vi-VN' : undefined) }}</div>
    <div class="activation-actions">
      <button type="button" class="activation-primary" :disabled="busy" @click="useRealData">{{ busy ? (locale() === 'vi' ? 'Đang mở…' : 'Opening…') : (locale() === 'vi' ? 'Dùng dữ liệu thật' : 'Use my real data') }}</button>
      <button type="button" class="activation-secondary" @click="dismiss">{{ locale() === 'vi' ? 'Để sau' : 'Later' }}</button>
    </div>
  </aside>
</template>

<style scoped>
.activation-card{position:fixed;right:24px;bottom:24px;z-index:80;width:min(430px,calc(100vw - 32px));border:1px solid #b7d8c1;border-radius:18px;background:#f4fff7;padding:22px;box-shadow:0 24px 70px rgba(20,70,40,.18);color:#16351f}.activation-close{position:absolute;right:10px;top:10px;border:0;background:transparent;width:36px;height:36px;border-radius:999px;font-size:22px;cursor:pointer;color:#58705f}.activation-close:hover{background:#e3f5e8}.activation-kicker{font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:.08em;color:#247444;padding-right:38px}.activation-card h2{margin:10px 0 0;font-size:22px;line-height:1.25}.activation-card p{margin:10px 0 0;font-size:14px;line-height:1.6;color:#4d6755}.activation-meta{margin-top:10px;font-size:12px;color:#6d8172}.activation-actions{display:flex;flex-wrap:wrap;gap:10px;margin-top:16px}.activation-actions button{min-height:42px;border-radius:999px;padding:0 16px;font-weight:700;cursor:pointer}.activation-primary{border:1px solid #247444;background:#247444;color:white}.activation-primary:disabled{opacity:.6;cursor:wait}.activation-secondary{border:1px solid #b7d8c1;background:white;color:#244c31}@media(max-width:640px){.activation-card{right:16px;bottom:16px}}
</style>
