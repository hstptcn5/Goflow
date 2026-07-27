# Installation

Goflow runs as a single Go binary with an embedded Web UI.

## Download A Release

Download the latest archive from GitHub Releases, extract it, then run:

```bash
./goflow serve
```

On Windows PowerShell:

```powershell
.\goflow.exe serve
```

Open `http://127.0.0.1:8080`.

## Build From Source

```bash
git clone https://github.com/hstptcn5/Goflow.git
cd Goflow
go build -o goflow main.go static_embed.go
./goflow serve
```

On Windows:

```powershell
git clone https://github.com/hstptcn5/Goflow.git
cd Goflow
go build -o goflow.exe main.go static_embed.go
.\goflow.exe serve
```

## Useful Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `GOFLOW_HOST` | `127.0.0.1` | HTTP bind host |
| `GOFLOW_PORT` | `8080` | HTTP bind port |
| `GOFLOW_API_KEY` | empty | Admin API key; required for non-loopback binds |
| `GOFLOW_DB_PATH` | local `goflow.db` | SQLite database path |
| `GOFLOW_MASTER_KEY_FILE` | `goflow.master.key` | Credential encryption master key path |
| `GOFLOW_MAX_CONCURRENT_EXECUTIONS` | `10` | Global running workflow limit |
| `GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION` | `4` | Per-execution node parallelism |
| `GOFLOW_EXECUTION_RETENTION_DAYS` | `30` | Execution cleanup by age |
| `GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW` | `1000` | Execution cleanup by count |

## Secure Mode

For anything beyond trusted localhost use, set `GOFLOW_API_KEY`.

```bash
GOFLOW_API_KEY=change-me ./goflow serve
```

When binding publicly:

```bash
GOFLOW_HOST=0.0.0.0 GOFLOW_API_KEY=change-me ./goflow serve
```

Clients must send:

```http
Authorization: Bearer <api-key-or-scoped-token>
```

Run public deployments behind HTTPS and prefer scoped tokens for automation clients.

