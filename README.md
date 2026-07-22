# Goflow: Super Lightweight, Zero-Dependency Workflow Automation Engine in Go

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Binary Size](https://img.shields.io/badge/Single_Binary-%3C_35MB-success?style=flat-square)]()
[![RAM Consumption](https://img.shields.io/badge/RAM_Usage-%3C_50MB-blueviolet?style=flat-square)]()

**Goflow** is a super lightweight, local-first, zero-dependency alternative to heavy workflow automation platforms like n8n, Zapier, or Make. Compiled into a **single executable binary** (<35 MB) with minimal memory footprint (<50 MB RAM), Goflow features a **Pure Go CGO-free SQLite storage engine** (`modernc.org/sqlite`) and an embedded **Vue 3 drag-and-drop Web UI**.

---

## 📌 Repository About Summary (GitHub Sidebar)

> **Short Description / Tagline for GitHub:**  
> ⚡ **Goflow** — Super Lightweight, Zero-Dependency, Local-First Workflow Automation Engine written in Go with Embedded Drag-and-Drop Web UI. Single binary <35MB, RAM <50MB, Pure Go SQLite storage, and AES-256 encrypted credentials.

**Topics / Tags for GitHub Repo:**  
`go` `golang` `workflow-automation` `dag-engine` `n8n-alternative` `zapier-alternative` `sqlite` `vue3` `vue-flow` `zero-dependency` `single-binary`

---

## ✨ Key Features

- ⚡ **Single Binary & Zero External Dependencies**: Requires NO Docker, Node.js, or PostgreSQL in production. Everything is bundled into one executable file.
- 🚀 **DAG Execution Engine**: Concurrent execution of independent nodes via **Goroutines & Channels** with Kahn's topological sort and cyclic dependency detection.
- 💾 **Pure Go SQLite Storage**: High performance Write-Ahead Logging (`WAL` mode), isolated Single Writer pool (`MaxOpenConns(1)`), and Reader connection pool (`MaxOpenConns(8)`).
- 🔒 **AES-256-GCM Encrypted Credentials**: Authenticated encryption with Argon2id key derivation protecting API keys, passwords, and Bot Tokens.
- 📡 **Real-Time WebSocket Execution Timeline**: Push real-time step execution updates, status badges, and JSON payload logs via WebSockets.
- 🎨 **Modern High-Contrast Light Theme Canvas**: Visual workflow builder powered by Vue 3, Vite, Vue Flow, and Pinia with manual wire connection tools.
- 📥 **Export & Import Workflow JSON**: Portable JSON format allowing easy backup and sharing of workflows across instances.
- 🔁 **Built-in Auto-Retry Engine**: Automatic retry loop (up to 3 attempts with exponential backoff) for resilient network calls.

---

## 🧩 Built-in Node Executors

1. 🔗 **Webhook Trigger**: Triggers a workflow execution upon receiving an incoming HTTP Webhook request payload.
2. ⏰ **Cron Schedule Trigger**: Automatically runs workflows based on standard Cron expressions (e.g., `*/5 * * * *`).
3. 🌐 **HTTP Request Action**: Sends REST API HTTP requests (`GET`, `POST`, `PUT`, `DELETE`, `PATCH`) with custom headers, authentication, and body.
4. 📬 **SMTP Email Action**: Sends automated emails via any SMTP server (Gmail, Mailgun, custom SMTP) with HTML formatting.
5. 💬 **Telegram Bot Action**: Sends instant HTML-formatted notifications to Telegram chats or channels via Telegram Bot API.
6. ⏳ **Delay / Sleep Logic**: Pauses workflow execution for a configured duration ($N$ seconds).
7. ⚙️ **JSON Transform Action**: Constructs or extracts dynamic JSON data structures.
8. 🔀 **IF / ELSE Condition**: Branches workflow execution paths based on comparison operators (`equals`, `contains`, `is_not_empty`).

---

## 🏗️ Architecture & Project Structure

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

## 🚀 Quick Start Guide

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
# Build single executable binary
go build -o goflow.exe main.go static_embed.go
```
Run `goflow.exe` directly on any server without installing runtime dependencies!

---

## 📡 REST API & Endpoint Specifications

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

## 📄 License

This project is licensed under the [MIT License](LICENSE).
