# CLI

The `goflow` binary includes an alpha CLI. The CLI calls the running Goflow REST API; it does not open SQLite directly and does not run a separate workflow engine.

## Environment

```bash
GOFLOW_URL=http://127.0.0.1:8080
GOFLOW_API_KEY=your-api-key-or-scoped-token
```

`GOFLOW_API_KEY` can be the admin API key or a scoped token.

## Common Commands

```bash
goflow status
goflow workflow list
goflow workflow describe <workflow-id-or-slug>
goflow workflow run <workflow-id-or-slug> --set source=cli --wait
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

## PowerShell JSON Quoting

On Windows PowerShell, prefer `--set` or `--input` because inline JSON quoting can be fragile.

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

## Scoped Token Example

```bash
goflow token create mcp-runner \
  --scope workflow:list \
  --scope workflow:read \
  --scope workflow:run \
  --scope execution:read \
  --workflow <workflow-id>
```

Add `--scope execution:cancel` only if the runner needs cancellation access.

