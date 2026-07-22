# Goflow: Super Lightweight, Zero-Dependency Workflow Automation Engine in Go

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Binary Size](https://img.shields.io/badge/Single_Binary-%3C_25MB-success?style=flat-square)]()
[![RAM Consumption](https://img.shields.io/badge/RAM_Usage-15--25MB-blueviolet?style=flat-square)]()

Goflow is a super lightweight, local-first, zero-dependency alternative to heavy workflow automation platforms like n8n, Zapier, or Make. Compiled into a single executable binary (~24 MB) with minimal memory footprint (15 - 25 MB RAM), Goflow features a Pure Go CGO-free SQLite storage engine (`modernc.org/sqlite`) and an embedded Vue 3 drag-and-drop Web UI.

---

## Resource and Performance Comparison

| Feature / Benchmark | Goflow (Go) | n8n (Node.js) | Zapier / Make |
| :--- | :---: | :---: | :---: |
| **RAM Footprint (Idle)** | **15 - 25 MB** | 400 - 800 MB | Cloud SaaS |
| **Binary & Packaging** | **Single File (~24 MB)** | Heavy Docker Image | Closed SaaS |
| **External Dependencies** | **NONE (Zero)** | Node.js, PostgreSQL | Closed SaaS |
| **Node Delay Overhead** | **~2 - 5 us** (Goroutines) | ~50 - 150 ms | ~500 - 2000 ms |
| **Database Storage** | **Pure Go SQLite (WAL)** | PostgreSQL / SQLite | Cloud SaaS |
| **Infrastructure Cost** | **$0 (Any $1 VPS / Pi)** | $10 - $40/mo VPS | $20 - $100+/mo |

---

## Repository Overview

> **Tagline:**  
> Goflow is an ultra-lightweight, zero-dependency alternative to n8n and Zapier. Single Go binary (<25MB, 15-25MB RAM), local-first, pure Go SQLite, and embedded visual drag-and-drop UI. Fast, private, and effortless automation.

**Topics:** `go` `golang` `workflow-automation` `dag-engine` `n8n-alternative` `zapier-alternative` `sqlite` `vue3` `vue-flow` `zero-dependency` `single-binary` `local-first`

---

## Key Features

- **Single Binary & Zero External Dependencies**: Requires NO Docker, Node.js, or PostgreSQL in production. Everything is bundled into one executable file.
- **DAG Execution Engine**: Concurrent execution of independent nodes via Goroutines & Channels with Kahn's topological sort and cyclic dependency detection.
- **Pure Go SQLite Storage**: High performance Write-Ahead Logging (WAL mode), isolated Single Writer pool (`MaxOpenConns(1)`), and Reader connection pool (`MaxOpenConns(8)`).
- **AES-256-GCM Encrypted Credentials**: Authenticated encryption with Argon2id key derivation protecting API keys, passwords, and Bot Tokens.
- **Real-Time WebSocket Execution Timeline**: Push real-time step execution updates, status badges, and JSON payload logs via WebSockets.
- **Modern High-Contrast Light Theme Canvas**: Visual workflow builder powered by Vue 3, Vite, Vue Flow, and Pinia with manual wire connection tools.
- **Export & Import Workflow JSON**: Portable JSON format allowing easy backup and sharing of workflows across instances.
- **Built-in Auto-Retry Engine**: Automatic retry loop (up to 3 attempts with exponential backoff) for resilient network calls.

---

## Built-in Node Executors

1. **Webhook Trigger**: Triggers a workflow execution upon receiving an incoming HTTP Webhook request payload.
2. **Cron Schedule Trigger**: Automatically runs workflows based on standard Cron expressions (e.g., `*/5 * * * *`).
3. **HTTP Request Action**: Sends REST API HTTP requests (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`) with custom headers, authentication, and body.
4. **OpenAI ChatGPT Action**: Generates text and completions via OpenAI API (GPT-4o, GPT-3.5).
5. **DeepSeek AI Action**: Generates responses via DeepSeek-V3 (`deepseek-chat`) and DeepSeek-R1 (`deepseek-reasoner`).
6. **SMTP Email Action**: Sends automated emails via any SMTP server (Gmail, Mailgun, custom SMTP) with HTML formatting.
7. **Telegram Bot Action**: Sends instant HTML-formatted notifications to Telegram chats or channels via Telegram Bot API.
8. **Discord Webhook Action**: Sends notifications and Embed cards to Discord channels.
9. **Slack Webhook Action**: Sends notifications to Slack channels.
10. **Delay / Sleep Logic**: Pauses workflow execution for a configured duration ($N$ seconds).
11. **JSON Transform Action**: Constructs or extracts dynamic JSON data structures.
12. **JS Code Runner Action**: Executes custom JavaScript expressions and transformations.
13. **IF / ELSE Condition**: Branches workflow execution paths based on comparison operators (`equals`, `contains`, `is_not_empty`).

---

## Project Structure

```
d:/Bot2026/Goflow/
├── main.go                       # Application entrypoint & HTTP web server
├── static_embed.go               # Go embed.FS embedding Vue 3 UI into single binary
├── go.mod                        # Go module definition & dependencies
├── config/                       # System configuration loader
│   └── config.go
├── internal/
│   ├── api/                      # REST API & WebSocket handlers (go-chi/chi router)
│   ├── engine/                   # DAG Execution Engine, EventBus & Auto-retry Scheduler
│   ├── nodes/                    # Node Executors & Plugin Registry
│   ├── storage/                  # SQLite storage layer, schemas & AES encryption
│   └── crypto/                   # Argon2id + AES-256-GCM cryptography
└── ui/                           # Vue 3 Frontend Project (Vite, Vue Flow, Pinia)
    └── dist/                     # Bundled production Web UI embedded into Go
```

---

## Quick Start Guide

### 1. Download Dependencies & Generate `go.sum`
```bash
go mod tidy
```

### 2. Run in Development Mode
```bash
go run main.go static_embed.go
```
Open your browser and navigate to: `http://localhost:8080`

### 3. Build Single Binary Executable for Production
```bash
go build -o goflow.exe main.go static_embed.go
```
Run `goflow.exe` directly on any server without installing runtime dependencies.

---

## REST API & Endpoint Specifications

- `GET /api/v1/workflows`: List all workflows.
- `POST /api/v1/workflows`: Create a new workflow.
- `GET /api/v1/workflows/{id}`: Fetch workflow detail.
- `PUT /api/v1/workflows/{id}`: Update workflow nodes and edges JSON.
- `DELETE /api/v1/workflows/{id}`: Delete a workflow.
- `POST /api/v1/workflows/{id}/trigger`: Trigger manual workflow execution.
- `GET /api/v1/workflows/{workflowId}/executions`: Fetch execution history timeline logs.
- `POST /api/v1/credentials`: Save a new encrypted credential secret (AES-256-GCM).
- `GET /api/v1/nodes/definitions`: Retrieve available node metadata definitions.
- `POST /webhook/{workflowId}`: Public HTTP Webhook endpoint.
- `GET /ws`: WebSocket real-time execution event stream.

---

## License

This project is licensed under the [MIT License](LICENSE).
