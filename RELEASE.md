# Release Guide

This guide describes how to prepare a Goflow preview release.

## Release Checklist

1. Update `VERSION`.
2. Update `CHANGELOG.md`.
3. Run backend checks:

   ```bash
   go test ./...
   go vet ./...
   go build ./...
   ```

4. Build the Web UI:

   ```bash
   cd ui
   npm ci
   npm run build
   npm run test
   npm run test:e2e
   ```

5. Build the binary:

   ```bash
   go build -trimpath -ldflags="-s -w" -o goflow.exe main.go static_embed.go
   ```

6. Check the MCP stdio bridge:

   ```bash
   node scripts/mcp-smoke-test.mjs --tools-only
   ```

7. Check script syntax:

   ```bash
   node --check scripts/mcp-smoke-test.mjs
   node --check scripts/mcp-http-smoke-test.mjs
   ```

8. Run the local GOAL smoke test on Windows:

   ```powershell
   .\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key
   ```

   This uses a temporary database and validates CLI, cron trigger source, MCP stdio, MCP Streamable HTTP, scoped token allowlist, dynamic MCP metadata, custom MCP origin, safe MCP execution DTOs, UI trigger source, unknown-node failure handling, cancellation, audit, and concurrent idempotency.

   The frontend E2E gate also validates the UX Milestone 1 and 2 browser flows: routed app shell, workflow creation, Add step picker, quick-add, validation summary, incomplete draft save, dirty graph save-before-run, activation blocking for invalid runnable workflows, node picker focus trap, undo/redo, duplicate, auto-layout, keyboard shortcuts, visual screenshot baselines, and separated 10/50/100-node performance smoke.

   To run the Playwright UX closure tests against the compiled embedded binary instead of `go run`, build `goflow.exe` first and run:

   ```powershell
   cd ui
   $env:GOFLOW_E2E_BINARY = ".\goflow.exe"
   npx playwright test tests/e2e/milestone2-closure.spec.js
   Remove-Item Env:\GOFLOW_E2E_BINARY
   ```

9. Start Goflow locally and check:

   - UI loads at `http://127.0.0.1:8080`.
   - Direct frontend route refresh works for `/workflows`, `/executions`, and `/credentials`.
   - `NODES.md` opens from the Docs button.
   - Workflow templates can be imported or loaded.
   - A simple workflow can run.
   - Credentials can be created and selected.
   - Scoped token creation and audit event listing work.
   - A workflow exposed through **Workflows > Interface > Expose to MCP** can pass:

     ```bash
     node scripts/mcp-smoke-test.mjs --url http://127.0.0.1:8080 --workflow <workflow-id-or-slug> --input '{"source":"release-smoke","secret":"mcp-stdio-secret-value"}'
     ```

   - The same workflow can pass HTTP MCP smoke:

     ```bash
     node scripts/mcp-http-smoke-test.mjs --url http://127.0.0.1:8080/mcp --api-key <scoped-token> --origin http://127.0.0.1:8080 --workflow <workflow-id-or-slug> --input '{"source":"release-http-mcp-smoke","secret":"mcp-http-secret-value"}'
     ```

   - Dynamic MCP tool exposure can pass:

     ```bash
     node scripts/mcp-http-smoke-test.mjs --url http://127.0.0.1:8080/mcp --api-key <scoped-token> --origin http://127.0.0.1:8080 --expect-tool goflow.<tool_name> --dynamic-tool goflow.<tool_name> --input '{"source":"release-dynamic","_goflow":{"idempotency_key":"release-dynamic-smoke"}}'
     ```

   - Workflow validation rejects structural problems:

     ```bash
     .\goflow.exe workflow validate .\templates\release_smoke_test.json
     ```

   - The workflow editor can save an incomplete draft, reload it, block Test/Activate until configured, and then run the saved current graph after configuration.
   - Node picker focus behavior passes in the embedded UI: search is focused on open, favorite does not add a node, Tab/Shift+Tab stay inside the dialog, Escape closes, and focus returns to the opener.

10. Package the release with:

   - Binary: `goflow.exe` or `goflow`.
   - `README.md`
   - `NODES.md`
   - `MCP_HTTP.md`
   - `PLUGINS.md`
   - `BACKUP.md`
   - `ROADMAP.md`
   - `CLI_MCP_ROADMAP.md`
   - `ROADMAP_PROGRESS.md`
   - `CHANGELOG.md`
   - `COMMERCIAL.md`
   - `TRADEMARK.md`
   - `scripts/mcp-smoke-test.mjs`
   - `scripts/mcp-http-smoke-test.mjs`
   - `scripts/goal-smoke-test.ps1`
   - `templates/`

## Windows Packaging

```powershell
.\scripts\build-release.ps1
```

The archive is written to `release/`.

## Linux / macOS Packaging

```bash
chmod +x scripts/build-release.sh
./scripts/build-release.sh
```

The archive is written to `release/`.

## Credential Warning

Do not include local runtime secrets in release archives:

- `goflow.db`
- `goflow.db-wal`
- `goflow.db-shm`
- `goflow.master.key`
- `.env`
- API keys, OAuth tokens, private keys, and service account JSON files

For production backups, use `BACKUP.md` instead.
