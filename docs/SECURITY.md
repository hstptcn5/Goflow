# Security

Goflow is designed for trusted local or small internal self-hosted deployments.

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

## Current Limitations

Goflow does not yet provide multi-user workspaces, full RBAC, SSO, or enterprise governance features.

