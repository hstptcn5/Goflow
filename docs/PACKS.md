# Goflow Pack Format v1

Goflow Pack is a directory format for distributing one workflow with pack metadata and optional local resources. This foundation only validates packs. It does not install, run, build, or update them.

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

## Security Boundary

Pack validation is static. It does not execute plugins, start the server, read credentials, write credentials, or modify the database. `required_credentials` is metadata only; it describes logical needs such as `smtp_account` or `slack_bot_token`, not secret values.

`pack.json` symlinks are rejected. Entry workflow, plugin, and asset symlinks are allowed only when the fully resolved target remains inside the pack directory and is a regular file.

Pack validation is not a trust system. A valid pack may still contain plugin files or workflow behavior that operators should review before any future install or run command uses it.

## Non-Goals

This v1 foundation does not support:

- Building `.exe` files.
- Installing packs.
- Running packs directly.
- Marketplace publishing or discovery.
- Plugin signing.
- Auto-update.
- Secret distribution.
- Appliance UI.
