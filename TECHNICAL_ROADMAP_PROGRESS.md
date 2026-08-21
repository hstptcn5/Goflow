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
2. Update this ledger whenever checkpoint status changes.
3. Do not mark `DONE` before merge; record the final merged commit afterward.
4. Record concrete verification evidence, not vague completion claims.
5. If implementation deliberately changes the roadmap shape, update `TECHNICAL_ROADMAP.md` so the plan remains truthful.

## Current Summary

Technical roadmap adopted: 2026-08-21.

Current focus: final verification and merge of PR #40, which consolidates the remaining technical-capability checkpoints after `GF-CORE-002`.

Verified code head before final audit adjustments: `c096fcae2e6f9ff4be46212bc373f448323f6cd0`.

Verification evidence on that head:

- CI #456: `success` — formatting, backend unit tests, race tests, vet, vulnerability scan, frontend tests/build, Pack contracts, DailyOps, Playwright/appliance E2E, Linux/macOS/Windows Community builds, and Windows DailyOps pilot.
- Vietnam Morning Brief Windows Pilot #180: `success`.
- Daily Business Report Public Beta #232: `success`.

Final audit adjustments made after that green head:

- Python environment is now a real selector populated from configured runtime profiles.
- File automation is documented and labeled accurately as restart-safe polling (`File Watch (Polling)`) intended to run after Cron, not as an OS event watcher.
- Technical roadmap metadata scope was aligned with the actual rich-parameter implementation instead of claiming an unused dynamic-option protocol.

These final audit commits require their own latest-head CI before PR #40 can be merged.

## Checkpoint Ledger

| Checkpoint | Phase | Outcome | Status | PR / Commit | Verification / Notes |
|---|---:|---|---|---|---|
| `GF-CORE-001` | 0 | Operation-aware retry | `DONE` | PR #37; merge `f3648925a34bbdb1e9ac394b588e0b8087af3c01` | CI #374 green. Safe/read operations retain retry; audited mutating operations default to one implicit attempt. |
| `GF-CORE-002` | 0 | Node error policy/routing | `DONE` | PR #39; merge `90931c4b1b7fea162478dd6edce97ad9522647ad` | CI #385 green. Stop / Continue / explicit error-output routing with backward-compatible Stop default. |
| `GF-DB-001` | 0 | Parameterized PostgreSQL/MySQL queries | `IN_PROGRESS` | PR #40 | Driver-bound parameter arrays; legacy SQL without parameters remains supported. |
| `GF-LOGIC-001` | 1 | Typed IF | `IN_PROGRESS` | PR #40 | Typed general/number/string/regex/boolean comparisons. |
| `GF-LOGIC-002` | 1 | Switch | `IN_PROGRESS` | PR #40 | Ordered cases, dynamic editor handles and default route. |
| `GF-LOGIC-003` | 1 | Subworkflow error collection | `IN_PROGRESS` | PR #40 | Stop all / Continue / Collect errors while preserving existing bounded loop engine. |
| `GF-HTTP-001` | 2 | Structured query + generic auth | `IN_PROGRESS` | PR #40 | Query object plus None/Bearer/API-key/Basic/OAuth2/custom-header modes. |
| `GF-HTTP-002` | 2 | Request/response modes | `IN_PROGRESS` | PR #40 | JSON/raw/urlencoded/multipart fields/FileRef request; Auto/JSON/Text/FileRef response. |
| `GF-HTTP-003` | 2 | Pagination | `IN_PROGRESS` | PR #40 | Bounded cursor and page-number pagination. |
| `GF-HTTP-004` | 2 | Import cURL | `IN_PROGRESS` | PR #40 | Parser + editor import; discovered auth secret is moved into encrypted Credentials rather than workflow params. |
| `GF-STATE-001` | 3 | Persistent workflow/global state | `IN_PROGRESS` | PR #40 | SQLite GET/SET/DELETE/INCREMENT; migration hardened against duplicate versions. |
| `GF-PY-001` | 4 | Python runtime profiles | `IN_PROGRESS` | PR #40 | External CPython profiles; no bundled Python or pip manager. |
| `GF-PY-002` | 4 | Python execution protocol | `IN_PROGRESS` | PR #40 | Bounded child process using JSON stdin/stdout; credentials not auto-exposed. |
| `GF-PY-003` | 4 | Python node UI | `IN_PROGRESS` | PR #40 | Profile selector, code control and existing inspector input/output preview. |
| `GF-PY-004` | 4 | Python runtime controls | `IN_PROGRESS` | PR #40 | Timeout/cancel/process kill/stdout-stderr-output bounds; explicitly trusted code, not a sandbox. |
| `GF-FILE-001` | 5 | FileRef data model | `IN_PROGRESS` | PR #40 | Managed UUID references with MIME/size/SHA metadata; binary bytes remain outside workflow JSON/logs. |
| `GF-FILE-002` | 6 | Local File | `IN_PROGRESS` | PR #40 | Read/Write/List with allowed roots, traversal/symlink protection and size bounds; Windows path portability fixed. |
| `GF-FILE-003` | 6 | File Watch (Polling) | `IN_PROGRESS` | PR #40 | Restart-safe Created/Modified detection via Workflow State; schedule with Cron. Native OS watcher intentionally not required in v1. |
| `GF-TABLE-001` | 7 | CSV read/write | `IN_PROGRESS` | PR #40 | Structured rows via FileRef. |
| `GF-TABLE-002` | 7 | XLSX read/write | `IN_PROGRESS` | PR #40 | First-sheet structured rows using ZIP/XML without a new runtime dependency. |
| `GF-SHEETS-001` | 8 | Google Sheets v2 | `IN_PROGRESS` | PR #40 | READ/APPEND/UPDATE/UPSERT with multi-row support. |
| `GF-DRIVE-001` | 8 | Google Drive + FileRef | `IN_PROGRESS` | PR #40 | LIST/DOWNLOAD/UPLOAD/DELETE; FileRef used for bytes. |
| `GF-MAIL-001` | 8 | File attachments | `IN_PROGRESS` | PR #40 | SMTP MIME attachments backed by FileRef with count/size limits. |
| `GF-NODE-001` | 9 | Rich parameter schema/UI | `IN_PROGRESS` | PR #40 | visibleWhen, Advanced, placeholder metadata, code/key-value/FileRef controls and server-populated select options. |
| `GF-PLUGIN-001` | 10 | First-class custom node manifest | `IN_PROGRESS` | PR #40 | `custom.*` manifests declare identity/version/params/outputs/capabilities while built-in types remain immutable. |
| `GF-CODE-001` | 11 | Promote code to reusable node | `IN_PROGRESS` | PR #40 | Versioned `user.*` JS/Python manifests with declared inputs/outputs. |
| `GF-AI-001` | 12 | AI-assisted JS/Python code | `IN_PROGRESS` | PR #40 | Generate/fix endpoint returns code only and explicitly reports `executed:false`. |
| `GF-MCP-001` | 13 | Workflow-as-tool refinement | `IN_PROGRESS` | PR #40 | MCP tool schema now declares `_goflow.idempotency_key`; existing scope/allowlist execution model is retained. |
| `GF-PACK-001` | 14 | Pack capability tiers | `IN_PROGRESS` | PR #40 | `bounded` vs `trusted_external`; Python/plugin/SSH/Git/custom code require explicit trusted tier + capability. |

## Final Audit Notes for PR #40

Risk-focused audit areas reviewed before merge:

- external Python process boundaries and credential exposure;
- custom/plugin node type replacement boundaries;
- cURL secret handling and authenticated API routing;
- FileRef storage and local path containment;
- SQLite migration ordering/duplicate detection and persistent state;
- SQL driver parameter binding;
- HTTP auth/body/response/pagination/FileRef paths;
- Pack fail-closed trusted-execution classification;
- editor Switch/error handles and rich parameter rendering;
- Windows portability via dedicated pilot workflows.

Issues found and fixed during verification:

1. duplicate database migration version for workflow state;
2. `:memory:` SQLite test mismatch with Goflow's dual DB connections;
3. Windows Local File containment failure caused by path canonicalization differences;
4. Python environment UI was text rather than the roadmap's intended selector;
5. file-watch semantics were named too broadly and are now explicitly documented as polling.

No checkpoint in PR #40 is marked `DONE` until the final audited head passes CI and the PR is merged.

## Progress Log

### 2026-08-21

- Adopted `TECHNICAL_ROADMAP.md` and created this canonical ledger.
- Completed and merged `GF-CORE-001` through PR #37.
- Completed and merged `GF-CORE-002` through PR #39.
- Consolidated the remaining capability roadmap into PR #40, with focused tests for the new primitives and integration paths.
- CI #456 passed on code head `c096fcae2e6f9ff4be46212bc373f448323f6cd0` after migration, SQLite test and Windows path fixes.
- Dedicated Vietnam Morning Brief Windows Pilot #180 and Daily Business Report #232 also passed on that head.
- Final audit tightened Python profile UX and documentation semantics before the merge gate.

After PR #40 merges, create a small ledger-only follow-up that marks the PR #40 checkpoints `DONE` and records the squash merge SHA plus final CI run.
