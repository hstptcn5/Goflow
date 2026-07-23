# Goflow Node Guide / Huong dan Node Goflow

This guide explains how to use Goflow nodes in practical workflows. Each section is bilingual: English first, Vietnamese right after.

Tai lieu nay giai thich cach dung node trong Goflow theo huong thuc te. Moi phan co hai ngon ngu: English truoc, Tieng Viet ngay sau.

---

## 1. Core Concepts / Khai Niem Cot Loi

### Workflow

**EN:** A workflow is a directed graph of nodes. Trigger nodes start an execution. Action, logic, database, AI, and communication nodes process data. Edges define the order.

**VI:** Workflow la mot so do cac node co huong. Trigger node khoi dong mot lan chay. Cac node action, logic, database, AI va communication xu ly du lieu. Edge quy dinh thu tu chay.

### Node

**EN:** A node is one step in the workflow. Each node has:

- `id`: unique identifier used by placeholders.
- `type`: node executor type, for example `httpRequest`.
- `name`: display name.
- `params`: settings used by the node.

**VI:** Node la mot buoc trong workflow. Moi node co:

- `id`: dinh danh duy nhat, dung trong placeholder.
- `type`: loai executor, vi du `httpRequest`.
- `name`: ten hien thi.
- `params`: cau hinh cua node.

### Edge

**EN:** An edge connects one node to another. For `conditionIf`, use `sourceHandle: "true"` or `sourceHandle: "false"` to choose branches.

**VI:** Edge noi node nay sang node khac. Voi `conditionIf`, dung `sourceHandle: "true"` hoac `sourceHandle: "false"` de tach nhanh.

### Placeholder Syntax

**EN:** Use `{{node_id.path}}` to read output from previous nodes.

**VI:** Dung `{{node_id.path}}` de lay output tu node truoc do.

Examples / Vi du:

```text
{{http_1.status_code}}
{{http_1.data.name}}
{{telegram_1.ok}}
{{ai_1.ai_response}}
{{cron_1.triggered_at}}
{{$trigger.body.email}}
```

Common mistake / Loi thuong gap:

```text
Wrong: {{ http_1.status_code }}
Right: {{http_1.status_code}}
```

Keep node IDs simple: `cron_1`, `fetch_user`, `if_status_ok`, `send_telegram`.

Dat node ID don gian: `cron_1`, `fetch_user`, `if_status_ok`, `send_telegram`.

---

## 2. How to Design a Workflow / Cach Thiet Ke Workflow

**EN:**

1. Start with one trigger: `webhookTrigger`, `cronTrigger`, or `githubWebhook`.
2. Fetch or receive data.
3. Transform or check data with `jsonTransform`, `jsCodeRunner`, or `conditionIf`.
4. Call services: HTTP API, database, AI, email, Telegram, Slack, Discord.
5. Add error/alert branches when possible.
6. Test with a simple payload before adding many nodes.

**VI:**

1. Bat dau bang mot trigger: `webhookTrigger`, `cronTrigger`, hoac `githubWebhook`.
2. Lay du lieu hoac nhan du lieu.
3. Bien doi/kiem tra du lieu bang `jsonTransform`, `jsCodeRunner`, hoac `conditionIf`.
4. Goi dich vu: HTTP API, database, AI, email, Telegram, Slack, Discord.
5. Neu co the, them nhanh bao loi/canh bao.
6. Test bang payload don gian truoc khi them qua nhieu node.

Good basic shape / Mau co ban tot:

```text
Trigger -> HTTP Request -> IF Condition -> JSON Transform -> Notification
```

More advanced shape / Mau nang cao:

```text
Webhook -> IF big order -> AI summary -> Telegram
        -> Google Sheets
        -> IF has email -> SMTP Email
```

---

## 3. Credentials / Thong Tin Bao Mat

**EN:** Prefer using Credentials Manager instead of pasting tokens directly into node params. Use `credential_id` where available.

**VI:** Nen dung Credentials Manager thay vi dan token truc tiep vao params. Neu node co `credential_id`, hay dung no.

Recommended credential types / Loai credential nen dung:

| Use case | Credential type |
|---|---|
| OpenAI Assistant or OpenAI node | `OpenAI` or `API_KEY` |
| DeepSeek Assistant or DeepSeek node | `DeepSeek` or `API_KEY` |
| Telegram bot token | `TELEGRAM_BOT` or `API_KEY` |
| Database connection string | `API_KEY` or `BEARER_TOKEN` |
| Google service account JSON | `API_KEY` |
| SMTP password | `BASIC_AUTH` or `API_KEY` |

**EN:** After the recent secure key migration, old credentials are auto-migrated when read successfully.

**VI:** Sau thay doi ve master key bao mat, credential cu se duoc tu dong migrate khi doc thanh cong.

---

## 4. Trigger Nodes / Node Khoi Dong

### Webhook Trigger - `webhookTrigger`

**EN:** Starts a workflow when Goflow receives an HTTP POST at `/webhook/{workflowId}`.

**VI:** Khoi dong workflow khi Goflow nhan HTTP POST tai `/webhook/{workflowId}`.

Params / Tham so:

- `path`: optional label/subpath for documentation.
- `secret`: optional shared secret. Callers must send `X-Goflow-Webhook-Secret`.

Typical output / Output thuong gap:

```json
{
  "body": { "email": "user@example.com", "total": 750000 },
  "body_raw": "{\"email\":\"user@example.com\",\"total\":750000}",
  "headers": {},
  "method": "POST",
  "path": "/webhook/workflow-id",
  "query": {}
}
```

Use when / Khi dung:

- Receive order events.
- Receive alerts from another system.
- Build small internal APIs.

### Cron Schedule - `cronTrigger`

**EN:** Runs a workflow on a schedule.

**VI:** Chay workflow theo lich.

Params:

- `cron_expression`: 5-field cron expression.

Examples / Vi du:

```text
*/5 * * * *     every 5 minutes / moi 5 phut
0 9 * * *       every day at 09:00 / moi ngay 09:00
0 * * * *       hourly / moi gio
```

Output:

```json
{
  "triggered_at": "2026-07-24T09:00:00+07:00",
  "schedule": "*/5 * * * *"
}
```

### GitHub Webhook - `githubWebhook`

**EN:** Receives GitHub webhook payloads and can verify `X-Hub-Signature-256` using a secret.

**VI:** Nhan payload webhook tu GitHub va co the xac thuc `X-Hub-Signature-256` bang secret.

Params:

- `secret`: GitHub webhook secret.

Use when / Khi dung:

- Run workflow on push, release, issue, pull request.
- Build changelog or deployment notification.

---

## 5. HTTP and Data Nodes / Node HTTP va Du Lieu

### HTTP Request - `httpRequest`

**EN:** Calls external HTTP APIs.

**VI:** Goi API HTTP ben ngoai.

Params:

- `method`: `GET`, `POST`, `PUT`, `DELETE`.
- `url`: endpoint URL.
- `headers`: JSON object string, for example `{"Authorization":"Bearer xxx"}`.
- `body`: request body for POST/PUT.

Output:

```json
{
  "status_code": 200,
  "headers": {},
  "data": {}
}
```

Example / Vi du:

```text
URL: https://api.github.com/repos/openai/openai-go/releases/latest
Headers: {"User-Agent":"Goflow"}
Use later: {{github_api.data.tag_name}}
```

### JSON Transform - `jsonTransform`

**EN:** Builds a new JSON object from templates and previous node outputs.

**VI:** Tao JSON moi tu template va output cua node truoc.

Param:

- `json_template`: JSON template.

Example:

```json
{
  "title": "New release {{github_api.data.tag_name}}",
  "url": "{{github_api.data.html_url}}",
  "summary": "{{ai_summary.ai_response}}"
}
```

Output:

```json
{
  "transformed": {
    "title": "New release v1.2.3"
  }
}
```

### JS Code Runner - `jsCodeRunner`

**EN:** Runs JavaScript to transform or compute custom values.

**VI:** Chay JavaScript de bien doi hoac tinh toan du lieu tuy bien.

Use when / Khi dung:

- Need custom math.
- Need array mapping/filtering.
- Need logic that is too complex for `conditionIf`.

Common output / Output:

The node returns whatever your JS code returns.

Node input is available through execution outputs. Use the UI data picker where possible.

Nen dung data picker trong UI de chen du lieu tu node truoc.

### IF / ELSE Condition - `conditionIf`

**EN:** Routes execution to `true` or `false` branch.

**VI:** Re nhanh workflow sang nhanh `true` hoac `false`.

Params:

- `field`: input value, often a placeholder.
- `operator`: comparison operator.
- `value`: target value to compare.

Output:

```json
{
  "result": true,
  "target_handle": "true"
}
```

Edge rule / Quy tac edge:

```json
{
  "source": "if_status_ok",
  "sourceHandle": "true",
  "target": "send_success"
}
```

### Delay / Sleep - `delaySleep`

**EN:** Pauses a branch for a number of seconds.

**VI:** Tam dung mot nhanh trong so giay nhat dinh.

Param:

- `seconds`: delay duration.

Use when / Khi dung:

- Wait before retrying a third-party API.
- Delay notification.

---

## 6. AI Nodes / Node AI

### OpenAI ChatGPT - `openAIGPT`

**EN:** Sends a prompt to OpenAI and returns the generated answer.

**VI:** Gui prompt toi OpenAI va tra ve cau tra loi.

Params:

- `model`: model name.
- `prompt`: user prompt.
- `system_message`: optional role/instruction.
- `api_key` or `credential_id`: prefer credential.

Output:

```json
{
  "ai_response": "Generated answer",
  "model_used": "gpt-4o",
  "raw_result": {}
}
```

Use later / Dung ve sau:

```text
{{ai_summary.ai_response}}
```

### DeepSeek AI - `deepseekAI`

**EN:** Calls DeepSeek chat/reasoner models.

**VI:** Goi model chat/reasoner cua DeepSeek.

Params are similar to OpenAI:

- `model`: `deepseek-chat` or `deepseek-reasoner`.
- `prompt`
- `system_message`
- `api_key` or `credential_id`

Output:

```json
{
  "ai_response": "Generated answer",
  "model_used": "deepseek-chat",
  "raw_result": {}
}
```

### AI Assistant in UI / AI Assistant trong giao dien

**EN:** AI Assistant helps draft or modify workflows. It does not save directly. You must click `Load Onto Canvas`. Backend validates AI-generated workflow JSON before the UI shows it as validated.

**VI:** AI Assistant giup tao hoac sua workflow. No khong tu luu truc tiep. Ban phai bam `Load Onto Canvas`. Backend se validate JSON workflow do AI tao truoc khi UI hien la da hop le.

Good prompt / Prompt tot:

```text
Create a workflow:
- Webhook receives an order.
- If total > 500000, summarize the order with OpenAI.
- Send Telegram alert.
- Append order to Google Sheets.
- Use clear node IDs and valid condition branches.
```

Prompt tieng Viet:

```text
Tao workflow:
- Webhook nhan don hang.
- Neu total > 500000 thi dung OpenAI tom tat don.
- Gui canh bao Telegram.
- Ghi don hang vao Google Sheets.
- Dat node ID ro rang va dung nhanh condition hop le.
```

---

## 7. Communication Nodes / Node Gui Thong Bao

### Telegram Bot - `telegramBot`

**EN:** Sends a message to Telegram chat, group, or channel.

**VI:** Gui tin nhan toi chat, group hoac channel Telegram.

Params:

- `bot_token` or `credential_id`
- `chat_id`
- `message`

Example:

```text
Chat ID: -1001234567890
Message: New release {{github_api.data.tag_name}}: {{github_api.data.html_url}}
```

Output is Telegram API response, often including `ok` and `result`.

Output la response cua Telegram API, thuong co `ok` va `result`.

### SMTP Email - `emailSMTP`

**EN:** Sends email through SMTP.

**VI:** Gui email qua SMTP.

Params:

- `host`, `port`
- `username`, `password` or `credential_id`
- `to`, `subject`, `body`

Use when / Khi dung:

- Simple email alerts.
- Gmail app password or private SMTP server.

### Gmail REST API - `gmailREST`

**EN:** Sends email using Google Gmail REST API, usually with service account credentials.

**VI:** Gui email bang Gmail REST API, thuong dung service account.

Params:

- `credential_id` or `service_account_json`
- `impersonate_user`
- `to`, `subject`, `body`

Use when / Khi dung:

- Google Workspace automation.
- Need Gmail API instead of raw SMTP.

### Discord Webhook - `discordBot`

**EN:** Sends content and embed cards to Discord through a webhook URL.

**VI:** Gui noi dung va embed card vao Discord qua webhook URL.

Params:

- `webhook_url`
- `username`
- `content`
- `embed_title`
- `embed_desc`

### Slack Webhook - `slackBot`

**EN:** Sends a message to Slack through an Incoming Webhook URL.

**VI:** Gui tin nhan toi Slack qua Incoming Webhook URL.

Params:

- `webhook_url`
- `text`
- `username`

---

## 8. Database Nodes / Node Co So Du Lieu

### PostgreSQL Query - `postgresQuery`

**EN:** Executes SQL against PostgreSQL.

**VI:** Chay SQL tren PostgreSQL.

Params:

- `credential_id` or `connection_string`
- `query_type`: `SELECT` or `EXECUTE`
- `query`

Output:

- `SELECT`: array of rows.
- `EXECUTE`: `{ "status": "success", "rows_affected": 1 }`

### MySQL Query - `mysqlQuery`

**EN:** Executes SQL against MySQL.

**VI:** Chay SQL tren MySQL.

Params:

- `credential_id` or `connection_string`
- `query_type`: `SELECT` or `EXECUTE`
- `query`

Output is similar to PostgreSQL.

Output tuong tu PostgreSQL.

### MongoDB Command - `mongodbCommand`

**EN:** Runs MongoDB collection operations.

**VI:** Chay thao tac tren collection MongoDB.

Params:

- `credential_id` or `connection_string`
- `database`
- `collection`
- `command`: find/insert/update/delete style command.
- `filter_json`
- `document_json`

### Redis Command - `redisCommand`

**EN:** Runs Redis commands.

**VI:** Chay lenh Redis.

Params:

- `credential_id`
- `address`
- `password`
- `db`
- `command`
- `key`, `field`, `value`

Typical output / Output thuong gap:

```json
{
  "status": "success",
  "result": "value"
}
```

---

## 9. Google and SaaS Nodes / Node Google va SaaS

### Google Sheets - `googleSheets`

**EN:** Reads from or appends rows to Google Sheets.

**VI:** Doc hoac them dong vao Google Sheets.

Params:

- `credential_id` or `service_account_json`
- `spreadsheet_id`
- `sheet_name`
- `action`: read or append.
- `values_json`: values to append.

Example append values:

```json
[
  ["{{webhook_1.body.email}}", "{{webhook_1.body.total}}", "{{cron_1.triggered_at}}"]
]
```

### Google Drive - `googleDrive`

**EN:** Lists files or uploads text content to Google Drive.

**VI:** Liet ke file hoac upload noi dung text len Google Drive.

Params:

- `credential_id` or `service_account_json`
- `action`
- `folder_id`
- `filename`
- `content`

### Notion Page - `notionPage`

**EN:** Creates a page in a Notion database.

**VI:** Tao page trong database Notion.

Params:

- `credential_id` or `notion_token`
- `database_id`
- `properties_json`

Tip / Meo:

Notion `properties_json` must match your database property names and types.

`properties_json` cua Notion phai khop ten cot va kieu cot trong database.

---

## 10. Developer Nodes / Node Cho Developer

### SSH Runner - `sshRunner`

**EN:** Connects to a remote server and runs shell commands.

**VI:** Ket noi server tu xa va chay lenh shell.

Params:

- `credential_id`
- `address`
- `username`
- `password`
- `private_key`
- `command`

Output:

```json
{
  "stdout": "command output",
  "stderr": "",
  "exit_code": 0
}
```

Security note / Luu y bao mat:

**EN:** Do not expose workflows with SSH commands to untrusted users.

**VI:** Khong cho nguoi khong tin cay truy cap workflow co lenh SSH.

### Git Command - `gitCommand`

**EN:** Runs local Git CLI operations.

**VI:** Chay lenh Git CLI tren may local.

Params:

- `action`: clone, pull, commit/push style action.
- `repository_url`
- `target_directory`
- `branch`
- `commit_message`

Use when / Khi dung:

- Pull a repo before deployment.
- Commit generated files.
- Clone a repo for processing.

### Goflow Plugin - `goflowPlugin`

**EN:** Executes a local binary from `./plugins/` and communicates through JSON stdin/stdout.

**VI:** Chay binary local trong `./plugins/` va giao tiep bang JSON stdin/stdout.

Param:

- `plugin_name`: file name only, for example `my_plugin.exe`.

Security note / Luu y bao mat:

**EN:** Plugins are executable code. Treat them like trusted local programs.

**VI:** Plugin la code co the thuc thi. Chi dung plugin ban tin cay.

---

## 11. Workflow Logic Nodes / Node Logic Workflow

### Sub-workflow Runner - `subWorkflow`

**EN:** Runs another workflow as a child workflow.

**VI:** Chay mot workflow khac nhu workflow con.

Params:

- `sub_workflow_id`
- `payload_json`
- `loop_mode`
- `parallel`
- `concurrency_limit`

Use when / Khi dung:

- Reuse common logic.
- Process each item in an array.
- Split a large workflow into smaller pieces.

### Delay / Sleep - `delaySleep`

See section 5. Use it to wait between steps.

Xem muc 5. Dung de tam dung giua cac buoc.

### IF / ELSE - `conditionIf`

See section 5. Use it for branching.

Xem muc 5. Dung de re nhanh.

---

## 12. Practical Recipes / Cong Thuc Thuc Te

### Recipe 1: Webhook Order Alert

**EN:** Receive an order, check total, send Telegram, save to Sheets.

**VI:** Nhan don hang, kiem tra tong tien, gui Telegram, luu vao Sheets.

```text
webhookTrigger -> conditionIf(total > 500000)
                -> true: openAIGPT -> telegramBot
                -> googleSheets
```

Useful placeholders:

```text
{{$trigger.body.customer.email}}
{{$trigger.body.total}}
{{ai_summary.ai_response}}
```

### Recipe 2: GitHub Release Monitor

**EN:** Poll GitHub releases, summarize release notes, notify chat.

**VI:** Kiem tra GitHub release, tom tat release note, gui thong bao.

```text
cronTrigger -> httpRequest(GitHub latest release) -> openAIGPT -> telegramBot
```

Useful placeholders:

```text
{{github_release.data.tag_name}}
{{github_release.data.body}}
{{github_release.data.html_url}}
{{ai_summary.ai_response}}
```

### Recipe 3: Server Health Check

**EN:** Ping an API. If status is not OK, send Discord alert.

**VI:** Kiem tra API. Neu status khong OK, gui canh bao Discord.

```text
cronTrigger -> httpRequest -> conditionIf(status_code equals 200)
                         false -> discordBot
```

Condition:

```text
field: {{health_check.status_code}}
operator: equals
value: 200
```

### Recipe 4: Database Report Email

**EN:** Query database, transform results, email report.

**VI:** Query database, bien doi ket qua, gui email bao cao.

```text
cronTrigger -> postgresQuery -> jsonTransform -> emailSMTP
```

---

## 13. Troubleshooting / Xu Ly Loi

### Credential decrypt failed

**EN:** If you see `cipher: message authentication failed`, the credential was encrypted with a different master key. Current code can migrate old default-key credentials automatically when the legacy key succeeds. If it still fails, recreate the credential.

**VI:** Neu thay `cipher: message authentication failed`, credential duoc ma hoa bang master key khac. Code hien tai co the migrate credential cu dung default key. Neu van fail, hay tao lai credential.

### AI generated workflow but cannot load

**EN:** The backend validator rejected the workflow. Check the validation message. Usually it is missing required params, unknown node type, or bad branch edges.

**VI:** Backend validator da tu choi workflow. Xem thong bao validation. Thuong la thieu param bat buoc, sai node type, hoac edge re nhanh sai.

### Placeholder returns empty

**EN:** Check exact node ID and output property. Use execution logs to inspect real output.

**VI:** Kiem tra dung node ID va ten property output. Dung execution logs de xem output that.

### Condition branch does not run

**EN:** Make sure edges from `conditionIf` use `sourceHandle: "true"` or `sourceHandle: "false"`.

**VI:** Dam bao edge tu `conditionIf` co `sourceHandle: "true"` hoac `sourceHandle: "false"`.

### HTTP request fails

**EN:** Check URL, method, headers JSON, network access, and API auth.

**VI:** Kiem tra URL, method, headers JSON, ket noi mang va auth API.

---

## 14. Node Selection Cheat Sheet / Bang Chon Node Nhanh

| Goal / Muc tieu | Use node / Dung node |
|---|---|
| Receive external event / Nhan su kien ben ngoai | `webhookTrigger` |
| Run on schedule / Chay theo lich | `cronTrigger` |
| Call REST API / Goi REST API | `httpRequest` |
| Branch true/false / Re nhanh dung/sai | `conditionIf` |
| Format JSON / Tao JSON moi | `jsonTransform` |
| Custom logic / Logic tuy bien | `jsCodeRunner` |
| Summarize or generate text / Tom tat, sinh text | `openAIGPT`, `deepseekAI` |
| Send Telegram / Gui Telegram | `telegramBot` |
| Send email / Gui email | `emailSMTP`, `gmailREST` |
| Send Slack/Discord / Gui Slack/Discord | `slackBot`, `discordBot` |
| Query SQL / Query SQL | `postgresQuery`, `mysqlQuery` |
| Use Redis / Dung Redis | `redisCommand` |
| Use MongoDB / Dung MongoDB | `mongodbCommand` |
| Write Google Sheet / Ghi Google Sheet | `googleSheets` |
| Upload/list Drive / Upload/list Drive | `googleDrive` |
| Create Notion page / Tao Notion page | `notionPage` |
| Run shell remotely / Chay shell tu xa | `sshRunner` |
| Run Git operations / Chay Git | `gitCommand` |
| Reuse workflow / Tai su dung workflow | `subWorkflow` |
| Run local custom binary / Chay binary tuy bien | `goflowPlugin` |

---

## 15. Suggested Learning Path / Lo Trinh Hoc De Nghi

**EN:**

1. Build `cronTrigger -> httpRequest -> telegramBot`.
2. Add `conditionIf` between HTTP and Telegram.
3. Add `jsonTransform` to format the message.
4. Add one credential and replace direct token input.
5. Try AI Assistant to generate a draft, then inspect every node.
6. Add database or Google Sheets only after the basics feel clear.

**VI:**

1. Tao `cronTrigger -> httpRequest -> telegramBot`.
2. Chen `conditionIf` giua HTTP va Telegram.
3. Them `jsonTransform` de format message.
4. Them mot credential va thay token truc tiep bang credential.
5. Thu AI Assistant tao ban nhap, sau do tu xem tung node.
6. Chi them database hoac Google Sheets sau khi da nam cac buoc co ban.

