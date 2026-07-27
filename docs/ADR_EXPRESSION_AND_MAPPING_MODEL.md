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

Parameter mode is held as temporary component state while the inspector is open and is re-derived from the saved value when a node is selected or reloaded:

- **Fixed**: normal literal parameter value.
- **Expression**: value is a complete runtime placeholder such as `{{json_1.transformed.user.email}}`.

Switching to Expression mode opens the data picker and keeps the existing literal value until the user chooses a source path. Number and integer fields render as text/code inputs while they are in Expression mode so placeholders remain visible and editable; they render as numeric inputs only in Fixed mode.

Switching back to Fixed converts a complete expression only when the current resolved preview is a primitive value. Object and array previews stay in Expression mode by default. JSON fields expose an explicit `Convert to JSON literal` action for object and array previews; canceling that action by not invoking it leaves both mode and value unchanged. The UI must not enter a `Fixed` mode with a `{{...}}` expression value.

Numeric node parameters that support only whole numbers should use the `integer` ParamDefinition type. At runtime, complete expressions can resolve to typed numbers rather than strings. Executors must parse accepted numeric shapes explicitly instead of silently falling back to defaults. For example, `delaySleep.seconds` accepts integer string, int/int32/int64, integer float32/float64, and `json.Number`, rejects fractional, zero, negative, NaN/Infinity, and oversized values, and preserves the configured numeric delay.

## Preview

Expression preview is frontend-only for Milestone 3 and uses the same placeholder contract:

- Source data comes from the latest server-redacted execution inspector DTO returned by the REST API.
- Direct input nodes are shown first, then earlier transitive upstream nodes, then trigger input.
- Preview never executes a node or workflow.
- Missing source/path returns a clear preview error.
- Preview is bounded by tree/raw output truncation.

No new backend preview endpoint was added in this milestone. If preview needs backend parity for more complex expressions later, add an authenticated, read-only endpoint with size limits, timeout, workflow allowlist enforcement, and redaction tests.

## Security

Inspector API responses are server-redacted before they reach the UI. The DTO includes execution metadata, redacted trigger input, redacted node logs, redacted node output, redacted node error, duration, and attempts. Inspector rendering also keeps frontend display redaction as defense in depth for common secret keys and token-like values:

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
ghp_
github_pat_
xoxb-
```

Backend redaction remains the source of truth for inspector API responses. Frontend redaction must not be treated as permission enforcement.

## Compatibility

- Existing workflows continue to load because params remain plain JSON values.
- Export/import behavior is unchanged.
- CLI validation is not affected by new UI-only fields because no UI-only fields are written.
- The frontend validator now blocks invalid JSON/URL/number/integer/select literals, invalid expression syntax, missing expression sources, and sources that are not upstream of the selected node before Test or Activate.
- Save draft still only blocks structural graph errors.
- `$trigger` is resolved by the backend through the shared execution context.
- Backend execution preserves the resolved type for complete expressions over strings, numbers, booleans, objects, arrays, and `$trigger` values. Mixed interpolation stringifies non-string values according to the current runtime placeholder contract.
- Skipped branch nodes are logged as `SKIPPED` so inspector/runtime parity tests can distinguish the executed path.
- Execution debugging views must use one active execution context at a time. Canvas status, edge overlays, inspector tabs, and debug bundles read from the selected execution, latest execution, or matching live execution consistently; WebSocket events from other executions must not override a selected historical execution.
- Replay uses stored execution input with the current saved workflow definition. It is not an exact historical replay because workflow snapshots are not stored yet.

## Migration

No database migration is required.

No workflow JSON migration is required.

## Known Limitations

- Preview supports complete single-placeholder expressions only.
- Embedded string interpolation still runs in backend at execution time, but the UI preview reports that mixed placeholder text cannot be previewed exactly yet.
- Resolved parameters are only shown in Logs when the backend records redacted resolved parameters; current node logs usually contain status, attempts, duration, output, and error.
- A full execution selector and pinned sample data remain Milestone 4/5 work; Milestone 3 uses the latest execution/sample context.
