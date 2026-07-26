# Goflow GOAL Progress

North Star: `GOAL.md`. Current project status: `IN_PROGRESS`.

Status values: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED`, `DONE`, `MANUAL_VERIFICATION_REQUIRED`.

| Definition of Done Area | Status | Implementation Evidence | Test Evidence | Files |
|---|---|---|---|---|
| Shared execution path for API, webhook, cron, CLI, MCP | DONE | CLI/MCP call REST API; API routes use `TriggerService`; cron uses `TriggerService`. | `go test ./internal/cli ./internal/mcpserver ./internal/application`; GOAL smoke includes cron execution with `trigger_source=cron`. | `internal/cli/cli.go`, `internal/mcpserver/server.go`, `internal/application/trigger_service.go`, `main.go` |
| Unknown node fails without hanging | DONE | Unregistered executor path decrements scheduler remaining count and closes completion channel. | `TestUnknownNodeTypeMarksExecutionFailedWithoutHanging`; GOAL smoke unknown-node check. | `internal/engine/engine.go`, `internal/engine/engine_test.go`, `scripts/goal-smoke-test.ps1` |
| Execution source/principal/audit | DONE | UI/CLI/MCP/webhook/API source mapping; principal comes from auth context; audit persisted for authenticated API. | `TestRequestTriggerSourceAllowsUIAndRejectsUnknownValues`; GOAL smoke UI source and audit checks. | `internal/api/workflow_handler.go`, `ui/src/services/api.js`, `internal/api/workflow_interface_test.go` |
| Concurrent idempotency | DONE | Duplicate idempotency lookup wins before admission/concurrency rejection; storage race returns existing execution. | `TestTriggerServiceAsyncConcurrentIdempotency`, per-workflow/global idempotency limit tests, GOAL smoke 20-job duplicate test. | `internal/application/trigger_service_test.go`, `internal/storage/execution_store.go` |
| Global, node, workflow, MCP limits | DONE | Global execution limit, node semaphore default 4, per-workflow reject policy, MCP inflight and HTTP rate limit. | `TestDefaultMaxParallelNodesPerRun`, `TestHTTPHandlerRateLimitsByHashedBearerAcrossRequests`, application concurrency tests. | `config/config.go`, `internal/engine/engine.go`, `internal/mcpserver/http.go` |
| CLI/MCP exposure enforcement | DONE | Server rejects CLI/MCP sources when workflow exposure is disabled; MCP hides inactive/requires-approval workflows. | `TestTriggerServiceEnforcesCLIExposure`, `TestTriggerServiceEnforcesMCPExposureAndApproval`, MCP tests and smoke. | `internal/application/trigger_service.go`, `internal/mcpserver/server.go` |
| Scoped token workflow allowlist | DONE | Scoped list returns only allowlisted summaries; no graph JSON. MCP fetches interface metadata safely. | `TestListWorkflowsWithScopedTokenReturnsAllowlistedSummaries`, `TestHTTPHandlerWorksWithSDKStreamableClient`, GOAL smoke scoped token check. | `internal/api/workflow_handler.go`, `internal/mcpserver/server.go`, `internal/client/client.go` |
| MCP execution output hides raw input | DONE | MCP `get_execution` and `list_executions` use safe DTO without `input_json`. | `TestMCPGetExecutionDoesNotExposeInputJSON`, `TestMCPListExecutionsDoesNotExposeInputJSON`, MCP smoke leak checks. | `internal/mcpserver/server.go`, `internal/mcpserver/server_test.go`, `scripts/mcp-smoke-test.mjs`, `scripts/mcp-http-smoke-test.mjs` |
| Input/header/log/credential redaction | DONE | Header filtering and recursive redaction with cycle/depth guard. | Redaction tests, webhook security tests, user crash repro rerun, `go test ./internal/engine ./internal/api`. | `internal/engine/redact.go`, `internal/api/workflow_handler.go`, `internal/api/workflow_security_test.go` |
| Sub-workflow safety | DONE | Depth env limit and cycle detection; child workflows reuse root execution permit. | Engine sub-workflow cycle/depth tests. | `internal/engine/engine.go`, `internal/nodes/sub_workflow.go`, `internal/engine/engine_test.go` |
| CLI import/export/validate | DONE | Import/export preserve interface metadata; validate checks structure, DAG, node type, required params, schema keywords. | CLI validator tests for duplicate node, bad edge, cycle, unknown type, missing param, unsupported schema. | `internal/cli/cli.go`, `internal/cli/cli_test.go` |
| MCP stdio and HTTP with official SDK/client | DONE | Static tools, dynamic tools, reload command, Streamable HTTP handler, custom origin support. | `TestHTTPHandlerWorksWithSDKStreamableClient`; GOAL smoke for stdio, HTTP, dynamic tool, custom origin. | `internal/mcpserver/server.go`, `internal/mcpserver/http.go`, `scripts/goal-smoke-test.ps1` |
| Migration safety | DONE | Versioned migrations via `schema_migrations`; rerun checks exist. | Storage migration tests. | `internal/storage/schema.go`, `internal/storage/execution_store_test.go` |
| Windows binary local smoke | DONE | Rebuilt `goflow.exe`; smoke uses temp DB/key and port 18080. | `.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key` passed. | `scripts/goal-smoke-test.ps1`, `goflow.exe` |
| Linux clean-machine smoke | MANUAL_VERIFICATION_REQUIRED | Release scripts exist, but not run in this Windows session. | Pending on Linux or GitHub Actions artifact. | `scripts/build-release.sh`, `RELEASE.md` |
| Final local release gate | DONE | Docs updated after implementation/tests. | `go test ./...`, `go vet ./...`, `go build ./...` passed. | `README.md`, `CHANGELOG.md`, `ROADMAP_PROGRESS.md`, `RELEASE.md` |

## Current Evidence Log

- Passed: `go test ./config ./internal/api ./internal/application ./internal/cli ./internal/engine ./internal/mcpserver`
- Passed: `npm run build` in `ui/`
- Passed: `node --check scripts/mcp-smoke-test.mjs; node --check scripts/mcp-http-smoke-test.mjs`
- Passed: `.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key` including cron trigger smoke
- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `cd ui && npm ci`
- Passed: `cd ui && npm run build`
- Passed: `cd ui && npm run test` with 5 files and 8 tests passing
- Passed: `cd ui && npm run test:e2e` with 1 Playwright smoke passing
- Passed: embedded route refresh smoke for `/workflows`, `/workflows/test-refresh`, `/executions`, `/credentials`, and `/NODES.md`
- Passed after UX Milestone 2: `go test ./...`, `go vet ./...`, `go build ./...`, `cd ui && npm ci`, `npm run build`, `npm run test` with 7 files and 16 tests, `npm run test:e2e` with 5 browser tests, embedded binary UX smoke, and `scripts/goal-smoke-test.ps1`.

## Remaining Blockers

- No code blocker found.
- Linux clean-machine smoke still requires a Linux runner or GitHub Actions artifact validation.
