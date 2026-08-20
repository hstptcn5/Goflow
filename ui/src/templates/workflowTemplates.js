import aiAssistant from '../../../templates/workflow_ai_assistant.json';
import githubRepoMonitor from '../../../templates/github_repo_monitor.json';
import weatherAlert from '../../../templates/weather_alert_flow.json';
import multiBranchStressTest from '../../../templates/multi_branch_stress_test.json';
import uptimeIncidentResponse from '../../../templates/uptime_incident_response.json';
import customerSupportAiTriage from '../../../templates/customer_support_ai_triage.json';
import releaseSmokeTest from '../../../templates/release_smoke_test.json';
import dailySalesDigest from '../../../templates/daily_sales_digest.json';
import webhookOrderFraudCheck from '../../../templates/webhook_order_fraud_check.json';
import rssToDiscordDigest from '../../../templates/rss_to_discord_digest.json';
import githubPrReviewReminder from '../../../templates/github_pr_review_reminder.json';
import serverBackupHealthCheck from '../../../templates/server_backup_health_check.json';
import formToNotionAndEmail from '../../../templates/form_to_notion_and_email.json';
import googleSheetsLeadRouter from '../../../templates/google_sheets_lead_router.json';
import redisQueueWorker from '../../../templates/redis_queue_worker.json';
import webhookPayloadValidatorPlugin from '../../../templates/webhook_payload_validator_plugin.json';
import contentModerationPipeline from '../../../templates/content_moderation_pipeline.json';
import incidentPostmortemGenerator from '../../../templates/incident_postmortem_generator.json';
import pluginLeadScoringRouter from '../../../templates/plugin_lead_scoring_router.json';
import apiErrorBudgetMonitor from '../../../templates/api_error_budget_monitor.json';
import customerChurnSignalMonitor from '../../../templates/customer_churn_signal_monitor.json';

export const workflowTemplates = [
  {
    id: 'plugin-lead-scoring-router',
    title: 'Chấm điểm lead bằng Plugin',
    category: 'Plugin',
    difficulty: 'Nâng cao',
    summary: 'Chấm điểm lead đầu vào bằng plugin tùy chỉnh rồi chuyển lead giá trị cao tới đội bán hàng.',
    requirements: ['Plugin lead_scorer đã biên dịch', 'Credential Telegram', 'Máy chủ Redis'],
    workflow: pluginLeadScoringRouter,
  },
  {
    id: 'webhook-payload-validator-plugin',
    title: 'Kiểm tra payload Webhook bằng Plugin',
    category: 'Plugin',
    difficulty: 'Trung bình',
    summary: 'Kiểm tra payload webhook bằng plugin tùy chỉnh trước khi chuyển tiếp hoặc gửi cảnh báo.',
    requirements: ['Plugin payload_validator đã biên dịch', 'Webhook Discord'],
    workflow: webhookPayloadValidatorPlugin,
  },
  {
    id: 'customer-support-ai-triage',
    title: 'AI phân loại yêu cầu hỗ trợ',
    category: 'AI + Hỗ trợ',
    difficulty: 'Nâng cao',
    summary: 'Dùng AI phân loại ticket hỗ trợ rồi chuyển trường hợp khẩn cấp và bình thường sang các kênh khác nhau.',
    requirements: ['Credential DeepSeek', 'Chat ID Telegram', 'Webhook Slack'],
    workflow: customerSupportAiTriage,
  },
  {
    id: 'content-moderation-pipeline',
    title: 'Pipeline kiểm duyệt nội dung',
    category: 'AI + Kiểm duyệt',
    difficulty: 'Trung bình',
    summary: 'Dùng AI kiểm duyệt nội dung người dùng gửi vào và chuyển nội dung không an toàn sang Slack.',
    requirements: ['Credential OpenAI', 'Webhook Slack', 'Máy chủ Redis'],
    workflow: contentModerationPipeline,
  },
  {
    id: 'incident-postmortem-generator',
    title: 'Tạo báo cáo sau sự cố bằng AI',
    category: 'AI + Vận hành',
    difficulty: 'Nâng cao',
    summary: 'Tạo bản nháp báo cáo sau sự cố từ dữ liệu incident, lưu vào Notion và gửi email cho người liên quan.',
    requirements: ['Credential DeepSeek', 'Credential Notion', 'Credential SMTP'],
    workflow: incidentPostmortemGenerator,
  },
  {
    id: 'uptime-incident-response',
    title: 'Giám sát uptime và phản ứng sự cố',
    category: 'Giám sát',
    difficulty: 'Trung bình',
    summary: 'Kiểm tra health endpoint, lưu trạng thái vào Redis và cảnh báo Discord khi dịch vụ không khỏe.',
    requirements: ['Health URL', 'Máy chủ Redis', 'Webhook Discord'],
    workflow: uptimeIncidentResponse,
  },
  {
    id: 'api-error-budget-monitor',
    title: 'Giám sát Error Budget API',
    category: 'Giám sát',
    difficulty: 'Trung bình',
    summary: 'Lấy metrics dịch vụ, đánh giá sức khỏe error budget và cảnh báo khi burn rate quá cao.',
    requirements: ['URL Metrics API', 'Webhook Discord'],
    workflow: apiErrorBudgetMonitor,
  },
  {
    id: 'server-backup-health-check',
    title: 'Kiểm tra sức khỏe bản sao lưu máy chủ',
    category: 'Giám sát',
    difficulty: 'Trung bình',
    summary: 'Chạy kiểm tra backup từ xa qua SSH và gửi cảnh báo khi backup thất bại.',
    requirements: ['Credential SSH', 'Webhook Discord', 'Máy chủ Redis'],
    workflow: serverBackupHealthCheck,
  },
  {
    id: 'release-smoke-test',
    title: 'Smoke test sau khi release',
    category: 'DevOps',
    difficulty: 'Nâng cao',
    summary: 'Pull code, khởi động lại dịch vụ từ xa, kiểm tra health endpoint và thông báo thành công hoặc thất bại.',
    requirements: ['Đường dẫn Git repo', 'Credential SSH', 'Telegram hoặc Discord'],
    workflow: releaseSmokeTest,
  },
  {
    id: 'redis-queue-worker',
    title: 'Worker xử lý hàng đợi Redis',
    category: 'Tác vụ backend',
    difficulty: 'Trung bình',
    summary: 'Đọc hàng đợi Redis, gửi job tới HTTP API và ghi nhận công việc đã xử lý.',
    requirements: ['Máy chủ Redis', 'URL Worker API'],
    workflow: redisQueueWorker,
  },
  {
    id: 'github-pr-review-reminder',
    title: 'Nhắc review Pull Request GitHub',
    category: 'Công cụ lập trình',
    difficulty: 'Cơ bản',
    summary: 'Kiểm tra pull request đang mở và nhắc đội ngũ trên Slack khi còn việc cần review.',
    requirements: ['URL repo GitHub', 'Webhook Slack'],
    workflow: githubPrReviewReminder,
  },
  {
    id: 'daily-sales-digest',
    title: 'Báo cáo bán hàng hằng ngày',
    category: 'Vận hành kinh doanh',
    difficulty: 'Trung bình',
    summary: 'Lấy dữ liệu bán hàng, tổng hợp bằng JavaScript và gửi email báo cáo mỗi ngày.',
    requirements: ['URL Sales API', 'Credential SMTP'],
    workflow: dailySalesDigest,
  },
  {
    id: 'webhook-order-fraud-check',
    title: 'Kiểm tra gian lận đơn hàng từ Webhook',
    category: 'Vận hành kinh doanh',
    difficulty: 'Trung bình',
    summary: 'Phân loại đơn hàng đầu vào và chuyển đơn đáng ngờ sang bước kiểm tra thủ công.',
    requirements: ['Credential Telegram', 'Máy chủ Redis'],
    workflow: webhookOrderFraudCheck,
  },
  {
    id: 'form-to-notion-and-email',
    title: 'Form → Notion → Email',
    category: 'CRM',
    difficulty: 'Trung bình',
    summary: 'Tạo trang Notion từ dữ liệu form website và gửi email xác nhận.',
    requirements: ['Credential Notion', 'Credential SMTP'],
    workflow: formToNotionAndEmail,
  },
  {
    id: 'google-sheets-lead-router',
    title: 'Phân luồng lead bằng Google Sheets',
    category: 'CRM',
    difficulty: 'Trung bình',
    summary: 'Ghi lead đầu vào vào Google Sheets và thông báo đội bán hàng khi gặp lead doanh nghiệp.',
    requirements: ['Credential Google Sheets', 'Webhook Slack'],
    workflow: googleSheetsLeadRouter,
  },
  {
    id: 'customer-churn-signal-monitor',
    title: 'Theo dõi tín hiệu khách hàng sắp rời bỏ',
    category: 'Chăm sóc khách hàng',
    difficulty: 'Trung bình',
    summary: 'Phát hiện hoạt động có nguy cơ churn, tạo follow-up trong Notion và báo cho đội CS.',
    requirements: ['URL Customer API', 'Credential Notion', 'Webhook Slack'],
    workflow: customerChurnSignalMonitor,
  },
  {
    id: 'rss-to-discord-digest',
    title: 'RSS → AI tổng hợp → Discord',
    category: 'Vận hành nội dung',
    difficulty: 'Cơ bản',
    summary: 'Lấy RSS feed, dùng AI tóm tắt rồi đăng bản tổng hợp lên Discord.',
    requirements: ['Credential DeepSeek', 'Webhook Discord'],
    workflow: rssToDiscordDigest,
  },
  {
    id: 'weather-alert',
    title: 'Cảnh báo thời tiết',
    category: 'Tự động hóa API',
    difficulty: 'Cơ bản',
    summary: 'Lấy dữ liệu thời tiết trực tiếp từ Open-Meteo và rẽ nhánh theo trạng thái lấy dữ liệu.',
    requirements: ['Kết nối Internet'],
    workflow: weatherAlert,
  },
  {
    id: 'github-repo-monitor',
    title: 'Theo dõi repository GitHub',
    category: 'Công cụ lập trình',
    difficulty: 'Cơ bản',
    summary: 'Theo lịch, gọi GitHub API và xử lý trạng thái repository.',
    requirements: ['URL GitHub API'],
    workflow: githubRepoMonitor,
  },
  {
    id: 'ai-assistant-pipeline',
    title: 'Pipeline xử lý văn bản bằng AI',
    category: 'AI',
    difficulty: 'Cơ bản',
    summary: 'Nhận webhook, chuẩn bị prompt, gọi DeepSeek và định dạng kết quả.',
    requirements: ['Khóa API DeepSeek'],
    workflow: aiAssistant,
  },
  {
    id: 'multi-branch-stress-test',
    title: 'Stress test workflow nhiều nhánh',
    category: 'Kiểm thử',
    difficulty: 'Trung bình',
    summary: 'Kiểm tra các nhánh workflow chạy song song với nhiều yêu cầu HTTP.',
    requirements: ['Kết nối Internet'],
    workflow: multiBranchStressTest,
  },
];
