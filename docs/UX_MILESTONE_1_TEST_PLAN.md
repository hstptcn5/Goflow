# UX Milestone 1 Test Plan

This test plan validates the UX foundation without changing Goflow's runtime contract.

## Automated Gate

Backend:

```powershell
go test ./...
go vet ./...
go build ./...
```

Frontend:

```powershell
cd ui
npm ci
npm run build
npm run test
npm run test:e2e
```

Embedded route refresh:

1. Build frontend with `npm run build`.
2. Start the Go server from the repository root.
3. Open `http://127.0.0.1:<port>/workflows`.
4. Open `http://127.0.0.1:<port>/workflows/test-refresh` directly.
5. Confirm both return the Vue app shell, not a backend 404.

## Milestone 1 Browser Smoke

1. Open `/workflows`.
2. Confirm the sidebar shows Workflows, Executions, Credentials, Templates, Nodes, Settings, Help.
3. Create a blank workflow.
4. Confirm the URL changes to `/workflows/:id`.
5. Add a delay step from the empty canvas action.
6. Confirm the topbar shows `Unsaved changes`.
7. Save the workflow and confirm it shows `Saved`.
8. Activate the workflow.
9. Click `Test Workflow`.
10. Confirm the app opens `/executions`.
11. Open `/credentials`.
12. Confirm credentials are listed without plaintext secret values.

## Manual Windows Checks

Use a packaged archive or clean extracted directory:

```powershell
.\goflow.exe
```

Then verify:

- Root app loads.
- Direct route refresh works for `/workflows`, `/executions`, and `/credentials`.
- New workflow creation works.
- Save state changes from `Unsaved changes` to `Saved`.
- Inactive workflows fail visibly instead of silently.
- Active test run opens execution history.
- Credentials can be created and deleted.
- Browser console has no startup error.

## Out of Scope For Milestone 1

- Node picker search and keyboard navigation.
- Undo/redo.
- Auto-layout.
- Full pre-run node validation UI.
- Expression preview.
- Retry/replay/debug bundle.
- Vue Flow replacement decision.

