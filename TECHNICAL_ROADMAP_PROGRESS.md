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

Technical capability roadmap adopted: 2026-08-21.

All implementation checkpoints from `GF-CORE-001` through `GF-PACK-001` are now merged to `main` and verified.

Final roadmap consolidation:

- PR #40 squash merge: `efe6d4442cd9286c9b31e2c2af54428295a5db4b`
- Audited PR #40 head: `44554acc504544dab311a8620ed457493b217364`
- CI #461: `success`
- Vietnam Morning Brief Windows Pilot #185: `success`
- Daily Business Report Public Beta #237: `success`

The technical capability roadmap is complete. Phase 15 productization/commercial work in `TECHNICAL_ROADMAP.md` remains a separate continuation of the existing product roadmap rather than part of these implementation checkpoint IDs.

## Checkpoint Ledger

| Checkpoint | Phase | Outcome | Status | PR / Commit | Verification / Notes |
|---|---:|---|---|---|---|
| `GF-CORE-001` | 0 | Operation-aware retry | `DONE` | PR #37; merge `f3648925a34bbdb1e9ac394b588e0b8087af3c01` | CI #374 green. Safe/read operations retain retry; audited mutating operations default to one implicit attempt. |
| `GF-CORE-002` | 0 | Node error policy/routing | `DONE` | PR #39; merge `90931c4b1b7fea162478dd6edce97ad9522647ad` | CI #385 green. Stop / Continue / explicit error-output routing with backward-compatible Stop default. |
| `GF-DB-001` | 0 | Parameterized PostgreSQL/MySQL queries | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Driver-bound parameter arrays; legacy SQL without parameters remains supported. CI #461 green. |
| `GF-LOGIC-001` | 1 | Typed IF | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Typed general/number/string/regex/boolean comparisons. CI #461 green. |
| `GF-LOGIC-002` | 1 | Switch | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Ordered cases, dynamic editor handles and default route. CI #461 green. |
| `GF-LOGIC-003` | 1 | Subworkflow error collection | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Stop all / Continue / Collect errors while preserving the bounded loop engine. CI #461 green. |
| `GF-HTTP-001` | 2 | Structured query + generic auth | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Query object plus None/Bearer/API-key/Basic/OAuth2/custom-header modes. CI #461 green. |
| `GF-HTTP-002` | 2 | Request/response modes | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | JSON/raw/urlencoded/multipart fields/FileRef request; Auto/JSON/Text/FileRef response. CI #461 green. |
| `GF-HTTP-003` | 2 | Pagination | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Bounded cursor and page-number pagination. CI #461 green. |
| `GF-HTTP-004` | 2 | Import cURL | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Parser + editor import; discovered auth secret is moved into encrypted Credentials rather than workflow params. CI #461 green. |
| `GF-STATE-001` | 3 | Persistent workflow/global state | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | SQLite GET/SET/DELETE/INCREMENT; migration registry hardened against duplicate versions. CI #461 green. |
| `GF-PY-001` | 4 | Python runtime profiles | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | External CPython profiles; no bundled Python or pip manager. CI #461 green. |
| `GF-PY-002` | 4 | Python execution protocol | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Bounded child process using JSON stdin/stdout; credentials not auto-exposed. CI #461 green. |
| `GF-PY-003` | 4 | Python node UI | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Configured profile selector, code control and existing inspector input/output preview. CI #461 green. |
| `GF-PY-004` | 4 | Python runtime controls | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Timeout/cancel/process kill/stdout-stderr-output bounds; explicitly trusted code, not a sandbox. CI #461 green. |
| `GF-FILE-001` | 5 | FileRef data model | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Managed UUID references with MIME/size/SHA metadata; binary bytes remain outside workflow JSON/logs. CI #461 green. |
| `GF-FILE-002` | 6 | Local File | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Read/Write/List with allowed roots, traversal/symlink protection and size bounds; Windows path portability verified. CI #461 + Windows pilot #185 green. |
| `GF-FILE-003` | 6 | File Watch (Polling) | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Restart-safe Created/Modified detection via Workflow State, scheduled with Cron. Native OS watcher intentionally not required in v1. CI #461 green. |
| `GF-TABLE-001` | 7 | CSV read/write | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Structured rows via FileRef. CI #461 green. |
| `GF-TABLE-002` | 7 | XLSX read/write | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | First-sheet structured rows using ZIP/XML without a new runtime dependency. CI #461 green. |
| `GF-SHEETS-001` | 8 | Google Sheets v2 | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | READ/APPEND/UPDATE/UPSERT with multi-row support. CI #461 green. |
| `GF-DRIVE-001` | 8 | Google Drive + FileRef | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | LIST/DOWNLOAD/UPLOAD/DELETE; FileRef used for bytes. CI #461 green. |
| `GF-MAIL-001` | 8 | File attachments | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | SMTP MIME attachments backed by FileRef with count/size limits. CI #461 green. |
| `GF-NODE-001` | 9 | Rich parameter schema/UI | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | visibleWhen, Advanced, placeholder metadata, code/key-value/FileRef controls and server-populated select options. CI #461 green. |
| `GF-PLUGIN-001` | 10 | First-class custom node manifest | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | `custom.*` manifests declare identity/version/params/outputs/capabilities while built-in types remain immutable. CI #461 green. |
| `GF-CODE-001` | 11 | Promote code to reusable node | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Versioned `user.*` JS/Python manifests with declared inputs/outputs. CI #461 green. |
| `GF-AI-001` | 12 | AI-assisted JS/Python code | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | Generate/fix endpoint returns code only and explicitly reports `executed:false`. CI #461 green. |
| `GF-MCP-001` | 13 | Workflow-as-tool refinement | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | MCP tool schema declares `_goflow.idempotency_key`; existing scope/allowlist execution model retained. CI #461 green. |
| `GF-PACK-001` | 14 | Pack capability tiers | `DONE` | PR #40; merge `efe6d4442cd9286c9b31e2c2af54428295a5db4b` | `bounded` vs `trusted_external`; Python/plugin/SSH/Git/custom code require explicit trusted tier + capability. CI #461 green. |

## Final Audit Notes for PR #40

Risk-focused audit covered:

- external Python process boundaries and credential exposure;
- custom/plugin node type replacement boundaries;
- cURL secret handling and authenticated API routing;
- FileRef storage and local path containment;
- SQLite migration ordering/duplicate detection and persistent state;
- SQL driver parameter binding;
- HTTP auth/body/response/pagination/FileRef paths;
- Pack fail-closed trusted-execution classification;
- editor Switch/error handles and rich parameter rendering;
- Windows portability through dedicated pilot workflows.

Issues found and fixed before merge:

1. duplicate database migration version for workflow state;
2. `:memory:` SQLite test mismatch with Goflow's dual DB connections;
3. Windows Local File containment failure caused by path canonicalization differences;
4. Python environment UI was text rather than the roadmap's intended selector;
5. file-watch semantics were named too broadly and are explicitly documented as polling.

## Progress Log

### 2026-08-21

- Adopted `TECHNICAL_ROADMAP.md` and created this canonical ledger.
- Completed and merged `GF-CORE-001` through PR #37.
- Completed and merged `GF-CORE-002` through PR #39.
- Implemented the remaining checkpoint set in PR #40.
- Verified audited head `44554acc504544dab311a8620ed457493b217364` with CI #461, Vietnam Morning Brief Windows Pilot #185, and Daily Business Report Public Beta #237 — all green.
- Squash-merged PR #40 to `main` as `efe6d4442cd9286c9b31e2c2af54428295a5db4b`.
- Marked every technical capability checkpoint through `GF-PACK-001` `DONE`.

Next step: preview and audit the merged product behavior on `main`; any follow-up issues discovered there should become new checkpoints rather than rewriting the completed ledger.
