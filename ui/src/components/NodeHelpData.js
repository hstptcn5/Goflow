export const nodeHelpMap = {
  webhookTrigger: {
    title: "Webhook Trigger",
    desc: "Starts workflow execution when Goflow receives an incoming HTTP POST request at `/webhook/{workflowId}`.",
    inputs: "None.",
    output: `{
  "headers": { "Content-Type": "application/json" },
  "query": { "source": "api" },
  "body": { "event": "user_signup" }
}`
  },
  cronTrigger: {
    title: "Cron Trigger",
    desc: "Triggers workflow automatically based on standard Cron expression schedule.",
    inputs: "- Cron Expression: Cron pattern (e.g. */5 * * * * for every 5 mins)",
    output: `{
  "triggered_at": "2026-07-23T08:00:00Z",
  "schedule": "*/5 * * * *"
}`
  },
  githubWebhook: {
    title: "GitHub Webhook Trigger",
    desc: "Listens to GitHub repository events with HMAC SHA-256 webhook signature verification.",
    inputs: "- Secret: GitHub webhook secret token",
    output: `{
  "event": "push",
  "payload": {
    "ref": "refs/heads/main",
    "repository": { "name": "goflow" }
  }
}`
  },
  postgresQuery: {
    title: "PostgreSQL Query",
    desc: "Executes raw SQL query scripts against external PostgreSQL database.",
    inputs: "- Connection URI: postgres://user:pass@host:5432/db\n- Query: SQL command",
    output: `{
  "rows": [
    { "id": 1, "name": "Alice", "role": "admin" }
  ],
  "rows_affected": 1
}`
  },
  mysqlQuery: {
    title: "MySQL Query",
    desc: "Executes raw SQL query scripts against remote MySQL database.",
    inputs: "- Connection URI: user:pass@tcp(host:3306)/db\n- Query: SQL command",
    output: `{
  "rows": [
    { "id": 1, "name": "Alice" }
  ],
  "rows_affected": 1
}`
  },
  mongodbCommand: {
    title: "MongoDB Command",
    desc: "Runs collection operations (FindOne, InsertOne, UpdateOne, DeleteOne) on MongoDB.",
    inputs: "- Connection URI: mongodb://host:27017\n- Database & Collection names\n- Query JSON: raw query settings",
    output: `{
  "matched_count": 1,
  "modified_count": 1,
  "data": { "name": "Alice" }
}`
  },
  redisCommand: {
    title: "Redis Command",
    desc: "Interacts with Redis key-value store database.",
    inputs: "- Address & Password\n- Command: GET, SET, DEL, HGET, HSET, LPUSH, LPOP\n- Key & optional Value parameters",
    output: `{
  "command": "GET",
  "key": "user:99",
  "result": "{\\"name\\": \\"Alice\\"}"
}`
  },
  googleSheets: {
    title: "Google Sheets",
    desc: "Appends rows or reads spreadsheet ranges via Google Service Account or OAuth2.",
    inputs: "- Credential ID\n- Spreadsheet ID\n- Range: Sheet1!A1:D\n- Operation: Read, Append\n- Values JSON: e.g. [[\"Alice\", \"Engineer\"]]",
    output: `{
  "range": "Sheet1!A1:B2",
  "values": [["Name", "Role"], ["Alice", "Engineer"]]
}`
  },
  googleDrive: {
    title: "Google Drive",
    desc: "Uploads files or lists directories inside Google Drive workspace.",
    inputs: "- Credential ID\n- Operation: ListFiles, UploadFile\n- Filename & Text file content",
    output: `{
  "file_id": "19c8828b812b...",
  "name": "report.txt",
  "webViewLink": "https://drive.google.com/..."
}`
  },
  gmailREST: {
    title: "Gmail REST",
    desc: "Sends HTML rich email using the official Google Gmail REST API.",
    inputs: "- Credential ID\n- To: recipient email address\n- Subject: email title\n- Body HTML: HTML content",
    output: `{
  "message_id": "18c8d8c227cc8f8f",
  "status": "SENT"
}`
  },
  notionPage: {
    title: "Notion Page",
    desc: "Creates database pages or updates elements in Notion databases.",
    inputs: "- Credential ID (Notion Token)\n- Database ID\n- Title & custom properties JSON schema",
    output: `{
  "page_id": "c8e88bb8-2a88-4c88-88aa-8ff288ccee12",
  "url": "https://notion.so/...",
  "status": "CREATED"
}`
  },
  emailSMTP: {
    title: "SMTP Email",
    desc: "Sends rich HTML/text email using standard SMTP server configurations.",
    inputs: "- Host & Port\n- Username & Password\n- From & To headers\n- Subject & Body content",
    output: `{
  "status": "sent",
  "to": "client@gmail.com",
  "sent_at": "2026-07-23T08:15:00Z"
}`
  },
  telegramBot: {
    title: "Telegram Bot",
    desc: "Sends rich notification text messages to groups or chats via Telegram API.",
    inputs: "- Bot Token: Bot token from @BotFather\n- Chat ID: Chat ID or Group ID\n- Message: Markdown/HTML text body",
    output: `{
  "ok": true,
  "message_id": 887,
  "chat_title": "Ops Alerts Group"
}`
  },
  discordBot: {
    title: "Discord Webhook",
    desc: "Sends notification strings and rich embeds to Discord server channels.",
    inputs: "- Webhook URL\n- Content: Message body\n- Username: Bot custom display name",
    output: `{
  "status": "ok",
  "statusCode": 204
}`
  },
  slackBot: {
    title: "Slack Webhook",
    desc: "Sends formatted messages and blocks layout payloads to Slack channels.",
    inputs: "- Webhook URL\n- Message: markdown or JSON blocks",
    output: `{
  "status": "ok",
  "statusCode": 200
}`
  },
  jsCodeRunner: {
    title: "JS Code Runner",
    desc: "Executes custom JavaScript ES5 sandboxed expressions to transform variables.",
    inputs: "- JavaScript Code block\n- Timeout (Seconds): Max execution limit (default 5s)",
    output: "Evaluates the return value of your code, e.g.:\n{\n  \"status\": \"processed\",\n  \"total_items\": 42\n}"
  },
  subWorkflow: {
    title: "Sub-workflow Runner",
    desc: "Executes another child workflow sequentially or in loop parallel mode.",
    inputs: "- Sub-workflow ID\n- Input Payload (JSON)\n- Loop mode: Run for each item in array\n- Parallel: Run concurrent goroutines\n- Concurrency Limit: max parallel runs (default 5)",
    output: "Array of child executions returned outputs:\n[\n  { \"node_1\": { \"status\": \"ok\" } }\n]"
  },
  conditionIf: {
    title: "IF / ELSE Condition",
    desc: "Branches workflow execution graph paths based on operators comparison.",
    inputs: "- Input Value: Source field (e.g. {{ node_id.status }})\n- Operator: equals, not_equals, contains, is_not_empty\n- Compare Value: value to compare",
    output: `{
  "result": true,
  "target_handle": "true",
  "evaluated": "'FETCHED' equals 'FETCHED'"
}`
  },
  delaySleep: {
    title: "Delay / Sleep",
    desc: "Pauses workflow execution path for configured seconds duration.",
    inputs: "- Delay Duration (Seconds): Pause time limit",
    output: `{
  "delayed_seconds": 3,
  "resumed_at": "2026-07-23T08:50:00Z"
}`
  },
  jsonTransform: {
    title: "JSON Transform",
    desc: "Parses template strings with variables to render dynamic JSON output structures.",
    inputs: "- JSON Template: raw template text (e.g. {\"val\": \"{{ prev_node.val }}\"})",
    output: "Returns parsed JSON object containing rendered data."
  },
  goflowPlugin: {
    title: "Goflow Plugin",
    desc: "Launches external native executable binary inside `./plugins/` via JSON IPC.",
    inputs: "- Plugin Executable Name (e.g., custom_action.exe)",
    output: "Returns parsed JSON payload output returned by standard output (stdout)."
  },
  openAIGPT: {
    title: "OpenAI GPT",
    desc: "Sends prompts and context to OpenAI API models.",
    inputs: "- API Key\n- Model: gpt-4o, gpt-4-turbo, gpt-3.5-turbo\n- Prompt: user request\n- System Message: role settings",
    output: `{
  "choices": [
    { "message": { "content": "AI answer..." } }
  ]
}`
  },
  deepseekAI: {
    title: "DeepSeek AI",
    desc: "Calls DeepSeek chat reasoning endpoints for cost-efficient intelligence.",
    inputs: "- API Key\n- Model: deepseek-chat, deepseek-reasoner\n- Prompt & optional System Message",
    output: `{
  "choices": [
    { "message": { "content": "AI reasoning..." } }
  ]
}`
  },
  sshRunner: {
    title: "SSH Runner",
    desc: "Connects to remote Linux servers to execute shell terminal commands.",
    inputs: "- Host & Port\n- Username\n- Auth Method: Password or PrivateKey\n- Password / PEM Certificate\n- Command: shell command",
    output: `{
  "stdout": "Command output...",
  "stderr": "",
  "exit_code": 0
}`
  },
  gitCommand: {
    title: "Git Command",
    desc: "Triggers local Git operations (Clone, Pull, CommitAndPush) via Git CLI.",
    inputs: "- Repository Local Path\n- Command: Clone, Pull, CommitAndPush\n- Repo URL & optional Commit Message",
    output: `{
  "output": "Already up to date.",
  "status": "success"
}`
  },
  httpRequest: {
    title: "HTTP Request",
    desc: "Sends HTTP API requests (GET, POST, PUT, DELETE) to external services.",
    inputs: "- Method: GET, POST, etc.\n- URL: Target API endpoint\n- Headers/Body configuration",
    output: `{
  "status": "success",
  "statusCode": 200,
  "body": { ... }
}`
  }
};
