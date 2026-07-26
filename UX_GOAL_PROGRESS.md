# Goflow UX GOAL Progress

North Star: `UX_GOAL.md`. Runtime contract: `GOAL.md`.

Current UX status: `IN_PROGRESS`.
Milestone 1 status: `DONE`.
Milestone 2 status: `MANUAL_VERIFICATION_REQUIRED`.

Allowed status values: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED`, `MANUAL_VERIFICATION_REQUIRED`, `DONE`.

## Milestone 1 - UX Audit And Foundation

| Requirement | Status | Implementation evidence | Test evidence | Manual evidence | Remaining work |
|---|---|---|---|---|---|
| UI audit, user journey, component inventory | DONE | `docs/UX_AUDIT.md` | Documentation review | Not required for closure | Keep updated as UX changes |
| Design token foundation | DONE | `ui/src/assets/main.css` | `npm run build`, component tests | Windows visual review done locally during smoke | Deeper token cleanup later |
| App shell and primary navigation | DONE | `ui/src/components/AppShell.vue`, `ui/src/router.js`, `ui/src/App.vue` | `ui/tests/app-shell.test.js`, Playwright navigation smoke | Local browser smoke | Continue polish later |
| Dedicated Workflows, Editor, Executions, Credentials routes | DONE | `ui/src/pages/*.vue`, route config | `ui/tests/pages.test.js`, E2E create/run/navigation | Local browser smoke | Execution page debugger is later milestone |
| Save state and route refresh | DONE | `ui/src/stores/workflowStore.js`, `internal/api/router.go` | Store/component tests, embedded route fallback test | Local binary route refresh smoke | None |

## Milestone 2 - Editor Usability Closure

| Requirement | Status | Implementation evidence | Test evidence | Manual evidence | Remaining work |
|---|---|---|---|---|---|
| Test Workflow uses current dirty graph | DONE | `ui/src/components/WorkflowEditor.vue` saves dirty graph before triggering and aborts on failed save | `ui/tests/workflow-editor.test.js`, `ui/tests/e2e/milestone2-closure.spec.js` | Local browser smoke | None |
| Save allows incomplete drafts | DONE | `saveCanvas()` blocks only structural graph errors; Test/Activate block missing required params/credentials | `ui/tests/workflow-editor.test.js`, `ui/tests/e2e/milestone2-closure.spec.js` | Local browser smoke | None |
| Save blocks structural graph errors | DONE | `splitValidationIssues()` classifies duplicate IDs, invalid edges, cycles, and unknown nodes as hard save blockers | `ui/tests/workflow-editor-utils.test.js` | Not required | None |
| Node picker search, categories, recent/favorites | DONE | `ui/src/components/NodePicker.vue` | `ui/tests/node-picker.test.js`, E2E picker flow | Local browser smoke | None |
| Node picker avoids nested interactive controls | DONE | Node item is a `role=option` wrapper with sibling select/favorite buttons | `ui/tests/node-picker.test.js`, `ui/tests/e2e/milestone2-closure.spec.js` | Local browser smoke | None |
| Node picker keyboard semantics | DONE | Arrow Up/Down select, Enter on result adds, Space/Enter on favorite toggles only, Escape closes | Component and E2E tests | Local browser smoke | None |
| Dialog focus behavior | DONE | Focus search on open, trap Tab/Shift+Tab, Escape/backdrop close, restore opener focus | `ui/tests/node-picker.test.js`, `ui/tests/e2e/milestone2-closure.spec.js` | Local browser smoke | None |
| Quick-add and no mandatory palette | DONE | Editor Add step and quick-add paths use picker; palette is optional | `ui/tests/workflow-editor.test.js`, E2E editor usability flow | Local browser smoke | Edge quick-add insertion remains later polish |
| Node cards and canvas cleanup | DONE | `WorkflowNode.vue`, neutral idle edges, IF true/false handles | E2E visual snapshot baselines | Local visual review | Broader execution overlay belongs to Milestone 4 |
| Dirty/saved snapshot is fingerprint based | DONE | `graphFingerprint()` and `savedFingerprint` compare current graph with saved snapshot | `ui/tests/workflow-editor-utils.test.js`, `ui/tests/workflow-editor.test.js` | Local browser smoke | None |
| Undo/redo, duplicate, copy/paste, delete, auto-layout | DONE | `WorkflowEditor.vue`, `workflowEditor.js` | Component tests and E2E keyboard/editor flows | Local browser smoke | Multi-select/select-all remains later |
| Robust ID generation | DONE | `generateGraphId()` uses `crypto.randomUUID()` with fallback and is used for nodes, pasted nodes, duplicated nodes, and edges | `ui/tests/workflow-editor-utils.test.js` rapid ID test | Not required | None |
| Pre-run validation | DONE | Missing required params/credentials, unknown nodes, duplicate IDs, invalid edges, cycles | Unit/component/E2E validation tests | Local browser smoke | Backend remains source of truth |
| Visual regression baselines | DONE | Playwright `toHaveScreenshot()` baselines for six editor states | `npm run test:e2e` verifies snapshots | Local visual review | Cross-platform baseline review if Linux CI adds screenshots |
| Performance smoke with separated timings | DONE | `ui/tests/e2e/performance.spec.js` measures editor ready, picker open, picker search, auto-layout for 10/50/100 nodes | `npm run test:e2e` | Local Windows Chromium run | Low-end Windows profiling remains manual |
| Screen-reader pass | MANUAL_VERIFICATION_REQUIRED | Accessible labels/focus behavior implemented | Automated focus/label tests | Pending screen-reader pass | Run NVDA or Windows Narrator pass |
| Node discovery under 15 seconds | MANUAL_VERIFICATION_REQUIRED | Searchable picker implemented | Automated picker tests | Pending timed user test | Test with target users |
| 4/5 usability tester task completion | MANUAL_VERIFICATION_REQUIRED | Editor path implemented | Automated workflow creation tests | Pending user testing | Run UX_GOAL Task A and D with 5 testers |
| Low-end Windows review | MANUAL_VERIFICATION_REQUIRED | Performance smoke exists | Local Chromium timing captured | Pending low-end machine check | Run editor smoke on clean lower-end Windows |

## Final UX Definition Of Done Snapshot

| # | Definition of Done | Status | Evidence | Remaining work |
|---:|---|---|---|---|
| 1 | Clear app shell information architecture | DONE | AppShell, router, E2E nav | Continue polish |
| 2 | Workflows, Executions, Credentials not modal-only | DONE | Dedicated pages/routes | Deeper debugger later |
| 3 | Node picker search/category/keyboard/quick-add | DONE | NodePicker, quick-add, tests | Manual timing evidence pending separately |
| 4 | Fixed node palette no longer mandatory | DONE | Add step and quick-add primary | Optional palette cleanup later |
| 5 | Canvas visual noise reduced | DONE | Neutral idle canvas and snapshots | Human review continues |
| 6 | Edge only animates for execution-related state | DONE | Idle edges not animated | Execution overlay later |
| 7 | Node card configuration/execution state clear | DONE | Node card badges/status | More summaries later |
| 8 | Workflow dirty/saved state | DONE | Store state plus graph fingerprint | None |
| 9 | Undo/redo | DONE | Snapshot history and tests | Multi-select later |
| 10 | Validation before run | DONE | Frontend validation blocks Test/Activate | Backend remains source of truth |
| 11 | Inspector Parameters/Input/Output/Logs | IN_PROGRESS | Current config/output tabs exist | Milestone 3 |
| 12 | Required field and credential inline errors | IN_PROGRESS | Badges/summary exist | Milestone 3 field-level refinement |
| 13 | Data mapping between nodes via UI | IN_PROGRESS | Existing dynamic picker exists | Milestone 3 hardening |
| 14 | Expression preview | NOT_STARTED | Not implemented | Milestone 3 |
| 15 | Execution selector | IN_PROGRESS | Executions page selector exists | Editor selector in Milestone 4 |
| 16 | Failed path highlight | IN_PROGRESS | Failed node state visible | Milestone 4 path highlight |
| 17 | Retry/replay full workflow | NOT_STARTED | Not implemented | Milestone 4 |
| 18 | Redacted debug bundle | NOT_STARTED | Runtime redaction exists | Milestone 4 bundle UI |
| 19 | Diagnose failed node under 2 minutes | MANUAL_VERIFICATION_REQUIRED | Debug surfaces partially exist | Timed user test after Milestone 4 |
| 20 | First workflow under 10 minutes | MANUAL_VERIFICATION_REQUIRED | E2E create/save/run path exists | Timed user test |
| 21 | 4/5 usability testers complete task | MANUAL_VERIFICATION_REQUIRED | Not automatable | Recruit/test users |
| 22 | Frontend component tests pass | DONE | `npm run test` | Maintain |
| 23 | E2E tests pass | DONE | `npm run test:e2e` | Maintain |
| 24 | Backend tests no regression | DONE | `go test ./...` gate | Maintain |
| 25 | Windows and Linux embedded binary smoke pass | MANUAL_VERIFICATION_REQUIRED | Windows local smoke available | Linux clean-machine smoke pending |
| 26 | UI does not bypass runtime security | DONE | UI uses REST/WebSocket only | Continue review |
| 27 | ADR for Vue Flow vs migration | NOT_STARTED | No ADR yet | Milestone 6 |
| 28 | No large frontend migration without prototype/scorecard | DONE | Vue Flow retained | Revisit only at Milestone 6 |
| 29 | Docs reflect tested behavior | IN_PROGRESS | README, CHANGELOG, ROADMAP_PROGRESS, RELEASE updated with tested items | Update after each gate |
| 30 | No feasible code/test skipped as manual-only | DONE | Milestone 2 closure added feasible automated coverage | Re-evaluate in each milestone |

## Milestone 2 Automated Gate Evidence

- Passed: `cd ui && npm run test` - 7 files, 23 tests.
- Passed: `cd ui && npm run build`.
- Passed: `cd ui && npx playwright test --update-snapshots` - 8 tests, created six Chromium/Windows baselines.
- Passed: `cd ui && npm run test:e2e` - 8 Chromium tests.

- Passed: `go test ./...`.
- Passed: `go vet ./...`.
- Passed: `go build ./...`.
- Passed: `go build -trimpath -ldflags="-s -w" -o goflow.exe main.go static_embed.go`.
- Passed: `.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key`.
- Passed: `cd ui && $env:GOFLOW_E2E_BINARY = ".\goflow.exe"; npx playwright test tests/e2e/milestone2-closure.spec.js` - 3 Chromium tests against the compiled embedded binary.
