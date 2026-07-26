# Goflow Roadmap Progress

This document tracks the current implementation progress for the Goflow CLI and MCP roadmap.

## Timeline Overview

```mermaid
timeline
    title Goflow CLI/MCP Roadmap Progress

    section Done
      v0.1 Preview : Web UI, workflow engine, credentials, templates, docs, plugin examples
      Foundation : Migration framework, workflow interface metadata, execution metadata
      Execution API : Async returns execution_id, idempotency, trigger source/principal
      Hardening : Secret redaction in execution logs/events
      CLI Alpha Start : status, workflow list/describe/run, execution get/watch
      P0 Complete : CLI alpha, MCP stdio, allowlist, global/per-client concurrency
      v0.4.0 MCP stdio Alpha : Released

    section P1 Complete
      P1 Hardening : Input validation, cancellation, node concurrency, workflow-as-code
      Scoped Tokens : Token scopes, workflow allowlists, audit metadata

    section Next
      Streamable HTTP MCP : HTTP beta endpoint, smoke test, client compatibility hardening
```

## Phase Status

| Phase | Goal | Status |
| :--- | :--- | :--- |
| `v0.2.0-foundation-preview` | Migration, execution metadata, async execution ID, idempotency | Mostly complete and tested manually |
| `v0.3.0-cli-alpha` | CLI status/list/describe/run/get/watch | Initial implementation complete; needs more real-workflow testing |
| `v0.4.0-mcp-stdio-alpha` | MCP stdio static tools | Released |
| `v0.5.0-mcp-dynamic-preview` | Workflow MCP exposure, dynamic tools, input schema validation | Dynamic workflow tools and input validation implemented |
| Hardening | Concurrency, cancellation, scoped token, audit | Complete for P1 |
| Streamable HTTP MCP | `/mcp` HTTP transport | P2 complete; ready for release-candidate testing |

## P0 Checklist

```text
[x] Migration framework
[x] Async execution ID
[x] Execution source/principal
[x] Idempotency
[x] Workflow input/output schema fields
[x] Secret redaction in execution logs
[x] CLI status/list/describe/run/get/watch initial implementation
[x] TriggerService service layer
[x] MCP stdio static tools
[x] MCP client smoke test script
[x] Workflow MCP allowlist UI/API
[x] Global/per-client MCP concurrency
```

## P1 Checklist

```text
[x] Dynamic MCP workflow tools
[x] Server-side input schema validation
[x] Node concurrency limit
[x] Sub-workflow nested slot fix
[x] Cancellation API
[x] CLI cancel
[x] MCP cancel
[x] CLI import/export/validate
[x] Scoped token
[x] Audit metadata
```

## P2 Checklist

```text
[x] Streamable HTTP MCP endpoint at /mcp
[x] Bearer auth through scoped tokens/API key
[x] Origin allowlist for browser/remote clients
[x] HTTP MCP smoke test script
[x] HTTP MCP workflow and dynamic tool smoke test support
[x] Production deployment notes for reverse proxies/TLS
[x] Real-client compatibility testing
[x] Release candidate checklist
[x] HTTP MCP setup/troubleshooting guide
[x] Workflow Interface MCP readiness UI
```

## Remaining Timeline

```mermaid
gantt
    title Goflow CLI/MCP Remaining Timeline
    dateFormat  YYYY-MM-DD
    axisFormat  %d/%m

    section Completed
    Foundation DB/API             :done, f1, 2026-07-20, 4d
    Idempotency + execution ID     :done, f2, 2026-07-23, 2d
    Secret redaction               :done, f3, 2026-07-25, 1d
    CLI alpha implementation       :done, cli1, 2026-07-25, 1d
    TriggerService extraction      :done, svc1, 2026-07-26, 1d
    MCP stdio static tools         :done, mcp1, 2026-07-26, 1d
    MCP smoke test script          :done, mcp2, 2026-07-26, 1d
    Workflow MCP allowlist API/UI  :done, mcp3, 2026-07-26, 1d
    Global/per-client concurrency  :done, hard0, 2026-07-26, 1d
    Release v0.4.0 alpha           :done, rel1, 2026-07-26, 1d
    Dynamic MCP workflow tools     :done, mcp4, 2026-07-26, 1d
    Input schema validation        :done, mcp5, 2026-07-26, 1d
    Cancellation API/CLI/MCP       :done, hard2, 2026-07-26, 1d
    Node/sub-workflow hardening    :done, hard1, 2026-07-26, 1d
    Scoped tokens and audit        :done, sec1, 2026-07-26, 1d

    section Completed
    Streamable HTTP MCP foundation :done, http1, 2026-07-26, 1d
    HTTP MCP smoke hardening       :done, http2, 2026-07-26, 1d
    HTTP MCP client compatibility  :done, http3, 2026-07-26, 1d
    Release candidate polish       :done, rc1, 2026-07-26, 1d
```

## Next Priorities

1. Package a preview release candidate.
2. Run the release checklist against the packaged archive.
3. Publish the release after smoke tests pass on a clean machine.
