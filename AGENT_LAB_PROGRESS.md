# Agent Lab Progress Ledger

Statuses: `PLANNED`, `IN_PROGRESS`, `DONE`, `BLOCKED`, `DEFERRED`.

| Checkpoint | Status | Evidence |
|---|---|---|
| GF-AGENT-001 Bounded workflow improvement loop | IN_PROGRESS | Branch `agent-loop-chat-history`; backend endpoint `/api/v1/ai/agent/iterate`, full registry context, validation/secret gates, bounded deterministic safe-test loop and tests added. Awaiting CI and product verification. |
| GF-CHAT-001 Persistent AI chat history | IN_PROGRESS | Branch `agent-loop-chat-history`; per-workflow localStorage persistence, bounded history, clear action and tests added. Awaiting CI and product verification. |

## Acceptance gates

- Existing AI Build and Reviewer tests remain green.
- New Agent safety tests pass.
- New chat persistence tests pass.
- Frontend build passes.
- Backend `go test ./...` passes.
- Existing technical roadmap smoke remains green or is unaffected by path classification.
- Manual verification with a DeepSeek credential:
  1. reopen AI drawer and confirm previous workflow chat returns;
  2. run Agent Lab on a deterministic workflow and observe `Safe test: PASS`;
  3. run Agent Lab on a workflow containing Python/HTTP and observe `Safe test: BLOCKED` with a validated proposal rather than automatic execution;
  4. load a proposal to canvas and verify it is not saved/activated automatically.

Do not mark either checkpoint `DONE` until the implementation PR is merged to `main` with final CI evidence.
