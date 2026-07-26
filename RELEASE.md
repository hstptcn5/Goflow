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

   This uses a temporary database and validates CLI, MCP stdio, MCP Streamable HTTP, scoped token allowlist, cancellation, audit, and concurrent idempotency.

9. Start Goflow locally and check:

   - UI loads at `http://127.0.0.1:8080`.
   - `NODES.md` opens from the Docs button.
   - Workflow templates can be imported or loaded.
   - A simple workflow can run.
   - Credentials can be created and selected.
   - Scoped token creation and audit event listing work.
   - A workflow exposed through **Workflows > Interface > Expose to MCP** can pass:

     ```bash
     node scripts/mcp-smoke-test.mjs --url http://127.0.0.1:8080 --workflow <workflow-id-or-slug> --input '{"source":"release-smoke"}'
     ```

   - The same workflow can pass HTTP MCP smoke:

     ```bash
     node scripts/mcp-http-smoke-test.mjs --url http://127.0.0.1:8080/mcp --api-key <scoped-token> --origin http://127.0.0.1:8080 --workflow <workflow-id-or-slug> --input '{"source":"release-http-mcp-smoke"}'
     ```

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
