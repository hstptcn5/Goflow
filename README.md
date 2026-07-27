# Goflow: Single-Binary, Local-First Workflow Automation in Go

Goflow is a single-binary, local-first workflow automation engine for small servers, homelabs, edge devices, and internal self-hosted deployments. It runs as one Go executable with an embedded Vue 3 drag-and-drop Web UI and SQLite storage, so production use does not require Docker, Node.js, PostgreSQL, or a separate frontend server.

The project is best treated as a production-capable preview for trusted, single-user self-hosted environments. It is not positioned as a shared multi-tenant SaaS platform.

---

## Current Project Status

Goflow is in secure preview for self-hosted workflow automation. The core workflow engine, credential vault, embedded UI, webhook execution, and common integration nodes are implemented, with ongoing work focused on documentation, release packaging, examples, and operational polish.

Key achievements in the current version include:
- UX Milestone 1 Foundation: Added a routed app shell with dedicated Workflows, Workflow Editor, Executions, Credentials, Templates, Nodes, Settings, and Help pages, plus visible workflow save states and frontend unit/E2E test coverage.
- UX Milestone 2 Editor Usability: Added searchable node picker, quick-add, validation badges and summary, undo/redo, duplicate/copy/paste, keyboard shortcuts, explicit auto-layout, draft-safe saves, save-before-run behavior, and visual/performance regression smoke coverage without changing the runtime engine.
- UX Milestone 3 Inspector and Data Mapping: Added Parameters/Input/Output/Logs inspector tabs, inline validation, credential actions, JSON tree/table/raw viewers, transitive upstream data picker, Fixed/Expression switching, runtime placeholder expression preview/parity, server-redacted output/log inspection, and dirty save-before-activate behavior.
- UX Milestone 4 Execution Debugging: Added an editor execution selector, canvas execution overlay, multi-hop failed-path edge highlight, retry selected execution using stored input, server-side execution replay, cancel action, contextual node error actions, and redacted debug bundle preview/copy with valid JSON export.
- Core DAG Engine Optimization: Implemented Node Skip Logic where non-matching conditional branches are marked as skipped to prevent execution waste.
- Sub-workflow Execution (Looping and Batching): Added a Sub-workflow Runner node that executes a child workflow by iterating over a list of items, supporting both sequential looping and concurrent parallel execution using Goroutines.
- Centralized Credentials Vault with AES-256-GCM: Implemented secure credentials storage utilizing authenticated encryption.
- Automated OAuth2 Authorization Flow: Added browser-based OAuth2 code flow directly in Goflow with automatic background token refresh using Refresh Tokens.
- Smart JavaScript Code Editor: Integrated CodeMirror into the Web UI to provide syntax highlighting and line numbers for custom JS scripting.
- Dynamic Data Picker: Enabled users to visually click and select outputs from previous nodes on the canvas.
- Live Output Inspector: Implemented a split-tab configuration sidebar displaying step logs, status indicators, and JSON response payloads in real-time.

---

## Editions and Commercial Use

Goflow follows an open-core direction:

- **Goflow Community**: the self-hosted core in this repository, licensed under the MIT License.
- **Goflow Pro / Enterprise**: future commercial offerings for teams and companies that need governance, support, advanced operations, or premium integrations.
- **Goflow Cloud**: a possible hosted managed service for users who do not want to operate Goflow themselves.

Community users can run real local and internal automations without a paid license. Commercial offerings are expected to focus on features such as multi-user workspaces, RBAC, SSO, audit logs, version history, managed OAuth connectors, premium node packs, hosted infrastructure, and priority support.

See [ROADMAP.md](ROADMAP.md) for the product roadmap, [COMMERCIAL.md](COMMERCIAL.md) for the commercial boundary, and [TRADEMARK.md](TRADEMARK.md) for branding guidance.

---

## Positioning and Fit

| Area | Goflow | Larger automation platforms |
| :--- | :---: | :---: |
| **Best fit** | Local automations, internal tools, webhook routing, small server jobs | Teams, multi-user operations, enterprise governance |
| **Runtime shape** | Single executable with embedded UI | SaaS account, Docker stack, or larger runtime |
| **Production dependencies** | No external runtime services required by default | Often requires hosted service, Docker, Node.js, or database services |
| **Storage** | Local SQLite using WAL mode | Cloud storage or external database |
| **Operations model** | Simple backup, local credentials, API key protection | User management, billing, advanced permissions, managed scaling |
| **Extensibility** | Built-in nodes, Go nodes, process-based JSON plugins | Marketplace nodes, SDKs, partner integrations |

### Local Benchmarks

The numbers below are local benchmark results from a specific test environment. Treat them as directional smoke tests, not vendor-neutral performance claims. In asynchronous mode, webhook latency measures how quickly Goflow accepts the trigger and schedules execution; it does not mean the full external workflow finished in that time.

#### 1. Scenario A: Heavy API-Bound Integration (GitHub API + Google Sheets API)
This scenario performs external network calls and writes results into Google Sheets, showing the difference between synchronous blocking and background asynchronous executions.

| Metric | Synchronous Triggering | Asynchronous Triggering (async=true) | Notes |
| :--- | :---: | :---: | :---: |
| Total Time | 43.256 seconds | 2.356 seconds | Trigger acceptance is much faster in async mode |
| Throughput | 23.12 reqs/sec | 424.50 reqs/sec | Measures trigger acceptance rate |
| Success Rate | 100% (1000 / 1000) | 100% (1000 / 1000) | No timeouts |
| Average Latency | 2098.60 ms | 115.86 ms | 18.1x lower |
| Median Latency (P50) | 2136.33 ms | 25.12 ms | 85.0x lower |
| Latency P99 | 3489.21 ms | 1019.07 ms | 3.4x lower |
| Idle Memory | 22.05 MB | 22.05 MB | Baseline footprint |
| Peak Memory | ~45.00 MB | 162.07 MB | Highly efficient scaling |

Note: Under the measured peak concurrency load of 1,000 requests with 50 workers, Goflow reached 162.07 MB RAM and returned toward the baseline footprint after executions completed.

#### 2. Scenario B: CPU-Bound JS Scripting & JSON Transformation
This scenario computes recursive Fibonacci(15) inside Goflow's sandboxed Goja JavaScript VM and maps the data via JSON Transform.

- Total Time: 1.226 seconds
- Throughput: 815.98 reqs/sec
- Success Rate: 100%
- Average Latency: 60.47 ms
- Median Latency (P50): 33.95 ms
- Latency P99: 287.31 ms

#### 3. Scenario C: Pure Gateway Routing (Fast Pass-Through)
This scenario acts as a high-speed webhook router, transferring incoming JSON payloads directly to outputs without heavy JS calculations or external network calls.

- Total Time: 1.052 seconds
- Throughput: 950.29 reqs/sec
- Success Rate: 100%
- Average Latency: 52.25 ms
- Median Latency (P50): 12.66 ms
- Latency P99: 301.22 ms

#### Visual Comparison (Maximum Throughput - requests/sec)

```mermaid
graph TD
    classDef sync fill:#f87171,stroke:#ef4444,stroke-width:2px,color:#fff;
    classDef async fill:#60a5fa,stroke:#3b82f6,stroke-width:2px,color:#fff;

    subgraph Throughput Comparison ["Throughput: Requests per Second (Higher is Better)"]
        direction LR
        Sync["Sync Mode (Scenario A): 23.12 r/s"]:::sync
        AsyncA["API Async (Scenario A): 424.50 r/s"]:::async
        AsyncB["JS Async (Scenario B): 815.98 r/s"]:::async
        AsyncC["Routing Async (Scenario C): 950.29 r/s"]:::async
    end
```

- Scenario A (API Bound Async):   [######################] 424.50 reqs/sec
- Scenario B (CPU JS Logic Async): [###########################################] 815.98 reqs/sec
- Scenario C (Pure Routing Async):  [##################################################] 950.29 reqs/sec

#### Visual Comparison (P50 Latency - ms, lower is better)

```mermaid
graph TD
    classDef fast fill:#34d399,stroke:#10b981,stroke-width:2px,color:#fff;
    classDef slow fill:#fb923c,stroke:#f97316,stroke-width:2px,color:#fff;

    subgraph Latency Comparison ["P50 Latency: Milliseconds (Lower is Better)"]
        direction LR
        SyncL["Sync Mode (Scenario A): 2136.33 ms"]:::slow
        AsyncAL["API Async (Scenario A): 25.12 ms"]:::fast
        AsyncBL["JS Async (Scenario B): 33.95 ms"]:::fast
        AsyncCL["Routing Async (Scenario C): 12.66 ms"]:::fast
    end
```

- Scenario A (API Bound Async):   [################] 25.12 ms
- Scenario B (CPU JS Logic Async): [########################] 33.95 ms
- Scenario C (Pure Routing Async):  [########] 12.66 ms

---

## Key Features

- **Single Binary and No External Runtime Services by Default**: Requires no Docker, Node.js, or PostgreSQL in production. The Web UI is bundled into one executable file.
- **DAG Execution Engine**: Concurrent execution of independent nodes via Goroutines and Channels with Kahn's topological sort and cyclic dependency detection.
- **Pure Go SQLite Storage**: High performance Write-Ahead Logging (WAL mode), isolated Single Writer pool (MaxOpenConns=1), and Reader connection pool (MaxOpenConns=8).
- **AES-256-GCM Encrypted Credentials**: Authenticated encryption with Argon2id key derivation protecting API keys, passwords, and Bot Tokens.
- **Automated OAuth2 flow**: Local server code exchange handler and scheduler that keeps credentials valid using background refresh tokens.
- **Real-Time WebSocket Execution Timeline**: Push real-time step execution updates, status badges, and JSON payload logs via WebSockets.
- **Modern High-Contrast Light Theme Canvas**: Visual workflow builder powered by Vue 3, Vite, Vue Flow, and Pinia with manual wire connection tools.
- **Export and Import Workflow JSON**: Portable JSON format allowing easy backup and sharing of workflows across instances.
- **Built-in Auto-Retry Engine**: Automatic retry loop (up to 3 attempts with exponential backoff) for resilient network calls.
- **Common Trigger Path for API, CLI, and MCP**: CLI and MCP call the REST API and share the same `TriggerService`, idempotency, concurrency, execution history, scoped token checks, and audit trail.
- **Scoped Runtime Controls**: Workflow allowlists, redacted scoped workflow lists, server-enforced CLI/MCP exposure, cancellation, and per-workflow reject concurrency are enforced by the backend.
- **Routed Web UI Foundation**: Workflows, editor, executions, credentials, templates, node catalog, settings, and help have durable frontend routes with embedded-server refresh support.
- **Editor Usability Tools**: Searchable Add step picker, recent/favorite nodes, quick-add, pre-run graph validation, incomplete draft saves, save-before-run test execution, save-before-activate, undo/redo, duplicate/copy/paste, focus-managed picker dialog, visual snapshots, and auto-layout are available in the workflow editor.
- **Inspector, Data Mapping, and Execution Debugging**: The node inspector provides Parameters, Input, Output, and Logs tabs with inline required/credential/JSON/URL/number/select/expression validation, transitive upstream output browsing, copy path/value actions, runtime-compatible expressions such as `{{json_1.transformed.user.email}}`, resolved preview, Fixed/Expression switching, server-redacted JSON rendering, execution selection, failed-path highlighting, replay, cancellation, and parseable redacted debug bundles.

---

## Built-in Node Executors

Goflow includes 26 built-in nodes spanning triggers, databases, communication channels, AI models, developer tools, and workflow logic:

### Triggers
1. **Webhook Trigger**: Triggers a workflow execution upon receiving an incoming HTTP Webhook request payload.
2. **Cron Schedule Trigger**: Automatically runs workflows based on standard 5-field Cron expressions (e.g., `*/5 * * * *`).
3. **GitHub Webhook Trigger**: Activates workflows on Git push, pull requests, etc., with signature verification using HMAC SHA-256.

### Databases
4. **PostgreSQL Query**: Executes SQL SELECT or EXECUTE statements against external Postgres databases.
5. **MySQL Query**: Runs SQL SELECT or EXECUTE statements against external MySQL databases.
6. **MongoDB Command**: Performs Find One, Insert One, Update One, and Delete One queries on Mongo databases.
7. **Redis Command**: Interacts with Redis key-value stores (GET, SET, DEL, HSET, HGET, LPUSH, LPOP).

### SaaS and Communication
8. **Google Sheets**: Appends rows or reads sheets via Google Service Account or OAuth2 authorization.
9. **Google Drive**: Lists folder files or uploads documents using Service Accounts.
10. **Gmail REST**: Sends rich HTML emails via Gmail REST API (supports G-Suite impersonation).
11. **Notion Page**: Creates new pages with customizable database properties.
12. **SMTP Email**: Sends automated HTML-formatted emails via any standard SMTP server.
13. **Telegram Bot**: Sends instant HTML-formatted notifications via Bot API.
14. **Discord Webhook**: Sends messages and Rich Embed cards to channels.
15. **Slack Webhook**: Sends formatted notifications to Slack channels.

### Developer Tools
16. **SSH Runner**: Executes shell commands on remote servers using Password or Private Key authentication.
17. **Git Command**: Performs Git clone, pull, or commit and push operations using local Git CLI.

### Logic and Scripting
18. **Sub-workflow Runner**: Executes child workflows with loop batching (sequential or parallel).
19. **JS Code Runner**: Executes custom JavaScript sandboxed expressions using Goja engine.
20. **IF / ELSE Condition**: Branches workflow execution paths based on comparison operators.
21. **Delay / Sleep**: Pauses workflow execution for a configured duration in seconds.
22. **JSON Transform**: Dynamically extracts or structures JSON data payloads.
23. **Goflow Plugin**: Executes custom multi-language plugins located in the ./plugins/ directory using process-based JSON IPC.

---

## Project Structure

```
d:/build2026/Goflow/
├── main.go                       # Application entrypoint and HTTP web server
├── static_embed.go               # Go embed.FS embedding Vue 3 UI into single binary
├── go.mod                        # Go module definition and dependencies
├── NODES.md                      # Comprehensive guide for all 26 built-in nodes
├── templates/                    # Ready-to-import workflow template JSON files
├── config/                       # System configuration loader
│   └── config.go
├── internal/
│   ├── api/                      # REST API, WebSocket and OAuth2 handlers
│   ├── engine/                   # DAG Execution Engine, EventBus and Auto-retry Scheduler
│   ├── nodes/                    # Node Executors and Plugin Registry
│   ├── storage/                  # SQLite storage layer, schemas and AES encryption
│   └── crypto/                   # Argon2id + AES-256-GCM cryptography
└── ui/                           # Vue 3 Frontend Project (Vite, Vue Flow, Pinia)
    └── dist/                     # Bundled production Web UI embedded into Go
```

---

## Documentation & Workflow Templates

* **Detailed Node Reference**: See [NODES.md](NODES.md) for a bilingual English/Vietnamese guide to the built-in nodes, placeholders, credentials, recipes, and troubleshooting.
* **Plugin Guide**: See [PLUGINS.md](PLUGINS.md) for the plugin stdin/stdout contract, security notes, and sample plugin ideas.
* **Backup and Restore Guide**: See [BACKUP.md](BACKUP.md) for protecting the SQLite database, credential master key, environment variables, and workflow exports.
* **Roadmap**: See [ROADMAP.md](ROADMAP.md) for the current product direction and commercial/community boundary.
* **CLI and MCP Roadmap**: See [CLI_MCP_ROADMAP.md](CLI_MCP_ROADMAP.md) for the proposed CLI and MCP expansion plan.
* **HTTP MCP Setup**: See [MCP_HTTP.md](MCP_HTTP.md) for Streamable HTTP MCP setup, smoke tests, reverse proxy notes, and troubleshooting.
* **Roadmap Progress**: See [ROADMAP_PROGRESS.md](ROADMAP_PROGRESS.md) for the current implementation timeline and checklist.
* **Release Guide**: See [RELEASE.md](RELEASE.md) and [CHANGELOG.md](CHANGELOG.md) for packaging and release notes.
* **Commercial and Trademark Guidance**: See [COMMERCIAL.md](COMMERCIAL.md) and [TRADEMARK.md](TRADEMARK.md) for the open-core direction and branding boundaries.
* **Expression and Mapping ADR**: See [docs/ADR_EXPRESSION_AND_MAPPING_MODEL.md](docs/ADR_EXPRESSION_AND_MAPPING_MODEL.md) for the current placeholder syntax, preview model, security notes, and compatibility decision.
* **Ready-to-use Templates**: Find pre-configured workflows in the [templates/](templates/) directory. You can easily import them using the "Import" button in the Web UI:
  - `workflow_ai_assistant.json`: Webhook-triggered DeepSeek text summary pipeline.
  - `github_repo_monitor.json`: Periodically fetch repository stats with custom API calls.
  - `multi_branch_stress_test.json`: Stress test concurrency across 3 parallel workflow branches.
  - `weather_alert_flow.json`: Automatic hourly Open-Meteo weather fetch and condition checks.
  - `uptime_incident_response.json`: Health-check monitor with Redis incident logging and Discord alerts.
  - `customer_support_ai_triage.json`: AI-assisted support ticket triage with urgent/normal routing.
  - `release_smoke_test.json`: Deployment helper that pulls code, restarts a service, runs a smoke test, and sends success/failure alerts.
  - `daily_sales_digest.json`: Scheduled business digest using HTTP, JavaScript, and SMTP.
  - `webhook_order_fraud_check.json`: Webhook order risk routing with review alerts.
  - `rss_to_discord_digest.json`: AI-generated RSS digest posted to Discord.
  - `github_pr_review_reminder.json`: Open pull request reminder for Slack.
  - `server_backup_health_check.json`: SSH backup health check with alerts.
  - `form_to_notion_and_email.json`: Website form to Notion plus confirmation email.
  - `google_sheets_lead_router.json`: Lead capture to Sheets with enterprise routing.
  - `redis_queue_worker.json`: Redis-backed queue polling workflow.
  - `webhook_payload_validator_plugin.json`: Plugin-based webhook validation.
  - `content_moderation_pipeline.json`: AI moderation and Slack routing.
  - `incident_postmortem_generator.json`: AI postmortem draft to Notion and email.
  - `plugin_lead_scoring_router.json`: Custom plugin lead scoring workflow.
  - `api_error_budget_monitor.json`: Metrics check with error budget alerting.
  - `customer_churn_signal_monitor.json`: Customer success churn signal workflow.

---

## Quick Start Guide

### 1. Download Dependencies
```bash
go mod tidy
```

### 2. Run in Development Mode
```bash
go run main.go static_embed.go
```
Open your browser and navigate to: `http://127.0.0.1:8080` or the host/port specified by `GOFLOW_HOST` and `GOFLOW_PORT`.

### 3. Build Single Binary Executable for Production
```bash
go build -o goflow.exe main.go static_embed.go
```

### 4. Running Goflow

You can run the Goflow binary in two modes:

#### Option A: Local-only without Password (Default Mode)
By default, Goflow binds to `127.0.0.1:8080` and does not require a password on the Web UI. This mode is intended for trusted local use only:
- On Windows / Linux / macOS:
  ```bash
  ./goflow.exe
```

### 5. CLI Alpha

The same `goflow` binary also includes an early CLI. The CLI calls the running Goflow REST API; it does not open SQLite directly.

```bash
goflow status
goflow workflow list
  goflow workflow describe <workflow-id-or-slug>
  goflow workflow run <workflow-id-or-slug> --json '{"source":"cli"}' --wait
  goflow workflow export <workflow-id-or-slug> --output workflow.json
  goflow workflow import workflow.json --activate
  goflow workflow validate workflow.json
  goflow execution get <execution-id>
  goflow execution watch <execution-id>
  goflow execution cancel <execution-id>
  goflow token list
  goflow token create mcp-runner --scope workflow:list --scope workflow:read --scope workflow:run --scope execution:read --workflow <workflow-id>
  goflow token delete <token-id>
  goflow mcp stdio
```

On Windows PowerShell, prefer `--set` or `--input` because inline JSON quoting can be fragile:

```powershell
.\goflow.exe workflow run <workflow-id-or-slug> `
  --set source=cli `
  --set date=2026-07-25 `
  --wait
```

Or use a JSON file:

```powershell
@'
{
  "source": "cli",
  "date": "2026-07-25"
}
'@ | Set-Content payload.json

.\goflow.exe workflow run <workflow-id-or-slug> --input payload.json --wait
```

Use `GOFLOW_URL` and `GOFLOW_API_KEY` for remote or protected instances. `GOFLOW_API_KEY` can be the admin API key or a scoped token:

```bash
GOFLOW_URL=http://127.0.0.1:8080
GOFLOW_API_KEY=your-api-key
```

Scoped tokens are created with the admin API key and are returned only once:

```bash
goflow token create mcp-runner \
  --scope workflow:list \
  --scope workflow:read \
  --scope workflow:run \
  --scope execution:read \
  --workflow <workflow-id>
```

### 6. MCP stdio Alpha and HTTP Beta

Goflow can expose MCP tools over stdio or Streamable HTTP. Both transports call the configured Goflow REST API; they do not execute workflows directly.

```bash
goflow mcp stdio
```

Available static tools:

- `goflow_list_workflows`
- `goflow_get_workflow`
- `goflow_run_workflow`
- `goflow_get_execution`
- `goflow_list_executions`
- `goflow_cancel_execution`
- `goflow_reload_tools`

MCP workflow access is opt-in. Open **Workflows > Interface** and enable **Expose to MCP** for each workflow that an MCP client may see or run. The stdio and HTTP MCP bridges only list active workflows with MCP exposure enabled, and they block workflows marked **Requires Approval**.

When MCP stdio starts while the Goflow server is reachable, each active exposed workflow is also registered as a dynamic tool named `goflow.<mcp_tool_name>`. If `mcp_tool_name` is empty, Goflow falls back to the workflow slug or sanitized workflow name. Dynamic workflow tools start executions asynchronously and return an `execution_id` for `goflow_get_execution`.

Dynamic workflow tools are registered when an MCP session is created. After changing workflow MCP exposure, tool name, or input schema, reconnect the MCP client. The static `goflow_reload_tools` tool returns a `reconnect_required` message for clients that need an explicit refresh command.

For MCP clients, set the environment used by the launched process:

```bash
GOFLOW_URL=http://127.0.0.1:8080
GOFLOW_API_KEY=your-api-key
```

For MCP or automation clients, prefer a scoped token instead of the admin API key. Common scopes are:

- `workflow:list`: list workflows.
- `workflow:read`: read workflow metadata and node definitions.
- `workflow:run`: start workflow executions.
- `execution:read`: read execution status and logs.
- `execution:cancel`: cancel running executions.
- `workflow:write`: create or update workflows.

Use `--workflow <workflow-id>` while creating a token to restrict it to specific workflows. Omit `--workflow` to allow all workflows for the granted scopes.

The stdio transport writes MCP protocol messages to stdout. Diagnostic logs should go to stderr.

MCP stdio smoke test:

```bash
node scripts/mcp-smoke-test.mjs --url http://127.0.0.1:8080
```

The HTTP MCP endpoint is mounted at `/mcp` on the same Goflow server:

```bash
node scripts/mcp-http-smoke-test.mjs \
  --url http://127.0.0.1:8080/mcp \
  --api-key your-scoped-token \
  --origin http://127.0.0.1:8080
```

To also run a workflow through HTTP MCP:

```bash
node scripts/mcp-http-smoke-test.mjs \
  --url http://127.0.0.1:8080/mcp \
  --api-key your-scoped-token \
  --origin http://127.0.0.1:8080 \
  --workflow prepare-daily-report \
  --input '{"date":"2026-07-26"}' \
  --idempotency-key mcp-http-smoke-2026-07-26
```

The target workflow must be active, must have **Expose to MCP** enabled in its Interface settings, and must not have **Requires Approval** enabled for the current MCP alpha/beta bridge. Dynamic workflow tools reserve `_goflow` for control metadata; use `_goflow.idempotency_key` for execution idempotency so a business field named `idempotency_key` can remain part of the workflow input. MCP execution responses intentionally omit raw `input_json`; use REST execution endpoints with appropriate authorization when raw inputs are needed for local debugging.

To assert and call a dynamic workflow tool over HTTP MCP:

```bash
node scripts/mcp-http-smoke-test.mjs \
  --url http://127.0.0.1:8080/mcp \
  --api-key your-scoped-token \
  --origin http://127.0.0.1:8080 \
  --expect-tool goflow.prepare_daily_report \
  --dynamic-tool goflow.prepare_daily_report \
  --input '{"date":"2026-07-26"}'
```

For HTTP MCP clients, use `Authorization: Bearer <scoped-token>`. The recommended scoped token for CLI/MCP runners is:

```bash
goflow token create mcp-runner \
  --scope workflow:list \
  --scope workflow:read \
  --scope workflow:run \
  --scope execution:read \
  --workflow <workflow-id>
```

If the client needs to cancel executions, add `--scope execution:cancel`.

HTTP MCP deployment notes:

- Keep `/mcp` behind HTTPS when exposing it outside localhost.
- Forward `Authorization`, `Accept`, `Content-Type`, `MCP-Protocol-Version`, `Mcp-Session-Id`, `Last-Event-ID`, and `Origin` headers through the reverse proxy.
- Set `GOFLOW_MCP_ALLOWED_ORIGINS` to the exact browser/client origins that may call `/mcp`.
- Set `GOFLOW_MCP_BASE_URL` to the internal Goflow URL reachable from the Goflow process when the public URL is behind a reverse proxy.
- Prefer scoped tokens over the admin API key for MCP clients.

If the Goflow server is not running yet, test only the MCP stdio handshake and tool registration:

```bash
node scripts/mcp-smoke-test.mjs --tools-only
```

To also run a workflow through MCP, pass a workflow ID, slug, or exact name:

```bash
node scripts/mcp-smoke-test.mjs \
  --url http://127.0.0.1:8080 \
  --workflow prepare-daily-report \
  --input '{"date":"2026-07-25"}' \
  --idempotency-key mcp-smoke-2026-07-25
```

To assert and call a dynamic workflow tool:

```bash
node scripts/mcp-smoke-test.mjs \
  --url http://127.0.0.1:8080 \
  --expect-tool goflow.prepare_daily_report \
  --dynamic-tool goflow.prepare_daily_report \
  --input '{"date":"2026-07-25"}'
```

#### Option B: Running with API Key Authentication (Secure Mode)
In this mode, Goflow requires clients and the Web UI to authenticate. The browser will prompt for the API key on your first API request:
- On Windows PowerShell:
  ```powershell
  $env:GOFLOW_API_KEY="your_secret_password"
  ./goflow.exe
  ```
- On Linux / macOS / Bash:
  ```bash
  export GOFLOW_API_KEY="your_secret_password"
  ./goflow.exe
  ```
- To bind on a public interface, set both `GOFLOW_HOST` and `GOFLOW_API_KEY`. Goflow refuses to bind to a non-loopback host without an API key:
  ```bash
  GOFLOW_HOST=0.0.0.0 GOFLOW_API_KEY=your_secret_password ./goflow
  ```
- API authentication:
  Include `Authorization: Bearer your_secret_password` in the headers. Query-string tokens are not accepted. The admin API key has full access. Scoped tokens can be used as Bearer tokens for limited automation access.
- Scoped tokens:
  Use `goflow token create/list/delete` or the `/api/v1/tokens` endpoints. Token management requires the admin API key.
- Audit events:
  Authenticated API requests and token management actions are recorded in `audit_events`. Admins can read recent events with `GET /api/v1/audit-events?limit=100`.
- WebSocket authentication:
  The bundled Web UI forwards the saved API key through the WebSocket subprotocol during `/ws` connection setup.
- Webhook secret:
  If a webhook trigger defines a secret, callers must include `X-Goflow-Webhook-Secret: <secret>`.

### 5. Safety and Retention Settings

Goflow includes conservative defaults for local/self-hosted use. Override them with environment variables when needed:

| Variable | Default | Purpose |
| :--- | :---: | :--- |
| `GOFLOW_MAX_CONCURRENT_EXECUTIONS` | `10` | Maximum workflows executing at the same time. Returns HTTP 429 when full. |
| `GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION` | `4` | Maximum concurrently executing nodes inside one execution. Set `0` only when you explicitly want to disable the per-execution node limit. |
| `GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT` | `2` | Maximum concurrent MCP workflow run calls per stdio client process or HTTP token/principal. |
| `GOFLOW_MCP_RATE_LIMIT_PER_MINUTE` | `30` | Maximum HTTP MCP requests per minute per token/principal. Set `0` to disable. |
| `GOFLOW_MCP_BASE_URL` | derived from host/port | Internal base URL used by `/mcp` when it calls the Goflow REST API. |
| `GOFLOW_MCP_ALLOWED_ORIGINS` | local UI origins | Comma-separated Origin allowlist for `/mcp` browser/remote clients. |
| `GOFLOW_MAX_SUBWORKFLOW_DEPTH` | `5` | Maximum nested sub-workflow depth before execution fails with a clear error. |
| `GOFLOW_WEBHOOK_RATE_LIMIT_PER_MINUTE` | `60` | Maximum webhook requests per minute per workflow/IP. Set `0` to disable. |
| `GOFLOW_EXECUTION_RETENTION_DAYS` | `30` | Deletes execution records older than this many days. Set `0` to disable age cleanup. |
| `GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW` | `1000` | Keeps only the newest N executions per workflow. Set `0` to disable count cleanup. |

On startup, any execution left in `RUNNING` from a previous crash or shutdown is marked as `INTERRUPTED`.

---

## REST API and Endpoint Specifications

- `GET /api/v1/workflows`: List workflows. Admin requests return full workflow records for the editor; scoped tokens receive allowlisted summaries without graph JSON.
- `POST /api/v1/workflows`: Create a new workflow.
- `GET /api/v1/workflows/{id}`: Fetch workflow detail.
- `PUT /api/v1/workflows/{id}`: Update workflow nodes and edges JSON.
- `GET /api/v1/workflows/{id}/interface`: Fetch CLI/MCP interface metadata.
- `PUT /api/v1/workflows/{id}/interface`: Update CLI/MCP interface metadata.
- `DELETE /api/v1/workflows/{id}`: Delete a workflow.
- `POST /api/v1/workflows/{id}/trigger`: Trigger manual workflow execution.
- `POST /api/v1/workflows/{id}/executions`: Start an async workflow execution with idempotency support.
- `GET /api/v1/executions/{id}`: Fetch execution details.
- `POST /api/v1/executions/{id}/cancel`: Request cancellation for a running execution.
- `GET /api/v1/workflows/{workflowId}/executions`: Fetch execution history timeline logs.
- `POST /api/v1/credentials`: Save a new encrypted credential secret (AES-256-GCM).
- `GET /api/v1/tokens`: List scoped token metadata. Admin API key required.
- `POST /api/v1/tokens`: Create a scoped token. The raw token is returned once. Admin API key required.
- `DELETE /api/v1/tokens/{id}`: Delete a scoped token. Admin API key required.
- `GET /api/v1/audit-events`: List recent audit events. Admin API key required.
- `POST /mcp`: Streamable HTTP MCP endpoint. Bearer token required when `GOFLOW_API_KEY` is set.
- `GET /api/v1/oauth2/authorize`: Initiates OAuth2 authorization code flow redirect.
- `GET /api/v1/oauth2/callback`: Handle external OAuth2 provider redirection callback.
- `GET /api/v1/nodes/definitions`: Retrieve available node metadata definitions.
- `POST /webhook/{workflowId}`: Public HTTP Webhook endpoint.
- `GET /ws`: WebSocket real-time execution event stream.

---

## License

The Community edition in this repository is licensed under the MIT License. The MIT License covers the software code; it does not grant trademark rights to the Goflow name, logo, or branding.
