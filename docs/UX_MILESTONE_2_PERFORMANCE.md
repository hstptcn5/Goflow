# UX Milestone 2 Performance

Scope: editor usability foundation on Vue Flow with the embedded Go test server.

## Automated Smoke

Command:

```powershell
cd ui
npm run test:e2e
```

Observed Playwright output from the local Windows run:

| Graph size | Picker search smoke |
|---:|---:|
| 10 nodes | 660 ms |
| 50 nodes | 506 ms |
| 100 nodes | 766 ms |

The test creates real workflows through the REST API, opens each workflow in the editor, opens the node picker, searches for `http`, and verifies the HTTP Request result is visible.

## Checks

- Picker search does not show visible lag in Chromium during the smoke run.
- Auto-layout is explicit user action only.
- Auto-layout does not change node IDs.
- Undo history is capped at 60 graph snapshots.
- Execution status is not written into undo history; only graph nodes and edges are snapshotted.

## Remaining Manual Review

- Visual smoothness while dragging 100 nodes should be checked on a lower-end Windows machine.
- Memory growth over a long editing session should be profiled if large workflows become common.
