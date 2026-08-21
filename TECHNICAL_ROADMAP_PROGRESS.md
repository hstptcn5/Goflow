# Goflow Technical Roadmap Progress

This file is the canonical progress ledger for `TECHNICAL_ROADMAP.md`.

Status values:

- `PLANNED`: accepted in the roadmap but implementation has not started.
- `IN_PROGRESS`: branch/PR work exists but the checkpoint is not merged and verified.
- `DONE`: merged to `main`, automated verification passed, and the merged commit is recorded here.
- `BLOCKED`: implementation is intentionally paused with the reason recorded.
- `DEFERRED`: still valid but intentionally moved behind higher-priority work.

Rules:

1. Every implementation PR must name one or more checkpoint IDs.
2. Update this ledger in the same PR whenever a checkpoint status changes.
3. Do not mark `DONE` before merge; record the final merged commit afterward if the merge SHA differs.
4. Add short verification evidence instead of vague completion claims.
5. If sequencing changes, explain why in the Notes column and update `TECHNICAL_ROADMAP.md` if the change is strategic.

## Current Summary

Technical roadmap adopted: 2026-08-21.

Current focus: Phase 0 correctness foundation. Next implementation checkpoint: `GF-CORE-002`.

| Checkpoint | Phase | Outcome | Status | PR / Commit | Verification / Notes |
|---|---|---|---|---|---|
| `GF-CORE-001` | 0 | Operation-aware retry | `DONE` | PR #37; squash merge `f3648925a34bbdb1e9ac394b588e0b8087af3c01` | CI #374 green on tested head `f2c80669249899f180826e949fdae7c4b5870f41`: formatting, `go test ./...`, race tests, vet, vulnerability scan, frontend tests/build, Pack/DailyOps, Playwright/E2E, multi-platform Community builds and Windows DailyOps pilot all passed. |
| `GF-CORE-002` | 0 | Node error policy/routing | `PLANNED` | - | Next checkpoint. Add explicit Stop / Continue / Error output semantics while preserving current stop-on-error behavior by default. |
| `GF-DB-001` | 0 | Parameterized PostgreSQL/MySQL queries | `PLANNED` | - | Preserve expression mapping while binding values separately. |
| `GF-LOGIC-001` | 1 | Typed IF | `PLANNED` | - | Avoid using Python for basic typed comparisons. |
| `GF-LOGIC-002` | 1 | Switch | `PLANNED` | - | Multi-branch routing with default path. |
| `GF-LOGIC-003` | 1 | Subworkflow error collection | `PLANNED` | - | Extend existing bounded sequential/parallel loop. |
| `GF-HTTP-001` | 2 | Structured query + generic auth | `PLANNED` | - | Make HTTP the long-tail integration primitive. |
| `GF-HTTP-002` | 2 | Request/response modes | `PLANNED` | - | File modes depend on FileRef; JSON/raw/form can land earlier. |
| `GF-HTTP-003` | 2 | Pagination | `PLANNED` | - | Reuse lessons from normalized HTTP source cursor pagination. |
| `GF-HTTP-004` | 2 | Import cURL | `PLANNED` | - | Secrets should be migrated to Credentials, not persisted inline. |
| `GF-STATE-001` | 3 | Persistent workflow/global state | `PLANNED` | - | SQLite-backed GET/SET/DELETE/INCREMENT. |
| `GF-PY-001` | 4 | Python runtime profiles | `PLANNED` | - | External CPython; no bundled runtime or pip management in v1. |
| `GF-PY-002` | 4 | Python execution protocol | `PLANNED` | - | JSON stdin/stdout child process; credentials not auto-exposed. |
| `GF-PY-003` | 4 | Python node UI | `PLANNED` | - | Environment selector, code editor, input/output preview. |
| `GF-PY-004` | 4 | Python runtime controls | `PLANNED` | - | Timeout, cancel, stderr/stdout bounds, process kill. Trusted code, not sandbox. |
| `GF-FILE-001` | 5 | FileRef data model | `PLANNED` | - | Prevent binary/base64 expansion in execution context/logs. |
| `GF-FILE-002` | 6 | Local File node | `PLANNED` | - | Initial Read/Write/List with root/path/size bounds. |
| `GF-FILE-003` | 6 | Local File Trigger | `PLANNED` | - | High-value local-first capability. |
| `GF-TABLE-001` | 7 | CSV read/write | `PLANNED` | - | Structured rows; transformations remain JS/Python. |
| `GF-TABLE-002` | 7 | XLSX read/write | `PLANNED` | - | Structured rows; avoid many spreadsheet convenience nodes. |
| `GF-SHEETS-001` | 8 | Google Sheets v2 | `PLANNED` | - | READ/APPEND/UPDATE/UPSERT + multi-row. |
| `GF-DRIVE-001` | 8 | Google Drive + FileRef | `PLANNED` | - | LIST/DOWNLOAD/UPLOAD/DELETE. |
| `GF-MAIL-001` | 8 | File attachments | `PLANNED` | - | Depends on FileRef. |
| `GF-NODE-001` | 9 | Rich parameter schema/UI | `PLANNED` | - | visibleWhen, advanced, key-value, code/file controls. |
| `GF-PLUGIN-001` | 10 | First-class custom node manifest | `PLANNED` | - | Build on existing executable plugin stdin/stdout protocol. |
| `GF-CODE-001` | 11 | Promote code to reusable node | `PLANNED` | - | Versioned declared inputs/outputs. |
| `GF-AI-001` | 12 | AI-assisted JS/Python code | `PLANNED` | - | Generate/fix against sample input; no silent production execution. |
| `GF-MCP-001` | 13 | Workflow-as-tool refinement | `PLANNED` | - | MCP stays scoped interface to approved workflows. |
| `GF-PACK-001` | 14 | Pack capability tiers | `PLANNED` | - | Distinguish bounded/declarative capabilities from trusted external execution. |

## GF-CORE-001 Audit Notes

Audit performed against the pre-implementation `main`:

- Engine previously assigned up to 3 attempts whenever `executor.GetDefinition().Retryable` was true.
- PostgreSQL and MySQL node types combine `SELECT` and `EXECUTE` under one retry flag.
- Google Sheets combines `READ` and `APPEND` under one retry flag.
- Google Drive combines `LIST` and `UPLOAD` under one retry flag.
- MongoDB combines `FIND_ONE` with write operations under one retry flag.
- Redis combines read and mutating commands under one retry flag.
- HTTP Request supports safe and potentially non-idempotent methods under one retry flag.

Implemented behavior:

- `nodes.MaxAttemptsForNode` is the central policy.
- The engine calls the policy only after dynamic parameters are resolved, so an expression-supplied operation such as `{{$trigger.method}}` is classified correctly.
- HTTP GET/HEAD, SQL SELECT, Sheets READ, Drive LIST, Mongo FIND_ONE, and Redis GET/EXISTS/HGET retain automatic retry when the node definition permits it.
- Mutating operations covered by the audit are restricted to a single implicit attempt.
- Uniform node types retain their existing `Retryable` behavior.

Verification completed:

```text
[x] gofmt clean across repository
[x] go test ./...
[x] go test -race ./...
[x] go vet non-release packages
[x] PR CI green (CI #374)
[x] merged commit recorded: f3648925a34bbdb1e9ac394b588e0b8087af3c01
```

## Progress Log

### 2026-08-21

- Adopted the technical capability roadmap.
- Created branch `goflow-core-roadmap-foundation` from `main`.
- Added `TECHNICAL_ROADMAP.md` and this progress ledger.
- Audited mixed read/write retry behavior and implemented central operation-aware retry policy.
- Wired retry classification after expression/parameter resolution.
- Added table-driven coverage for HTTP, PostgreSQL, MySQL, Google Sheets, Google Drive, MongoDB and Redis, including expression-resolved operations.
- CI #374 passed formatting, backend unit/race/vet/vulnerability checks, frontend tests/build, Pack contracts/DailyOps, Playwright/appliance E2E, Community builds on Linux/macOS/Windows, and the Windows DailyOps pilot appliance job.
- Squash-merged PR #37 to `main` as `f3648925a34bbdb1e9ac394b588e0b8087af3c01`.
- Marked `GF-CORE-001` `DONE`. Next checkpoint is `GF-CORE-002`.

Future entries should record checkpoint transitions, PR numbers, merge commits and verification commands/results.
