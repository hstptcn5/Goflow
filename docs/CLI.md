# CLI

The `goflow` binary includes a CLI. Most commands call the running Goflow REST
API; `version`, help, workflow validation, and Pack authoring commands operate
locally.

## Environment

```bash
GOFLOW_URL=http://127.0.0.1:8080
GOFLOW_API_KEY=your-api-key-or-scoped-token
```

`GOFLOW_API_KEY` can be the admin API key or a scoped token.

## Common Commands

```bash
goflow version --output json
goflow status
goflow workflow list
goflow workflow describe <workflow-id-or-slug>
goflow workflow run <workflow-id-or-slug> --set source=cli --wait
goflow workflow export <workflow-id-or-slug> --output workflow.json
goflow workflow import workflow.json --activate
goflow workflow validate workflow.json
goflow pack init examples/packs/my-pack --id example.my-pack --name "My Pack"
goflow pack validate examples/packs/hello-webhook
goflow pack inspect examples/packs/hello-webhook --output table
goflow pack test examples/packs/hello-webhook --output json
goflow pack build examples/packs/hello-webhook --output release
goflow pack verify release/example.hello-webhook_0.1.0_windows-amd64.zip
goflow pack run examples/packs/hello-webhook --no-open
goflow execution get <execution-id>
goflow execution watch <execution-id>
goflow execution cancel <execution-id>
goflow token list
goflow token create mcp-runner --scope workflow:list --scope workflow:read --scope workflow:run --scope execution:read --workflow <workflow-id>
goflow token delete <token-id>
goflow mcp stdio
```

## Build Identity

`goflow version [--output table|json]` does not start or contact a server. JSON
has this stable schema:

```json
{
  "version": "1.0.0-rc.1",
  "channel": "community-rc",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "target": "linux-amd64",
  "go_version": "go1.25.13"
}
```

Official candidate builds bind all identity fields at compile time. Source and
development builds report `channel: development` and use `commit: unknown`
unless supplied with a complete validated build identity.

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

## Pack Author Toolkit

```bash
goflow pack init <directory> --id <id> --name <name> [--force]
goflow pack inspect <pack-directory-or-bundle> [--output table|json]
goflow pack test <pack-directory> [--output table|json]
goflow pack verify <bundle.zip-or-extracted-directory> [--output table|json]
goflow pack sign <bundle.zip> --output <signed.zip> --key-id <id> --private-key <path|->
goflow pack verify-signature <bundle.zip-or-extracted-directory> --key-id <id> --public-key <path> [--output table|json]
```

`pack init` creates a minimal deterministic pack scaffold with a bounded author-only offline fixture and refuses non-empty directories unless `--force` is supplied. `pack inspect` reports manifest, setup, capability, platform, file, fixture-presence, and integrity metadata without printing workflow or fixture values. `pack test` is offline: it validates setup metadata, applies fixture or synthetic non-secret config plus fake credential IDs in temporary state, prepares the managed workflow, and skips external connection checks. `pack verify` checks bundle inventory and hashes without running or importing the pack. See `docs/PACK_AUTHOR_TUTORIAL.md` for the complete author flow.

`pack sign` is an offline development command. It accepts one PEM PKCS#8
Ed25519 private key from an explicit file or stdin (`-`) and writes a different
ZIP atomically; it never prints or copies the key. `pack verify-signature`
requires a matching explicit key ID and PEM PKIX Ed25519 public-key file. No key
is trusted from the Pack or network. `pack verify` remains integrity-only and
reports `UNSIGNED` or `SIGNED_UNVERIFIED`; only `verify-signature` reports
`VERIFIED`. See `PACK_SIGNING.md` for trust limits.

## Pack Build

```bash
goflow pack build <pack-directory> --output <output-directory> [--target <goos-goarch>] [--force]
```

`pack build` creates a portable pack bundle ZIP for the current runtime platform. `--target` defaults to the running platform, for example `windows-amd64`, and must match both the current runtime platform and a platform listed in `supported_platforms`. Cross-target builds are not supported in this phase.

The bundle contains the current Goflow runtime, `pack.json`, the entry workflow, listed plugin files, listed asset files, `PACK_INFO.json`, and `README.txt`. It does not start the server, require `GOFLOW_API_KEY`, read credentials, use the database, or execute plugins.

Use `--force` to replace the exact destination archive after the new archive has been built successfully.

## Pack Run

```bash
goflow pack run <pack-directory> [--data-dir <directory>] [--port <port>] [--no-open]
```

`pack run` starts one pack as a local managed workflow. It always binds to `127.0.0.1`; `--port` defaults to `0` so the OS chooses a free loopback port. The command prints the actual URL. Use `--no-open` for scripts and tests.

By default, runtime state is stored outside the pack directory:

- Windows: `%LOCALAPPDATA%/Goflow/packs/<pack-id>/`
- macOS: `~/Library/Application Support/Goflow/packs/<pack-id>/`
- Linux: `$XDG_DATA_HOME/Goflow/packs/<pack-id>/` or `~/.local/share/Goflow/packs/<pack-id>/`

The data directory contains `goflow.db`, `goflow.master.key`, `pack-state.json`, `run-state.json`, and lock metadata. Use `--data-dir` to choose a controlled location.

The managed workflow ID is deterministic from the pack ID, so repeated runs update the same workflow record instead of creating duplicates. Pack Run MVP does not embed secrets and does not claim credential requirements are satisfied by name. If `required_credentials` is not empty, the command prints those requirements and opens the credentials page unless `--no-open` is set.

Packaged plugin execution is not supported in Pack Run MVP. A pack with listed `plugins` fails early instead of silently ignoring or executing native code.
