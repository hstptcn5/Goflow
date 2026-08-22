# Agent Lab Progress Ledger

Statuses: `PLANNED`, `IN_PROGRESS`, `DONE`, `BLOCKED`, `DEFERRED`.

| Checkpoint | Status | Evidence |
|---|---|---|
| GF-AGENT-001 Bounded workflow improvement loop | DONE | Implemented in PR #43. Final tested head `93b8fec9a4c8b7ff76ffef0dcf21a0aff8d417b0`; CI #478 success; Technical Roadmap Smoke #14 success; Vietnam Morning Brief Windows Pilot #189 success; squash merge `7ff92a9878cb55f8ba91c509d6e8308e212d3fea`. |
| GF-AGENT-002 Node Contract Grounding | DONE | Implemented in PR #45 and structured-contract follow-up PR #47. Automated gates passed on PR #45 final head `13217e2be72573429208a6bbb4a7faaa73bf4905`: CI #485, Technical Roadmap Smoke #19 and Daily Business Report Public Beta #245. Merged as `8909e256703dab48bf2fd4f145142d77dade8d35`; follow-up merged as `6125f7bbbdfd96eb8a6799b096fbfa12e5763a26`. Manual DeepSeek verification completed 2026-08-22: the validated proposal used Python direct-root references, consumed the Webhook body contract, preserved Switch cases, added the requested parallel statistics branch, and correctly returned `Safe test: BLOCKED` for Python nodes. |
| GF-CHAT-001 Persistent AI chat history | DONE | Implemented in PR #43. Per-workflow bounded localStorage history and explicit clear action covered by frontend tests/build on final tested head `93b8fec9a4c8b7ff76ffef0dcf21a0aff8d417b0`; squash merge `7ff92a9878cb55f8ba91c509d6e8308e212d3fea`. |

## GF-AGENT-002 acceptance gates

- Node registry exposes `output_reference` and `trigger_contract` for grounded built-ins. PASS automated.
- Python Code contract states direct-root output and teaches `{{<node_id>.category}}`, not an invented `.output.category` envelope. PASS automated and manual DeepSeek verification.
- Webhook contract exposes `/webhook/{workflow_id}`, active-workflow requirement and request payload fields. PASS automated; manual proposal consumed `{{<webhook_node_id>.body}}` without inferring a node-output envelope.
- Switch contract exposes `matched`, `matched_handle`, `matched_index`, `target_handle` and `value`. PASS automated; manual proposal retained valid HIGH/NORMAL cases.
- `registry.Get(type).GetDefinition()` and `registry.ListDefinitions()` expose the same common `on_error` parameter and runtime contract. PASS automated.
- Agent Lab compact definitions contain the grounded Python and Webhook contract hints. PASS automated and strengthened in PR #47.
- Backend formatting, tests, race, vet and vulnerability scan pass. PASS via CI #485.
- Existing Technical Roadmap Smoke remains green. PASS via #19.
- Manual DeepSeek verification repeats the user-discovered contract cases without the wrong Python `.output` reference or a fabricated Webhook output envelope. PASS 2026-08-22.
- Python-containing proposal is validated but not autonomously executed. PASS: Agent Lab returned `Safe test: BLOCKED` for both Python nodes as required.

## Manual DeepSeek verification evidence

The operator tested Agent Lab against a Webhook Trigger → Python Code → Switch workflow and requested a second Python statistics branch. The exported validated proposal was reviewed directly:

- Switch value: `{{node_python_category.category}}`.
- Webhook body input: `{{node_b9187269-5e50-4b1a-b7cd-20142ec4e9a8.body}}`.
- Statistics output is direct-root JSON with `count`, `sum`, `average`, `min` and `max`.
- The sample numeric input produces `count=4`, `sum=100`, `average=25`, `min=10`, `max=40`.
- The proposal adds one direct Webhook → statistics edge and leaves the primary Webhook → category → Switch path intact.
- Agent Lab reports both Python nodes as blocked from autonomous safe-test execution.

This is manual acceptance evidence for contract grounding and the v0.1 safety boundary. It is not a production activation or unrestricted Python execution claim.

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
