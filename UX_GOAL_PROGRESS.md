# Goflow UX GOAL Progress

North Star: `UX_GOAL.md`. Runtime contract: `GOAL.md`.

Current UX status: `IN_PROGRESS`. Milestone 1 status: `DONE`.

Status values: `NOT_STARTED`, `IN_PROGRESS`, `BLOCKED`, `DONE`.

## Milestone 1 - UX audit and foundation

| Requirement | Status | Existing behavior | Implementation evidence | Automated test evidence | User-test evidence | Remaining work | Blocker |
|---|---|---|---|---|---|---|---|
| Audit current UI | DONE | UI was a single-screen editor with modal-based workflow, credential, and template management. | `docs/UX_AUDIT.md` documents inventory, journey, confusion points, reusable parts, and refactor targets. | Documentation review only. | Pending manual usability session. | Continue deeper usability audit in later milestones. | None |
| User journey map | DONE | User had to discover workflow, credentials, docs, save, run, and execution state from one overloaded navbar. | `docs/UX_AUDIT.md` includes before and after journeys. | Documentation review only. | Pending manual usability session. | Validate with target users. | None |
| Component inventory | DONE | Existing components were reusable but mixed page, modal, and editor responsibilities. | `docs/UX_AUDIT.md` lists current components and reuse decisions. | Documentation review only. | Pending manual usability session. | Keep inventory updated as editor changes. | None |
| Design token foundation | DONE | CSS used scattered colors, spacing, radii, and legacy variables. | `ui/src/assets/main.css` defines color, spacing, radius, shadow, typography, z-index, and layout tokens with backward-compatible aliases. | `cd ui && npm run build`; component tests. | Pending visual review on Windows browser. | Token usage still needs deeper cleanup in older components. | None |
| New app shell | DONE | `Navbar.vue` mixed global navigation and workflow actions; `MVP` label remained visible. | `ui/src/components/AppShell.vue`, `ui/src/router.js`, `ui/src/App.vue`. | `ui/tests/app-shell.test.js`; E2E navigation smoke. | Pending manual review. | Later milestones can tune density and keyboard navigation. | None |
| Remove overloaded navbar from primary workflow path | DONE | Run, Save, Canvas, Executions, Credentials, Docs, Workflows, Export, and connection status were in one row. | Workflow actions moved into `WorkflowEditor.vue` topbar; global navigation moved to `AppShell.vue`. | `ui/tests/workflow-editor.test.js` checks one primary action. | Pending manual review. | Older `Navbar.vue` remains in repo but is no longer used by `App.vue`. | None |
| Workflow page | DONE | Workflow list was opened through modal state. | `ui/src/pages/WorkflowsPage.vue` provides list, empty state, create, create from template, open, run. | E2E creates and opens workflow. | Pending manual review. | Add search/filter in a later UX milestone. | None |
| Workflow editor route | DONE | Editor had no direct URL route. | `ui/src/pages/WorkflowPage.vue` and `/workflows/:id` route. | E2E creates workflow and lands on `/workflows/:id`; embedded route refresh backend test. | Pending manual review. | Later editor usability work remains. | None |
| Execution page | DONE | Execution viewer existed as editor/modal state, not as a page. | `ui/src/pages/ExecutionsPage.vue` lists workflow-specific history and documents current global-list limitation. | E2E triggers workflow and opens `/executions`. | Pending manual review. | Global execution list endpoint remains future work. | None |
| Credential page | DONE | Credentials were managed through modal state. | `ui/src/pages/CredentialsPage.vue` lists, creates, deletes credentials without plaintext secret display. | E2E navigates to `/credentials`. | Pending manual review. | Add edit/test-credential actions later. | None |
| Supporting pages | DONE | Templates, nodes, settings, and help did not have direct routes. | `ui/src/pages/TemplatesPage.vue`, `NodesPage.vue`, `SettingsPage.vue`, `HelpPage.vue`. | Route rendering covered by build and router tests. | Pending manual review. | Add deeper page tests later. | None |
| Dirty/saved state | DONE | Dirty state was a boolean and not consistently surfaced. | `workflowStore.saveState`, editor topbar states, route leave guard. | `ui/tests/workflow-store.test.js`, `ui/tests/workflow-editor.test.js`, E2E Unsaved -> Saved. | Pending manual review. | Undo/redo and field-level validation are Milestone 2. | None |
| Standard loading/empty/error states | DONE | Failures often used `alert()` or console-only errors. | `StateBlock.vue` plus page-level loading, empty, and error states. | `ui/tests/pages.test.js`. | Pending manual review. | Apply to all remaining legacy editor internals later. | None |
| Frontend unit/component test foundation | DONE | Frontend only had build verification. | Vitest config, setup, router mount helper, focused tests. | `cd ui && npm run test` passes. | Not applicable. | Expand with later UX features. | None |
| E2E smoke foundation | DONE | No browser-level UX smoke test existed. | Playwright config, temp backend launcher, Milestone 1 smoke spec. | `cd ui && npm run test:e2e` passes. | Pending manual clean-machine run. | Add cross-browser/mobile later. | None |
| Embedded frontend route refresh | DONE | Direct frontend routes could miss SPA fallback. | `internal/api/router.go` serves real assets when present and rewrites unknown frontend routes to `index.html`. | `TestRouterServesSPAIndexForFrontendRoutes`; E2E starts embedded Go server. | Pending manual binary refresh check. | None. | None |

## Milestone 2 - Editor usability

| Requirement | Status | Existing behavior | Implementation evidence | Automated test evidence | User-test evidence | Remaining work | Blocker |
|---|---|---|---|---|---|---|---|
| Node picker with search and keyboard | DONE | Fixed palette was the primary add path. | `ui/src/components/NodePicker.vue` opens from Add step, empty canvas, quick-add, double-click canvas, and `A`/`Ctrl+K`; supports search, Arrow Up/Down, Enter, Escape, recent, favorites, credentials and experimental badges. | `ui/tests/node-picker.test.js`, `ui/tests/e2e/milestone2.spec.js`. | Pending tester timing. | None. | None |
| Quick-add flow | DONE | Users manually dragged and connected nodes. | Node card `+` opens picker with source context; added node is placed to the right, connected, selected, and marks dirty while preventing duplicate source-target edge. | `ui/tests/workflow-editor.test.js`, E2E Milestone 2 flow. | Pending manual review. | Edge insertion quick-add not implemented because current edge template would require broader edge rendering changes. | None |
| Palette no longer mandatory | DONE | Palette always occupied fixed width. | Palette is hidden by default and available through More actions; Add step is primary. | E2E adds nodes without using palette. | Pending manual review at 1366x768. | Later cleanup can remove or further collapse legacy palette. | None |
| Node card clarity | DONE | Card emphasized internal type. | Node card now shows friendly node name, operation summary, configuration state, execution state text, and only secondary category text. | Component and E2E tests assert validation/status text. | Pending manual visual review. | More node-specific operation summaries can be added later. | None |
| Canvas visual cleanup | DONE | Category colors were prominent and handles were small. | Idle edges are neutral and not animated; background is quieter; selected state is restrained; IF handles show true/false labels; handles are larger. | `workflow-editor-utils.test.js`, E2E visual smoke. | Pending manual visual review. | Execution-related edge animation can be expanded in debugger milestone. | None |
| Pre-run validation | DONE | Backend validation existed; frontend had no focused summary. | `validateWorkflowGraph` checks missing required param, missing credential, unknown node, duplicate ID, invalid edge reference, and cycles; summary can focus node issue; Test Workflow blocks invalid graphs. | `workflow-editor-utils.test.js`, `workflow-editor.test.js`, E2E Milestone 2 validation flow. | Pending manual review. | Backend remains source of truth. | None |
| Dirty/save integration | DONE | Dirty state covered some actions. | Add, quick-add, delete, move, connect, delete edge, duplicate, undo, redo, auto-layout, rename, params, AI/template load all mark dirty. | `workflow-editor.test.js`, E2E Unsaved/Saved flow. | Pending manual review. | Fine-grained saved snapshot comparison can be improved later. | None |
| Undo/redo | DONE | Not implemented. | Graph snapshot stacks with 60-entry limit; supports add/delete/connect/move/params/rename/quick-add/duplicate/auto-layout through shared mutation helpers and shortcuts. | `workflow-editor.test.js`, E2E keyboard and editor flow. | Pending manual review. | Multi-select batch operations can be improved later. | None |
| Duplicate/copy/paste/delete shortcuts | DONE | Delete existed only through properties panel. | `Ctrl+D`, `Ctrl+C`, `Ctrl+V`, Delete/Backspace implemented without firing while typing in fields; pasted node gets a new ID and offset position. | `workflow-editor.test.js`, E2E keyboard smoke. | Pending manual review. | Select-all depends on Vue Flow selection stability and remains later work. | None |
| Auto-layout | DONE | No explicit layout command. | Lightweight left-to-right DAG layout keeps IDs/edges, supports branching buckets, runs only on user action, and is undoable. | `workflow-editor-utils.test.js`, `workflow-editor.test.js`, E2E flow. | Pending manual review. | Larger layout polish can be revisited if workflows grow. | None |
| Empty canvas onboarding | DONE | Empty state offered template/AI and a fixed Delay shortcut. | Empty state now offers Add first step, Browse templates, Use AI assistant; Add first step opens picker. | E2E Milestone 1 and 2. | Pending manual review. | None. | None |
| Keyboard shortcuts | DONE | No central editor shortcuts. | `A`/`Ctrl+K`, Delete/Backspace, `Ctrl+Z`, `Ctrl+Shift+Z`/`Ctrl+Y`, `Ctrl+S`, `Ctrl+D`, Escape implemented with input guard. | `workflow-editor.test.js`, E2E keyboard smoke. | Pending manual review. | Shortcut help page can be richer. | None |
| Accessibility | DONE | Some form controls lacked accessible names. | Node picker uses dialog/listbox/options, focus, Escape; node cards include validation aria label; properties inputs have aria labels; state text is not color-only. | Node picker/component/E2E tests. | Pending manual screen-reader pass. | Full a11y audit remains manual. | None |
| Automated tests | DONE | Milestone 1 tests existed. | Added utility, picker, editor, E2E editor, keyboard, visual screenshot smoke, and performance smoke tests. | `npm run test` and `npm run test:e2e`. | Not applicable. | None. | None |
| Visual regression smoke | DONE | No screenshot coverage. | E2E captures blank canvas, node picker, and empty search result at 1366x768 and asserts nonblank screenshot output. | `ui/tests/e2e/milestone2.spec.js`. | Pending human visual review. | Snapshot baselines intentionally not introduced yet to avoid brittle false positives. | None |
| Performance smoke | DONE | No editor scale smoke. | `ui/tests/e2e/performance.spec.js` opens 10, 50, and 100-node workflows and measures picker search. | `docs/UX_MILESTONE_2_PERFORMANCE.md`. | Pending lower-end machine check. | None. | None |

Milestone 2 automated gate evidence:

- Passed: `go test ./...`
- Passed: `go vet ./...`
- Passed: `go build ./...`
- Passed: `cd ui && npm ci`
- Passed: `cd ui && npm run build`
- Passed: `cd ui && npm run test` with 7 files and 16 tests
- Passed: `cd ui && npm run test:e2e` with 5 Chromium tests
- Passed: embedded binary UX smoke for route refresh, add node, save, reload, run workflow, and WebSocket status
- Passed: `.\scripts\goal-smoke-test.ps1 -Binary .\goflow.exe -Port 18080 -AdminKey goal-admin-key`

## Final UX Definition of Done snapshot

| Definition of Done | Status | Evidence | Remaining work |
|---|---|---|---|
| 1. Clear app shell information architecture | DONE | `AppShell.vue`, route tests, E2E nav smoke. | Continue usability tuning. |
| 2. Workflows, Executions, Credentials not modal-only | DONE | Dedicated pages and routes. | Add deeper page actions later. |
| 3. Node picker search/category/keyboard/quick-add | DONE | `NodePicker.vue`, Add step, quick-add, keyboard tests, E2E. | User timing test still pending. |
| 4. Fixed node palette no longer mandatory | DONE | Palette hidden by default; Add step and quick-add are primary. | Later cleanup can remove legacy palette. |
| 5. Canvas visual noise reduced | DONE | Quieter background, neutral idle edges, restrained selected state. | Manual visual review pending. |
| 6. Edge animates only for execution-related state | DONE | Idle and generated edges use `animated: false`. | Add explicit execution animation later if needed. |
| 7. Node card config/execution state clear | DONE | Card shows name, operation summary, config state, execution state text. | More node summaries can be added later. |
| 8. Workflow dirty/saved state | DONE | Store save state and editor topbar. | Undo/redo remains separate. |
| 9. Undo/redo | DONE | Snapshot undo/redo with shortcuts and tests. | Multi-select polish later. |
| 10. Validation before run | DONE | Frontend graph validation blocks Test Workflow and focuses node issue. | Backend remains source of truth. |
| 11. Inspector Parameters/Input/Output/Logs | IN_PROGRESS | Config and live output tabs exist. | Milestone 3. |
| 12. Inline required/credential errors | IN_PROGRESS | Some node config metadata shown. | Milestone 3. |
| 13. UI data mapping between nodes | IN_PROGRESS | Existing dynamic data picker exists. | Milestone 3 hardening. |
| 14. Expression preview | NOT_STARTED | Not implemented. | Milestone 3. |
| 15. Execution selector | IN_PROGRESS | Executions page has workflow selector. | Milestone 4 execution selector in editor/debugger. |
| 16. Failed path highlight | IN_PROGRESS | Failed node status is visible. | Milestone 4 canvas path highlight. |
| 17. Retry/replay full workflow | NOT_STARTED | Not implemented. | Milestone 4. |
| 18. Redacted debug bundle | NOT_STARTED | Runtime redaction exists; bundle UI does not. | Milestone 4. |
| 19. Identify failed node under 2 minutes | IN_PROGRESS | Node failure output panel exists; no usability proof yet. | Milestone 4/5 user test. |
| 20. First workflow under 10 minutes | IN_PROGRESS | App shell, workflow page, templates, and E2E creation path exist. | User testing required. |
| 21. 4/5 usability testers complete task | NOT_STARTED | No user-test evidence yet. | Manual usability test. |
| 22. Frontend component tests pass | DONE | `npm run test` covers Milestone 1 and 2. | Maintain coverage. |
| 23. E2E tests pass | DONE | `npm run test:e2e` covers Milestone 1 and 2 editor smoke. | Add debugger scenarios later. |
| 24. Backend tests no regression | DONE | `go test ./...` in final gate. | Maintain. |
| 25. Windows and Linux embedded smoke pass | IN_PROGRESS | Windows local/E2E embedded smoke covered. | Linux clean-machine smoke pending. |
| 26. UI does not bypass runtime security | DONE | UI uses REST API/WebSocket; no direct DB/engine access. | Continue enforcing via reviews. |
| 27. ADR for Vue Flow vs migration | NOT_STARTED | No ADR yet. | Milestone 6. |
| 28. No large frontend migration without prototype/scorecard | DONE | This milestone keeps Vue Flow. | Milestone 6 if migration considered. |
| 29. Docs reflect tested behavior | IN_PROGRESS | Milestone 1 and 2 docs updated after implementation. | Final verification results must remain current. |
| 30. No feasible code/test skipped as manual-only | IN_PROGRESS | Milestone 1 and 2 feasible automated tests added. | Later milestones still open. |
