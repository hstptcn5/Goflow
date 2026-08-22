<script setup>
import { onMounted, ref } from 'vue';
import { api } from '@/services/api';
import { defaultInputMode, fieldsFromJSONSchema } from '@/utils/appBuilder';

const props = defineProps({
  workflow: { type: Object, required: true },
  nodes: { type: Array, default: () => [] },
});
const emit = defineEmits(['close']);
const report = ref(null);
const loading = ref(true);
const building = ref(false);
const error = ref('');
const form = ref({
  id: '', name: props.workflow.name || 'Goflow App', version: '1.0.0',
  description: props.workflow.description || '', icon: '⚡', accent: '#2563EB',
  inputMode: defaultInputMode(props.nodes), outputNodeID: props.nodes.at(-1)?.id || '',
  outputMode: 'auto', submitLabel: 'Chạy',
  fields: fieldsFromJSONSchema(props.workflow.input_schema_json),
});

function addField() {
  form.value.fields.push({ key: `field_${form.value.fields.length + 1}`, label: 'Trường mới', type: 'string', required: false, description: '', placeholder: '' });
}
function removeField(index) { form.value.fields.splice(index, 1); }
function nodeLabel(node) { return node.data?.name || node.name || node.id; }
function cleanFields() {
  return form.value.fields.map((field) => ({
    key: String(field.key || '').trim(), label: String(field.label || field.key || '').trim(),
    description: String(field.description || '').trim(), type: field.type || 'string',
    required: Boolean(field.required), placeholder: String(field.placeholder || '').trim(),
    ...(field.type === 'select' ? { options: String(field.optionsText || '').split(',').map((item) => item.trim()).filter(Boolean) } : {}),
  }));
}
async function build() {
  error.value = ''; building.value = true;
  try {
    const result = await api.buildWorkflowApp(props.workflow.id, {
      id: form.value.id, name: form.value.name, version: form.value.version,
      description: form.value.description,
      branding: { icon: form.value.icon, accent_color: form.value.accent },
      run_ui: {
        input_mode: form.value.inputMode, input_fields: cleanFields(),
        output_node_id: form.value.outputNodeID, output_mode: form.value.outputMode,
        submit_label: form.value.submitLabel,
      },
    });
    const url = URL.createObjectURL(result.blob);
    const link = document.createElement('a'); link.href = url; link.download = result.filename; link.click();
    URL.revokeObjectURL(url);
  } catch (err) { error.value = err.message || 'Build thất bại'; }
  finally { building.value = false; }
}
onMounted(async () => {
  try { report.value = await api.analyzeWorkflowApp(props.workflow.id); }
  catch (err) { error.value = err.message; }
  finally { loading.value = false; }
});
</script>

<template>
  <div class="builder-backdrop" role="presentation" @click.self="emit('close')">
    <section class="builder-modal" role="dialog" aria-modal="true" aria-labelledby="app-builder-title">
      <header><div><span class="builder-kicker">GF-APP</span><h2 id="app-builder-title">Xuất workflow thành ứng dụng</h2><p>Một file ứng dụng có form nhập và màn hình kết quả. Phụ thuộc YELLOW cần có trên máy chạy app.</p></div><button class="btn btn-secondary" type="button" @click="emit('close')">Đóng</button></header>
      <div v-if="loading" class="builder-state">Đang kiểm tra portability…</div>
      <div v-else-if="report" class="portability" :data-level="report.level"><strong>{{ report.level.toUpperCase() }}</strong><span>{{ report.summary }}</span></div>
      <ul v-if="report?.blockers?.length" class="builder-issues"><li v-for="item in report.blockers" :key="item">{{ item }}</li></ul>
      <details v-if="report?.warnings?.length" :open="report?.level === 'yellow'"><summary>{{ report.warnings.length }} yêu cầu cần chuẩn bị ở máy đích</summary><ul class="builder-issues"><li v-for="item in report.warnings" :key="item">{{ item }}</li></ul></details>

      <form class="builder-form" @submit.prevent="build">
        <div class="builder-grid"><label>Tên ứng dụng<input v-model="form.name" class="form-input" required /></label><label>App ID<input v-model="form.id" class="form-input" placeholder="Tự tạo từ tên" /></label><label>Phiên bản<input v-model="form.version" class="form-input" required /></label><label>Màu chủ đạo<input v-model="form.accent" class="form-input" type="color" /></label><label class="wide">Mô tả<textarea v-model="form.description" class="form-input" rows="2" /></label></div>
        <h3>Form nhập liệu</h3>
        <div class="builder-grid"><label>Cách truyền input<select v-model="form.inputMode" class="form-input"><option value="direct">Input trực tiếp</option><option value="webhook_body">Đặt trong body webhook</option></select></label><label>Nút chạy<input v-model="form.submitLabel" class="form-input" /></label></div>
        <div class="field-list"><div v-for="(field, index) in form.fields" :key="index" class="field-row"><input v-model="field.key" class="form-input" placeholder="key" required /><input v-model="field.label" class="form-input" placeholder="Nhãn" required /><select v-model="field.type" class="form-input"><option value="string">Text</option><option value="textarea">Text dài</option><option value="number">Số</option><option value="integer">Số nguyên</option><option value="boolean">Bật/tắt</option><option value="select">Chọn</option><option value="number_list">Danh sách số</option><option value="json">JSON</option></select><label class="required-check"><input v-model="field.required" type="checkbox" /> Bắt buộc</label><button type="button" class="btn btn-secondary" @click="removeField(index)">Xóa</button><input v-if="field.type === 'select'" v-model="field.optionsText" class="form-input field-options" placeholder="Các lựa chọn, cách nhau bằng dấu phẩy" /></div><button type="button" class="btn btn-secondary" @click="addField">+ Thêm trường</button></div>
        <h3>Kết quả</h3>
        <div class="builder-grid"><label>Node output<select v-model="form.outputNodeID" class="form-input"><option value="">Node thành công cuối cùng</option><option v-for="node in nodes" :key="node.id" :value="node.id">{{ nodeLabel(node) }}</option></select></label><label>Kiểu hiển thị<select v-model="form.outputMode" class="form-input"><option value="auto">Tự nhận diện</option><option value="cards">Thẻ chỉ số</option><option value="table">Bảng</option><option value="json">JSON</option></select></label></div>
        <p v-if="error" class="builder-error" role="alert">{{ error }}</p>
        <footer><span>{{ report?.level === 'yellow' ? 'Có thể build. Máy người dùng không cần Go, nhưng phải có các phụ thuộc YELLOW liệt kê phía trên.' : 'Build cho hệ điều hành đang chạy Goflow; không cần Go trên máy người dùng.' }}</span><button class="btn btn-primary" type="submit" :disabled="building || !report?.can_build">{{ building ? 'Đang đóng gói…' : 'Build & tải ứng dụng' }}</button></footer>
      </form>
    </section>
  </div>
</template>

<style scoped>
.builder-backdrop{position:fixed;inset:0;z-index:1200;background:#0f172acc;display:grid;place-items:center;padding:24px}.builder-modal{background:#fff;border:2px solid #0f172a;border-radius:14px;box-shadow:8px 8px 0 #0f172a;width:min(980px,96vw);max-height:92vh;overflow:auto;padding:24px}.builder-modal>header,.builder-modal footer{display:flex;justify-content:space-between;gap:20px;align-items:flex-start}.builder-modal h2{margin:4px 0}.builder-modal h3{margin:22px 0 10px}.builder-modal p{margin:4px 0;color:#475569}.builder-kicker{font-size:12px;font-weight:900;letter-spacing:.12em}.portability{display:flex;gap:12px;padding:12px;margin:18px 0;border:2px solid #0f172a;background:#dcfce7}.portability[data-level="yellow"]{background:#fef3c7}.portability[data-level="red"]{background:#fee2e2}.builder-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.builder-grid label{display:grid;gap:5px;font-weight:700}.builder-grid .wide{grid-column:1/-1}.field-list{display:grid;gap:8px}.field-row{display:grid;grid-template-columns:1.1fr 1.2fr 1fr auto auto;gap:8px;align-items:center}.field-options{grid-column:1/-1}.required-check{white-space:nowrap}.builder-issues{margin:8px 0;color:#7f1d1d}.builder-error{padding:10px;background:#fee2e2;color:#991b1b}.builder-modal footer{margin-top:24px;align-items:center;border-top:1px solid #cbd5e1;padding-top:16px}.builder-modal footer span{font-size:13px;color:#475569}@media(max-width:760px){.builder-grid,.field-row{grid-template-columns:1fr}.builder-grid .wide,.field-options{grid-column:auto}.builder-modal>header,.builder-modal footer{flex-direction:column}}
</style>
