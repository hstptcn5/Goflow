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

    section Current
      Dynamic MCP Tools : Exposed workflows register as separate AI tools

    section Next
      Input Schema Validation : Validate workflow run input server-side
      Concurrency Hardening : MCP inflight limits, node concurrency, sub-workflow slot fix
      Cancellation : Cancel endpoint, CLI cancel, MCP cancel
      Streamable HTTP MCP : Remote/local HTTP MCP beta with auth and Origin validation
```

## Phase Status

| Phase | Goal | Status |
| :--- | :--- | :--- |
| `v0.2.0-foundation-preview` | Migration, execution metadata, async execution ID, idempotency | Mostly complete and tested manually |
| `v0.3.0-cli-alpha` | CLI status/list/describe/run/get/watch | Initial implementation complete; needs more real-workflow testing |
| `v0.4.0-mcp-stdio-alpha` | MCP stdio static tools | Released |
| `v0.5.0-mcp-dynamic-preview` | Workflow MCP exposure, dynamic tools, input schema validation | Dynamic workflow tools implemented; input validation pending |
| Hardening | Concurrency, cancellation, scoped token, audit | P0 concurrency and secret redaction complete; cancellation/scoped tokens pending |
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
[x] Workflow MCP allowlist UI/API
[x] Global/per-client MCP concurrency
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
    Workflow MCP allowlist API/UI  :done, mcp3, 2026-07-27, 1d
    Global/per-client concurrency  :done, hard0, 2026-07-27, 1d
    Release v0.4.0 alpha           :done, rel1, 2026-07-27, 1d

    section Now
    Dynamic MCP workflow tools     :active, mcp4, 2026-07-27, 2d

    section Next
    Input schema validation        :mcp5, after mcp4, 2d

    section Later
    Concurrency hardening          :hard1, after mcp5, 4d
    Cancellation API               :hard2, after hard1, 4d
```

## Next Priorities

1. Test `--expect-tool` and `--dynamic-tool` against a real exposed workflow.
2. Add server-side input schema validation for workflow runs.
3. Add node concurrency hardening.
4. Add cancellation API / CLI cancel / MCP cancel.
