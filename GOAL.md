# GOAL: Hoàn thiện Goflow thành Automation Runtime local-first, an toàn và có thể dùng production

## 1. Mục tiêu sản phẩm

Goflow phải trở thành một **local-first, self-hosted automation runtime** cho phép người dùng xây và thực thi workflow thông qua bốn giao diện thống nhất:

1. Web UI dành cho người thiết kế và quản lý workflow.
2. REST API và Webhook dành cho ứng dụng bên ngoài.
3. CLI dành cho developer, DevOps, script và CI/CD.
4. MCP dành cho AI agent và các MCP client.

Tất cả các giao diện phải sử dụng chung:

* Một workflow engine.
* Một execution model.
* Một concurrency control.
* Một credential vault.
* Một execution history.
* Một permission model.
* Một audit trail.

Không giao diện nào được tạo đường thực thi riêng, khởi tạo engine riêng hoặc truy cập trực tiếp SQLite để chạy workflow.

## 2. Định vị cuối cùng

Goflow không chỉ là một visual workflow editor.

Goflow phải trở thành:

> Một single-binary automation runtime có thể được điều khiển an toàn bởi con người, script, ứng dụng và AI agent.

Người dùng phải có thể:

```text
Thiết kế workflow trên Web UI
→ khai báo input schema và quyền truy cập
→ chạy bằng CLI, REST, webhook hoặc MCP
→ theo dõi execution
→ hủy execution
→ xem audit log
→ không làm lộ credential
→ không chạy trùng do retry
→ không vượt giới hạn tài nguyên
```

## 3. Nguyên tắc kiến trúc bắt buộc

### 3.1. Single execution path

Tất cả trigger phải đi qua:

```text
Web UI
REST API
Webhook
Cron
CLI
MCP stdio
MCP HTTP
    ↓
TriggerService
    ↓
Admission Control
    ↓
Workflow Engine
    ↓
Execution Store
```

Không được:

* Gọi engine trực tiếp từ CLI.
* Gọi engine trực tiếp từ MCP.
* Mở SQLite trực tiếp từ CLI.
* Tạo engine thứ hai trong MCP process.
* Bỏ qua authentication, idempotency hoặc concurrency.

### 3.2. Single binary

Giữ một executable:

```bash
goflow
```

Các mode:

```bash
goflow serve
goflow status
goflow workflow ...
goflow execution ...
goflow token ...
goflow mcp stdio
```

Goflow mặc định vẫn phải:

* Chạy không cần Docker.
* Chạy với SQLite.
* Không bắt buộc Redis.
* Không bắt buộc PostgreSQL.
* Hoạt động tốt trên Windows và Linux.

## 4. GOAL chức năng

### 4.1. CLI hoàn chỉnh

CLI phải hỗ trợ:

```bash
goflow status

goflow workflow list
goflow workflow describe <workflow>
goflow workflow run <workflow>
goflow workflow validate <file>
goflow workflow import <file>
goflow workflow export <workflow>

goflow execution get <execution-id>
goflow execution watch <execution-id>
goflow execution cancel <execution-id>

goflow token list
goflow token create
goflow token delete

goflow mcp stdio
```

CLI phải:

* Gọi REST API.
* Hỗ trợ JSON output.
* Có exit code ổn định.
* Hoạt động trong CI/CD.
* Hỗ trợ input từ JSON, file, stdin và `--set`.
* Hỗ trợ idempotency key.
* Hỗ trợ chờ execution hoàn tất.
* Tôn trọng `expose_cli`.
* Không được chạy workflow bị tắt CLI exposure.

### 4.2. MCP hoàn chỉnh

MCP phải hỗ trợ:

```text
goflow_list_workflows
goflow_get_workflow
goflow_run_workflow
goflow_get_execution
goflow_list_executions
goflow_cancel_execution
```

Workflow có `expose_mcp=true` phải có thể trở thành dynamic MCP tool.

Ví dụ:

```text
goflow.prepare_daily_report
goflow.process_document
goflow.backup_server
```

MCP phải hỗ trợ:

* stdio transport.
* Streamable HTTP transport.
* Official MCP Go SDK.
* Structured tool output.
* Input schema.
* Tool description.
* Workflow allowlist.
* Scoped token.
* Audit log.
* Async execution.
* Idempotency.
* Cancellation.
* Origin validation cho HTTP MCP.
* Không expose workflow inactive.
* Không expose workflow `requires_approval=true`.
* Không expose workflow chưa bật MCP.

## 5. Execution model

Execution status chuẩn:

```text
QUEUED
RUNNING
SUCCESS
FAILED
CANCELLED
INTERRUPTED
REJECTED
```

Mỗi execution phải lưu:

```text
execution_id
workflow_id
status
trigger_source
trigger_principal
request_id
idempotency_key
started_at
finished_at
duration
error_message
logs
```

Trigger source phải phân biệt chính xác:

```text
ui
api
webhook
cron
cli
mcp_stdio
mcp_http
sub_workflow
```

Không được ghi CLI và MCP chung thành `api`.

## 6. Concurrency model

Goflow phải hỗ trợ ba lớp concurrency.

### 6.1. Global execution limit

```env
GOFLOW_MAX_CONCURRENT_EXECUTIONS=10
```

Tất cả trigger phải chia sẻ cùng giới hạn.

### 6.2. Node concurrency

```env
GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION=4
```

Một workflow không được tạo số goroutine node không giới hạn.

Default phải lớn hơn `0`.

### 6.3. Per-workflow concurrency

Workflow có thể cấu hình:

```json
{
  "max_concurrent_runs": 1,
  "concurrency_policy": "reject"
}
```

Các policy được hỗ trợ thật:

```text
global
allow
reject
queue
```

Không được hiển thị hoặc lưu policy chưa được engine thực thi.

Nếu chưa triển khai queue, phải:

* Không cho chọn `queue`.
* Không ghi tài liệu rằng queue đã hoàn thành.

### 6.4. MCP per-client control

MCP phải giới hạn theo client/token principal.

```env
GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT=2
GOFLOW_MCP_RATE_LIMIT_PER_MINUTE=30
```

Limiter phải tồn tại xuyên suốt nhiều HTTP request.

Không được tạo limiter mới cho từng request.

Phải xác định rõ:

* Tool-call concurrency.
* Running execution concurrency.
* Rate limit.

Không dùng một setting cho ba ý nghĩa khác nhau.

## 7. Idempotency

Idempotency phải an toàn khi có request đồng thời.

Khóa:

```text
workflow_id + idempotency_key
```

Khi nhiều request cùng lúc gửi cùng key:

* Chỉ được tạo một execution.
* Tất cả request nhận cùng execution ID.
* Không trả unique constraint error.
* Không trả HTTP 500.
* Không chạy side effect hai lần.

Cần xử lý race:

```text
SELECT chưa thấy
→ nhiều request cùng INSERT
→ một request thắng
→ request còn lại đọc execution đã tồn tại
```

## 8. Authentication và authorization

### 8.1. Admin API key

Admin key có toàn quyền.

### 8.2. Scoped token

Token phải hỗ trợ:

```text
workflow:list
workflow:read
workflow:write
workflow:run
execution:read
execution:cancel
admin:tokens
admin:audit
admin:settings
```

Token có thể giới hạn workflow:

```json
{
  "allowed_workflows": [
    "workflow-id-1",
    "workflow-id-2"
  ]
}
```

### 8.3. Workflow list filtering

Token chỉ được xem workflow trong allowlist.

Không được trả toàn bộ workflow graph cho scope `workflow:list`.

`workflow:list` chỉ trả summary:

```json
{
  "id": "...",
  "name": "...",
  "slug": "...",
  "description": "...",
  "is_active": true,
  "expose_cli": true,
  "expose_mcp": true,
  "risk_level": "low"
}
```

`nodes_json`, `edges_json`, credential references và cấu hình nhạy cảm chỉ được trả cho endpoint có quyền `workflow:read`.

### 8.4. Principal

Principal phải lấy từ authentication context.

Không tin trực tiếp:

```text
X-Goflow-Principal
```

Caller không được tự giả mạo danh tính audit.

## 9. Data security

### 9.1. Execution input

Không được trả raw `input_json` cho MCP theo mặc định.

Execution summary chỉ trả metadata an toàn.

Raw input phải:

* Được redact.
* Hoặc chỉ đọc bằng scope mạnh hơn.
* Hoặc không lưu nếu workflow đánh dấu sensitive.

### 9.2. Webhook headers

Phải loại bỏ hoặc che:

```text
Authorization
Cookie
Set-Cookie
X-Goflow-Webhook-Secret
Proxy-Authorization
API-Key
X-API-Key
```

Không lưu các header này vào execution input hoặc log.

### 9.3. Logs

Logs, node output, error và event payload phải đi qua secret redaction.

Credential plaintext không được xuất hiện trong:

* CLI output.
* MCP output.
* Execution logs.
* Audit log.
* WebSocket event.
* Error response.

## 10. Workflow interface contract

Mỗi workflow phải có interface metadata:

```json
{
  "slug": "prepare-daily-report",
  "input_schema_json": {},
  "output_schema_json": {},
  "expose_cli": true,
  "expose_mcp": true,
  "mcp_tool_name": "prepare_daily_report",
  "mcp_description": "Prepare the daily report",
  "risk_level": "low",
  "requires_approval": false,
  "max_concurrent_runs": 1,
  "concurrency_policy": "reject"
}
```

`expose_cli=false` phải thực sự chặn CLI.

`expose_mcp=false` phải thực sự chặn:

* Static MCP run.
* Dynamic MCP tool.
* MCP workflow list.

## 11. JSON Schema

Workflow input validation phải dùng JSON Schema validator chuẩn hoặc công bố rõ subset được hỗ trợ.

Ưu tiên hỗ trợ:

```text
type
properties
required
additionalProperties
items
enum
const
minimum
maximum
minLength
maxLength
pattern
oneOf
anyOf
```

Không được âm thầm bỏ qua schema keyword khiến người dùng tưởng validation đã chạy.

## 12. Sub-workflow safety

Sub-workflow phải dùng chung root execution permit, không lấy thêm global slot.

Phải có:

```env
GOFLOW_MAX_SUBWORKFLOW_DEPTH=5
```

Phải phát hiện cycle:

```text
A → B → A
```

Khi cycle hoặc vượt depth:

* Dừng execution.
* Trả lỗi rõ ràng.
* Không tạo recursion vô hạn.
* Không làm crash process.

## 13. Cancellation

Cancellation phải:

* Hủy execution đang chạy.
* Propagate qua `context.Context`.
* Dừng Delay node.
* Dừng HTTP request hỗ trợ context.
* Dừng retry loop.
* Dừng node đang chờ semaphore.
* Đánh dấu execution là `CANCELLED`.
* Ghi `cancelled_at`.
* Không đổi execution đã terminal.

Cancellation là best-effort đối với thư viện không hỗ trợ context, nhưng trạng thái và behavior phải nhất quán.

## 14. Workflow-as-code

CLI export/import phải round-trip đầy đủ:

```text
name
description
nodes
edges
slug
input schema
output schema
expose_cli
expose_mcp
mcp tool metadata
risk level
requires approval
max concurrent runs
concurrency policy
```

Export rồi import phải giữ nguyên interface policy, trừ ID và timestamps.

`workflow validate` phải kiểm tra:

* JSON hợp lệ.
* Có name.
* Nodes và edges là array.
* Node ID không trùng.
* Node type tồn tại.
* Required parameters đầy đủ.
* Edge tham chiếu node tồn tại.
* DAG không có cycle.
* Input/output schema hợp lệ.
* Sub-workflow reference hợp lệ khi có thể kiểm tra.

## 15. MCP dynamic tool behavior

Dynamic tool không được chiếm field business của workflow.

Không lấy trực tiếp:

```json
{
  "idempotency_key": "..."
}
```

khỏi workflow input.

Control metadata phải nằm trong envelope riêng:

```json
{
  "input": {
    "idempotency_key": "business-value"
  },
  "_goflow": {
    "idempotency_key": "execution-idempotency-key"
  }
}
```

Hoặc chỉ hỗ trợ idempotency qua static tool.

Dynamic tools phải refresh khi:

* Workflow được activate/deactivate.
* `expose_mcp` thay đổi.
* Tool name thay đổi.
* Input schema thay đổi.

## 16. HTTP MCP

HTTP MCP phải:

* Dùng Streamable HTTP.
* Validate Origin.
* Bind localhost mặc định.
* Bắt buộc API key khi public bind.
* Hỗ trợ scoped token.
* Dùng HTTPS khi deploy public.
* Có request-size limit.
* Có rate limit.
* Có audit identity.
* Không bị global CORS chặn sai khi origin đã nằm trong MCP allowlist.

## 17. Migration

Database phải có versioned migration.

Migration phải:

* Chạy transactionally.
* Không chạy lại migration đã áp dụng.
* Không làm mất workflow.
* Không làm mất execution.
* Không làm mất credential.
* Không làm mất access token.
* Dừng startup nếu migration lỗi.

Không được thay đổi schema bằng logic không version.

## 18. Testing bắt buộc

GOAL chưa hoàn thành nếu thiếu các test sau:

### Execution

* Async trả execution ID.
* Sync execution thành công.
* Cancellation.
* Interrupted recovery.
* Global concurrency.
* Node concurrency.
* Per-workflow concurrency.
* Sub-workflow cycle.
* Sub-workflow depth.

### Idempotency

* Sequential duplicate.
* Concurrent duplicate với ít nhất 20 goroutine.
* Tất cả nhận cùng execution ID.
* Chỉ có một execution trong database.

### CLI

* CLI gọi API thật qua `httptest.Server`.
* Workflow run.
* Wait.
* Timeout.
* Cancel.
* Exit codes.
* JSON output.
* Import/export round-trip.
* `expose_cli=false`.

### MCP

* stdio initialization.
* Streamable HTTP initialization.
* Static tools.
* Dynamic tools.
* Workflow allowlist.
* Requires approval.
* Input schema.
* Cancellation.
* Scoped token.
* Per-client limit qua nhiều requests.
* Custom Origin.
* Workflow tool refresh.

### Security

* Token không thấy workflow ngoài allowlist.
* `workflow:list` không trả nodes/edges.
* Execution input được redact.
* Webhook secret không nằm trong database.
* Credential không xuất hiện trong logs.
* Principal không spoof được.

### Migration

* Database cũ nâng cấp thành công.
* Migration chạy lại không thay đổi dữ liệu.
* Migration lỗi rollback đúng.

## 19. Release criteria

GOAL chỉ được xem là hoàn thành khi:

```text
go test ./...
go vet ./...
go build ./...
```

đều thành công.

Ngoài ra phải có smoke test trên clean machine:

### Windows

```powershell
.\goflow.exe serve
.\goflow.exe status
.\goflow.exe workflow list
.\goflow.exe workflow run <workflow> --wait
.\goflow.exe mcp stdio
```

### Linux

```bash
./goflow serve
./goflow status
./goflow workflow list
./goflow workflow run <workflow> --wait
./goflow mcp stdio
```

### MCP HTTP

* Initialize bằng official MCP client.
* List static tools.
* List dynamic tools.
* Run workflow.
* Poll execution.
* Cancel execution.
* Reject invalid token.
* Reject invalid Origin.

## 20. Non-goals

Không thực hiện trong GOAL này:

* Multi-tenant SaaS.
* Kubernetes operator.
* Redis queue.
* PostgreSQL migration.
* Distributed workers.
* Multi-region.
* Visual approval workflow.
* Marketplace.
* Billing.
* RBAC theo team.
* SSO.
* Durable execution sau process crash.
* Exactly-once guarantee tuyệt đối.

Các tính năng này để roadmap sau.

## 21. Definition of Done cuối cùng

Dự án đạt GOAL khi:

1. CLI và MCP dùng chung execution path.
2. Workflow chạy từ UI, API, webhook, cron, CLI và MCP.
3. Execution có ID, source, principal và audit đầy đủ.
4. Idempotency an toàn khi concurrent.
5. Global, node, workflow và MCP limits hoạt động thật.
6. CLI/MCP exposure được enforce trên server.
7. Scoped token và workflow allowlist không rò rỉ dữ liệu.
8. Input, header, log và credential được redact.
9. Sub-workflow không recursion vô hạn.
10. CLI import/export round-trip đầy đủ.
11. MCP stdio và HTTP chạy với official client.
12. Test concurrency, security và migration đều pass.
13. Binary chạy trên Windows và Linux.
14. Không cần Docker, Redis hoặc PostgreSQL.
15. Tài liệu không tuyên bố tính năng chưa được triển khai.

## 22. Yêu cầu đối với Codex

Hãy tiếp tục sửa code cho đến khi toàn bộ Definition of Done ở trên được đáp ứng.

Quy trình:

1. Audit repo hiện tại.
2. Tạo checklist theo từng mục.
3. Viết failing test cho mỗi bug trước.
4. Sửa code tối thiểu để test pass.
5. Không phá backward compatibility nếu không cần thiết.
6. Không thay đổi kiến trúc single-binary.
7. Không thêm Redis, PostgreSQL hoặc Docker dependency.
8. Không đánh dấu mục hoàn thành chỉ vì có model hoặc UI.
9. Chỉ đánh dấu hoàn thành khi behavior đã được test.
10. Cuối cùng chạy:

```bash
go test ./...
go vet ./...
go build ./...
```

11. Cập nhật:

* CHANGELOG.md
* ROADMAP_PROGRESS.md
* RELEASE.md
* README.md

12. Báo cáo cuối cùng gồm:

* File đã sửa.
* Bug đã sửa.
* Test đã thêm.
* Các lệnh kiểm tra đã chạy.
* Tính năng còn chưa hoàn thành.
* Rủi ro còn lại.

Không dừng ở việc lập kế hoạch. Hãy thực thi lần lượt cho đến khi GOAL đạt hoặc gặp trở ngại kỹ thuật thực sự không thể giải quyết trong repo hiện tại.
