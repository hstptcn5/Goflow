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

    section Current
      CLI Alpha Validation : Test workflow run/watch from CLI on real workflows
      CLI Polish : Better output, docs, Windows/Linux packaging checks

    section Next
      MCP stdio Alpha : Static MCP tools over REST client
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
| `v0.4.0-mcp-stdio-alpha` | MCP stdio static tools | Not started |
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

    section Now
    CLI alpha real testing         :active, cli2, 2026-07-26, 2d
    CLI docs and release note      :cli3, after cli2, 1d

    section Next
    MCP client smoke tests         :mcp2, after cli3, 2d

    section Later
    Dynamic MCP workflow tools     :mcp3, after mcp2, 5d
    Interfaces UI                  :ui1, after mcp3, 4d
    Concurrency hardening          :hard1, after ui1, 4d
    Cancellation API               :hard2, after hard1, 4d
```

## Next Priorities

1. Test `goflow workflow run --wait` against real workflows.
2. Add CLI import/export if workflow-as-code becomes the immediate focus.
3. Smoke test `goflow mcp stdio` from an MCP client.
4. Add workflow MCP allowlist API/UI before dynamic tools.
