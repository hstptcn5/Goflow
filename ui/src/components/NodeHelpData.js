export const nodeHelpMap = {
  webhookTrigger: {
    title: 'Kích hoạt Webhook',
    desc: 'Khởi chạy workflow khi Goflow nhận yêu cầu HTTP tại endpoint webhook của workflow.',
    inputs: '- Đường dẫn webhook (tùy chọn)\n- Secret webhook (khuyến nghị nếu endpoint được public)',
    output: `{
  "headers": { "Content-Type": "application/json" },
  "query": { "source": "api" },
  "body": { "event": "user_signup" }
}`,
  },
  cronTrigger: {
    title: 'Lịch Cron',
    desc: 'Khởi chạy workflow tự động theo lịch Cron chuẩn.',
    inputs: '- Biểu thức Cron, ví dụ */5 * * * * để chạy mỗi 5 phút',
    output: `{
  "triggered_at": "2026-07-23T08:00:00Z",
  "schedule": "*/5 * * * *"
}`,
  },
  githubWebhook: {
    title: 'Webhook GitHub',
    desc: 'Nhận sự kiện từ GitHub và hỗ trợ xác minh chữ ký webhook HMAC SHA-256.',
    inputs: '- Secret webhook GitHub',
    output: `{
  "event": "push",
  "payload": { "ref": "refs/heads/main" }
}`,
  },
  sourcePolicy: {
    title: 'Chính sách nguồn',
    desc: 'Kiểm tra nguồn dữ liệu trước khi cho phép workflow xử lý tiếp. Dùng node này để ghi rõ loại nguồn, mức rủi ro và cách sử dụng được phép.',
    inputs: '- URL hoặc thông tin nguồn\n- Loại nguồn / chính sách áp dụng\n- Metadata cần kiểm tra',
    output: `{
  "allowed": true,
  "source_type": "official_api",
  "risk": "low"
}`,
  },
  aiExtract: {
    title: 'AI Trích xuất',
    desc: 'Dùng OpenAI hoặc DeepSeek để biến văn bản/JSON đầu vào thành dữ liệu có cấu trúc theo schema.',
    inputs: '- Nhà cung cấp: OpenAI hoặc DeepSeek\n- Credential đã mã hóa đúng nhà cung cấp\n- Đầu vào\n- Yêu cầu trích xuất\n- JSON Schema',
    output: `{
  "data": { "field": "value" },
  "provider": "deepseek"
}`,
  },
  zaloOA: {
    title: 'Zalo OA',
    desc: 'Gửi tin nhắn qua Zalo Official Account. Cần credential/token Zalo hợp lệ.',
    inputs: '- Credential Zalo OA\n- Người nhận / target\n- Nội dung tin nhắn',
    output: `{
  "status": "sent"
}`,
  },
  postgresQuery: {
    title: 'Truy vấn PostgreSQL',
    desc: 'Chạy câu lệnh SQL trên PostgreSQL bên ngoài.',
    inputs: '- Connection URI\n- Câu lệnh SQL',
    output: `{
  "rows": [{ "id": 1, "name": "Alice" }],
  "rows_affected": 1
}`,
  },
  mysqlQuery: {
    title: 'Truy vấn MySQL',
    desc: 'Chạy câu lệnh SQL trên MySQL từ xa.',
    inputs: '- Connection URI\n- Câu lệnh SQL',
    output: `{
  "rows": [{ "id": 1, "name": "Alice" }],
  "rows_affected": 1
}`,
  },
  mongodbCommand: {
    title: 'Lệnh MongoDB',
    desc: 'Chạy các thao tác FindOne, InsertOne, UpdateOne hoặc DeleteOne trên MongoDB.',
    inputs: '- Connection URI\n- Database và collection\n- Query JSON',
    output: `{
  "matched_count": 1,
  "modified_count": 1
}`,
  },
  redisCommand: {
    title: 'Lệnh Redis',
    desc: 'Đọc hoặc ghi dữ liệu trong Redis.',
    inputs: '- Địa chỉ và mật khẩu\n- Lệnh GET, SET, DEL, HGET, HSET, LPUSH, LPOP\n- Key và value nếu cần',
    output: `{
  "command": "GET",
  "key": "user:99",
  "result": "value"
}`,
  },
  googleSheets: {
    title: 'Google Sheets',
    desc: 'Đọc hoặc thêm dòng vào Google Sheets. Nên dùng credential đã mã hóa thay vì dán Service Account JSON trực tiếp vào node.',
    inputs: '- Credential\n- Spreadsheet ID\n- Tên sheet / phạm vi\n- Thao tác READ hoặc APPEND\n- Mảng giá trị JSON',
    output: `{
  "range": "Sheet1!A1:B2",
  "values": [["Name", "Role"], ["Alice", "Engineer"]]
}`,
  },
  googleDrive: {
    title: 'Google Drive',
    desc: 'Liệt kê tệp hoặc tải tệp lên Google Drive.',
    inputs: '- Credential\n- Thao tác\n- Tên tệp và nội dung nếu tải lên',
    output: `{
  "file_id": "19c8828b812b...",
  "name": "report.txt"
}`,
  },
  gmailREST: {
    title: 'Gmail REST',
    desc: 'Gửi email qua Gmail REST API.',
    inputs: '- Credential\n- Email người nhận\n- Tiêu đề\n- Nội dung HTML',
    output: `{
  "message_id": "18c8d8c227cc8f8f",
  "status": "SENT"
}`,
  },
  notionPage: {
    title: 'Trang Notion',
    desc: 'Tạo hoặc cập nhật trang trong database Notion.',
    inputs: '- Credential Notion\n- Database ID\n- Tiêu đề và properties JSON',
    output: `{
  "page_id": "c8e88bb8-2a88-4c88-88aa-8ff288ccee12",
  "status": "CREATED"
}`,
  },
  emailSMTP: {
    title: 'Email SMTP',
    desc: 'Gửi email HTML/text qua máy chủ SMTP.',
    inputs: '- Host và port\n- Tài khoản xác thực\n- From / To\n- Tiêu đề và nội dung',
    output: `{
  "status": "sent",
  "to": "client@gmail.com"
}`,
  },
  telegramBot: {
    title: 'Telegram Bot',
    desc: 'Gửi thông báo tới chat hoặc nhóm Telegram.',
    inputs: '- Bot Token / credential\n- Chat ID\n- Nội dung tin nhắn',
    output: `{
  "ok": true,
  "message_id": 887
}`,
  },
  discordBot: {
    title: 'Webhook Discord',
    desc: 'Gửi thông báo tới kênh Discord.',
    inputs: '- Webhook URL\n- Nội dung\n- Tên bot tùy chọn',
    output: `{
  "status": "ok",
  "statusCode": 204
}`,
  },
  slackBot: {
    title: 'Webhook Slack',
    desc: 'Gửi thông báo tới kênh Slack.',
    inputs: '- Webhook URL\n- Tin nhắn hoặc JSON blocks',
    output: `{
  "status": "ok",
  "statusCode": 200
}`,
  },
  jsCodeRunner: {
    title: 'Chạy JavaScript',
    desc: 'Chạy JavaScript trong sandbox để biến đổi dữ liệu.',
    inputs: '- Mã JavaScript\n- Timeout tối đa',
    output: 'Trả về giá trị return của đoạn mã.',
  },
  subWorkflow: {
    title: 'Workflow con',
    desc: 'Chạy một workflow khác tuần tự hoặc theo từng phần tử đầu vào.',
    inputs: '- ID workflow con\n- Payload đầu vào\n- Chế độ lặp / song song\n- Giới hạn đồng thời',
    output: 'Mảng kết quả của các lần chạy workflow con.',
  },
  conditionIf: {
    title: 'Điều kiện IF / ELSE',
    desc: 'Rẽ nhánh workflow theo phép so sánh.',
    inputs: '- Giá trị đầu vào\n- Toán tử\n- Giá trị so sánh',
    output: `{
  "result": true,
  "target_handle": "true"
}`,
  },
  delaySleep: {
    title: 'Trì hoãn',
    desc: 'Tạm dừng nhánh workflow trong số giây được cấu hình.',
    inputs: '- Số giây trì hoãn',
    output: `{
  "delayed_seconds": 3
}`,
  },
  jsonTransform: {
    title: 'Biến đổi JSON',
    desc: 'Dùng template và biến từ node trước để tạo JSON đầu ra.',
    inputs: '- Mẫu JSON, ví dụ {"val": "{{ prev_node.val }}"}',
    output: 'JSON đã được render và parse.',
  },
  goflowPlugin: {
    title: 'Plugin Goflow',
    desc: 'Chạy executable trong thư mục plugins qua JSON IPC.',
    inputs: '- Tên executable plugin',
    output: 'JSON trả về từ stdout của plugin.',
  },
  openAIGPT: {
    title: 'OpenAI GPT',
    desc: 'Gửi prompt và context tới OpenAI API.',
    inputs: '- Credential OpenAI\n- Model\n- Prompt\n- System message',
    output: 'Nội dung phản hồi của model OpenAI.',
  },
  deepseekAI: {
    title: 'DeepSeek AI',
    desc: 'Gửi prompt và context tới DeepSeek API.',
    inputs: '- Credential DeepSeek\n- Model\n- Prompt\n- System message tùy chọn',
    output: 'Nội dung phản hồi của model DeepSeek.',
  },
  sshRunner: {
    title: 'Chạy SSH',
    desc: 'Kết nối máy Linux từ xa để chạy lệnh shell.',
    inputs: '- Host và port\n- Username\n- Kiểu xác thực\n- Credential\n- Lệnh shell',
    output: `{
  "stdout": "...",
  "stderr": "",
  "exit_code": 0
}`,
  },
  gitCommand: {
    title: 'Lệnh Git',
    desc: 'Chạy Clone, Pull hoặc CommitAndPush qua Git CLI.',
    inputs: '- Đường dẫn repo\n- Lệnh Git\n- URL repo / commit message nếu cần',
    output: `{
  "output": "Already up to date.",
  "status": "success"
}`,
  },
  httpRequest: {
    title: 'Yêu cầu HTTP',
    desc: 'Gửi GET, POST, PUT, DELETE hoặc PATCH tới dịch vụ ngoài. Output chuẩn của Goflow luôn có status_code, headers và data.',
    inputs: '- method\n- url\n- headers (JSON)\n- body nếu cần\n- credential nếu API cần xác thực',
    output: `{
  "status_code": 200,
  "headers": { ... },
  "data": { ... }
}`,
  },
};
