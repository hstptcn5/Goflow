# Goflow Productization Plan

Status: Phases 1-5 complete; external manual gates remain pending

This plan turns the ecosystem alpha into a local-first Productization Beta
Candidate through five stacked draft pull requests. It does not authorize a
merge, release, production signing, remote telemetry, or vendor integration.

## Branch And Pull Request Stack

| Phase | Branch | Planned base | Checkpoints |
| --- | --- | --- | --- |
| Scheduler | `feature/goflow-productization-scheduler` | `feature/goflow-pilot-ux-hardening` while PR #14 is open | A-C |
| Lifecycle | `feature/goflow-productization-lifecycle` | Scheduler branch while unmerged | D-F |
| Trust | `feature/goflow-productization-trust` | Lifecycle branch while unmerged | G-H |
| Adapters | `feature/goflow-productization-adapters` | Trust branch while unmerged | I-J |
| Beta | `feature/goflow-productization-beta` | Adapters branch while unmerged | K-M |

Every phase remains draft, open, and unmerged. A later phase starts only after
the previous phase's exact pushed head has passed CI.

## Existing Architecture Inventory

- `internal/serverapp` owns SQLite, the engine, `TriggerService`, HTTP serving,
  background cleanup, the existing advanced-workflow cron scanner, and graceful
  shutdown.
- `internal/application.TriggerService` is the single workflow admission path.
  It applies active/exposure/input checks and delegates concurrency and
  idempotency to the engine.
- The execution table has a unique `(workflow_id, idempotency_key)` index. The
  engine resolves concurrent inserts to the existing execution.
- Managed Pack workflows use stable workflow IDs. Setup/config/credential state
  is stored outside the extracted bundle.
- The existing `cronTrigger` node accepts raw cron expressions for advanced
  workflows. It is not the appliance schedule contract and must remain
  backward-compatible.
- Pack Run's data-directory lock and healthy-instance reuse establish one
  primary process for a managed appliance. The scheduler starts only inside
  that primary process.

## Phase 1: Scheduler And Safe Automation

### Checkpoint A: Contract Before Runtime Mutation

- Record baseline repository, PR, ancestry, artifact, backend, and frontend
  evidence.
- Define the appliance schedule schema and deterministic scheduler semantics.
- Document lifecycle ownership, DST and clock behavior, idempotency, migration,
  threats, and the test matrix.
- Make no runtime or persistence behavior change.

### Checkpoint B: Backend MVP

- Add a transactional schedule store and schema migration.
- Add a deterministic schedule calculator with an injected clock.
- Start and stop one managed-appliance scheduler with `serverapp` lifecycle.
- Trigger only through `TriggerService` with stable source, principal, request
  ID, and idempotency metadata.
- Fail closed for corrupt, incomplete, inactive, or revalidation-required state.

### Checkpoint C: Appliance UX And DailyOps

- Add simple daily time/timezone controls, default off.
- Show enabled state, next run, timezone, and last scheduled result.
- Share manual/scheduled concurrency and prove exact-once delivery with loopback
  source and Telegram mocks.
- Prove persistence, restart behavior, and absence of production test seams.

## Later Phases

- Phase 2 adds ordered Pack migration, bounded diagnostics/history, and a safe
  Windows offline upgrade path or an explicitly documented installer blocker.
- Phase 3 adds Pack author compatibility tooling and a signing specification;
  implementation proceeds only if trust semantics are reviewable and complete.
- Phase 4 adds a generic adapter contract and an evidence-based vendor dossier,
  without vendor-specific production code before the decision gate.
- Phase 5 proves the complete beta journey, runs clean-checkout acceptance, and
  supplies the pilot operations package.

## Scheduler Contract Test Matrix

All correctness tests use an injected clock or direct deterministic tick; they
must not depend on long real-time sleeps.

| Area | Positive cases | Negative and race cases | Acceptance evidence |
| --- | --- | --- | --- |
| Validation | disabled daily schedule; valid `HH:MM`; valid IANA zone | raw cron; unknown kind/zone/field; oversized text; invalid schema version | table-driven unit tests |
| Calculation | ordinary day; month/year boundary; DST spring forward and fall back | nonexistent local time; repeated local time; clock rollback/large forward jump | exact UTC instant assertions |
| Persistence | create/update/disable; restart load; forward-compatible nullable metadata | corrupt row; future schema; cross-pack/workflow mismatch; failed transaction | SQLite store tests |
| Triggering | one due tick; next future period; terminal run permits later period | duplicate tick; restart around due time; concurrent manual run; inactive workflow | exact execution and side-effect counts |
| Lifecycle | primary start after storage; context cancellation; graceful shutdown | secondary instance; startup failure; cancellation during tick | deterministic lifecycle tests and race CI |
| Appliance | setup schedule; status/next run; change time/zone; credentials preserved | setup incomplete; revalidation required; stale UI request | API/UI unit tests |
| End to end | scheduled fire then next fire after restart | manual click at due instant; restart does not replay missed periods | real backend/UI loopback E2E |
| Security | redacted diagnostics and logs | canary token, URL query, chat ID, local/temp paths | bundle, UI, log, and export scans |

## Phase Gates

Each checkpoint records its commit and verification in `PROGRESS.md`. Each phase
requires full local and clean-checkout acceptance appropriate to its scope, a
green GitHub Actions run on the exact pushed head, and a draft unmerged PR before
the next phase starts.

## Checkpoint G Author Contract Test Matrix

| Area | Positive cases | Negative cases | Evidence |
| --- | --- | --- | --- |
| Compatibility | omitted declaration for legacy v1; known required capabilities; supported target | unknown/duplicate/oversized capability; unsupported target | Pack validator tables |
| Offline fixture | scaffold fixture; bounded config overrides; fake credential slots; repeated deterministic prepare | unknown/missing key; wrong type; secret-like value; path escape/symlink/oversize | fixture parser and CLI tests |
| Setup contract | config, credentials, bindings, response contract | duplicate target/source errors with exact JSON-style locations | Pack unit tests |
| Schedule/migration | host-managed capabilities declared when required | executable migration or raw appliance cron declaration remains unsupported | contract and documentation tests |
| Build inventory | runtime, manifest, workflow, declared assets/plugins exactly inventoried | missing/extra/duplicate/tampered archive member | bundle tests |
| Clean author flow | init, validate, offline test, build, verify | internet access is unnecessary and connection tests are skipped | clean-checkout CLI acceptance |

## Checkpoint I Adapter Contract Test Matrix

| Area | Positive cases | Negative cases | Evidence |
| --- | --- | --- | --- |
| Validation | single object; cursor pages; none/bearer/API-key auth | non-GET behavior; bad URL/field/auth/bounds/contract | table-driven node validation |
| Pagination | one/multiple pages; terminal null cursor; stable ordering | repeated/oversized cursor; page/item overflow | exact requests and normalized output |
| Rate/retry | bounded 429 then success; cancellation | invalid/large Retry-After; retry exhaustion | injected waiter, no sleeps |
| Errors | normalized contract result | timeout, unreachable, 4xx, 5xx, invalid JSON/contract | closed safe categories |
| Secrets | vault credential applied to request | cross-origin redirect, output/log/error leak | captured requests and canary scans |
| Side effects | normalized output reaches downstream | every adapter failure blocks destination | real engine fixture |
| Compatibility | DailyOps generic REST remains valid | native Pack/plugin execution remains disabled | regression and registry tests |

## Phase 5 Beta Acceptance Matrix

| Journey area | Required evidence | Failure evidence |
| --- | --- | --- |
| Artifact acquisition | exact head, stable unsigned-beta name, SHA-256 and expiry in CI | no Release/latest/signing claim |
| First launch | extracted `goflow.exe` starts without terminal arguments; health/UI/bootstrap pass | tamper and wrong layout reject before runtime state |
| Setup | source contract test; Telegram bot/chat test sends no message; manual-only and daily schedule paths | invalid source/chat/credential remains incomplete with a concrete next action |
| Execution | one manual report and one scheduled report contain loopback source values | simultaneous run is 409 and creates no duplicate execution/message |
| Persistence | stable workflow/config/credential/schedule/history across clean restart | no unsolicited source/Telegram call during restart |
| Upgrade | verified candidate, retained external data and rollback; revalidation behavior visible | tampered/unhealthy/running/reparse candidates reject or roll back before data loss |
| Diagnostics | bounded redacted export and local-only counters | token, chat ID, raw payload, mock URL, username/hostname/path absent |
| Operations | stop/restart/backup/rotation/uninstall/reinstall and feedback instructions | no production-ready, signed, vendor-compatible or customer-validated claim |
| Clean checkout | final exact head in a fresh LF checkout passes critical Go/UI/Pack/E2E gates | no dependency on current worktree output or committed generated artifacts |

Manual pilot consent, usefulness, willingness-to-pay, production signing, vendor
approval/access, and release remain external gates. They must be reported as
pending rather than inferred from automated acceptance.
