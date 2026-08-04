# Goflow Pack Format v1

Goflow Pack is a directory format for distributing one workflow with pack metadata and optional local resources. This foundation only validates packs. It does not install, run, build, or update them.

## Directory Layout

```text
example-pack/
├── pack.json
├── workflows/
│   └── main.json
├── plugins/
└── assets/
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
| `id` | Yes | Stable ID using lowercase letters, numbers, dots, or hyphens. |
| `name` | Yes | Human-readable pack name. |
| `version` | Yes | Valid SemVer, including optional prerelease/build metadata. |
| `description` | No | Human-readable description. |
| `entry_workflow` | Yes | Relative path to a workflow JSON file inside the pack directory. |
| `required_credentials` | Yes | Logical credential names or credential type requirements only. Values and secrets are rejected. |
| `supported_platforms` | Yes | Informational platform strings such as `windows-amd64`. |
| `plugins` | No | Optional list of relative plugin resource paths reserved for future use. The validator checks path safety only and does not execute plugins. |
| `assets` | No | Optional list of relative asset paths reserved for future use. The validator checks path safety only. |

Unknown fields are accepted for forward compatibility, but known fields are validated strictly. Manifest fields that look like secret-bearing fields, such as `secrets`, `password`, `token`, or `api_key`, are rejected when they contain values.

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
- `schema_version` is `1`.
- `id`, `name`, `version`, `entry_workflow`, and `required_credentials` follow the v1 rules.
- `entry_workflow` is a relative path inside the pack directory.
- Absolute paths and `..` traversal are rejected.
- Symlink escapes are rejected for the entry workflow.
- The entry workflow exists and is a regular file.
- Workflow JSON is at most 10 MiB.
- The existing Goflow workflow validator accepts the entry workflow.
- Plugin and asset paths listed in the manifest are resolved as pack-local paths only.

## Security Boundary

Pack validation is static. It does not execute plugins, start the server, read credentials, write credentials, or modify the database. `required_credentials` is metadata only; it describes logical needs such as `smtp_account` or `slack_bot_token`, not secret values.

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
