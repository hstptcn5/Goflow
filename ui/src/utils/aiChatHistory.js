const PREFIX = 'goflow.ai.chat.v1';
const MAX_MESSAGES = 80;
const MAX_BYTES = 500_000;

export const AI_WELCOME_MESSAGE = {
  id: 'welcome',
  sender: 'ai',
  type: 'text',
  text: '👋 Tôi là Trợ lý AI của Goflow. Tôi có thể tạo, chỉnh sửa, đánh giá và chạy Agent Lab có giới hạn. Lịch sử chat được lưu cục bộ theo workflow; mọi proposal vẫn cần bạn chủ động nạp lên canvas và lưu.'
};

function storageAvailable() {
  return typeof window !== 'undefined' && window.localStorage;
}

function keyFor(workflowId) {
  const id = String(workflowId || '').trim();
  return id ? `${PREFIX}.${id}` : '';
}

function normalizeMessages(messages) {
  const list = Array.isArray(messages) ? messages.filter((item) => item && item.id !== 'welcome') : [];
  return list.slice(-MAX_MESSAGES);
}

export function loadAIChatHistory(workflowId) {
  const key = keyFor(workflowId);
  if (!key || !storageAvailable()) return [AI_WELCOME_MESSAGE];
  try {
    const parsed = JSON.parse(window.localStorage.getItem(key) || '[]');
    return [AI_WELCOME_MESSAGE, ...normalizeMessages(parsed)];
  } catch {
    return [AI_WELCOME_MESSAGE];
  }
}

export function saveAIChatHistory(workflowId, messages) {
  const key = keyFor(workflowId);
  if (!key || !storageAvailable()) return;
  let list = normalizeMessages(messages);
  try {
    let encoded = JSON.stringify(list);
    while (encoded.length > MAX_BYTES && list.length > 1) {
      list = list.slice(Math.max(1, Math.ceil(list.length / 4)));
      encoded = JSON.stringify(list);
    }
    if (encoded.length <= MAX_BYTES) window.localStorage.setItem(key, encoded);
  } catch {
    // Chat persistence is best-effort and must never break the editor.
  }
}

export function clearAIChatHistory(workflowId) {
  const key = keyFor(workflowId);
  if (key && storageAvailable()) window.localStorage.removeItem(key);
}
