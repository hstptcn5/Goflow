# UX Editor Performance Smoke

Scope: editor usability foundation on Vue Flow with the embedded Go test server.

This is a regression smoke, not an absolute product benchmark. Results depend on the local machine, browser, power mode, antivirus, and background processes.

## Environment

- Date: 2026-07-26
- OS: Windows
- Browser: Playwright Chromium
- Server: embedded Goflow test server started by Playwright
- UI build: Vite production build through `npm run test:e2e`
- Viewport: 1366 x 768

## Command

```powershell
cd ui
npm run test:e2e
```

## Milestone 2 Observed Results

Latest local verification output:

| Graph size | Editor ready | Picker open | Picker search | Auto-layout |
|---:|---:|---:|---:|---:|
| 10 nodes | 677 ms | 92 ms | 71 ms | 96 ms |
| 50 nodes | 366 ms | 84 ms | 85 ms | 205 ms |
| 100 nodes | 561 ms | 145 ms | 109 ms | 316 ms |

The automated thresholds are intentionally broad:

- Editor ready: under 10 seconds.
- Picker open: under 3 seconds.
- Picker search: under 3 seconds.
- Auto-layout: under 3 seconds.

## What The Smoke Covers

- Creates real workflows through the REST API.
- Opens 10, 50, and 100-node workflows in the editor.
- Measures editor ready time separately from picker interactions.
- Opens the node picker and searches for `http`.
- Runs explicit auto-layout for each graph size.
- Confirms picker search remains responsive at 100 nodes.

## Milestone 3 Inspector Results

Latest local verification output:

| Fixture | Inspector open | Input JSON tree | Input search | Expression preview |
|---|---:|---:|---:|---:|
| 12 upstream JSON sources | 154 ms | 116 ms | 52 ms | 201 ms |

The automated thresholds are intentionally broad:

- Inspector open: under 3 seconds.
- JSON tree: under 3 seconds.
- Input search: under 3 seconds.
- Expression preview/data picker insert: under 3 seconds.

The Milestone 3 smoke creates real upstream JSON Transform nodes, runs the workflow, opens the selected node inspector, renders input data, searches a nested value, inserts a runtime placeholder expression, and confirms the preview updates.

## Remaining Manual Review

- Check drag/pan/zoom smoothness on a lower-end Windows machine.
- Profile memory growth during long editing sessions if large workflows become common.
- Add larger execution-output payload profiling if users commonly inspect outputs above the current truncation limit.
