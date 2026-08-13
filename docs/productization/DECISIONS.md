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
- Timezone database behavior is supplied by Go's `time.LoadLocation`; tests use
  stable named zones with known transitions.

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

## ADR-002: No Installer Decision In Phase 1

Status: Accepted

Scheduler work does not select or download an installer toolchain. Installer or
portable-upgrade decisions belong to Checkpoint F after a pinned-toolchain and
supply-chain comparison.
