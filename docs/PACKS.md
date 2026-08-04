# Goflow Pack Format v1

Goflow Pack is a directory format for distributing one workflow with pack metadata and optional local resources. Goflow can validate a pack and build a portable pack bundle. It does not install, run, or update packs.

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

Unknown fields are accepted for forward compatibility, but known fields are validated strictly. Manifest fields that look like secret-bearing fields, such as `secrets`, `password`, `token`, or `api_key`, are rejected when they contain values.

Required fields must be present in `pack.json`; zero values caused by missing fields are not accepted. `required_credentials` and `supported_platforms` must be arrays and cannot be `null`.

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

### Determinism And Output Safety

Builds use sorted ZIP entries, a fixed ZIP timestamp of `1980-01-01T00:00:00Z`, fixed compression method, and stable JSON formatting. The builder writes a temporary archive in the output directory and renames it to the final archive only after the ZIP is complete.

By default, an existing output archive is not overwritten. With `--force`, the new temporary archive is fully built before replacing the existing archive. On Unix-like systems `rename` can replace atomically. On Windows, replacing an existing file may require removing the old archive after the new temporary archive has been completed, so there can be a small final replacement window.

### Size Limits

Named limits:

- `pack.json`: 1 MiB.
- Entry workflow JSON: 10 MiB.
- Each listed plugin or asset file: 100 MiB.
- Total pack payload excluding the runtime: 512 MiB.

The builder checks file sizes with `stat` before writing the ZIP and streams files while hashing and archiving, so it does not allocate memory for entire plugin or asset files.

## Security Boundary

Pack validation and build are static. They do not execute plugins, start the server, read credentials, write credentials, or modify the database. `required_credentials` is metadata only; it describes logical needs such as `smtp_account` or `slack_bot_token`, not secret values.

`pack.json` symlinks are rejected. Entry workflow, plugin, and asset symlinks are allowed only when the fully resolved target remains inside the pack directory and is a regular file.

Pack validation is not a trust system. A valid pack may still contain plugin files or workflow behavior that operators should review before any future install or run command uses it.

## Non-Goals

This v1 foundation does not support:

- Building new `.exe` files or cross-target runtimes.
- Installing packs.
- Running packs directly.
- `pack run`.
- Marketplace publishing or discovery.
- Plugin signing.
- Auto-update.
- Secret distribution.
- Appliance UI.
