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
