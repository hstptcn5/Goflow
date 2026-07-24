# Release Guide

This guide describes how to prepare a Goflow preview release.

## Release Checklist

1. Update `VERSION`.
2. Update `CHANGELOG.md`.
3. Run backend checks:

   ```bash
   go test ./...
   go vet ./...
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

6. Start Goflow locally and check:

   - UI loads at `http://127.0.0.1:8080`.
   - `NODES.md` opens from the Docs button.
   - Workflow templates can be imported or loaded.
   - A simple workflow can run.
   - Credentials can be created and selected.

7. Package the release with:

   - Binary: `goflow.exe` or `goflow`.
   - `README.md`
   - `NODES.md`
   - `BACKUP.md`
   - `CHANGELOG.md`
   - `COMMERCIAL.md`
   - `TRADEMARK.md`
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
