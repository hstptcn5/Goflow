# Goflow Roadmap

This roadmap describes the current product direction for Goflow. It is directional, not a promise that every item will ship exactly as written.

Goflow Community remains a local-first, single-binary workflow automation engine for individuals, homelabs, and small internal deployments. Commercial editions may add team governance, managed operations, premium integrations, and support.

## Product Principles

- Keep the Community edition useful for real local and internal automations.
- Prefer secure defaults, clear docs, and reliable execution before adding broad integration surface.
- Make workflow creation easier through templates, examples, inline guidance, and predictable node behavior.
- Keep the runtime simple: one executable, embedded UI, SQLite, and no required external services by default.
- Treat team, governance, and managed deployment features as commercial candidates.

## Current Preview

Implemented or substantially in place:

- Single-binary Go backend with embedded Vue UI.
- SQLite storage with local credential vault encryption.
- Cron, webhook, HTTP, AI, database, communication, scripting, logic, sub-workflow, and plugin nodes.
- AI Assistant workflow generation with validation and repair.
- Template Gallery with ready-to-import workflows.
- Bilingual node documentation in `NODES.md`.
- Backup, release, plugin, commercial, and trademark guidance.
- Local security hardening for API access, WebSocket access, webhook limits, cleanup, and stale execution recovery.

## Phase 1: Usability and Onboarding

Goal: make Goflow easier for new users to understand and use without knowing every node upfront.

- First-run setup checklist for API key, master key, credentials, and first template.
- Credential setup hints directly inside node configuration panels.
- Better empty states for workflows, executions, credentials, templates, and plugins.
- Template search, categories, tags, and difficulty labels.
- One-click sample workflow load for common use cases.
- Node output schema hints for placeholder discovery.
- Import validation that explains missing credentials, unsupported nodes, and malformed edges.
- More screenshots and short docs for common workflows.

## Phase 2: Reliability and Operations

Goal: make self-hosted use safer and easier to operate over time.

- Workflow version history and rollback.
- Backup and restore UI.
- Execution search, filtering, and export.
- Per-node retry policy configuration.
- Error routing for failed branches.
- Dead-letter or quarantine workflow pattern for failed webhook payloads.
- Log export bundle for debugging.
- Credential rotation helper and stale credential warnings.
- Health endpoint and basic operational diagnostics.

## Phase 3: Plugin and Template Ecosystem

Goal: let advanced users extend Goflow without changing the core binary.

- Plugin SDK and local test harness.
- Plugin manifest validation.
- Plugin examples for validation, redaction, enrichment, scoring, and alert formatting.
- Template contribution guidelines.
- Public examples catalog.
- Optional node packs for domain-specific integrations.
- Security model for trusted plugin directories and signed plugin metadata.

See `CLI_MCP_ROADMAP.md` for the deeper plan to expose Goflow workflows through a CLI and MCP-compatible AI tool interface.

## Phase 4: Team and Pro Foundations

Goal: define the commercial boundary without weakening Community.

Commercial candidates:

- Multi-user login.
- Team workspaces.
- Role-based access control.
- Audit logs.
- Workflow approvals.
- Managed OAuth connectors.
- Advanced observability dashboards.
- Encrypted remote backups.
- Priority support.
- SSO/OIDC/SAML/SCIM for Enterprise.
- Hosted Goflow Cloud.

Community should continue to support single-user self-hosting, workflow building, local credentials, templates, plugins, and common built-in nodes.

## Not Planned Right Now

- Turning the Community edition into a multi-tenant SaaS server.
- Requiring Docker, PostgreSQL, Redis, or Node.js for production runtime.
- Shipping an unrestricted public plugin marketplace before plugin safety and trust rules are mature.
- Trying to match every Zapier, Make, or n8n connector before Goflow's local-first core is polished.

## Contribution Focus

The most valuable near-term contributions are:

- Clear workflow templates with realistic placeholders.
- Node docs with examples, common errors, and credential instructions.
- Bug fixes in execution visibility, validation, and import/export behavior.
- Small focused integrations that fit the local-first model.
- Plugin examples that demonstrate safe input/output contracts.
