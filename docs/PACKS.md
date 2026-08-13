# Goflow Pack Format v1

Goflow Pack is a directory format for distributing one workflow with pack metadata and optional local resources. Goflow can validate a pack, build a portable pack bundle, and run a pack locally as a managed workflow.

## Directory Layout

```text
example-pack/
+-- pack.json
+-- workflows/
|   +-- main.json
+-- plugins/
+-- assets/
```

Only `pack.json` and the `entry_workflow` file are required. `plugins/` and `assets/` are reserved for future packaging and may be absent.

## Manifest

Minimal `pack.json`:

```json
{
  "schema_version": 1,
  "id": "example.daily-report",
  "name": "Example Daily Report",
  "version": "0.1.0",
  "description": "Example Goflow automation pack",
  "entry_workflow": "workflows/main.json",
  "required_credentials": [],
  "supported_platforms": [
    "windows-amd64"
  ]
}
```

Known fields:

| Field | Required | Rules |
| :--- | :--- | :--- |
| `schema_version` | Yes | Must be `1`. |
| `id` | Yes | Stable ID using lowercase alphanumeric segments separated by dots or hyphens. It cannot start or end with a delimiter and cannot contain empty segments such as `..` or mixed delimiters such as `.-`. |
| `name` | Yes | Human-readable pack name. |
| `version` | Yes | Valid SemVer 2.0.0, including optional prerelease/build metadata. Numeric prerelease identifiers cannot contain leading zeroes. |
| `description` | No | Human-readable description. |
| `entry_workflow` | Yes | Portable slash path to a workflow JSON file inside the pack directory. |
| `required_credentials` | Yes | JSON array of logical credential names or credential type requirements only. An empty array is valid. Values and secrets are rejected. |
| `supported_platforms` | Yes | Non-empty JSON array of non-empty platform strings such as `windows-amd64`. |
| `plugins` | No | Optional JSON array of portable slash paths to plugin resource files. Listed files must exist, resolve inside the pack, and be regular files. The validator does not execute plugins. |
| `assets` | No | Optional JSON array of portable slash paths to asset files. Listed files must exist, resolve inside the pack, and be regular files. The validator does not interpret asset contents. |
| `config_schema` | No | Optional setup metadata for non-secret pack configuration. |
| `credential_requirements` | No | Optional structured credential slots without values. |
| `bindings` | No | Optional declarative mappings from setup values to existing workflow node parameters. |

Unknown fields are accepted for forward compatibility, but known fields are validated strictly. Manifest fields that look like secret-bearing fields, such as `secrets`, `password`, `token`, or `api_key`, are rejected when they contain values.

Required fields must be present in `pack.json`; zero values caused by missing fields are not accepted. `required_credentials` and `supported_platforms` must be arrays and cannot be `null`.

## Optional Setup Metadata

Pack Format v1 supports optional setup metadata while keeping `schema_version: 1`. Existing packs that omit these fields remain valid.

### config_schema

`config_schema` is an optional array of non-secret configuration fields for appliance setup. Supported field types are:

- `string`
- `url`
- `integer`
- `boolean`
- `select`

Each item includes:

- `key`: lowercase letters, numbers, and underscores.
- `label`: human-readable text.
- `description`: optional bounded text.
- `type`: one of the supported field types.
- `required`: boolean.
- `test_kind`: optional closed-allowlist server-side validation for the current value. `http_json_contract` is supported only for `url` fields bound to an HTTP Request `url` parameter whose node declares `response_contract`.
- `default`: optional non-secret default with the correct JSON type.
- `options`: required for `select`, unique bounded scalar values.
- `min` and `max`: optional integer limits.
- `min_length` and `max_length`: optional string limits.
- `display_only`: optional marker for required values that are intentionally shown but not bound into the workflow.

Configuration is not a secret store. Keys, labels, descriptions, defaults, and options that imply or look like tokens, passwords, API keys, private keys, authorization headers, credentials, or other secret material are rejected.

### credential_requirements

`credential_requirements` is an optional structured replacement for UI setup needs. It describes slots, not values:

- `key`
- `label`
- `description`
- `type`
- `required`
- `test_kind`
- `display_only`

Allowed credential types are currently:

- `API_KEY`
- `TELEGRAM_BOT`
- `BEARER_TOKEN`
- `BASIC_AUTH`
- `OPENAI_API_KEY`
- `DEEPSEEK_API_KEY`
- `GOOGLE_SERVICE_ACCOUNT`
- `DATABASE_URL`
- `SSH_KEY`
- `SMTP_ACCOUNT`

Allowed connection test kinds are currently:

- `telegram_get_me`
- `http_head`
- `smtp_noop`
- `database_ping`

No arbitrary test URL, command, script, plugin, or secret value is allowed in manifest setup metadata.

The compatibility allowlist is closed:

| Credential type | Allowed test kinds |
| :--- | :--- |
| `TELEGRAM_BOT` | `telegram_get_me` |
| `API_KEY` | `http_head` |
| `BEARER_TOKEN` | `http_head` |
| `BASIC_AUTH` | `http_head` |
| `SMTP_ACCOUNT` | `smtp_noop` |
| `DATABASE_URL` | `database_ping` |
| `OPENAI_API_KEY` | none yet |
| `DEEPSEEK_API_KEY` | none yet |
| `GOOGLE_SERVICE_ACCOUNT` | none yet |
| `SSH_KEY` | none yet |

An empty `test_kind` is always allowed. Impossible combinations, such as `SSH_KEY` with `telegram_get_me`, are rejected.

Legacy `required_credentials` remains valid. When `credential_requirements` is present, appliance setup should use the structured requirements. When it is absent, legacy entries are treated as simple logical credential requirements for display and backwards compatibility.

### bindings

`bindings` maps setup values to parameters on existing nodes in the entry workflow:

```json
{
  "source": "config.source_url",
  "target": {
    "node_id": "fetch",
    "param": "url"
  }
}
```

Rules:

- `source` must be exactly `config.<key>` or `credential.<key>`.
- The source key must exist in `config_schema` or `credential_requirements`.
- The target node must exist in the entry workflow.
- The target parameter must be declared by that node type.
- Credential sources may bind only to parameters of type `credential`.
- Config sources may not bind to credential or secret-like parameters.
- Duplicate source/target pairs are rejected.
- A target parameter may be bound at most once. A second binding to the same `node_id` plus `param` is rejected even when the sources differ.
- One source may fan out to multiple distinct targets.
- Required setup items must be bound or explicitly marked `display_only: true`.
- Bindings are applied only to a runtime copy of the managed workflow. Source packs and extracted bundles remain immutable.

### Setup Metadata Limits

Named limits:

- `config_schema`: 32 fields.
- `credential_requirements`: 32 entries.
- `bindings`: 128 entries.
- Setup key: 64 characters.
- Label: 120 characters.
- Description: 1000 characters.
- Select option: 120 characters.
- String default: 1000 characters.
- Integer absolute value: 1,000,000,000.
- Total serialized setup metadata: 64 KiB.

Validation errors identify the field and logical item involved without echoing possible secret values.

### URL Defaults

For `config_schema` fields with `type: "url"`, a non-null `default` must be an absolute URL with a host. Only `http` and `https` are accepted in Pack Format v1 setup metadata. Relative URLs, malformed URLs, and local file or custom schemes are rejected.

### Runtime Resolution

Pack setup bindings are applied to a cloned runtime workflow, never to the source pack, extracted bundle, or stored source workflow definition. Config bindings copy validated non-secret config values. Credential bindings copy credential IDs only; decrypted values remain in the encrypted credential store.

Pack runtime parameter resolution uses a small path-only expression language:

- `{{input.store_name}}`
- `{{nodes.fetch.data.total}}`
- `{{pack.config.report_title}}`

The resolver reads only trigger input, prior node outputs, and non-secret pack config. It does not read credentials, environment variables, files, functions, JavaScript, templates with loops, reflection, or commands. A parameter that is exactly one expression preserves the resolved JSON type. Inline string interpolation converts non-string values with deterministic JSON formatting. Missing or unsupported paths fail with bounded errors that name the expression, not the resolved value.

### Runtime Node Network Policy

Pack-compatible HTTP Request execution accepts only absolute `http` and `https` URLs and bounded request bodies, response bodies, and headers. Supported methods are `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, and `HEAD`. Custom headers must be a JSON object of strings. Redirect handling does not automatically carry `Authorization` or `Cookie` headers to a different origin.

Telegram execution uses the execution context for cancellation and bounds/redacts API error bodies. When `credential_id` is present, the encrypted credential value resolved by the runtime is preferred and a missing credential is an error; the node does not fall back to a literal `bot_token` in that case. Pack validation already rejects literal `bot_token` values in pack workflows.

### Appliance Runtime API

Pack Run may start Goflow with an explicit in-memory appliance context. In that mode only, `/api/appliance/*` endpoints expose bootstrap, setup, workflow status, run-now, execution summaries, and diagnostics for the single managed workflow. Generic `goflow serve` does not mount these routes.

State-changing appliance endpoints require a loopback Host match, exact Origin match, JSON content type, strict body limits, and the per-process appliance session token from bootstrap. Credential connection tests are explicit POST operations and are rate/concurrency limited.

Source tests are also explicit, rate-limited, and serialized. `http_json_contract` performs a bounded `GET` with a maximum 10-second timeout, accepts only `http`/`https` and HTTP 2xx, parses JSON, and validates the workflow node's reusable response contract. It never returns the response body or a URL query to the appliance UI. Setup completion repeats required source and credential tests against persisted current values, so a result for an older URL or chat cannot complete setup.

### HTTP JSON Response Contracts

An HTTP Request node may declare an optional generic `response_contract`. When present, runtime execution requires HTTP 2xx, valid JSON, and all declared fields before any downstream node can run. Unknown response fields remain allowed for forward compatibility.

```json
{
  "response_contract": {
    "required": {
      "report_date": { "type": "string", "non_empty": true },
      "revenue": { "type": "number" },
      "order_count": { "type": "integer", "minimum": 0 }
    }
  }
}
```

Supported rule types are `string`, `number`, `integer`, and `boolean`. `non_empty` applies only to strings; `minimum` applies only to numbers and integers. Pack validation rejects a config source test unless its binding reaches an HTTP `url` with a valid contract. Runtime failures use bounded public categories such as `source_timeout`, `source_unreachable`, `source_http_error`, `source_non_json`, `source_invalid_json`, `source_contract_invalid`, and `source_response_too_large`; response bodies are not included.

For `TELEGRAM_BOT`, `telegram_get_me` verifies both bot identity with `getMe` and the configured bound `chat_id` with `getChat`. It sends no test message. Public failures distinguish `telegram_unauthorized` from `telegram_chat_inaccessible` and never return the token, credential ID, or Telegram response body.

Runtime and diagnostics responses expose pack identity, logical setup readiness, workflow state, and bounded execution summaries only. They do not expose decrypted credentials, credential IDs, database contents, master keys, full logs, arbitrary files, environment variables, hostnames, usernames, or absolute source/build paths.

### Pack-Only Workflow Secret Scan

In pack context, workflow parameters known to carry secrets must not contain literal values. For example, a Telegram node may bind or select `credential_id`, but a pack workflow with a non-empty literal `bot_token` is rejected. This restriction applies to packs only and does not silently rewrite generic non-pack workflows.

## Portable Paths

Manifest paths are logical slash paths. They use `/` regardless of the host operating system. The validator rejects:

- Empty paths.
- Backslashes.
- Unix absolute paths such as `/workflows/main.json`.
- Windows drive paths such as `C:/file`, `C:\file`, or `C:file`.
- UNC or device paths.
- Empty, `.`, or `..` segments.
- Path segments containing `:`.
- Path segments ending in `.` or a space.
- Windows reserved device names such as `CON`, `PRN`, `AUX`, `NUL`, `COM1` through `COM9`, and `LPT1` through `LPT9`, including when an extension is present.

After this logical validation, paths are converted to the host OS path format and resolved with symlinks before containment checks.

## Validation

Run:

```bash
goflow pack validate examples/packs/hello-webhook
```

Success output is intentionally short:

```text
Pack is valid
ID: example.hello-webhook
Version: 0.1.0
Entry workflow: workflows/main.json
```

Validation checks:

- `pack.json` exists, is JSON, and is at most 1 MiB.
- `pack.json` is a regular file and is not a symlink.
- `schema_version` is `1`.
- Required fields are present and use the expected JSON types.
- `id`, `name`, `version`, `entry_workflow`, `required_credentials`, and `supported_platforms` follow the v1 rules.
- `entry_workflow` is a portable slash path inside the pack directory.
- Absolute paths, backslashes, drive paths, and dot segments are rejected.
- Symlink escapes are rejected for the entry workflow.
- The entry workflow exists and is a regular file.
- Workflow JSON is at most 10 MiB.
- The existing Goflow workflow validator accepts the entry workflow.
- Plugin and asset paths listed in the manifest must exist, resolve inside the pack, and be regular files.

## Build

Run:

```bash
goflow pack build examples/packs/hello-webhook --output release
```

Optional flags:

```bash
goflow pack build <pack-directory> \
  --output <output-directory> \
  [--target <goos-goarch>] \
  [--force]
```

The output is a portable pack bundle named:

```text
<pack-id>-<pack-version>-<target>.zip
```

Example:

```text
example.hello-webhook-0.1.0-windows-amd64.zip
```

`--target` defaults to the platform of the running Goflow binary, such as `windows-amd64`, `linux-amd64`, or `darwin-arm64`. This phase only supports same-platform runtime packaging. Cross-target builds fail clearly and must be implemented in a later phase. The target must also be listed in `supported_platforms`.

ZIP layout:

```text
goflow.exe
pack/pack.json
pack/workflows/main.json
pack/plugins/...
pack/assets/...
PACK_INFO.json
README.txt
```

On Linux and macOS the runtime entry is `goflow` instead of `goflow.exe`. The runtime is not renamed to the pack ID.

Only controlled files are included:

- The current Goflow runtime executable.
- `pack.json`.
- The entry workflow.
- Plugin files listed in `plugins`.
- Asset files listed in `assets`.
- Generated `PACK_INFO.json`.
- Generated `README.txt`.

The builder does not copy the whole pack directory. Unlisted files such as `.env`, `goflow.db`, `goflow.master.key`, notes, unlisted plugins, and unlisted assets are excluded.

### PACK_INFO.json

`PACK_INFO.json` is deterministic machine-readable metadata:

```json
{
  "schema_version": 1,
  "pack_id": "example.hello-webhook",
  "pack_version": "0.1.0",
  "target": "windows-amd64",
  "runtime_entry": "goflow.exe",
  "entry_workflow": "pack/workflows/main.json",
  "files": [
    {
      "path": "pack/pack.json",
      "sha256": "...",
      "size": 123
    }
  ]
}
```

The file inventory is sorted by archive path and includes SHA-256 plus size for all files except `PACK_INFO.json` itself. It includes the runtime and generated `README.txt`. It does not contain absolute local paths, usernames, hostnames, build machine paths, or timestamps.

After writing the temporary ZIP, the builder reopens that ZIP and verifies the bytes actually stored in the archive against `PACK_INFO.json`. It rejects missing inventory entries, extra ZIP entries, duplicate ZIP entry names, malformed or missing `PACK_INFO.json`, and hash or size mismatches before publishing the final artifact.

When a portable bundle is extracted and launched directly, Goflow verifies the extracted directory before starting Pack Run. The verifier reads `PACK_INFO.json` with a bounded reader, validates `pack/pack.json`, and confirms:

- Every inventory path is unique, relative, safe, and not `PACK_INFO.json` itself.
- Every inventoried regular file exists, is not a symlink, stays inside the extracted directory, and matches the recorded SHA-256 and uncompressed size.
- No unexpected regular file or symlink exists in the extracted controlled bundle directory, except `PACK_INFO.json` itself.
- `PACK_INFO.pack_id`, `pack_version`, and `entry_workflow` match `pack/pack.json`.
- `PACK_INFO.target` is listed in `supported_platforms`.
- `runtime_entry` matches the target platform: `goflow.exe` for Windows targets and `goflow` for Linux/macOS targets.

These hashes detect extraction or local file inconsistencies. They do not establish publisher authenticity because `PACK_INFO.json` is unsigned.

### Determinism And Output Safety

Builds use sorted ZIP entries, a fixed ZIP timestamp of `1980-01-01T00:00:00Z`, fixed compression method, and stable JSON formatting. The builder writes a temporary archive in the output directory and renames it to the final archive only after the ZIP is complete.

By default, an existing output archive is not overwritten. With `--force`, the new temporary archive is fully built and verified before replacing the existing archive. If direct replacement is not supported by the platform, Goflow moves the old archive to a backup name in the same directory, moves the new archive into place, and attempts to restore the backup if the final move fails. The backup is removed only after the final artifact is in place.

### Permission Policy

The runtime ZIP entry is marked executable (`0755`). Listed plugin files preserve executable intent: if the resolved source plugin has an execute bit, the ZIP entry is written with sanitized executable mode (`0755`); otherwise it remains non-executable (`0600`). Manifest, workflow, assets, `README.txt`, and `PACK_INFO.json` are non-executable. Special permission bits such as setuid, setgid, and sticky are not stored. Symlink objects are not stored; allowed internal symlinks are dereferenced into regular file entries under their logical manifest path.

### Size Limits

Named limits:

- `pack.json`: 1 MiB.
- Entry workflow JSON: 10 MiB.
- Each listed plugin or asset file: 100 MiB.
- Total pack payload excluding the runtime: 512 MiB.
- Runtime executable copied into the bundle: 256 MiB.
- `PACK_INFO.json` read during verification: 1 MiB.
- ZIP entries processed during verification: 4096.

The builder checks file sizes with `stat` before writing the ZIP, rechecks inventory sizes after hashing, and verifies actual uncompressed ZIP entry sizes by streaming archive contents with limits. It streams files while hashing and archiving, so it does not allocate memory for entire plugin or asset files.

## Author CLI

Pack authors can use the stable local workflow below:

```bash
goflow pack init <directory> --id <id> --name <name>
goflow pack validate <directory>
goflow pack inspect <directory|bundle.zip|extracted-directory> --output table
goflow pack test <directory> --output json
goflow pack build <directory> --output <output-directory>
goflow pack verify <bundle.zip|extracted-directory> --output table
```

`pack init` creates a deterministic safe scaffold and refuses non-empty target directories unless `--force` is supplied. `pack inspect` reports pack identity, target support, setup counts, controlled file counts, plugin/asset counts, and integrity status without printing workflow parameter values. `pack test` is offline: it validates setup metadata, applies synthetic non-secret config and fake credential IDs in temporary state, prepares the managed workflow idempotently, and reports connection tests as skipped when they require an external service. `pack verify` reuses bundle verification and does not run or import the pack.

See [PACK_AUTHOR_TUTORIAL.md](PACK_AUTHOR_TUTORIAL.md) for a PowerShell and POSIX walkthrough.

Operator docs:

- [APPLIANCE_QUICKSTART.md](APPLIANCE_QUICKSTART.md)
- [APPLIANCE_TROUBLESHOOTING.md](APPLIANCE_TROUBLESHOOTING.md)
- [DATA_BACKUP_RESTORE.md](DATA_BACKUP_RESTORE.md)
- [CREDENTIAL_ROTATION.md](CREDENTIAL_ROTATION.md)
- [DAILYOPS_DEMO_GUIDE.md](DAILYOPS_DEMO_GUIDE.md)
- [DEVELOPMENT_ARTIFACTS.md](DEVELOPMENT_ARTIFACTS.md)
- [PILOT_GUIDE.md](PILOT_GUIDE.md)

## Run

Run a source pack directory:

```bash
goflow pack run examples/packs/hello-webhook --no-open
```

Optional flags:

```bash
goflow pack run <pack-directory> \
  [--data-dir <directory>] \
  [--port <port>] \
  [--no-open]
```

Pack Run MVP starts Goflow in-process on loopback only. The host is always `127.0.0.1`; `--port` defaults to `0` so the OS selects a free port. The command prints the actual URL. `--no-open` suppresses browser launch.

Runtime state is stored outside the pack directory by default:

- Windows: `%LOCALAPPDATA%/Goflow/packs/<pack-id>/`
- macOS: `~/Library/Application Support/Goflow/packs/<pack-id>/`
- Linux: `$XDG_DATA_HOME/Goflow/packs/<pack-id>/` or `~/.local/share/Goflow/packs/<pack-id>/`

Use `--data-dir` for tests or controlled deployments. The data directory contains `goflow.db`, `goflow.master.key`, `pack-state.json`, `run-state.json`, setup files, and lock metadata. Back up `goflow.db` together with `goflow.master.key`.

Runtime setup state is stored outside the source pack and extracted bundle:

- `pack-config.json` contains non-secret config values, the pack ID, and a config schema version. Values are revalidated against the current manifest before use. Unknown fields are retained only when they are safe and are not applied as current config.
- `pack-credentials.json` contains credential slot assignments as credential IDs plus expected credential types. It never contains decrypted credential values. Slot assignments are valid only when the referenced credential still exists and has the type declared by `credential_requirements`. Unknown slots are retained only when their keys and IDs are safe.

Both setup files are written atomically with restricted file permissions where supported. Parent setup directories are created with restricted permissions where supported.

The pack workflow is a managed workflow. Its workflow ID is deterministic from the pack ID, so repeated runs update the same record instead of creating duplicates. A same-version restart preserves completed setup and the active bound workflow. A pack version change preserves the stable workflow ID, database, credential records, slot assignments, config, and execution history, but fails setup closed to incomplete until current source and destination checks are run again. This conservative migration prevents a behavior-changing pack from inheriting stale acceptance evidence without requiring the user to re-enter the encrypted credential.

### Versioned setup migration

Pack setup migration is host-managed. A Pack cannot declare executable
migration code, commands, scripts, plugins, or download locations. Goflow uses
a closed registry keyed by Pack ID and exact source version:

- steps run in an ordered forward-only chain and never downgrade automatically;
- known config transforms operate on bounded non-secret JSON values;
- credential slots are retained as IDs and expected types only; migration never
  resolves, copies, or decrypts a credential value;
- a pre-mutation snapshot with a sorted SHA-256 inventory is written under the
  Pack data directory, outside the application/bundle directory;
- transformed values are validated against the destination manifest in memory,
  then setup files are atomically replaced; a failed multi-file operation
  compensates by restoring every original file from memory;
- completion becomes incomplete before the appliance starts serving, and an
  enabled schedule retains its configuration but enters
  `NEEDS_ATTENTION/revalidation_required`;
- repeated startup recognizes the recorded migration and does not reapply it;
- unknown changes preserve safe data but require explicit user review;
- corrupt or future migration/config/credential/state schemas fail closed.

Migration categories are `revalidation` (values retained), `config` (a
registered non-secret transform ran), and `user_review` (no complete registered
chain exists). They are user-attention states, not claims that validation
succeeded. The latest Pack workflow definition replaces the inactive managed
workflow content while the stable workflow ID and execution history remain.

`required_credentials` remains metadata only. Pack Run prints credential requirements and opens the credentials page when requirements exist, but it does not embed secrets, create placeholder credentials, or treat matching names as proof that requirements are satisfied.

Packaged plugin execution is not supported in Pack Run MVP. Packs with non-empty `plugins` fail early.

## Security Boundary

Pack validation and build are static. They do not execute plugins, start the server, read credentials, write credentials, or modify the database. Pack Run starts a loopback server and writes runtime state only to its data directory. `required_credentials` is metadata only; it describes logical needs such as `smtp_account` or `slack_bot_token`, not secret values.

`pack.json` symlinks are rejected. Entry workflow, plugin, and asset symlinks are allowed only when the fully resolved target remains inside the pack directory and is a regular file.

Pack validation is not a trust system. A valid pack may still contain plugin files or workflow behavior that operators should review before any future install or run command uses it.

`PACK_INFO.json` is integrity metadata, not a signature. It can detect that extracted files no longer match the bundle metadata, but it does not prove who created the bundle.

## Development Artifacts

GitHub Actions may produce unsigned alpha artifacts only from the manual `workflow_dispatch` path. Artifact names must include `UNSIGNED-DEVELOPMENT-ALPHA`, include SHA-256 checksum files and deterministic build metadata, and must not create tags, GitHub Releases, installers, signatures, or latest-version pointers.

Development artifacts inherit the repository artifact retention policy configured in GitHub Actions. They are temporary CI outputs for pilot verification, not production releases or authenticity claims. Before acceptance, generated artifact members and extracted contents are scanned for canary secrets, local paths, usernames, hostnames, database/key files, `.env`, and unlisted runtime state.

## Non-Goals

This v1 foundation does not support:

- Building new `.exe` files or cross-target runtimes.
- Installing packs.
- Marketplace publishing or discovery.
- Plugin signing.
- Auto-update.
- Secret distribution.
