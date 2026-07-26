# Goflow HTTP MCP Setup

This guide covers the Streamable HTTP MCP endpoint at `/mcp`.

## Requirements

- Start Goflow with `GOFLOW_API_KEY` when exposing it beyond trusted local use.
- Create a scoped token for MCP clients.
- The target workflow must be active.
- The target workflow must have **Expose to MCP** enabled in **Workflows > Interface**.
- **Requires Approval** must be disabled for the current MCP alpha/beta bridge.

## Recommended Token

```powershell
$adminKey = "your-admin-api-key"
$workflowId = "your-workflow-id"

.\goflow.exe token create mcp-runner `
  --api-key $adminKey `
  --scope workflow:list `
  --scope workflow:read `
  --scope workflow:run `
  --scope execution:read `
  --workflow $workflowId
```

Add `--scope execution:cancel` only when the MCP client needs to cancel running executions.

## Smoke Tests

Basic HTTP MCP tool listing:

```powershell
node scripts/mcp-http-smoke-test.mjs `
  --url http://127.0.0.1:8080/mcp `
  --api-key $scopedToken `
  --origin http://127.0.0.1:8080
```

Run a workflow through HTTP MCP:

```powershell
node scripts/mcp-http-smoke-test.mjs `
  --url http://127.0.0.1:8080/mcp `
  --api-key $scopedToken `
  --origin http://127.0.0.1:8080 `
  --workflow $workflowId `
  --input '{\"source\":\"mcp-http-smoke\"}'
```

Run a dynamic workflow tool:

```powershell
node scripts/mcp-http-smoke-test.mjs `
  --url http://127.0.0.1:8080/mcp `
  --api-key $scopedToken `
  --origin http://127.0.0.1:8080 `
  --expect-tool goflow.daily_report `
  --dynamic-tool goflow.daily_report `
  --input '{\"source\":\"mcp-http-dynamic-smoke\"}'
```

## Reverse Proxy Notes

Use HTTPS for any non-local deployment. Forward these headers:

- `Authorization`
- `Accept`
- `Content-Type`
- `MCP-Protocol-Version`
- `Mcp-Session-Id`
- `Last-Event-ID`
- `Origin`

Configure:

```bash
GOFLOW_MCP_ALLOWED_ORIGINS=https://your-client.example.com
GOFLOW_MCP_BASE_URL=http://127.0.0.1:8080
```

`GOFLOW_MCP_ALLOWED_ORIGINS` must match the browser or MCP client origin exactly. `GOFLOW_MCP_BASE_URL` should be the internal URL that the Goflow process can use to call its own REST API.

## Troubleshooting

`workflow is not exposed to MCP`

Enable **Expose to MCP** in the workflow Interface settings.

`workflow is inactive`

Turn on the workflow **Active** toggle.

`workflow requires approval and cannot be run through MCP alpha`

Disable **Requires Approval** for the workflow while testing MCP alpha/beta.

`Forbidden: origin is not allowed for MCP HTTP`

Set `GOFLOW_MCP_ALLOWED_ORIGINS` to the exact Origin header sent by the client.

`Unauthorized: invalid or missing API key`

Pass `Authorization: Bearer <scoped-token>` or use `--api-key <scoped-token>` with the smoke script.

`MCP HTTP tools/list passed (6 tools)` but no dynamic tool appears

Dynamic tools are registered only for workflows that are active, exposed to MCP, and do not require approval.
