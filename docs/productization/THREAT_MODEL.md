# Goflow Productization Threat Model

Status: Checkpoint A scheduler design baseline

This document extends the ecosystem alpha threat model. Existing Pack,
credential, localhost, path, diagnostics, and artifact controls remain required.

## Scheduler Assets And Trust Boundary

Protected assets:

- schedule configuration and enabled state;
- the stable managed workflow and execution history;
- exactly-once admission for each scheduled instant;
- credentials and source/Telegram data;
- truthful next-run and last-result status.

Untrusted or failure-prone inputs include persisted schedule rows, wall clock and
timezone transitions, Pack upgrades, browser mutations, process crashes, and
concurrent manual/scheduled requests.

## Threats And Required Controls

### Duplicate Delivery

Threat: duplicate ticks, restart races, a second process, or a clock rollback
creates more than one Telegram report for one scheduled instant.

Controls:

- only the primary Pack Run process starts the appliance scheduler;
- deterministic per-instant idempotency key;
- existing database uniqueness for workflow/idempotency;
- shared TriggerService and workflow concurrency `reject`;
- persisted last scheduled instant never moves backward.

Evidence required: concurrent tick, restart-before/after-trigger, clock rollback,
and manual-at-due-time tests with exact execution/source/send counts.

### Catch-Up Storm

Threat: a long outage or large forward clock jump replays many old reports.

Controls: fixed `skip` policy; on startup or forward jump, calculate only the
next future occurrence. No unbounded queue of missed instants is stored.

Evidence required: multi-day downtime and large forward-jump tests produce no
immediate execution and one future next run.

### DST Ambiguity

Threat: nonexistent or repeated local times cause a shifted or duplicate report.

Controls: IANA zones only; nonexistent local time is skipped; repeated local time
admits its first occurrence once; canonical UTC instant is used for idempotency.

Evidence required: deterministic spring-forward and fall-back fixtures.

### Corrupt Or Future Schedule State

Threat: malformed, cross-Pack, or unsupported state silently enables a schedule.

Controls: strict bounded schema; Pack/workflow ownership checks; unknown future
schema fails closed to `NEEDS_ATTENTION`; no silent repair or downgrade.

Evidence required: corrupt JSON/row, invalid enum/timezone, future version, and
cross-Pack tests with zero trigger calls.

### Stale Primary Or Lifecycle Leak

Threat: a secondary instance starts a scheduler, or shutdown leaves a goroutine
that fires after storage/server teardown.

Controls: scheduler is created only after the primary lock and storage are ready;
it is owned by serverapp context and WaitGroup; cancellation precedes database
close; secondary reuse does not start serverapp.

Evidence required: secondary-launch, context-cancel, startup-failure, shutdown,
and race tests.

### Incomplete Or Unsafe Workflow State

Threat: a schedule triggers while setup is incomplete, migration/revalidation is
pending, workflow is inactive, or credentials are invalid.

Controls: fail-closed readiness check at each admission; TriggerService active
check; no automatic credential test or decryption by scheduler; concurrency
errors do not crash the appliance.

Evidence required: each blocked state produces zero side effects and a bounded
public category.

### Schedule Mutation Race

Threat: a stale browser request re-enables or overwrites a newer schedule.

Controls: bounded authenticated appliance mutation route, transactional store,
strict current Pack/workflow identity, and update timestamps/version semantics.
UI state is descriptive; backend state is authoritative.

Evidence required: concurrent update/disable and stale request tests.

### Information Leakage

Threat: scheduler logs, status, diagnostics, request IDs, or persisted metadata
expose token, chat ID, source URL/query, payload, response, or local path.

Controls: allowlisted safe fields and error categories; deterministic identifiers
contain only Pack/workflow/instant or their digest; no credentials in schedule
state; existing redaction and artifact scans remain mandatory.

Evidence required: seeded canary scans across database-safe exports, logs, UI,
Playwright output, bundles, and extracted files.

Diagnostics are local-only and created on request. Their versioned DTO omits
workflow/execution/credential identifiers, timestamps, input/output/log data,
URLs, chat IDs, host identity, and paths. Recent history is capped at ten
status/duration/public-category rows. Unknown legacy errors become fixed
`internal_error` text. Copy/download requires an explicit user action; no
analytics, remote collection, fingerprinting, or background telemetry exists.

Execution retention uses validated finite bounds and one transaction for age
and per-workflow count pruning. Both deletion paths exclude `RUNNING` rows;
engine concurrency separately bounds the number of active rows.

### Version Upgrade

Threat: Pack or schema upgrade loses a schedule, silently runs an incompatible
workflow, or duplicates the managed workflow/schedule.

Controls: ordered compensating migrations; stable workflow ID; schedule remains
persisted and retains the enabled preference but is operationally suspended
while migration/revalidation is required; unknown future schema fails closed;
no automatic downgrade.

Additional controls: migration code is compiled into a closed host registry,
never loaded from a Pack; it sees non-secret config plus credential ID/type
references only. A pre-mutation snapshot includes a sorted hash inventory.
Multi-file persistence compensates to original bytes on failure. Unknown chains
require review; corrupt/future migration metadata and downgrade attempts fail
closed before the appliance serves.

Evidence required: DailyOps 0.1.0 and 0.2.0 fixture upgrades, intentional
retain/remove/rename transforms, rollback injection, repeated restart, future
schema/downgrade rejection, and workflow/credential/schedule/history
cardinality assertions.

### Offline Windows Update

Threat: an untrusted candidate, active process, cross-Pack archive, reparse
point, partial rename, failed migration/startup, or cleanup race mutates the
application or external state without a recoverable prior version.

Controls: update is explicit and offline; no network/download path exists. The
current runtime verifies the archive and extracted inventory, the candidate
runtime verifies itself, and Pack ID plus Windows AMD64 target must match before
mutation. Any active instance or reparse point in the app/data tree fails
closed. External data is snapshotted and the entire prior app directory is
retained before two-step activation. State tracks the window between old-app
rename and candidate activation; every later error compensates app and data.
Local health must succeed before update acceptance. Tampered and unhealthy
candidates are deterministic Windows CI fixtures.

Residual risk: the candidate is unsigned, so hashes/inventory prove integrity
but not publisher authenticity. The portable helper is not an installer and
does not provide automatic distribution, Windows registration, or signed
anti-rollback. These remain blocked on trust/signing review and an installer
toolchain decision.

Evidence required: native valid update, active-instance/reparse rejection,
tamper rejection before mutation, health-failure compensation, preserved
external sentinel/state, retained rollback directories, PE/inventory checks,
restart/migration/revalidation, and artifact-wide leakage scans.

### Pack Author Compatibility Inputs

Threat: a Pack claims unsupported runtime behavior, hides executable migration
or schedule hooks in author metadata, uses an offline fixture to smuggle secret
material or a path escape, or ships files that are not covered by inventory.

Controls: closed and bounded capability declarations; strict known-field
validation with legacy omission compatibility; host-managed schedule/migration
only; bounded regular-file fixture contained under the Pack root; non-secret
fixture values and fake logical credential slots; no network execution during
`pack test`; and exact archive inventory over runtime plus every shipped Pack
file. Unknown capability and malformed future metadata fail closed.

Evidence required: table-driven capability/fixture failures, symlink and path
containment tests, secret canaries, deterministic repeated offline preparation,
and missing/extra/duplicate/tampered bundle tests.

### Pack Publisher Authenticity

Threats: modified archives; a malicious publisher; stolen private key; stale
or revoked key; key-ID collision; replayed metadata; downgrade to an older
signed Pack; an unsigned legacy Pack treated as trusted; trust-on-first-use;
signature metadata bombs, duplicates, symlinks, target swaps, or circular
inventory; and private/public key material leaking into logs or artifacts.

Controls: the v1 spec in `PACK_SIGNING.md`; standard-library Ed25519; one strict
bounded root signature member outside self-inventory; deterministic binary
canonical payload binding exact `PACK_INFO` bytes, Pack/version/target, and
capabilities; inventory verification before authenticity; explicit trusted
public key plus matching key ID; no embedded/downloaded/TOFU key; no automatic
downgrade; private key accepted only from an explicit local file or stdin and
never printed or copied; unsigned execution remains visibly development-only.

Residual risk: a valid signature cannot prove a publisher is benign, a key has
not been stolen, or an older signed version is acceptable. Rotation, revocation,
certificate identity, release transparency, and minimum-version policy require
production trust governance and remain out of scope.

Evidence required: valid signature; changed manifest/workflow/asset/runtime/
inventory; wrong key/ID; unknown schema/algorithm; duplicate/oversized metadata;
path/symlink tricks; target/version/capability mismatch; unsigned policy; key
canary scans; and equal canonical payload for equal metadata.

### Declarative Source Adapters

Threats: SSRF/unsafe redirects; credentials in Pack fields, URLs, output, or
logs; cross-origin authorization forwarding; pagination loops or unbounded item
growth; retry storms; hostile `Retry-After`; duplicate reads; malformed vendor
errors/payloads; vendor-specific fields reaching a destination; and arbitrary
native adapter execution.

Controls: one reviewed built-in GET-only adapter; absolute HTTP(S) and existing
safe redirect policy; credentials resolved only from vault IDs and attached
after URL validation; bounded headers/body/pages/items/cursor lengths; visited-
cursor loop detection; fixed retry count and bounded delta wait through an
injectable waiter; normalized response contract before downstream execution;
closed safe error categories; no URL/header/raw body in output; no Pack plugin
or process execution.

Evidence required: single-object and cursor pagination fixtures; cursor loop,
page/item/body bounds; 429 retry and invalid `Retry-After`; 4xx/5xx/error mapping;
wrong JSON/contract; redirect credential stripping; cancellation; exact request
counts; destination not called after adapter error; and token/URL/payload scans.

## Residual Risk

The scheduler can prevent duplicate admission inside the local appliance but
cannot prove a remote provider did not process a request after a network timeout.
Destination-specific idempotency is outside the current Telegram API contract.
The beta therefore uses conservative retry behavior and reports uncertainty
without automatically resending a potentially delivered message.
