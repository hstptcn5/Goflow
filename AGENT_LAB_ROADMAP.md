# Goflow Agent Lab Roadmap

This roadmap starts after the completed technical capability roadmap. It is intentionally narrow: use AI to improve workflow drafts without granting an LLM unrestricted production execution.

## GF-AGENT-001 — Bounded workflow improvement loop

Goal: let the configured OpenAI/DeepSeek model inspect the live node registry, the current workflow draft and the latest execution evidence, propose a complete workflow, validate it, and optionally run a bounded safe test before returning the proposal to the user.

### v0.1 contract

1. Agent receives the full runtime node registry dynamically; new built-in/custom/reusable definitions do not require a hard-coded catalog in the prompt.
2. Agent receives the current workflow and redacted latest execution evidence when available.
3. Every proposal must pass Goflow workflow validation before it can be returned as validated.
4. Secrets and credential parameters are stripped/preserved using the existing reviewer safety boundary.
5. Agent may auto-test only deterministic local graph types:
   - Manual Trigger
   - Cron Trigger
   - Webhook Trigger
   - GitHub Webhook trigger
   - JSON Transform
   - IF
   - Switch
6. HTTP, AI provider nodes, Python/JS, DB, File, State, messaging, SaaS, SSH/Git, plugins/custom executables and other side-effect/trusted nodes are never auto-run by Agent Lab v0.1.
7. A safe test uses an ephemeral inactive workflow and is deleted after the synchronous test. It is never exposed through CLI/MCP and never activated.
8. If a safe test fails, Agent Lab can use the redacted test evidence for another model iteration, bounded to at most 3 iterations.
9. Agent Lab never saves, activates, schedules or production-runs the proposed workflow. The user must explicitly load the proposal onto the canvas and save it.

### Not in v0.1

- Autonomous HTTP/network reads.
- Autonomous Python/JS/plugin execution.
- Autonomous messaging, file writes or database mutations.
- Production activation or background self-healing.
- Unbounded self-improvement loops.

## GF-AGENT-002 — Node Contract Grounding

Goal: stop AI clients from guessing output envelopes, expression paths, trigger URLs or parameters by making runtime semantics part of the same node definition used by the editor and validator.

### v0.1 contract

1. `NodeDefinition` exposes a machine-readable `output_reference` contract with root mode, expression pattern, examples and dynamic/static output semantics.
2. Trigger nodes can expose a machine-readable `trigger_contract` with invocation kind, public endpoint template, activation requirement and payload-root semantics.
3. Core built-ins are seeded with verified runtime outputs and examples: Manual Trigger, Cron, Webhook, HTTP Request, JSON Transform, IF, Switch and Python Code.
4. Python Code explicitly declares direct-root output semantics: if code assigns `output = {"category":"NORMAL"}`, downstream nodes use `{{pythonCode.category}}`, not `{{pythonCode.output.category}}`.
5. Webhook explicitly declares the current public invocation endpoint `/webhook/{workflow_id}` and that the node `path` parameter does not replace that endpoint in the current runtime.
6. Plugin/custom nodes that already declare outputs receive a default direct-root output-reference contract until a richer manifest contract exists.
7. The registry applies common `on_error` policy and runtime contracts before definitions are returned through either `Get` or `ListDefinitions`, so AI-advertised parameters are the same parameters workflow validation accepts.
8. Agent Lab receives the grounded contract from the live registry; contract hints are derived from the same structured definition rather than maintained as a separate Python/Webhook prompt catalog.

### Acceptance examples

- The Agent must be grounded toward `{{pythonCode.category}}` for direct Python output and away from the invented `{{pythonCode.output.category}}` envelope.
- The Agent must describe webhook invocation using `/webhook/{workflow_id}`, not `/order` merely because a node has `path: "/order"`.
- A proposal containing common `on_error` must not be rejected solely because ListDefinitions and validator disagree about the common policy parameter.

## GF-CHAT-001 — Persistent AI chat history

Goal: closing/reopening the AI drawer or remounting the editor should not discard the conversation for a saved workflow.

### v0.1 contract

1. History is stored locally in browser localStorage, keyed by workflow ID.
2. Build, review, Agent Lab and workflow proposal messages can be restored.
3. History is bounded to 80 messages and approximately 500 KB per workflow.
4. Persistence is best-effort and must never break the workflow editor.
5. The drawer exposes an explicit `Xóa lịch sử` action.
6. Unsaved/no-ID drafts are not persisted.

A future sync/session store can replace localStorage if multi-device history becomes a product requirement.
