# Local Pilot Diagnostics Contract

The appliance diagnostics endpoint and its explicit **Copy diagnostics** and
**Download diagnostics** actions produce the same local-only JSON summary. No
analytics SDK, remote collection endpoint, device fingerprint, or background
telemetry is part of this contract.

## Data Dictionary

| Field | Values and bound | Privacy rationale |
| --- | --- | --- |
| `schema_version` | integer, currently `1` | Supports fail-closed consumer changes without identifying a user or device. |
| `app.name` | constant `Goflow` | Identifies the software being diagnosed. |
| `app.version` | bounded build version | Distinguishes behavior across builds; contains no machine data. |
| `app.platform` | Go OS/architecture pair | Identifies the supported runtime target without hostnames or hardware fingerprints. |
| `pack.id` | validated Pack ID | Identifies the installed workflow contract, not a local path. |
| `pack.version` | validated SemVer | Identifies the Pack contract under test. |
| `setup.state` | `READY` or `NEEDS_SETUP` | Reports readiness without configuration values. |
| `setup.category` | `ready`, `setup_incomplete`, `migration_required`, or `revalidation_required` | Gives a bounded next-action reason without migration internals. |
| `schedule.configured` | boolean | Reports whether a managed schedule exists. |
| `schedule.enabled` | boolean | Reports the user's local schedule preference. |
| `schedule.state` | bounded schedule state | Reports operational schedule health without trigger identifiers or times. |
| `schedule.error_category` | closed public category, optional | Gives a safe failure class without raw errors. |
| `recent_executions` | at most 10 newest summaries | Bounds export size and excludes IDs, timestamps, inputs, outputs, logs, and request metadata. |
| `recent_executions[].status` | bounded execution status | Reports outcome only. |
| `recent_executions[].duration_ms` | non-secret duration | Supports local performance feedback without wall-clock timestamps. |
| `recent_executions[].error_category` | closed public category, optional | Replaces raw and legacy error text with a fixed safe class. |
| `integrity.state` | `verified`, `source_validated`, or `unknown` | Reports the local verification path without filenames or digests tied to a machine. |
| `privacy.local_only` | constant `true` | Makes the collection boundary explicit. |
| `privacy.credential_ids_hidden` | constant `true` | Confirms credential record identifiers are omitted. |
| `privacy.secrets_hidden` | constant `true` | Confirms secret-bearing fields are omitted. |

The export never includes credential IDs or values, workflow/execution IDs,
source URLs or query strings, Telegram chat IDs, payloads, response bodies,
logs, request/idempotency metadata, hostnames, usernames, databases, keys, or
filesystem paths. The JSON has no generation timestamp, so an unchanged local
state produces the same summary after restart.

## Public Error Categories

The closed public vocabulary covers source invalid/unreachable/timeout/contract
mismatch; Telegram bot unauthorized/chat not found/unreachable; already
running; schedule invalid/missed skipped; migration/revalidation required;
artifact tamper; setup incomplete; bounded request conflicts; cancellation; and
internal failure. Unknown or malformed legacy categories become
`internal_error` with fixed text. Raw persisted error messages are never copied
into diagnostics.

## Execution Retention

Execution cleanup defaults to 30 days and 1,000 records per workflow. Supported
configuration ranges are 1-365 days and 1-10,000 records per workflow; invalid
environment values fall back to the defaults. Age and count pruning run in one
database transaction and never delete `RUNNING` executions. Active execution
count remains separately bounded by engine concurrency.
