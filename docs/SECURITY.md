# Security

Goflow is designed for trusted local or small internal self-hosted deployments.

## Reporting A Vulnerability

Do not post credentials, exploit details, private workflow data, database files,
master keys, or unsanitized logs in a public issue.

Private vulnerability reporting is not enabled for this repository at the
Community `1.0.0` stable-candidate checkpoint. Until a private GitHub reporting
route is enabled, open a minimal issue at <https://github.com/hstptcn5/Goflow/issues>
titled `Security contact request`. State only the affected version and that a
private contact route is needed; do not include vulnerability details. A
maintainer can then arrange a private channel. No security email address is
claimed by this project.

Security fixes are prioritized for the current Community stable candidate or
published stable version and its explicitly documented upgrade source. Older
snapshots and unsupported forks are best effort. This project does not claim an
independent security audit, guaranteed response time, or complete protection
from unsafe workflows.

## Authentication

Set `GOFLOW_API_KEY` for any deployment beyond trusted localhost.

```bash
GOFLOW_API_KEY=change-me ./goflow serve
```

Goflow refuses non-loopback binds without an API key.

API clients authenticate with:

```http
Authorization: Bearer <api-key-or-scoped-token>
```

Query-string API keys are not accepted.

## Scoped Tokens

Scoped tokens are intended for CLI, MCP, scripts, and automation clients. They can restrict:

- Scopes such as `workflow:list`, `workflow:read`, `workflow:run`, `execution:read`, and `execution:cancel`.
- Allowed workflow IDs.

Use the admin API key only for local administration and token creation.

## Credentials

Credentials are encrypted with AES-256-GCM. Keep both the SQLite database and `goflow.master.key` secure. Losing the master key means encrypted credentials cannot be decrypted.

Do not commit:

- `goflow.db`
- `goflow.master.key`
- API keys
- webhook URLs with embedded secrets
- exported workflows containing secrets

## Webhooks

If a webhook trigger defines a secret, callers must include:

```http
X-Goflow-Webhook-Secret: <secret>
```

## MCP

For MCP clients:

- Prefer scoped tokens.
- Expose only approved workflows.
- Keep `/mcp` behind HTTPS outside localhost.
- Configure `GOFLOW_MCP_ALLOWED_ORIGINS` for browser or remote HTTP clients.

## Audit

Authenticated API requests and token management actions are recorded in audit events. Admins can read recent events from:

```http
GET /api/v1/audit-events?limit=100
```

## Workflow Capability Boundary

Workflow creation and editing are administrator-level capabilities. Depending on the configured nodes and credentials, a workflow can call arbitrary HTTP endpoints, connect to databases, invoke external AI providers, run remote SSH commands, and modify local Git repositories.

Scoped tokens restrict which approved workflows a caller may run. They do not sandbox an unsafe workflow.

The HTTP Request node can reach private-network and link-local services. Do not allow untrusted users to create or edit workflows. Run Goflow with the operating-system permissions and network access that its workflows actually require.

The SSH node currently does not verify host keys. Use it only on trusted networks while strict known-host verification is being implemented.

## Current Limitations

Goflow does not yet provide multi-user workspaces, full RBAC, SSO, or enterprise governance features.

