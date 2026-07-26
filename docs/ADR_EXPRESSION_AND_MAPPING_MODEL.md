# ADR: Expression And Data Mapping Model

Status: Accepted for UX Milestone 3

Date: 2026-07-26

## Context

Goflow already resolves runtime placeholders in node parameters through the backend node evaluator. The supported syntax is:

```text
{{node_id.path.to.value}}
{{$trigger.path.to.value}}
```

When the entire parameter value is one placeholder, the runtime preserves the resolved value type. When the placeholder is embedded inside other text, the runtime interpolates it as a string.

## Decision

Milestone 3 keeps the existing placeholder syntax. The Web UI does not introduce a new expression language and does not store UI-only expression metadata inside node params.

Parameter mode is derived from the saved value:

- **Fixed**: normal literal parameter value.
- **Expression**: value is a complete runtime placeholder such as `{{json_1.transformed.user.email}}`.

Switching to Expression mode opens the data picker and keeps the existing value unless the user chooses a source path. Switching back to Fixed does not rewrite the value.

## Preview

Expression preview is frontend-only for Milestone 3 and uses the same placeholder contract:

- Source data comes from latest execution logs returned by the REST API.
- Upstream node output is preferred before trigger input.
- Preview never executes a node or workflow.
- Missing source/path returns a clear preview error.
- Preview is bounded by tree/raw output truncation.

No new backend preview endpoint was added in this milestone. If preview needs backend parity for more complex expressions later, add an authenticated, read-only endpoint with size limits, timeout, workflow allowlist enforcement, and redaction tests.

## Security

Inspector rendering applies frontend display redaction for common secret keys and bearer-like values:

```text
password
api_key
authorization
cookie
private_key
access_token
refresh_token
secret
bearer
```

Backend redaction remains the source of truth for stored execution logs and API responses. The frontend redaction is an additional display guard and must not be treated as permission enforcement.

## Compatibility

- Existing workflows continue to load because params remain plain JSON values.
- Export/import behavior is unchanged.
- CLI validation is not affected by new UI-only fields because no UI-only fields are written.
- The frontend validator now blocks expressions that reference a missing node or a node that is not upstream of the selected node.
- `$trigger` remains allowed as a source reference.

## Migration

No database migration is required.

No workflow JSON migration is required.

## Known Limitations

- Preview supports complete single-placeholder expressions only.
- Embedded string interpolation still runs in backend at execution time, but the UI preview reports that mixed placeholder text cannot be previewed exactly yet.
- Resolved parameters are only shown in Logs when the backend records redacted resolved parameters; current node logs usually contain status, attempts, duration, output, and error.
