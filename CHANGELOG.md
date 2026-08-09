# Changelog

All notable changes to Goflow are tracked here.

## Unreleased

### Added

- Vue Router app shell with durable pages for Workflows, Workflow Editor, Executions, Credentials, Templates, Nodes, Settings, and Help.
- `UX_GOAL_PROGRESS.md`, `docs/UX_AUDIT.md`, and `docs/UX_MILESTONE_1_TEST_PLAN.md` for UX Milestone 1 tracking and verification.
- Frontend Vitest and Playwright test foundations covering navigation, workflow save state, API auth prompt behavior, empty/error states, and a browser smoke path.
- UX Milestone 2 editor usability foundation with searchable node picker, quick-add, validation summary, node config badges, undo/redo, duplicate/copy/paste, keyboard shortcuts, auto-layout, visual smoke, and performance smoke docs.
- UX Milestone 2 closure coverage for save-before-run behavior, incomplete draft saves, activation blocking, node picker focus trap, stable graph fingerprints, robust graph ID generation, visual regression baselines, and separated editor performance measurements.
- UX Milestone 3 inspector with Parameters/Input/Output/Logs tabs, inline field validation, data picker, runtime placeholder expression preview, JSON tree/table/raw views, output/log inspection, visual baselines, and inspector performance smoke.
- UX Milestone 4 execution debugger controls in the editor: execution selector, canvas execution overlay, failed-path edge highlight, retry workflow, replay execution, cancel execution, contextual node error actions, and a redacted debug bundle preview/copy action.
- Windows-safe Playwright E2E runner (`ui/tests/run-e2e.mjs`) that starts a temporary local Goflow server and cleans up the Go process tree after tests.
- `docs/ADR_EXPRESSION_AND_MAPPING_MODEL.md` documenting the existing `{{node.path}}` / `{{$trigger.path}}` expression contract, Fixed/Expression mode storage, preview behavior, compatibility, and security model.
- Server-redacted execution inspector DTOs for UI/debug surfaces, including redacted trigger input, node logs, node output, errors, and attempts.
- `scripts/goal-smoke-test.ps1` to run a reusable Windows smoke test for CLI, MCP stdio, MCP HTTP, scoped tokens, cancellation, audit, and concurrent idempotency against a temporary local instance.
- `GOFLOW_MCP_RATE_LIMIT_PER_MINUTE` for HTTP MCP request rate limiting per token/principal.
- `goflow_reload_tools` MCP tool to tell clients when dynamic workflow tools require reconnect/reload.
- CLI workflow validation for duplicate node IDs, bad edge references, DAG cycles, unknown node types, missing required node parameters, and unsupported schema keywords.
- Goflow Pack validation foundation and `goflow pack build` for same-platform portable pack bundle ZIPs with deterministic metadata.

### Changed

- The primary Web UI navigation now uses a sidebar app shell; workflow run/save/export actions moved into the workflow editor topbar.
- Workflow save UX now exposes `Saved`, `Unsaved changes`, `Saving`, and `Save failed` states.
- Embedded frontend serving now falls back to the Vue app for direct frontend route refresh while still serving real embedded assets such as `NODES.md`.
- The node palette is now optional; Add step and node quick-add are the primary ways to add workflow nodes.
- `GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION` now defaults to `4` instead of unlimited.
- Server-side workflow execution now rejects inactive workflows and enforces CLI/MCP exposure flags for CLI and MCP trigger sources.
- MCP dynamic workflow tools now use `_goflow.idempotency_key` as control metadata instead of consuming a business input field named `idempotency_key`.
- MCP execution tool outputs now use a safe DTO and omit raw `input_json` by default.
- MCP dynamic workflow tools now fetch interface metadata through the safe workflow interface endpoint, so scoped tokens can still see tool name, description, and schemas without receiving graph JSON.
- Scoped `workflow:list` responses now return allowlisted workflow summaries only, without graph JSON or node configuration.
- HTTP MCP per-client inflight limiting now persists across requests for the same token/principal.
- HTTP MCP custom origins are included in global CORS handling before the MCP origin middleware validates them.
- Workflow input schema validation now supports the documented subset: `type`, `properties`, `required`, `additionalProperties`, `items`, `enum`, `const`, `minimum`, `maximum`, `minLength`, `maxLength`, `pattern`, `oneOf`, and `anyOf`; unsupported keywords fail clearly.
- CLI workflow import now preserves full interface metadata including approval, max concurrent runs, and concurrency policy.
- Pack path validation now rejects SemVer parsing regressions, empty ID delimiter segments, unsafe Windows archive names, and unlisted pack files during portable bundle builds.

### Fixed

- Fixed the node properties panel overlapping workflow topbar actions after selecting a node.
- Fixed node configuration form controls so Playwright and assistive technology can address fields by accessible labels.
- Fixed validation error banners clearing after graph edits so they do not block editor toolbar actions.
- Fixed unknown node types causing async executions to hang instead of reaching `FAILED`.
- Fixed Web UI execution requests so they persist trigger source as `ui` instead of `api`.
- Fixed principal spoofing by ignoring caller-provided `X-Goflow-Principal` and deriving the principal from authentication context.
- Fixed concurrent idempotency races so duplicate inserts return the existing execution instead of surfacing a unique constraint or HTTP 500.
- Fixed idempotent duplicate requests so they return the existing execution before workflow/global concurrency limits reject them.
- Fixed webhook execution input persistence so sensitive headers such as `Authorization`, `Cookie`, `X-Goflow-Webhook-Secret`, and API key headers are omitted.
- Fixed sub-workflow recursion safety with cycle detection and `GOFLOW_MAX_SUBWORKFLOW_DEPTH`.
- Fixed an engine status race where a single failing node could be marked `SUCCESS` if the scheduler observed completion before `hasFailed` was set.
- Fixed `Test Workflow` so it persists the current dirty graph before triggering, aborts if save fails, and never runs a stale saved graph.
- Fixed workflow draft saving so incomplete node parameters or missing credentials can be saved as drafts while Test and Activate still block invalid runnable workflows.
- Fixed node picker semantics by removing nested interactive controls and making favorite toggles independent from node selection.
- Fixed node picker dialog focus handling so open, Tab/Shift+Tab trap, Escape close, backdrop close, and focus restore are covered by tests.
- Fixed saved/unsaved state after undo and redo by comparing the current graph to a saved graph fingerprint.
- Fixed Activate behavior for dirty valid workflows so the current graph is saved before activation.
- Fixed inspector rendering to redact common secret keys and bearer-like values from visible input/output/log/preview data.
- Fixed Fixed/Expression mode switching so converting back to Fixed uses the resolved primitive preview instead of showing contradictory mode state.
- Fixed number/integer expression fields so Expression mode renders an editable text/code input while Fixed mode keeps numeric input validation.
- Fixed object/array Expression-to-Fixed switching so JSON/object/array previews stay in Expression mode unless the user explicitly converts supported JSON fields to a JSON literal.
- Fixed redacted debug bundle generation so secrets embedded inside JSON string parameters are parsed and redacted before display/copy.
- Fixed pre-run validation so JSON, URL, number, integer, select, and expression field errors block Test/Activate while Save draft remains allowed.
- Fixed data picker source discovery to include transitive upstream nodes and exclude downstream or unrelated nodes.
- Fixed runtime expression parity for `$trigger` references by exposing trigger input through the shared execution context.
- Fixed runtime expression parity for complete placeholders so backend execution preserves string, number, boolean, object, array, and `$trigger` types while mixed interpolation stringifies values.
- Fixed branch debugging by recording skipped branch nodes as `SKIPPED` in execution logs.
- Fixed `delaySleep.seconds` runtime parsing so numeric expressions and integer literals are accepted as typed values instead of falling back to one second; invalid, fractional, zero, negative, NaN/Infinity, and oversized values are rejected.
- Fixed editor execution context isolation so selected execution, latest execution, and live execution views do not overwrite each other's canvas status, inspector output/logs, or debug bundle.
- Fixed failed-path highlighting to traverse the actual ancestry path to a failed node while keeping successful sibling branches, skipped branches, and not-run edges distinct.
- Fixed replay semantics so replay uses stored raw input through `TriggerService`, records authenticated principal/request ID, creates a new execution without old idempotency, enforces scoped workflow allowlists, and returns trigger status codes.
- Fixed Retry behavior so `Retry selected execution` uses stored input via replay instead of triggering the workflow with `{}`.
- Fixed debug bundle copy/export so copied content is valid JSON while large previews remain valid truncated JSON envelopes; redaction now covers URL userinfo, webhook URLs, token query parameters, generic secret keys, nested JSON strings, arrays, bearer/GitHub/OpenAI/Slack token shapes, cookies, authorization values, and private keys.

## 0.5.0-http-mcp-beta - 2026-07-26

### Added

- Dynamic MCP workflow tools for active workflows explicitly exposed through the Interface settings.
- MCP smoke test options for asserting and calling dynamic workflow tools.
- Server-side workflow input schema validation for all trigger paths.
- Execution cancellation API, CLI command, and MCP tool.
- Per-execution node concurrency limit through `GOFLOW_MAX_PARALLEL_NODES_PER_EXECUTION`.
- Sub-workflow executions reuse the root workflow global execution slot.
- CLI workflow export, import, and validate commands.
- Scoped API tokens with per-token scopes and optional workflow allowlists.
- Audit event metadata for authenticated API requests and token management.
- Admin token management endpoints and CLI commands.
- Streamable HTTP MCP beta endpoint at `/mcp` with Bearer auth, origin allowlist, and HTTP smoke test script.
- HTTP MCP smoke test can call static and dynamic workflow tools.
- HTTP MCP compatibility test covers the official SDK streamable client transport.
- HTTP MCP setup and troubleshooting guide, release candidate checklist, and packaged smoke test scripts.
- Workflow Interface UI now shows MCP readiness and can copy an HTTP MCP smoke command.

## 0.4.0-mcp-stdio-alpha - 2026-07-26

### Added

- Secure local-first defaults: Goflow binds to `127.0.0.1` by default.
- API key protection for public bindings and WebSocket connections.
- AES-256-GCM credential vault with generated local master key support.
- Execution concurrency limit, webhook rate limiting, and execution retention cleanup.
- Startup recovery that marks stale `RUNNING` executions as `INTERRUPTED`.
- AI Assistant workflow validation and repair pass.
- Bilingual node documentation in `NODES.md`.
- Backup and restore guide in `BACKUP.md`.
- Product roadmap in `ROADMAP.md`.
- Commercial strategy and trademark guidance in `COMMERCIAL.md` and `TRADEMARK.md`.
- CLI alpha commands for status, workflow list/describe/run, and execution get/watch.
- Shared TriggerService foundation for API, webhook, cron, CLI, and future MCP trigger paths.
- MCP stdio alpha with static workflow and execution tools backed by the REST client.
- MCP smoke test script for `tools/list`, `goflow_list_workflows`, and optional workflow execution.
- Workflow interface API/UI for MCP allowlisting and MCP bridge filtering.
- Per-client MCP workflow run inflight limit through `GOFLOW_MCP_MAX_INFLIGHT_PER_CLIENT`.
- GitHub Actions CI for backend tests, frontend build, vet, and cross-platform binary builds.
- Ready-to-import workflow templates for AI triage, uptime incident response, release smoke tests, weather alerts, GitHub monitoring, and stress tests.

### Changed

- README positioning now describes Goflow as a single-binary, local-first automation engine for trusted self-hosted environments.
- Webhook trigger payloads now include request body, query, and headers.
- Failed node details are surfaced in the node properties panel.
- Workflow names and descriptions can be edited from the workflow manager.

### Notes

- This is a preview release intended for local, homelab, and small internal deployments.
- Goflow is not yet positioned as a multi-user SaaS automation platform.
- CLI and MCP stdio support are alpha features; dynamic MCP workflow tools, cancellation, scoped tokens, and Streamable HTTP MCP arrive after this release in `Unreleased`.
