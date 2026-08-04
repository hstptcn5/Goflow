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
goflow pack validate examples/packs/hello-webhook
goflow pack build examples/packs/hello-webhook --output release
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

## Pack Validation

```bash
goflow pack validate <pack-directory>
```

`pack validate` checks `pack.json`, resolves pack-local paths safely, and validates the entry workflow with the same workflow validator used by `goflow workflow validate`. It does not start the server, require `GOFLOW_API_KEY`, execute plugins, or modify the database.

See [Pack Format v1](PACKS.md) for manifest rules, security boundaries, examples, and non-goals.

## Pack Build

```bash
goflow pack build <pack-directory> --output <output-directory> [--target <goos-goarch>] [--force]
```

`pack build` creates a portable pack bundle ZIP for the current runtime platform. `--target` defaults to the running platform, for example `windows-amd64`, and must match both the current runtime platform and a platform listed in `supported_platforms`. Cross-target builds are not supported in this phase.

The bundle contains the current Goflow runtime, `pack.json`, the entry workflow, listed plugin files, listed asset files, `PACK_INFO.json`, and `README.txt`. It does not start the server, require `GOFLOW_API_KEY`, read credentials, use the database, or execute plugins.

Use `--force` to replace the exact destination archive after the new archive has been built successfully.
