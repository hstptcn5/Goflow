# Goflow Ecosystem Alpha Plan

Status: Checkpoint A

Foundation: PR #2, `feature/goflow-pack-run-mvp` at `8dd397a21f00cb8828753d177069e97f09db8a90`.

This plan maps the ecosystem alpha goal to the existing Goflow architecture, the smallest compatible contracts, and the verification evidence required before each checkpoint is considered complete.

## Current Architecture Inventory

### Server and Runtime

- `internal/serverapp/serverapp.go` owns the single runtime bootstrap: config, SQLite, credential store, workflow store, execution store, token store, audit store, node registry, engine, `TriggerService`, cron scanner, API router, and graceful shutdown.
- `internal/api/router.go` exposes the generic REST API, webhooks, WebSocket, MCP HTTP mount, health endpoint, and embedded Vue SPA fallback.
- Generic mode currently has no appliance context. Appliance endpoints must therefore be enabled only through an explicit startup option, not by probing files after server start.
- All workflow starts must keep using `application.TriggerService`; appliance run-now and setup smoke paths cannot call the engine directly.

### Pack Foundation

- `internal/pack/pack.go` validates Pack Format v1 manifests, portable paths, entry workflow JSON, listed plugins, and listed assets.
- `internal/pack/build.go` builds deterministic portable bundles and verifies `PACK_INFO.json` against ZIP and extracted contents.
- `internal/packrun/run.go` validates/extracts packs, creates external per-pack state, uses a stable managed workflow ID, binds loopback, rejects packaged plugins, and reuses healthy instances only when `RunState.PackID` matches.
- `docs/PACKS.md` documents that `PACK_INFO` hashes are unsigned integrity metadata, not publisher authenticity.

### Workflow, Credentials, and Execution

- `internal/workflow` provides bounded workflow reading and structural validation.
- `internal/storage/credential_store.go` stores credential data encrypted with AES-GCM and returns plaintext only to node execution paths.
- `internal/storage/execution_store.go` persists execution state and input/output/log metadata used by API, UI, CLI, and MCP.
- `internal/application/trigger_service.go` is the shared admission and trigger path for API, UI, CLI, MCP, webhook, cron, and replay.

### Nodes Relevant to DailyOps

- `internal/nodes/http_request.go` already uses execution context for outbound HTTP and has a response byte limit, but needs stricter method, URL, header, body, redirect, timeout, and redacted error handling for pack appliance use.
- `internal/nodes/telegram_bot.go` supports `credential_id`, but still accepts literal `bot_token`, uses a token-bearing URL, lacks injectable base URL/transport, and needs pack-only literal token rejection plus non-mutating connection tests.
- `internal/nodes/json_transform.go`, manual/webhook/cron triggers, and expression resolution are the likely minimal building blocks for the DailyOps reference pack.

### CLI and Author Tooling

- `internal/cli/cli.go` currently supports `pack validate`, `pack build`, and `pack run`.
- New author commands should extend the existing `goflow pack` tree without changing current exit-code conventions.
- Pack inspect/test/verify/init must avoid real network, browser, database mutation outside temp state, plugin execution, or secret printing.

### Frontend

- `ui/src/router.js` routes generic pages only: workflows, editor, executions, credentials, templates, nodes, settings, help.
- Existing pages and components are generic workflow editor surfaces. Appliance UI must be route-gated by backend-provided appliance bootstrap data and absent in ordinary `goflow serve`.
- Existing UI services in `ui/src/services/api.js` should be extended with appliance-only APIs rather than bypassing REST.

## Compatibility Contracts To Preserve

- Pack Format v1 keeps `schema_version: 1`; new setup metadata is optional.
- Existing valid source packs and bundles continue to validate, build, verify, and run.
- Unknown manifest fields remain forward-compatible unless they contain secret-bearing values or conflict with known setup metadata.
- `required_credentials` remains supported. Structured `credential_requirements` take precedence for setup UI; legacy entries are exposed as simple required credential slots when structured metadata is absent.
- Generic `goflow serve`, CLI workflow commands, MCP stdio/HTTP, REST clients, workflow import/export, and existing UI routes keep their current behavior.
- Pack validation, build, inspect, test, setup, and diagnostics do not execute packaged plugins or arbitrary third-party code.
- Secret values never belong in `pack.json`, workflow JSON, `PACK_INFO.json`, diagnostics, logs, URLs, query strings, generated fixtures, or bundle contents.

## Proposed Alpha Contracts

### Pack Setup Metadata

Add optional manifest fields:

- `config_schema`: bounded array of non-secret config field definitions.
- `credential_requirements`: bounded array of credential slot definitions.
- `bindings`: bounded array mapping `config.<key>` or `credential.<key>` to existing entry-workflow node parameters.

Validation rules:

- Keep all setup fields optional for backwards compatibility.
- Reject secret-like keys, labels, descriptions where they imply a secret store, and reject secret-looking defaults.
- Reject unknown credential types and unknown connection test kinds.
- Validate binding target node IDs against the loaded entry workflow.
- Validate binding target parameter names and kinds against node definitions.
- Apply bindings only to a cloned managed workflow, never to source packs or extracted bundles.
- Pack-only secret scan rejects literal values in known secret-bearing workflow parameters such as Telegram `bot_token`.

### Runtime Setup Storage

Store appliance state in the external per-pack data directory:

- `pack-config.json` for non-secret config values.
- `pack-credentials.json` for credential slot to credential ID bindings.
- `pack-setup-state.json` for setup completion state and schema version.

Files should be written atomically. Use `0600` file mode and `0700` parent directory where the OS supports it. Values are revalidated against the current manifest before use.

### Appliance Backend

Add an explicit `serverapp.ApplianceContext` passed from `packrun` into `serverapp` and then `api.NewRouter`.

Appliance endpoints should be mounted under one appliance-only prefix, for example `/api/v1/appliance`, and return 404 when no appliance context is present.

Minimum API groups:

- Bootstrap/session token.
- Pack identity and unsigned integrity notice.
- Setup schema, redacted readiness, save config, credential slot assignment, connection test, complete/reopen setup.
- Status, run-now, latest execution, recent executions.
- Redacted diagnostics export.

State-changing endpoints require exact loopback origin, valid loopback host, JSON content type, bounded body, and a high-entropy per-process token from same-origin bootstrap.

### Appliance UI

Add an appliance-specific route set that renders only when bootstrap confirms appliance mode.

First-run flow:

- Shows pack identity and unsigned development/integrity notice.
- Renders config fields from `config_schema`.
- Renders credential slots from `credential_requirements` or legacy `required_credentials`.
- Provides create/select/replace credential actions and explicit non-mutating connection tests.
- Prevents completion until required setup is valid.

Dashboard:

- Shows pack identity, readiness, server/workflow state, run-now, latest execution, recent executions, redacted errors, diagnostics export, and reconfigure action.
- Keeps advanced workflow editing secondary, not the default path.

### DailyOps Reference Pack

Add an experimental first-party example pack, suggested ID `official.dailyops-rest-telegram`, with:

- Manual trigger for tests.
- Optional cron disabled until setup is ready.
- Configured HTTP(S) JSON source URL, chat ID, report title, thresholds.
- Telegram credential slot of type `TELEGRAM_BOT`.
- Mock-only automated tests using local HTTP fixtures.
- No vendor claims and no real Telegram credentials in tests.

### CI and Artifacts

Pull-request CI should eventually cover backend, frontend, race, vulnerability, appliance E2E, pack CLI contracts, DailyOps mock E2E, deterministic bundle checks, canary scans, and existing platform builds.

Manual `workflow_dispatch` can build unsigned development alpha artifacts, upload them with explicit `UNSIGNED-DEVELOPMENT-ALPHA` naming, and never create tags, releases, installers, signatures, or latest pointers.

## Alternatives Considered

### Separate Appliance Binary

Rejected for alpha because Goflow's existing product contract is one binary with embedded UI and SQLite. A separate runtime would duplicate deployment, testing, and security boundaries.

### Pack Format v2

Rejected for alpha because optional v1 fields are enough for setup metadata and preserve existing pack compatibility.

### Storing Setup Inside Extracted Packs

Rejected because source and extracted bundles must remain immutable and transferable. Setup belongs in the external per-pack data directory.

### Executing Pack Plugins For Setup

Rejected because plugin/native execution is explicitly out of scope and would weaken the current foundation security boundary.

### Marketplace-Like Catalog

Rejected for alpha. The goal needs one first-party reference pack and local author tooling, not remote discovery or install.

## Acceptance Mapping

| Goal area | Primary implementation files | Required tests |
|---|---|---|
| Setup metadata | `internal/pack`, `internal/workflow`, `internal/nodes` definitions | Pack validation, deterministic build, bundle tamper, binding validation, secret scan |
| Runtime config | new `internal/packsetup` or `internal/packrun` helper | Atomic storage, mode best effort, upgrade preservation, stale field behavior |
| Appliance API | `internal/serverapp`, `internal/api` | State machine, origin/token/host/body limits, generic 404 isolation |
| Appliance UI | `ui/src/router.js`, new appliance pages/components/services | Unit, component, Playwright first-run/dashboard/secret disappearance |
| HTTP and Telegram hardening | `internal/nodes/http_request.go`, `internal/nodes/telegram_bot.go` | Context, limits, redirects, redaction, mock connection tests |
| Pack author CLI | `internal/cli`, `internal/pack` | Golden CLI, JSON output, exit codes, fresh scaffold end-to-end |
| DailyOps pack | `examples/packs/dailyops-rest-telegram`, tests | Mock source, mock Telegram, exact-once send, deterministic build |
| CI artifacts | `.github/workflows/ci.yml` | PR checks, manual artifact workflow, canary scan |
| Docs and handoff | `docs/*`, `README.md`, `RELEASE.md` | Docs match behavior, threat model maps mitigations to tests |

## Checkpoint Gates

### Checkpoint A

- Preflight confirms PR #2 state and foundation commit.
- Existing backend and frontend baseline passes before changes.
- This plan maps later acceptance criteria to code and tests.
- No production behavior changes.

### Checkpoints B Through I

Each checkpoint must:

- Start from a clean tracked worktree.
- Add focused tests before or with behavior changes.
- Run relevant local gates.
- Update `docs/ECOSYSTEM_ALPHA_PROGRESS.md`.
- Commit and push a reviewable checkpoint.
- Keep PRs draft and unmerged.

## Current Baseline Evidence

Preflight on 2026-08-09:

- Branch before alpha: `feature/goflow-pack-run-mvp`.
- Foundation HEAD: `8dd397a21f00cb8828753d177069e97f09db8a90`.
- PR #2: open, draft, unmerged, mergeable clean.
- PR #2 CI run #55: completed success.
- Branch relation to `origin/main`: ahead 10, behind 0.
- Tracked worktree modifications: none.
- Untracked files intentionally not included: `GOFLOW_ECOSYSTEM_MASTER_GOAL.md`, `roseblade-ranger.audit.html`.
- Tracked binary/ZIP/database/key/secret/temp artifact scan: no matches.

Baseline commands before code changes:

- `go test ./...`: pass.
- `go vet ./...`: pass.
- `go build ./...`: pass.
- `cd ui && npm ci`: completed; npm audit reported 1 moderate and 1 high vulnerability in the existing dependency tree.
- `cd ui && npm run test`: pass, 10 files and 48 tests.
- `cd ui && npm run build`: pass.
