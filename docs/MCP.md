# MCP

Goflow exposes MCP tools over stdio and Streamable HTTP. Both transports call the configured Goflow REST API and share the same backend execution path, authorization checks, workflow allowlist, idempotency, and audit behavior.

## MCP stdio

```bash
goflow mcp stdio
```

Set environment variables for the launched MCP process:

```bash
GOFLOW_URL=http://127.0.0.1:8080
GOFLOW_API_KEY=your-scoped-token
```

## MCP HTTP

The HTTP MCP endpoint is mounted at `/mcp` on the same Goflow server.

```bash
node scripts/mcp-http-smoke-test.mjs \
  --url http://127.0.0.1:8080/mcp \
  --api-key your-scoped-token \
  --origin http://127.0.0.1:8080
```

For detailed HTTP setup, reverse proxy headers, allowed origins, dynamic tools, and troubleshooting, see [../MCP_HTTP.md](../MCP_HTTP.md).

## Workflow Exposure

MCP workflow access is opt-in:

1. Open a workflow.
2. Open Interface settings.
3. Enable **Expose to MCP**.
4. Keep the workflow active.
5. Use a scoped token that includes the workflow in its allowlist.

Workflows marked **Requires Approval** are blocked by the current MCP alpha/beta bridge.

## Recommended Token

```bash
goflow token create mcp-runner \
  --scope workflow:list \
  --scope workflow:read \
  --scope workflow:run \
  --scope execution:read \
  --workflow <workflow-id>
```

Add `--scope execution:cancel` if the MCP client must cancel executions.

