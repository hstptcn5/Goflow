# Goflow Technical Capability Roadmap

This roadmap is the canonical implementation plan for strengthening Goflow as a lightweight, local-first workflow engine.

It complements, rather than replaces, `ROADMAP.md`. The existing product/commercial roadmap remains authoritative for Community/Pro/Pack/Teams/OEM direction. This document defines the technical capability sequence that should be proven before productization work expands.

## Product Principle

> Few powerful primitives. Wide coverage. Small core.

Goflow should not compete by maximizing node count. It should cover common capabilities with first-class nodes and provide strong escape hatches for long-tail work.

Decision rule:

```text
COMMON CAPABILITY      -> Built-in node
LONG-TAIL API          -> HTTP Request
CUSTOM COMPUTATION     -> JavaScript / Python
SPECIAL TOOL / LIBRARY -> Plugin executable
REUSED CUSTOM LOGIC    -> Custom node
```

Python is intentionally an escape hatch, not the center of the product. Built-in nodes own orchestration, credentials, retries, integrations, state and file lifecycle. Python handles custom computation and specialized libraries.

## Delivery Rules

- Implement one independently testable checkpoint at a time.
- Prefer strengthening existing primitives over adding shallow connector nodes.
- Do not add a new built-in node when HTTP, Python, JavaScript or Plugin already covers the use case adequately.
- Preserve existing workflow compatibility unless a migration is explicitly documented.
- Record every checkpoint in `TECHNICAL_ROADMAP_PROGRESS.md` with status, PR and commit evidence.
- Do not mark a checkpoint `DONE` until automated verification has passed and the merged commit is recorded.

---

# Phase 0 - Correctness Foundation

Goal: make current workflow execution semantics safe before broadening capability.

## GF-CORE-001 - Operation-aware retry

Replace node-type-only retry decisions with operation-aware retry behavior.

Required behavior:

- Safe/read-only operations may use automatic retry.
- Non-idempotent write operations must not be automatically retried by default.
- Initial policy must cover at least:
  - HTTP: GET/HEAD safe; POST/PATCH/DELETE not automatically retried; PUT conservative by default unless explicitly proven idempotent.
  - PostgreSQL/MySQL: SELECT safe; EXECUTE unsafe by default.
  - Google Sheets: READ safe; APPEND unsafe.
  - Google Drive: LIST safe; UPLOAD unsafe.
  - MongoDB: FIND_ONE safe; INSERT_ONE/UPDATE_ONE/DELETE_ONE unsafe.
  - Redis: GET/EXISTS/HGET safe; SET/DEL/LPUSH/RPUSH/HSET unsafe by default.
- Preserve current retry count/backoff behavior for operations that remain retryable.
- Add tests proving write operations do not receive implicit multiple attempts.

## GF-CORE-002 - Node error policy and routing

Add explicit per-node behavior:

```text
Stop workflow
Continue
Continue via error output
```

Later extensions may add configurable retry attempts and backoff strategies, but correctness comes before UI richness.

## GF-DB-001 - Parameterized SQL

Add parameter arrays to PostgreSQL/MySQL nodes so user-controlled data does not need to be interpolated into SQL strings.

Target:

```sql
SELECT * FROM customers WHERE email = $1
```

with parameters resolved separately from expressions.

---

# Phase 1 - Core Workflow Language

Goal: keep routine workflow logic out of Python.

## GF-LOGIC-001 - Typed IF

Expand IF beyond string comparison.

Minimum operators:

- General: equals, not equals, exists, not exists, empty, not empty.
- Number: greater than, greater/equal, less than, less/equal.
- String: contains, not contains, starts with, ends with, regex.
- Boolean: true, false.

Preserve value types during comparison.

## GF-LOGIC-002 - Switch

Add multi-branch routing with a default output.

## GF-LOGIC-003 - Subworkflow loop error collection

Keep the existing bounded sequential/parallel subworkflow loop and add:

```text
Stop all
Continue
Collect errors
```

Return structured successes/errors rather than requiring an entirely separate loop engine.

---

# Phase 2 - Universal HTTP

Goal: make HTTP Request the long-tail API integration primitive.

## GF-HTTP-001 - Query and authentication

Add structured query parameters and generic authentication modes:

```text
None
Bearer
API Key
Basic Auth
OAuth2
Custom Header
```

## GF-HTTP-002 - Request and response modes

Request body modes:

```text
JSON
Raw
x-www-form-urlencoded
multipart/form-data
File (after FileRef)
```

Response modes:

```text
Auto
JSON
Text
File (after FileRef)
```

## GF-HTTP-003 - Pagination

Unify useful generic capabilities already present in normalized HTTP source handling.

Initial target:

```text
Cursor
Page number
```

Later: offset/link pagination if demand warrants it.

## GF-HTTP-004 - Import cURL

Parse common cURL requests into HTTP node parameters while keeping secrets in Credentials rather than workflow source.

---

# Phase 3 - Workflow State

## GF-STATE-001 - Persistent workflow state

Add a small SQLite-backed state primitive.

Operations:

```text
GET
SET
DELETE
INCREMENT
```

Scopes:

```text
Workflow
Global
```

Primary use cases: polling cursors, deduplication, checkpoints, last-sync values, counters and lightweight cache.

---

# Phase 4 - Python Escape Hatch

Goal: cover long-tail computation without turning Goflow into a Python platform.

## GF-PY-001 - Python runtime profiles

Add configured external CPython interpreters/environments, e.g.:

```text
default -> C:\Python313\python.exe
data    -> D:\automation\.venv\Scripts\python.exe
```

Goflow does not bundle Python or manage pip in v1.

## GF-PY-002 - Python execution protocol

Execute Python in a child process with a bounded JSON stdin/stdout contract.

Expose:

```text
input
outputs
trigger
```

Credentials are not automatically exposed.

## GF-PY-003 - Python Code node UI

Add environment selector, code editor, input/output preview and actionable errors.

## GF-PY-004 - Python runtime controls

Add timeout, cancellation, stdout/stderr capture, output-size limits, working-directory handling and process termination.

Security statement: Python v1 is trusted local code, not a security sandbox.

---

# Phase 5 - File Data Model

## GF-FILE-001 - FileRef

Introduce a managed file reference rather than pushing binary/base64 through JSON outputs and execution logs.

Example logical representation:

```json
{
  "$type": "file",
  "id": "file_123",
  "name": "orders.xlsx",
  "mime": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "size": 48329
}
```

Execution logs store metadata/reference only.

---

# Phase 6 - Local File Automation

## GF-FILE-002 - Local File

Operations, initially:

```text
Read
Write
List
```

Later if justified:

```text
Move
Copy
Delete
```

Must enforce allowed roots, traversal protection and bounded sizes.

## GF-FILE-003 - File Trigger

Watch a local path/pattern for bounded events such as Created/Modified.

This is a high-priority local-first capability.

---

# Phase 7 - Table Data

## GF-TABLE-001 - CSV

Read/write CSV through structured rows.

## GF-TABLE-002 - XLSX

Read/write XLSX through structured rows.

Do not add separate filter/group/sort/pivot nodes initially; custom transformations belong in JS/Python until repeated demand proves otherwise.

---

# Phase 8 - Deepen Existing Integrations

## GF-SHEETS-001 - Google Sheets v2

Target common operations only:

```text
READ
APPEND
UPDATE
UPSERT
```

Support multi-row input.

Long-tail Sheets API behavior should use HTTP Request.

## GF-DRIVE-001 - Google Drive + FileRef

Target:

```text
LIST
DOWNLOAD -> FileRef
UPLOAD <- FileRef
DELETE
```

Long-tail Drive operations should use HTTP Request.

## GF-MAIL-001 - Attachments

Add FileRef-backed attachments to supported email paths.

---

# Phase 9 - Node Schema and UI v2

## GF-NODE-001 - Rich parameter schema

Add metadata needed for powerful nodes without giant forms:

```text
advanced
visibleWhen
group
placeholder
codeEditor
keyValueList
filePicker
dynamicOptions
```

Keep runtime node definitions as the source of truth.

---

# Phase 10 - First-class Custom Nodes

## GF-PLUGIN-001 - Custom node manifest

Build on the existing executable plugin JSON stdin/stdout protocol.

Allow a plugin directory such as:

```text
plugins/
  normalize-invoice/
    node.json
    normalize.exe
```

The manifest supplies node name, version, category, parameter schema and executable metadata so the plugin appears as a first-class node rather than a generic filename field.

---

# Phase 11 - Reusable Code

## GF-CODE-001 - Promote code to custom node

Allow a stable JS/Python snippet to be saved as a reusable, versioned custom node with declared inputs/outputs.

---

# Phase 12 - AI-assisted Code

## GF-AI-001 - Generate/fix code in Code nodes

Use the existing AI-assisted authoring foundation to generate or repair JS/Python logic against sample input/output.

AI must not silently execute generated code in production.

---

# Phase 13 - MCP/Agent Refinement

## GF-MCP-001 - Workflow-as-tool refinement

Keep MCP as an interface to approved workflows rather than turning Goflow into a generic agent platform.

Maintain scoped identity, workflow allowlists, auditability and bounded inputs.

---

# Phase 14 - Pack Capability Model

## GF-PACK-001 - Capability tiers

Classify Pack capabilities so trusted code is explicit.

Example direction:

```text
Bounded/declarative:
HTTP, DB, notification, state, bounded JS, FileRef

Trusted external execution:
Python, Plugin, SSH, Git
```

Python/Plugin execution must never be represented as sandboxed unless an actual sandbox is implemented and reviewed.

---

# Phase 15 - Productization

After core capability evidence is strong, continue the existing product roadmap:

```text
Community stability
-> Creator experiments
-> trusted Windows distribution
-> controlled Pack distribution
-> commercial validation
-> Teams/OEM only if demand exists
```

Do not build a connector marketplace, cloud orchestration platform, browser RPA suite, generic agent builder or Python package manager merely to match larger competitors.

---

# Explicit Non-goals for This Roadmap

- Hundreds of shallow SaaS nodes.
- Browser/desktop RPA.
- Full Python IDE.
- Bundled Python dependency ecosystem.
- Automatic pip/venv management in Python v1.
- Fake Python sandboxing.
- Docker as a requirement for the default runtime.
- Public Pack marketplace before trust/distribution evidence.
- Early Teams/RBAC work without user demand.
- Full Google API mirrors.
- Large families of CSV/Excel convenience nodes.

# Checkpoint Order

Canonical initial sequence:

```text
GF-CORE-001
GF-CORE-002
GF-DB-001
GF-LOGIC-001
GF-LOGIC-002
GF-LOGIC-003
GF-HTTP-001
GF-HTTP-002
GF-HTTP-003
GF-HTTP-004
GF-STATE-001
GF-PY-001
GF-PY-002
GF-PY-003
GF-PY-004
GF-FILE-001
GF-FILE-002
GF-FILE-003
GF-TABLE-001
GF-TABLE-002
GF-SHEETS-001
GF-DRIVE-001
GF-MAIL-001
GF-NODE-001
GF-PLUGIN-001
GF-CODE-001
GF-AI-001
GF-MCP-001
GF-PACK-001
```

This order may change only when a documented dependency, correctness issue or validated user need justifies reordering. Update `TECHNICAL_ROADMAP_PROGRESS.md` when that happens.
