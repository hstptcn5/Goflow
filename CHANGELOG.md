# Changelog

All notable changes to Goflow are tracked here.

## 0.1.0-preview - Unreleased

### Added

- Secure local-first defaults: Goflow binds to `127.0.0.1` by default.
- API key protection for public bindings and WebSocket connections.
- AES-256-GCM credential vault with generated local master key support.
- Execution concurrency limit, webhook rate limiting, and execution retention cleanup.
- Startup recovery that marks stale `RUNNING` executions as `INTERRUPTED`.
- AI Assistant workflow validation and repair pass.
- Bilingual node documentation in `NODES.md`.
- Backup and restore guide in `BACKUP.md`.
- Commercial strategy and trademark guidance in `COMMERCIAL.md` and `TRADEMARK.md`.
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
