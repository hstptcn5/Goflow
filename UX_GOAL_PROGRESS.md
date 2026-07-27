# Goflow UX GOAL Progress

North Star: `UX_GOAL.md`. Runtime contract: `GOAL.md`.

Current UX status: `IN_PROGRESS`.
Milestone 1 status: `DONE`.
Milestone 2 status: `MANUAL_VERIFICATION_REQUIRED`.
Milestone 3 status: `DONE` for automated closure and hardening; manual usability evidence remains `MANUAL_VERIFICATION_REQUIRED`.
Milestone 4 status: `DONE` for automated execution debugging closure after integration hardening; manual usability evidence remains `MANUAL_VERIFICATION_REQUIRED`.

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

## Milestone 3 - Inspector, Data Mapping, And Expression Preview

| Requirement | Status | Implementation evidence | Test evidence | Manual evidence | Remaining work |
|---|---|---|---|---|---|
| Parameters/Input/Output/Logs inspector tabs | DONE | `ui/src/components/PropertiesPanel.vue` | `ui/tests/properties-panel.test.js`, `ui/tests/e2e/milestone3-inspector.spec.js` | Local browser smoke | Full execution selector remains Milestone 4 |
| Inline field validation | DONE | `validateParamValue()` and field-level messages in `PropertiesPanel.vue` | `ui/tests/inspector-utils.test.js`, `properties-panel.test.js`, Milestone 3 E2E invalid expression/required screenshots | Local browser smoke | Backend remains final source of truth |
| Credential missing action and compatible filtering | DONE | `credentialsForParam()` and `Create credential` action | Component tests and visual baseline `m3-missing-credential` | Local browser smoke | Credential creation remains Credentials page |
| Advanced options collapse | DONE | `showAdvanced` advanced parameter group | Component/render coverage through inspector tests | Local browser smoke | More node metadata can improve classification |
| Input JSON tree/table/raw/search/copy/source selector | DONE | `buildJsonTree()`, `rowsForTable()`, Input tab | Utility tests, E2E data mapping and visual baseline `m3-input-json-tree` | Local browser smoke | Large payload virtualization beyond truncation remains later |
| Data mapping picker | DONE | Data picker uses upstream execution outputs and trigger payload; click inserts `{{node.path}}` expression | Component test and E2E data mapping save/reload | Local browser smoke | Drag/drop mapping not implemented |
| Fixed/Expression mode | DONE | Temporary component state keeps Fixed/Expression state clear without writing UI metadata; Expression values force Expression mode; number/integer fields render text inputs in Expression mode and numeric inputs in Fixed mode; Expression to Fixed converts safe primitive preview to literal; JSON object/array previews require explicit JSON literal conversion | `ui/tests/properties-panel.test.js`, `ui/tests/e2e/milestone3-inspector.spec.js` number expression and object/array conversion tests | Local browser smoke | None |
| Expression preview success/error | DONE | `resolveExpression()` frontend preview using runtime placeholder contract | Utility tests, E2E preview success and invalid reference flow | Local browser smoke | No backend preview endpoint yet |
| Output JSON/table/raw/copy/download | DONE | Output tab redacted tree/table/raw plus copy/download controls | Component tests, E2E output/log flow, visual baseline `m3-output-json` | Local browser smoke | Download is browser-only smoke covered visually |
| Logs status/error/attempts/execution ID | DONE | Logs tab with status, duration, attempts, execution ID, error, raw redacted log DTO | Component tests, E2E logs flow, visual baseline `m3-logs-error` | Local browser smoke | Redacted resolved parameters await backend storage |
| Execution/sample context clarity | DONE | Inspector context label shows latest/live/no execution | Component and E2E coverage | Local browser smoke | Full selector remains Milestone 4 |
| Security/redaction in inspector display | DONE | Server-redacted execution inspector DTO plus frontend display redaction defense in depth | `internal/api/execution_handler_test.go`, `ui/tests/properties-panel.test.js`, Milestone 3 E2E server DTO/redacted error tests | Local browser smoke | Backend remains source of truth |
| Dirty valid Activate saves before activation | DONE | `toggleWorkflowActive()` saves dirty graph before activating | `workflow-editor.test.js`, Milestone 3 E2E | Local browser smoke | None |
| ADR for expression/mapping model | DONE | `docs/ADR_EXPRESSION_AND_MAPPING_MODEL.md` | Documentation review | Not required | Revisit if backend preview endpoint is added |
| Visual regression baselines | DONE | Seven M3 screenshot baselines | `npm run test:e2e` | Local visual review | Cross-platform baselines only if CI adds screenshot tests |
| Inspector performance smoke | DONE | `ui/tests/e2e/performance.spec.js` second smoke | `npm run test:e2e` logs inspector timings | Local Windows Chromium run | Low-end Windows profiling remains manual |
| Runtime expression parity | DONE | Backend exposes `$trigger` in shared execution context, logs skipped branch nodes, and preserves complete-expression runtime types for string, number, boolean, object, array, and `$trigger` values | `internal/engine/engine_test.go`, Milestone 3 E2E branch parity | Local browser smoke | None |
| Transitive upstream picker | DONE | Data picker uses graph ancestor traversal shared with validation | Component test and Milestone 3 E2E A -> B -> C coverage | Local browser smoke | Drag/drop mapping not implemented |
| Tab accessibility automation | DONE | Inspector tabs use tablist/tab/tabpanel semantics, roving focus, Arrow/Home/End handling | Component keyboard test | Manual screen-reader remains pending | NVDA/Narrator pass |
| Usability timing for mapping data | MANUAL_VERIFICATION_REQUIRED | Automated flow exists | E2E mapping passes | Pending target user timing | Run UX_GOAL tester tasks |
| Screen-reader pass for inspector | MANUAL_VERIFICATION_REQUIRED | Labels and tabs implemented | Component/E2E focus/label coverage | Pending NVDA/Narrator pass | Manual accessibility pass |

## Milestone 4 - Execution Debugging

| Requirement | Status | Implementation evidence | Test evidence | Manual evidence | Remaining work |
|---|---|---|---|---|---|
| Execution selector in editor | DONE | `ui/src/components/WorkflowEditor.vue` execution selector bound to workflow execution history | `ui/tests/e2e/milestone4-debugging.spec.js` | Local browser smoke | Manual screen-reader pass |
| Canvas execution overlay | DONE | Node status/duration badges and route-scoped selected/latest/live execution state mapping in `WorkflowEditor.vue`; workflow switches clear stale live/history state | `ui/tests/e2e/milestone4-debugging.spec.js` cross-workflow live isolation assertion | Local browser smoke | Manual visual review on low-end Windows |
| Failed-path highlight | DONE | `buildExecutionPathState()` traverses execution logs and graph edges to distinguish failed ancestry, successful, skipped, running, and not-run paths | `ui/tests/workflow-editor-utils.test.js`, `ui/tests/e2e/milestone4-debugging.spec.js` multi-hop branch path assertion | Local browser smoke | Broader visual baselines can be added later |
| Retry selected execution | DONE | `retryFullWorkflow()` delegates to replay and uses stored input from the selected failed/cancelled execution; Test Workflow remains the current-empty-input path | `ui/tests/e2e/milestone4-debugging.spec.js` required-input retry assertion | Local browser smoke | None |
| Replay execution | DONE | `POST /api/v1/executions/{id}/replay` uses `TriggerService` with stored raw input, authenticated principal, request ID, current workflow definition, scoped allowlist, and async UI source | `internal/api/execution_handler_test.go`, `ui/tests/e2e/milestone4-debugging.spec.js` replay assertion | Local browser smoke | None |
| Cancel execution | DONE | Editor Cancel action calls the REST cancellation API only for the displayed running/queued execution; hidden running executions do not enable Cancel while latest/selected terminal execution is shown; button labels target the short execution ID | `ui/tests/e2e/milestone4-debugging.spec.js` exact-target cancellation assertion with concurrent executions | Local browser smoke | None |
| Redacted debug bundle | DONE | Workflow params, selected execution, node logs, validation issues, nested JSON strings, URL userinfo, webhook URLs, query tokens, and token formats are redacted; preview can truncate as valid JSON while copy/export returns full valid JSON | `ui/tests/inspector-utils.test.js`, `ui/tests/e2e/milestone4-debugging.spec.js` large-bundle parse/no-secret assertion | Local browser smoke | None |
| Contextual error actions | DONE | Logs tab shows Open Parameters and Copy error for failed node errors | `ui/tests/e2e/milestone4-debugging.spec.js` | Local browser smoke | Manual failed-node diagnosis timing |

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
| 11 | Inspector Parameters/Input/Output/Logs | DONE | Four inspector tabs with E2E output/log coverage | Full execution selector later |
| 12 | Required field and credential inline errors | DONE | Field-level validation and credential action tests | Manual screen-reader pass pending separately |
| 13 | Data mapping between nodes via UI | DONE | Upstream data picker inserts runtime placeholder expressions and save/reload E2E passes | Timed user test pending separately |
| 14 | Expression preview | DONE | Preview success/error tests and ADR | Backend preview endpoint not added |
| 15 | Execution selector | DONE | Editor execution selector and E2E M4 coverage | Manual screen-reader pass pending separately |
| 16 | Failed path highlight | DONE | Canvas node/edge execution overlay and E2E failed-path assertion | Broader visual baselines later |
| 17 | Retry/replay full workflow | DONE | Retry selected execution and replay both use stored execution input through TriggerService with current workflow definition | None |
| 18 | Redacted debug bundle | DONE | Editor debug bundle preview/copy redacts execution, params, nested JSON strings, URLs, webhook URLs, query tokens, and common token formats while keeping copied JSON parseable | None |
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
| 29 | Docs reflect tested behavior | DONE | README, CHANGELOG, ROADMAP_PROGRESS, RELEASE, ADR updated with tested items | Update after each future gate |
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

## Milestone 3 Automated Gate Evidence

- Passed before implementation: `cd ui && npm run test`; `cd ui && npm run test:e2e`; `go test ./...`; `go vet ./...`; `go build ./...`.
- Passed: `cd ui && npm run test` - 9 files, 38 tests.
- Passed: `cd ui && npx playwright test --update-snapshots` - 13 tests during baseline generation.
- Passed: `cd ui && npm run test:e2e` - 18 Chromium tests.
- Passed closure focused gate: `cd ui && npx playwright test tests/e2e/milestone3-inspector.spec.js` - 9 Chromium tests.
- Latest inspector performance smoke: `{"upstreamCount":12,"inspectorOpenMs":187,"inputTreeMs":142,"inputSearchMs":55,"expressionPreviewMs":248}`.
- Milestone 3 final patch added: number/integer Expression mode text input, primitive conversion back to Fixed numeric input, object/array Expression retention, explicit JSON literal conversion, and backend runtime type parity coverage.
- Passed focused final patch gates: `cd ui && npm run test -- properties-panel` - 11 tests; `go test ./internal/engine`; `cd ui && npm run build`; `cd ui && npx playwright test tests/e2e/milestone3-inspector.spec.js -g "number expression|object and array"` - 2 Chromium tests.

## Milestone 4 Automated Gate Evidence

- Milestone 4 integration hardening added: integer-safe delay runtime parsing for string/int/int32/int64/float32/float64/json.Number, active execution context isolation for live/latest/selected execution views, inspector status sourced from the same execution result as Input/Output/Logs, workflow-scoped live WebSocket state, exact-target cancellation behavior, multi-hop failed-path traversal, replay security/status-code coverage, retry-selected-input behavior, hardened debug bundle redaction, large valid JSON bundle copy, dynamic-port custom Playwright E2E runner with occupied-port smoke coverage, and stable screenshot readiness waits.
- Passed focused hardening gates: `go test ./internal/api ./internal/engine ./internal/nodes`; `cd ui && npm run test -- --run tests/workflow-editor-utils.test.js tests/inspector-utils.test.js`; `cd ui && npm run build`; `cd ui && node tests/run-e2e.mjs tests/e2e/milestone4-debugging.spec.js --reporter=line --timeout=45000` - 3 Chromium tests.
- Passed: `cd ui && npm ci` - 0 vulnerabilities.
- Passed: `cd ui && npm run build`.
- Passed: `cd ui && npm run test` - 10 files, 46 tests.
- Passed: `cd ui && npm run test:e2e:runner` - occupied-port runner smoke.
- Passed: `cd ui && npm run test:e2e` - 24 Chromium tests via `tests/run-e2e.mjs`, deterministic with one worker, dynamic temp port, API-key readiness, and Windows process-tree cleanup.
- Passed: `go test ./...`.
- Passed: `go vet ./...`.
- Passed: `go build ./...`.
- Passed: `go build -trimpath -ldflags="-s -w" -o goflow.exe main.go static_embed.go`.
- Passed: `.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key`.
- Passed embedded binary UX gate: `cd ui; $env:GOFLOW_E2E_BINARY = "goflow.exe"; node tests/run-e2e.mjs tests/e2e/milestone2-closure.spec.js tests/e2e/milestone2.spec.js tests/e2e/milestone3-inspector.spec.js tests/e2e/milestone4-debugging.spec.js --reporter=line --timeout=45000` - 21 Chromium tests.
