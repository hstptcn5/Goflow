# Goflow UX Audit - Milestone 1

Scope: current Vue UI in `ui/`, with `GOAL.md` preserved as the runtime contract.

## Existing Component Inventory

| Area | Component or file | Current role | Reuse decision |
|---|---|---|---|
| App bootstrap | `ui/src/App.vue` | Previously owned top-level modal/editor state. | Refactored into route bootstrap and app shell host. |
| Global navigation | `ui/src/components/Navbar.vue` | Overloaded app navigation and editor commands. | Removed from primary app path; retained until later cleanup confirms no references. |
| Editor | `ui/src/components/WorkflowEditor.vue` | Canvas, node actions, save/run, templates, AI assistant. | Reused with a clearer workflow topbar and route wrapper. |
| Node library | `ui/src/components/NodePalette.vue` | Fixed palette with categories and drag/drop. | Reused for Milestone 1; search/keyboard/quick-add belong to Milestone 2. |
| Node settings | `ui/src/components/PropertiesPanel.vue` | Node configuration and live output. | Reused; z-index/top offset fixed so it does not block the workflow topbar. |
| Execution detail | `ui/src/components/ExecutionViewer.vue` | Execution inspection. | Reused indirectly; page-level history added separately. |
| Templates | `ui/src/components/TemplateGallery.vue` | Template chooser modal. | Reused from Workflows/Templates pages and editor more actions. |
| Credentials | `ui/src/components/CredentialModal.vue` | Create credential modal. | Reused by Credentials page; no plaintext secret list added. |
| AI | `ui/src/components/AIAssistantDrawer.vue` | Workflow assistant drawer. | Reused; not expanded in Milestone 1. |
| Stores | `workflowStore.js`, `executionStore.js` | API state, workflow state, execution state. | Extended with save states and explicit error states. |
| API/WebSocket | `api.js`, `websocket.js` | REST and live updates. | Reused; WebSocket now exposes reactive connection state. |

## Previous User Journey

1. Open root app.
2. Work from a single editor screen.
3. Open workflow manager, credentials, templates, docs, executions, run, save, and export from one top navbar.
4. Use modals for primary navigation.
5. Infer save state from button behavior and alerts.
6. Inspect failures through node status/output or browser alert messages.

Confusion points:

- Workflows, credentials, templates, docs, and execution history looked like temporary panels instead of durable product areas.
- The top navbar mixed global navigation with destructive/editor actions.
- The `MVP` label reduced confidence in the product.
- Direct refresh on a frontend route was not guaranteed by the embedded server.
- Failed network or API states often relied on `alert()` or console output.
- The properties panel could overlap and block editor actions.

## Milestone 1 Target Journey

1. Open `/workflows`.
2. Create a blank workflow or start from a template.
3. Land on `/workflows/:id`.
4. See workflow name, saved/unsaved state, active state, one primary action, Save, and secondary actions.
5. Add or edit a node and see `Unsaved changes`.
6. Save and see `Saved`.
7. Activate and test the workflow.
8. Land on `/executions` to inspect history.
9. Navigate to `/credentials` without losing app context.

## Refactor Decisions

- Keep Vue 3, Pinia, Vite, Vue Flow, REST API, and WebSocket.
- Add Vue Router for durable product routes.
- Do not add a frontend backend or direct SQLite access.
- Do not create a separate execution path for UI.
- Introduce `AppShell.vue` and page components instead of rewriting editor internals.
- Introduce design tokens while keeping legacy variable aliases for existing components.
- Use automated component and E2E tests as the Milestone 1 regression gate.

## Known Limitations After Milestone 1

- `Navbar.vue` remains in the repository as unused legacy code until a later cleanup pass.
- Node picker search, keyboard navigation, auto-layout, undo/redo, validation badges, and quick-add are Milestone 2.
- Inspector parameters/input/output/logs, expression preview, and mapping hardening are Milestone 3.
- Full execution debugger, failed path highlight, retry/replay, and debug bundle are Milestone 4.
- Linux clean-machine smoke still needs validation outside this Windows session or through GitHub Actions artifacts.

