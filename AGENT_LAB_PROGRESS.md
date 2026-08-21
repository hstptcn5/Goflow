# Agent Lab Progress Ledger

Statuses: `PLANNED`, `IN_PROGRESS`, `DONE`, `BLOCKED`, `DEFERRED`.

| Checkpoint | Status | Evidence |
|---|---|---|
| GF-AGENT-001 Bounded workflow improvement loop | DONE | Implemented in PR #43. Final tested head `93b8fec9a4c8b7ff76ffef0dcf21a0aff8d417b0`; CI #478 success; Technical Roadmap Smoke #14 success; Vietnam Morning Brief Windows Pilot #189 success; squash merge `7ff92a9878cb55f8ba91c509d6e8308e212d3fea`. |
| GF-AGENT-002 Node Contract Grounding | IN_PROGRESS | Implemented in PR #45. Final tested head `13217e2be72573429208a6bbb4a7faaa73bf4905`; CI #485 success; Technical Roadmap Smoke #19 success; Daily Business Report Public Beta #245 success; squash merge `8909e256703dab48bf2fd4f145142d77dade8d35`. Automated gates are complete; awaiting manual DeepSeek verification of the user-discovered Python output-reference and webhook-invocation cases. |
| GF-CHAT-001 Persistent AI chat history | DONE | Implemented in PR #43. Per-workflow bounded localStorage history and explicit clear action covered by frontend tests/build on final tested head `93b8fec9a4c8b7ff76ffef0dcf21a0aff8d417b0`; squash merge `7ff92a9878cb55f8ba91c509d6e8308e212d3fea`. |

## GF-AGENT-002 acceptance gates

- Node registry exposes `output_reference` and `trigger_contract` for grounded built-ins. PASS automated.
- Python Code contract states direct-root output and teaches `{{<node_id>.category}}`, not an invented `.output.category` envelope. PASS automated.
- Webhook contract exposes `/webhook/{workflow_id}`, active-workflow requirement and request payload fields. PASS automated.
- Switch contract exposes `matched`, `matched_handle`, `matched_index`, `target_handle` and `value`. PASS automated.
- `registry.Get(type).GetDefinition()` and `registry.ListDefinitions()` expose the same common `on_error` parameter and runtime contract. PASS automated.
- Agent Lab compact definitions contain the grounded Python and Webhook contract hints. PASS automated.
- Backend formatting, tests, race, vet and vulnerability scan pass. PASS via CI #485.
- Existing Technical Roadmap Smoke remains green. PASS via #19.
- Manual DeepSeek verification repeats the two user-discovered cases and no longer claims the wrong Python reference or webhook invocation URL. PENDING MANUAL.

## Final verification for completed v0.1 checkpoints

- Existing AI Build and Reviewer tests: PASS.
- Agent safety tests: PASS.
- Chat persistence tests: PASS.
- Frontend tests/build: PASS.
- Backend formatting, `go test ./...`, race, vet and vulnerability scan: PASS.
- Technical Roadmap Smoke #14: PASS.
- Vietnam Morning Brief Windows Pilot #189: PASS.
- Full CI #478: PASS.

## v0.1 safety boundary retained

- Agent Lab never saves, activates, schedules or production-runs a proposal automatically.
- Automatic test execution is restricted to the deterministic allowlist documented in `AGENT_LAB_ROADMAP.md`.
- HTTP, Python/JS, database, file, messaging, SaaS and plugin/custom executable nodes remain blocked from autonomous test execution.
- Proposed workflow changes still require explicit user load/save.
