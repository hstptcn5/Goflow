# Goflow: Single-Binary, Local-First Workflow Automation in Go

Goflow is a single-binary, local-first workflow automation engine for small servers, homelabs, edge devices, and internal self-hosted deployments. It runs as one Go executable with an embedded Vue 3 drag-and-drop Web UI and SQLite storage, so production use does not require Docker, Node.js, PostgreSQL, or a separate frontend server.

The project is best treated as a production-capable preview for trusted, single-user self-hosted environments. It is not positioned as a shared multi-tenant SaaS platform.

---

## Current Project Status

Goflow is in secure preview for self-hosted workflow automation. The core workflow engine, credential vault, embedded UI, webhook execution, and common integration nodes are implemented, with ongoing work focused on documentation, release packaging, examples, and operational polish.

Key achievements in the current version include:
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

See [COMMERCIAL.md](COMMERCIAL.md) for the commercial roadmap boundary and [TRADEMARK.md](TRADEMARK.md) for branding guidance.

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

---

## Built-in Node Executors

Goflow includes 26 built-in nodes spanning triggers, databases, communication channels, AI models, developer tools, and workflow logic:

### Triggers
1. **Webhook Trigger**: Triggers a workflow execution upon receiving an incoming HTTP Webhook request payload.
2. **Cron Schedule Trigger**: Automatically runs workflows based on standard Cron expressions (e.g., */5 * * * * *).
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
* **Backup and Restore Guide**: See [BACKUP.md](BACKUP.md) for protecting the SQLite database, credential master key, environment variables, and workflow exports.
* **Release Guide**: See [RELEASE.md](RELEASE.md) and [CHANGELOG.md](CHANGELOG.md) for packaging and release notes.
* **Commercial and Trademark Guidance**: See [COMMERCIAL.md](COMMERCIAL.md) and [TRADEMARK.md](TRADEMARK.md) for the open-core direction and branding boundaries.
* **Ready-to-use Templates**: Find pre-configured workflows in the [templates/](templates/) directory. You can easily import them using the "Import" button in the Web UI:
  - `workflow_ai_assistant.json`: Webhook-triggered DeepSeek text summary pipeline.
  - `github_repo_monitor.json`: Periodically fetch repository stats with custom API calls.
  - `multi_branch_stress_test.json`: Stress test concurrency across 3 parallel workflow branches.
  - `weather_alert_flow.json`: Automatic hourly Open-Meteo weather fetch and condition checks.
  - `uptime_incident_response.json`: Health-check monitor with Redis incident logging and Discord alerts.
  - `customer_support_ai_triage.json`: AI-assisted support ticket triage with urgent/normal routing.
  - `release_smoke_test.json`: Deployment helper that pulls code, restarts a service, runs a smoke test, and sends success/failure alerts.

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
  Include `Authorization: Bearer your_secret_password` in the headers. Query-string tokens are not accepted.
- WebSocket authentication:
  The bundled Web UI forwards the saved API key through the WebSocket subprotocol during `/ws` connection setup.
- Webhook secret:
  If a webhook trigger defines a secret, callers must include `X-Goflow-Webhook-Secret: <secret>`.

### 5. Safety and Retention Settings

Goflow includes conservative defaults for local/self-hosted use. Override them with environment variables when needed:

| Variable | Default | Purpose |
| :--- | :---: | :--- |
| `GOFLOW_MAX_CONCURRENT_EXECUTIONS` | `10` | Maximum workflows executing at the same time. Returns HTTP 429 when full. |
| `GOFLOW_WEBHOOK_RATE_LIMIT_PER_MINUTE` | `60` | Maximum webhook requests per minute per workflow/IP. Set `0` to disable. |
| `GOFLOW_EXECUTION_RETENTION_DAYS` | `30` | Deletes execution records older than this many days. Set `0` to disable age cleanup. |
| `GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW` | `1000` | Keeps only the newest N executions per workflow. Set `0` to disable count cleanup. |

On startup, any execution left in `RUNNING` from a previous crash or shutdown is marked as `INTERRUPTED`.

---

## REST API and Endpoint Specifications

- `GET /api/v1/workflows`: List all workflows.
- `POST /api/v1/workflows`: Create a new workflow.
- `GET /api/v1/workflows/{id}`: Fetch workflow detail.
- `PUT /api/v1/workflows/{id}`: Update workflow nodes and edges JSON.
- `DELETE /api/v1/workflows/{id}`: Delete a workflow.
- `POST /api/v1/workflows/{id}/trigger`: Trigger manual workflow execution.
- `GET /api/v1/workflows/{workflowId}/executions`: Fetch execution history timeline logs.
- `POST /api/v1/credentials`: Save a new encrypted credential secret (AES-256-GCM).
- `GET /api/v1/oauth2/authorize`: Initiates OAuth2 authorization code flow redirect.
- `GET /api/v1/oauth2/callback`: Handle external OAuth2 provider redirection callback.
- `GET /api/v1/nodes/definitions`: Retrieve available node metadata definitions.
- `POST /webhook/{workflowId}`: Public HTTP Webhook endpoint.
- `GET /ws`: WebSocket real-time execution event stream.

---

## License

The Community edition in this repository is licensed under the MIT License. The MIT License covers the software code; it does not grant trademark rights to the Goflow name, logo, or branding.
