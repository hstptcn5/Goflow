# Changelog

All notable changes to Goflow are tracked here.

## Unreleased

### Added

- Dynamic MCP workflow tools for active workflows explicitly exposed through the Interface settings.
- MCP smoke test options for asserting and calling dynamic workflow tools.

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
- CLI and MCP stdio support are alpha features; dynamic MCP workflow tools, cancellation, scoped tokens, and Streamable HTTP MCP are not included yet.
