# Installation

Goflow runs as a single Go binary with an embedded Web UI.

## Current Distribution Status

Goflow does not currently publish a stable GitHub Release, installer, signed
binary, or `latest` download. Build the platform from source for normal local
development and evaluation.

GitHub Actions can also produce temporary unsigned CI artifacts tied to an
exact workflow run and commit. The native Windows pilot artifact is named
`UNSIGNED-PILOT-BETA-goflow-dailyops-windows-amd64`. It is a Beta evaluation
artifact for the focused DailyOps appliance, not a stable Goflow release,
installer, publisher-authentic binary, or general update channel. Artifact
availability and retention depend on GitHub Actions; obtain one only from a
trusted pilot coordinator who provides the exact run, commit, and checksum.

See [Windows pilot guide](WINDOWS_PILOT_GUIDE.md) and
[Beta limitations](BETA_LIMITATIONS.md) before evaluating that artifact. Do not
disable SmartScreen or other security controls to run unsigned software.

## Build From Source

Requirement: Go `1.25.13`, matching the repository's `go.mod`.

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

Open `http://127.0.0.1:8080`.

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
| `GOFLOW_EXECUTION_RETENTION_DAYS` | `30` | Execution cleanup by age; supported range 1-365, otherwise default |
| `GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW` | `1000` | Per-workflow cleanup cap; supported range 1-10000, otherwise default |

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
