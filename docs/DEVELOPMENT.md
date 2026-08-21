# Development

## Requirements

- Go 1.25.13, matching `go.mod`.
- Node.js and npm for frontend development and tests.

Production use does not require Node.js because the Web UI is embedded into the Go binary.

## Backend

```bash
go test ./...
go vet ./...
go build ./...
```

Build a local binary:

```bash
go build -o goflow main.go static_embed.go
```

On Windows:

```powershell
go build -o goflow.exe main.go static_embed.go
```

## Frontend

```bash
cd ui
npm ci
npm run build
npm run test
npm run test:e2e
```

`npm run test:e2e` builds the frontend, starts a temporary Goflow server on a dynamic free port, and runs Playwright against that isolated instance.

## Embedded Binary E2E

After building `goflow.exe`, run selected E2E tests against the compiled embedded binary:

```powershell
cd ui
$env:GOFLOW_E2E_BINARY="goflow.exe"
node tests/run-e2e.mjs tests/e2e/milestone4-debugging.spec.js --reporter=line --timeout=45000
Remove-Item Env:\GOFLOW_E2E_BINARY
```

## Technical Roadmap Smoke

For one-command verification of the deep-core roadmap on Windows, run from the repository root:

```powershell
.\scripts\technical-roadmap-smoke.ps1
```

The script builds a temporary Goflow binary, starts an isolated SQLite instance, creates temporary QA workflows through the public API, exercises representative runtime capabilities, restarts Goflow to verify persistence, prints a compact PASS/FAIL/SKIP summary, and removes its temporary data.

It covers rich node definitions, typed IF, Switch, handled error routing, local HTTP plus cURL secret separation, Workflow State persistence, external Python execution and timeout when CPython is available, FileRef/XLSX round-trip, Local File root protection, File Watch persistence, reusable custom code discovery after restart, and Pack validation.

Use `-RequirePython` when Python is mandatory for the environment. Use `-KeepTemp` only when debugging a failure. External services such as PostgreSQL/MySQL, Google APIs, SMTP, and AI providers are intentionally not contacted by this local smoke; deterministic contract/unit tests cover those integration paths in CI.

A focused `Technical Roadmap Smoke` GitHub Actions workflow runs this same script on Windows with CPython provisioned whenever core roadmap paths change.

## Release Smoke

From the repository root:

```powershell
.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key
```

This smoke script covers CLI, MCP stdio, MCP HTTP, cancellation, scoped token allowlist, concurrent idempotency, cron trigger, and audit behavior.
