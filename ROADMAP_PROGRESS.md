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
      MCP stdio Alpha : Static MCP tools over REST client and smoke test script

    section Current
      Workflow Interfaces : MCP allowlist API/UI and dynamic workflow tool mapping

    section Next
      Dynamic MCP Tools : Expose selected workflows as AI tools
      Concurrency Hardening : MCP inflight limits, node concurrency, sub-workflow slot fix
      Cancellation : Cancel endpoint, CLI cancel, MCP cancel
      Streamable HTTP MCP : Remote/local HTTP MCP beta with auth and Origin validation
```

## Phase Status

| Phase | Goal | Status |
| :--- | :--- | :--- |
| `v0.2.0-foundation-preview` | Migration, execution metadata, async execution ID, idempotency | Mostly complete and tested manually |
| `v0.3.0-cli-alpha` | CLI status/list/describe/run/get/watch | Initial implementation complete; needs more real-workflow testing |
| `v0.4.0-mcp-stdio-alpha` | MCP stdio static tools | Initial implementation complete; smoke script added |
| `v0.5.0-mcp-dynamic-preview` | Workflow MCP exposure, dynamic tools, input schema validation | Not started |
| Hardening | Concurrency, cancellation, scoped token, audit | Secret redaction complete; other items pending |
| Streamable HTTP MCP | `/mcp` HTTP transport | Deferred |

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
[ ] Workflow MCP allowlist UI/API
[ ] Global/per-client MCP concurrency
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

    section Now
    Workflow MCP allowlist API/UI  :active, mcp3, 2026-07-27, 4d

    section Next
    Dynamic MCP workflow tools     :mcp4, after mcp3, 5d

    section Later
    Concurrency hardening          :hard1, after mcp4, 4d
    Cancellation API               :hard2, after hard1, 4d
```

## Next Priorities

1. Run `node scripts/mcp-smoke-test.mjs --workflow <workflow>` against a real workflow before release.
2. Add workflow MCP allowlist API/UI before dynamic tools.
3. Add dynamic MCP workflow tools for explicitly exposed workflows.
4. Add per-client MCP concurrency limits.
