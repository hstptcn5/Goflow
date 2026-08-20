const NODE_VI = {
  webhookTrigger: ['Kích hoạt Webhook', 'Khởi chạy workflow khi Goflow nhận yêu cầu HTTP webhook.'],
  cronTrigger: ['Lịch Cron', 'Khởi chạy workflow tự động theo biểu thức lịch Cron.'],
  manualTrigger: ['Chạy thủ công', 'Khởi chạy workflow thủ công từ giao diện hoặc API.'],
  githubWebhook: ['Webhook GitHub', 'Nhận sự kiện webhook từ GitHub và xác minh chữ ký khi được cấu hình.'],
  httpRequest: ['Yêu cầu HTTP', 'Gửi yêu cầu HTTP tới API hoặc dịch vụ bên ngoài.'],
  sourcePolicy: ['Chính sách nguồn', 'Kiểm tra nguồn dữ liệu theo chính sách trước khi xử lý tiếp.'],
  aiExtract: ['AI Trích xuất', 'Dùng OpenAI hoặc DeepSeek để trích xuất dữ liệu có cấu trúc từ đầu vào.'],
  zaloOA: ['Zalo OA', 'Gửi tin nhắn qua Zalo Official Account.'],
  telegramBot: ['Telegram Bot', 'Gửi tin nhắn tới Telegram bằng bot.'],
  discordBot: ['Webhook Discord', 'Gửi thông báo tới kênh Discord.'],
  slackBot: ['Webhook Slack', 'Gửi thông báo tới kênh Slack.'],
  emailSMTP: ['Email SMTP', 'Gửi email qua máy chủ SMTP.'],
  googleSheets: ['Google Sheets', 'Đọc hoặc ghi dữ liệu Google Sheets bằng thông tin xác thực đã lưu.'],
  googleDrive: ['Google Drive', 'Liệt kê hoặc tải tệp lên Google Drive.'],
  gmailREST: ['Gmail REST', 'Gửi email qua Gmail REST API.'],
  notionPage: ['Trang Notion', 'Tạo hoặc cập nhật trang trong cơ sở dữ liệu Notion.'],
  jsonTransform: ['Biến đổi JSON', 'Tạo JSON đầu ra từ mẫu và biến của các node trước.'],
  jsCodeRunner: ['Chạy JavaScript', 'Chạy JavaScript trong sandbox để biến đổi dữ liệu.'],
  conditionIf: ['Điều kiện IF / ELSE', 'Rẽ nhánh workflow theo điều kiện so sánh.'],
  delaySleep: ['Trì hoãn', 'Tạm dừng nhánh workflow trong khoảng thời gian cấu hình.'],
  subWorkflow: ['Workflow con', 'Chạy một workflow khác như một bước trong workflow hiện tại.'],
  openAIGPT: ['OpenAI GPT', 'Gửi prompt và ngữ cảnh tới OpenAI API.'],
  deepseekAI: ['DeepSeek AI', 'Gửi prompt và ngữ cảnh tới DeepSeek API.'],
  postgresQuery: ['Truy vấn PostgreSQL', 'Chạy truy vấn SQL trên PostgreSQL.'],
  mysqlQuery: ['Truy vấn MySQL', 'Chạy truy vấn SQL trên MySQL.'],
  mongodbCommand: ['Lệnh MongoDB', 'Chạy thao tác trên MongoDB.'],
  redisCommand: ['Lệnh Redis', 'Đọc hoặc ghi dữ liệu Redis.'],
  sshRunner: ['Chạy SSH', 'Kết nối máy chủ từ xa và chạy lệnh shell qua SSH.'],
  gitCommand: ['Lệnh Git', 'Chạy thao tác Git bằng Git CLI.'],
  goflowPlugin: ['Plugin Goflow', 'Chạy plugin native bên ngoài qua JSON IPC.'],
};

const PARAM_VI = {
  method: ['Phương thức HTTP', 'Phương thức HTTP cần gửi.'],
  url: ['URL đích', 'Địa chỉ endpoint đích.'],
  headers: ['Header (JSON)', 'Header không nhạy cảm dưới dạng JSON.'],
  body: ['Nội dung yêu cầu', 'Body dùng cho POST, PUT hoặc PATCH.'],
  credential_id: ['Thông tin xác thực', 'Chọn credential đã mã hóa từ kho bí mật.'],
  credential_header: ['Header credential', 'Tên header nhận credential khi chạy.'],
  credential_prefix: ['Tiền tố credential', 'Tiền tố đặt trước secret, ví dụ Bearer.'],
  response_contract: ['Hợp đồng phản hồi JSON', 'Ràng buộc tùy chọn cho phản hồi JSON.'],
  provider: ['Nhà cung cấp AI', 'Chọn nhà cung cấp mô hình AI.'],
  model: ['Mô hình', 'Tên mô hình được gọi.'],
  input: ['Đầu vào', 'Dữ liệu đầu vào cho node.'],
  input_type: ['Kiểu đầu vào', 'Cách diễn giải dữ liệu đầu vào.'],
  instructions: ['Yêu cầu trích xuất', 'Mô tả chính xác dữ liệu cần trích xuất.'],
  schema: ['Schema JSON', 'Schema dùng để kiểm tra đầu ra có cấu trúc.'],
  json_schema: ['Schema JSON', 'Schema dùng để kiểm tra đầu ra có cấu trúc.'],
  chat_id: ['Chat ID', 'ID cuộc trò chuyện hoặc nhóm đích.'],
  message: ['Tin nhắn', 'Nội dung tin nhắn cần gửi.'],
  secret: ['Secret webhook', 'Secret dùng để xác thực webhook khi được cấu hình.'],
  path: ['Đường dẫn webhook', 'Đường dẫn con của webhook.'],
  cron: ['Biểu thức Cron', 'Lịch Cron chuẩn.'],
  expression: ['Biểu thức', 'Biểu thức dùng để tính hoặc so sánh.'],
  code: ['Mã JavaScript', 'Đoạn JavaScript chạy trong sandbox.'],
  spreadsheet_id: ['Spreadsheet ID', 'ID bảng tính trong URL Google Sheets.'],
  sheet_name: ['Tên sheet / phạm vi', 'Tên sheet hoặc phạm vi dữ liệu.'],
  action: ['Thao tác', 'Thao tác cần thực hiện.'],
  values_json: ['Mảng giá trị', 'Mảng JSON chứa các cột cần ghi.'],
  service_account_json: ['Service Account JSON', 'Không nên dán trực tiếp. Hãy dùng credential đã mã hóa.'],
};

const EXACT_VI = new Map(Object.entries({
  'Workflows': 'Workflow',
  'Executions': 'Lịch sử chạy',
  'Credentials': 'Thông tin xác thực',
  'Templates': 'Mẫu workflow',
  'Nodes': 'Danh sách node',
  'Settings': 'Cài đặt',
  'Help': 'Trợ giúp',
  'Workspace': 'Không gian làm việc',
  'Back': 'Quay lại',
  'Active': 'Đang bật',
  'Inactive': 'Đang tắt',
  'Saved': 'Đã lưu',
  'Saving': 'Đang lưu',
  'Save failed': 'Lưu thất bại',
  'Unsaved changes': 'Có thay đổi chưa lưu',
  'Test Workflow': 'Chạy thử workflow',
  'Testing...': 'Đang chạy thử...',
  'Save': 'Lưu',
  'More actions': 'Thêm thao tác',
  'Hide node library': 'Ẩn thư viện node',
  'Show node library': 'Hiện thư viện node',
  'Undo': 'Hoàn tác',
  'Redo': 'Làm lại',
  'Duplicate': 'Nhân bản',
  'Auto-layout': 'Tự sắp xếp',
  'Export': 'Xuất workflow',
  'Workflow settings': 'Cài đặt workflow',
  'Interface settings': 'Cài đặt giao diện',
  'Delete': 'Xóa',
  'Add step': 'Thêm bước',
  'AI Assistant': 'Trợ lý AI',
  'No nodes yet': 'Chưa có node',
  'Add a first step or start from a template.': 'Thêm bước đầu tiên hoặc bắt đầu từ một mẫu workflow.',
  'Use Add first step to search nodes without relying on the side palette.': 'Dùng “Thêm bước đầu tiên” để tìm node nhanh mà không cần mở thư viện bên cạnh.',
  'Add first step': 'Thêm bước đầu tiên',
  'Browse Templates': 'Xem mẫu workflow',
  'Use AI assistant': 'Dùng trợ lý AI',
  'Retry selected execution': 'Chạy lại lần đã chọn',
  'Replay on current workflow': 'Phát lại trên workflow hiện tại',
  'Cancel execution': 'Hủy lần chạy',
  'Copy debug bundle': 'Sao chép gói debug',
  'Debug bundle preview': 'Xem trước gói debug',
  'Load Template Into Canvas': 'Nạp mẫu vào canvas',
  'Load Template': 'Nạp mẫu',
  'Primary navigation': 'Điều hướng chính',
  'Live updates connected': 'Đã kết nối cập nhật trực tiếp',
  'Live updates disconnected': 'Mất kết nối cập nhật trực tiếp',
  'Reconnect': 'Kết nối lại',
  'Create credential': 'Tạo thông tin xác thực',
  'Name': 'Tên',
  'Provider template': 'Mẫu nhà cung cấp',
  'Provider ID': 'ID nhà cung cấp',
  'Authentication kind': 'Kiểu xác thực',
  'API Key': 'Khóa API',
  'Bearer / access token': 'Bearer / access token',
  'Basic Auth payload': 'Dữ liệu Basic Auth',
  'Service account payload': 'Dữ liệu service account',
  'Custom secret payload': 'Dữ liệu secret tùy chỉnh',
  'Save credential': 'Lưu credential',
  'No credentials saved': 'Chưa lưu credential nào',
  'Provider': 'Nhà cung cấp',
  'Auth kind': 'Kiểu xác thực',
  'Actions': 'Thao tác',
  'Retry': 'Thử lại',
  'Build / Edit': 'Tạo / Chỉnh sửa',
  'Review Workflow': 'Đánh giá workflow',
  'Review Latest Run': 'Đánh giá lần chạy gần nhất',
  'AI brain:': 'Bộ não AI:',
  '-- Choose OpenAI / DeepSeek Credential --': '-- Chọn credential OpenAI / DeepSeek --',
  'Human-gated review.': 'Đánh giá có xác nhận của người dùng.',
  'Review': 'Đánh giá',
  'Workflow Review': 'Đánh giá workflow',
  'Latest Run Review': 'Đánh giá lần chạy gần nhất',
  'Validated improvement proposal': 'Đề xuất cải thiện đã được kiểm tra',
  'Apply proposal to canvas': 'Áp dụng đề xuất lên canvas',
  'Reliability': 'Độ tin cậy',
  'Security': 'Bảo mật',
  'Data': 'Dữ liệu',
  'Cost': 'Chi phí',
  'Maintain': 'Dễ bảo trì',
  'Output': 'Chất lượng đầu ra',
  'Why:': 'Lý do:',
  'Impact:': 'Tác động:',
  'Suggested:': 'Đề xuất:',
  'Expected:': 'Kỳ vọng:',
  'HIGH': 'CAO',
  'MEDIUM': 'TRUNG BÌNH',
  'LOW': 'THẤP',
  'SUCCESS': 'THÀNH CÔNG',
  'FAILED': 'THẤT BẠI',
  'SKIPPED': 'BỎ QUA',
  'RUNNING': 'ĐANG CHẠY',
  'QUEUED': 'ĐANG CHỜ',
  'NOT_RUN': 'CHƯA CHẠY',
  'Configured': 'Đã cấu hình',
  'Unknown': 'Không xác định',
  'Read': 'Đọc',
  'Append': 'Thêm dòng',
  'READ': 'ĐỌC',
  'APPEND': 'THÊM DÒNG',
  'None.': 'Không có.',
}));

const ATTRS = ['placeholder', 'title', 'aria-label'];
const SKIP_TAGS = new Set(['SCRIPT', 'STYLE', 'CODE', 'PRE']);

export function localizeNodeDefinitions(definitions = []) {
  return (definitions || []).map((definition) => {
    const translated = NODE_VI[definition.type];
    return {
      ...definition,
      name: translated?.[0] || definition.name,
      description: translated?.[1] || definition.description,
      params: (definition.params || []).map((param) => {
        const paramVi = PARAM_VI[param.name];
        return {
          ...param,
          label: paramVi?.[0] || param.label,
          description: paramVi?.[1] || param.description,
        };
      }),
    };
  });
}

export function viNodeName(type, fallback = '') {
  return NODE_VI[type]?.[0] || fallback || type;
}

export function translateOperatorText(value) {
  if (typeof value !== 'string') return value;
  const trimmed = value.trim();
  if (!trimmed) return value;
  const exact = EXACT_VI.get(trimmed);
  if (exact) return value.replace(trimmed, exact);

  let match = trimmed.match(/^(\d+) findings$/i);
  if (match) return `${match[1]} phát hiện`;
  match = trimmed.match(/^(\d+) issues? need attention$/i);
  if (match) return `${match[1]} vấn đề cần xử lý`;
  match = trimmed.match(/^Latest execution\s+(.+)$/i);
  if (match) return `Lần chạy gần nhất ${match[1]}`;
  match = trimmed.match(/^Selected execution\s+(.+)$/i);
  if (match) return `Lần chạy đã chọn ${match[1]}`;
  match = trimmed.match(/^Live execution\s+(.+)$/i);
  if (match) return `Lần chạy trực tiếp ${match[1]}`;
  match = trimmed.match(/^Cancel\s+(.+)$/i);
  if (match) return `Hủy ${match[1]}`;
  match = trimmed.match(/^Name:\s*(.+)$/i);
  if (match) return `Tên: ${match[1]}`;
  return value;
}

function shouldSkip(node) {
  const parent = node?.parentElement || (node?.nodeType === 1 ? node : null);
  if (!parent) return false;
  if (SKIP_TAGS.has(parent.tagName)) return true;
  return Boolean(parent.closest?.('[data-no-translate], .debug-bundle-preview'));
}

function translateElement(element) {
  if (!element || element.nodeType !== 1) return;
  for (const attr of ATTRS) {
    const current = element.getAttribute?.(attr);
    if (!current) continue;
    const next = translateOperatorText(current);
    if (next !== current) element.setAttribute(attr, next);
  }
}

function translateTree(root) {
  if (!root) return;
  if (root.nodeType === 3 && !shouldSkip(root)) {
    const next = translateOperatorText(root.nodeValue || '');
    if (next !== root.nodeValue) root.nodeValue = next;
    return;
  }
  if (root.nodeType !== 1 && root.nodeType !== 9 && root.nodeType !== 11) return;
  if (root.nodeType === 1) translateElement(root);
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
  let node = walker.nextNode();
  while (node) {
    if (node.nodeType === 1) translateElement(node);
    else if (!shouldSkip(node)) {
      const next = translateOperatorText(node.nodeValue || '');
      if (next !== node.nodeValue) node.nodeValue = next;
    }
    node = walker.nextNode();
  }
}

export function installVietnameseOperatorUI() {
  if (typeof document === 'undefined' || typeof MutationObserver === 'undefined') return () => {};
  const root = document.body;
  if (!root) return () => {};
  translateTree(root);
  const observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type === 'characterData') translateTree(mutation.target);
      mutation.addedNodes?.forEach((node) => translateTree(node));
      if (mutation.type === 'attributes') translateElement(mutation.target);
    }
  });
  observer.observe(root, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATTRS });
  return () => observer.disconnect();
}
