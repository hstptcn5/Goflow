# Goflow Productization Decisions

## ADR-001: Managed Appliance Daily Scheduler

Status: Accepted for implementation after Checkpoint A CI

### Context

Goflow already scans active workflows for `cronTrigger` nodes and registers raw
cron expressions with `robfig/cron`. That feature serves advanced generic
workflows. A pilot appliance needs a smaller user contract, persisted status,
restart idempotency, setup gating, and deterministic tests.

### Decision

The managed appliance scheduler is a separate application service over the same
workflow and execution primitives, not a second execution path.

- A schedule belongs to one managed Pack workflow and is stored in SQLite next
  to workflow/execution state, never in the Pack or install directory.
- Manual and scheduled starts both call `application.TriggerService` and share
  the workflow's concurrency policy.
- Appliance scheduling supports `daily` at `HH:MM` in an IANA timezone. Raw cron
  is not exposed in the appliance beta path. Interval scheduling is deferred
  unless a later checkpoint demonstrates a necessary, bounded contract.
- New schedules are disabled by default. Setup completion does not silently
  enable a schedule.
- The primary Pack Run process owns scheduler lifecycle. A secondary launch
  reuses the primary process and never starts another scheduler.
- Missed-run policy is `skip`: startup and clock movement compute the next
  future eligible instant and do not replay elapsed periods.
- Every scheduled instant has a deterministic idempotency key. TriggerService
  and the existing unique execution index are the final duplicate barrier.
- The existing raw-cron scanner remains available to advanced workflows and is
  not changed into the appliance contract.

### Persisted Schema V1

The planned SQLite table has at most one row per managed workflow:

| Field | Type and bound | Meaning |
| --- | --- | --- |
| `workflow_id` | non-empty text, workflow FK, primary key | Stable managed workflow identity |
| `pack_id` | text, 1-200 characters | Prevents cross-Pack reuse |
| `schema_version` | integer, exactly `1` | Schedule row schema |
| `revision` | positive integer | Optimistic concurrency for config/status writes |
| `enabled` | boolean | Explicit user choice; defaults false |
| `kind` | enum `daily` | No raw cron |
| `local_time` | canonical `HH:MM` | Wall-clock target |
| `timezone` | valid IANA name, at most 255 characters | Location used for calculation |
| `missed_run_policy` | enum `skip` | No catch-up storm |
| `last_scheduled_for` | nullable UTC RFC3339 instant | Last instant admitted/observed |
| `next_run_at` | nullable UTC RFC3339 instant | Derived status, recomputed safely |
| `last_execution_id` | nullable bounded execution reference | Last scheduled result lookup |
| `state` | enum `OK`, `DISABLED`, `NEEDS_ATTENTION` | Fail-closed public state |
| `error_category` | nullable allowlisted bounded text | Safe diagnostics only |
| `created_at`, `updated_at` | UTC timestamps | Audit and optimistic status |

Known fields validate strictly. The store rejects unknown schema versions,
invalid enums/timestamps/zones, cross-Pack ownership, and impossible field
combinations. Database migration is transactional. Schedule schema upgrades are
ordered; unknown future versions fail closed and are never downgraded.

### Trigger Metadata

For a due instant `T` in UTC:

- source: a dedicated scheduled source value (kept distinct from UI/manual);
- principal: stable non-secret appliance scheduler principal;
- request ID: deterministic bounded identifier derived from Pack, workflow, and
  `T` without user data;
- idempotency key: versioned digest/domain string over Pack ID, workflow ID, and
  canonical UTC `T`.

The input contains only safe trigger metadata needed by the workflow. It does
not contain credentials, source responses, or local paths.

### Clock And DST Semantics

- The clock is injected into calculation and runner code.
- A normal daily target maps to the first matching instant after the cursor.
- If a local time does not exist during a forward DST transition, that local
  date is skipped; it is not shifted to an arbitrary time.
- If a local time occurs twice during a backward transition, the first instant
  is eligible and the deterministic key prevents a second delivery for that
  local date.
- A backward system-clock jump cannot clear `last_scheduled_for` or make an
  already admitted instant eligible again.
- A forward jump skips elapsed periods and selects the next future occurrence.
- A due instant has a one-minute admission grace so a short scheduling or
  restart delay can reconcile it. Anything older follows `skip` and advances to
  a future period without execution.
- Timezone database behavior is supplied by Go's `time.LoadLocation`. The
  IANA database is embedded with `time/tzdata` so a portable Windows appliance
  does not depend on a Go installation or host zoneinfo files.

### Persistence Order

The scheduler calls TriggerService with the deterministic key before advancing
`last_scheduled_for`. TriggerService creates or finds the execution record before
workflow side effects; the database uniqueness constraint is the atomic claim.
Only after that admission succeeds does the scheduler transactionally update its
schedule metadata. If the process fails after execution creation but before the
metadata update, restart uses the same key, receives the existing execution, and
reconciles the row without another delivery. If admission never created an
execution, missed-run `skip` semantics decide whether the instant is still due or
has elapsed; the service never marks it delivered in advance.

### Consequences

- Appliance scheduling is intentionally less flexible than advanced raw cron.
- SQLite becomes the authority for schedule state and duplicate prevention.
- Checkpoint B needs a clock interface, calculator, store, service, migration,
  and lifecycle injection, but no new execution engine.
- Cross-process correctness relies on Pack Run's existing primary lock plus the
  database idempotency uniqueness constraint, not an in-memory check alone.

### Appliance API And Test Control

- The appliance exposes one bounded schedule resource: GET/PUT with
  `enabled`, `local_time`, `timezone`, and optimistic `revision`.
- The response may include next/last instants and a redacted last execution
  summary. It never includes credentials, setup values, idempotency keys, or
  internal storage errors.
- A changed Pack configuration reopens setup and blocks scheduled admission
  until revalidation. Existing encrypted credential bindings remain assigned.
- Deterministic E2E control is injected only through Go interfaces and the
  `internal/testharness` binary. It is not a production CLI flag, environment
  variable, manifest field, HTTP route, or persisted value.
- DailyOps remains Pack version `0.2.0`: this checkpoint changes the appliance
  runtime and UI, not the Pack manifest/workflow contract. Therefore no Pack
  setup migration is required in Checkpoint C.

## ADR-002: No Installer Decision In Phase 1

Status: Accepted

Scheduler work does not select or download an installer toolchain. Installer or
portable-upgrade decisions belong to Checkpoint F after a pinned-toolchain and
supply-chain comparison.

## ADR-003: Host-Managed Pack Setup Migration

Status: Accepted

Pack upgrades use a closed migration registry compiled into Goflow. Pack
manifests cannot point at migration scripts, commands, plugins, binaries, or
network locations. A registry step is keyed by Pack ID and exact source/target
versions and is classified as `revalidation`, `config`, or `user_review`.

The migration runner is forward-only. It snapshots setup files with a sorted
SHA-256 inventory, reads bounded config and credential ID/type references,
applies the ordered chain in memory, validates against the destination manifest,
and atomically replaces individual files. A later write failure compensates by
restoring all original bytes. The migration record contains only versions,
category, step IDs, backup-relative path, and timestamp.

Unknown chains preserve safe values and require user review. Downgrades,
corrupt/future schemas, unsafe stale data, and invalid transformed data fail
closed. Setup becomes incomplete and the enabled schedule is suspended for
revalidation before the server starts. Stable workflow identity, encrypted
credential records, schedule configuration, and execution history remain in
their existing stores.

## ADR-004: Local-Only Bounded Diagnostics

Status: Accepted

Pilot diagnostics are an allowlisted, versioned DTO generated only in response
to a local appliance request and copied or downloaded only after an explicit UI
action. It contains build/Pack/platform identity, bounded setup and schedule
state, at most ten execution status/duration/error-category summaries, and an
integrity state. It excludes identifiers, timestamps, values, URLs, payloads,
responses, logs, and paths. There is no analytics SDK, remote collector,
fingerprint, or background telemetry.

Internal and legacy errors map to a closed public category catalog with fixed
messages. Unknown categories fail to `internal_error`; raw legacy messages are
not exported. Execution cleanup uses one transaction, validates finite safe
bounds, and excludes active `RUNNING` rows from both age and count pruning.

## ADR-005: Windows Beta Packaging And Offline Update

Status: Accepted for Checkpoint F implementation

### Compared Options

| Option | Per-user/admin and uninstall | Reproducibility and CI | Future signing path | Supply-chain assessment |
| --- | --- | --- | --- | --- |
| WiX Toolset 4 MSI | Can model per-user install, shortcuts, upgrades, and data-preserving uninstall, but Windows Installer identity and upgrade rules add a second lifecycle system | A specific .NET tool/NuGet version can be pinned, but this repository has no pinned .NET SDK, NuGet lock, or WiX verification gate | Supports Authenticode/MSI signing later | Adds .NET/NuGet/WiX compiler inputs and generated MSI behavior before signing infrastructure exists |
| Inno Setup | Straightforward per-user mode, shortcuts, upgrades, and uninstall scripting | Compiler can be version-pinned on `windows-latest`, but would require downloading or vendoring a new executable toolchain and verifying it | Supports Authenticode signing later | Adds a binary compiler and a second installer script runtime with no current repository provenance policy |
| MSIX / Windows SDK | Per-user deployment and clean uninstall are strong, but normal pilot installation depends on package trust/signing and Windows policy | Runner images include SDK tools, but image/SDK selection is not pinned by the repository today | Designed around package signing | Unsigned sideloading conflicts with the no-bypass pilot policy; SDK provenance would still need an explicit pin |
| Verified portable bundle plus PowerShell update helper | No installer or automatic shortcut; uninstall is explicit application-folder removal while external data is retained | Uses Windows PowerShell/.NET APIs already present on supported Windows and the exact current `goflow.exe pack verify`; no downloaded tool | The same PE/bundle can be Authenticode-signed later without changing state ownership | No new third-party build dependency; update remains explicit, offline, and inspectable |

### Decision

Checkpoint F does not add an installer. `BLOCKED_INSTALLER_TOOLCHAIN` records
that an installer candidate is deferred until its compiler/SDK, dependency
lock, provenance, uninstall identity, and signing verification can be pinned
and reviewed together. This is not a blocker for the required portable fallback.

The beta candidate remains a native Windows AMD64 portable bundle marked
`UNSIGNED-PILOT-BETA`. A bundled PowerShell helper performs only a user-started
offline update: reject an active instance, verify candidate archive and
extracted inventory before mutation, require matching Pack identity/target,
snapshot external data, retain the previous application directory, activate
the candidate, wait for local health, and compensate application/data state on
failure. It never downloads, silently updates, disables Windows protections,
or deletes user data. Successful update retains rollback material until the
user explicitly removes it.

### Revisit Gate

Reconsider an installer only after a security review chooses and pins the exact
toolchain, CI verifies deterministic inputs and uninstall behavior, and a
production code-signing process exists. Until then, no MSI, MSIX, setup EXE,
auto-update channel, or installer claim is permitted.

## ADR-006: Capability-Based Pack Author Compatibility

Status: Accepted for Checkpoint G implementation

Pack Format v1 gains optional `required_capabilities` and an optional
author-only `offline_test_fixture`. Omission preserves existing v1 behavior.
Capabilities use a versioned closed allowlist maintained by the runtime;
unknown, duplicate, malformed, or unavailable capabilities fail during Pack
validation with a precise manifest location. Platform compatibility continues
to use the existing explicit target list. Runtime min/max version guessing is
not added because the actual behavior boundary is the capability set.

The offline fixture is bounded JSON containing non-secret config values and
logical fake credential slots. It is validated, used only by `pack test`, and
never included in a built appliance. It cannot declare commands, scripts,
network endpoints to contact, schedule triggers, migration code, or credential
material. Connection checks remain skipped offline. Host-managed schedule and
migration behavior are represented by capabilities, not executable Pack hooks.

Bundle inventory remains authoritative over every shipped runtime and Pack
file. Only the manifest, entry workflow, and explicitly declared assets/plugins
are shipped; the verifier rejects extra, missing, duplicate, or modified archive
members. Author documentation and offline fixtures are source inputs, not
runtime assets, unless intentionally declared as an asset.

## ADR-007: Offline Ed25519 Pack Signing Foundation

Status: Accepted for consistency testing before implementation

Use standard-library Ed25519 over the deterministic binary payload specified in
`PACK_SIGNING.md`. Bind exact `PACK_INFO.json` bytes plus Pack ID, version,
target, required capabilities, algorithm, schema, and operator-selected key ID.
Store one strict root `PACK_SIGNATURE.json` outside `PACK_INFO.files` to avoid a
self-signing cycle. Verification checks inventory first and accepts trust only
from an explicit public key/key ID supplied by the operator.

Do not add certificates, a registry, remote key lookup, trust-on-first-use,
automatic trust-store mutation, downgrade acceptance, production keys, signing
claims, or release publishing. If canonical payload, container, trust input, or
failure semantics cannot be proven by the complete H matrix, record
`BLOCKED_SECURITY_REVIEW` and ship only the specification.

## ADR-008: Declarative Normalized HTTP Source Adapter

Status: Accepted for Checkpoint I implementation

The first adapter boundary is one reviewed built-in node,
`normalizedHttpSource`, not a Pack executable or plugin process. It owns a
bounded HTTP GET source call, optional vault-backed bearer/API-key auth,
same-origin safe redirects, cursor pagination, bounded rate-limit retry, fixed
vendor/source error mapping, and normalized output validation. Workflows consume
only its normalized data output. Destination behavior remains a separate node.

Declarative parameters are strict: absolute HTTP(S) URL, auth mode and optional
credential ID, pagination mode, cursor/query/response field names, finite page
and item limits, and a response contract. Single-object and cursor item outputs
contain only declared fields. Cursor mode requires
`{items: [...], next_cursor: string|null}` semantics and emits a bounded
`{items, page_count}` object. GET makes retry idempotent; redirects never forward
authorization cross-origin. `429 Retry-After` is accepted only as a small
bounded delta through an injectable waiter for deterministic tests.

The node returns no token, authorization header, source URL, raw response body,
or vendor-specific payload outside the declared normalized result. Errors map
to closed public categories before any destination side effect. DailyOps may
continue using the generic REST node because it already consumes the normalized
contract; vendor-specific mapping is not added before the decision gate.

A future process adapter remains design-only until sandboxing, protocol,
capability grants, executable authenticity, update, crash isolation, and secret
transport receive a separate security review. Arbitrary native Pack execution
remains fail closed.
