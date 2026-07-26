# UX Milestone 2 Performance Smoke

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

## Observed Results

Latest local verification output:

| Graph size | Editor ready | Picker open | Picker search | Auto-layout |
|---:|---:|---:|---:|---:|
| 10 nodes | 578 ms | 77 ms | 77 ms | 85 ms |
| 50 nodes | 346 ms | 78 ms | 76 ms | 198 ms |
| 100 nodes | 511 ms | 136 ms | 110 ms | 331 ms |

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

## Remaining Manual Review

- Check drag/pan/zoom smoothness on a lower-end Windows machine.
- Profile memory growth during long editing sessions if large workflows become common.
- Add a large execution-output payload smoke in Milestone 3 when the JSON viewer is hardened.
