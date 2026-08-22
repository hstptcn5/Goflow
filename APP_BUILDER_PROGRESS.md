# Goflow App Builder Progress

This is the canonical implementation ledger for `GF-APP-001` through
`GF-APP-005`.

Status values: `PLANNED`, `IN_PROGRESS`, `DONE`, `BLOCKED`, `DEFERRED`.

## Current summary

The same-platform App Builder MVP is complete. All five checkpoints are merged
to `main`, automated verification is green, and a generated Windows executable
has passed an operator-driven build, credential-setup, and execution test.

| Checkpoint | Status | Evidence |
| :--- | :--- | :--- |
| `GF-APP-001` | `DONE` | `RunUI`, `RunField`, `Branding`, capability and manifest validation merged in PR #49. |
| `GF-APP-002` | `DONE` | Node-level Green/Yellow/Red analyzer with explicit destination requirements and unit coverage merged in PR #49; Python Code classification corrected to Yellow in PR #50. |
| `GF-APP-003` | `DONE` | Appended ZIP executable, inventory hash verification, bounded extraction and startup detection merged in PR #49. |
| `GF-APP-004` | `DONE` | Typed form coercion, webhook/direct input modes, selected-node output and cards/table/JSON views merged in PR #49; focused layout and structured AI output refinement merged in PR #50. |
| `GF-APP-005` | `DONE` | Editor Build App wizard, schema-derived fields, portability report, credential externalization and artifact download merged in PR #49; first-run credential slots without stored workflow IDs added in PR #50. |

## Merge and automated verification evidence

- PR #49 merged as `2ea7bdb522e291a456333d333a7ab451d3cc8cc2`.
- PR #49 exact-head CI #497, Technical Roadmap Smoke #26, Vietnam Morning Brief
  Windows Pilot #193 and Daily Business Report Public Beta #248: `success`.
- PR #50 merged as `3a285914372a5769fd3b0f52e240f24b804fbb1b`.
- PR #50 exact-head CI #505, Vietnam Morning Brief Windows Pilot #200 and Daily
  Business Report Public Beta #250: `success`.
- CI #505 includes backend tests, race test, vet, vulnerability scan, frontend
  tests/build, Playwright appliance E2E, Pack contracts and Windows/Linux/macOS
  artifact builds.

## Manual Windows acceptance evidence

Completed by the operator on 2026-08-22:

- exported the workflow as a Windows `.exe`;
- completed first-run AI credential setup;
- ran the generated application successfully;
- received structured AI Extract output;
- reviewed the refined focused layout and approved it after PR #50.

This closes the same-platform MVP acceptance gate. It does not claim signed,
installer-grade, or generally available production distribution.

## Remaining productization boundary

The next work belongs to product/release validation rather than
`GF-APP-001`–`GF-APP-005`:

1. repeat the generated-app test on a second clean Windows machine;
2. select one real workflow Pack for a sustained pilot;
3. collect setup failures, run failures, and user-facing output feedback;
4. decide installer, signing, update, entitlement, and commercial boundaries
   only after that pilot evidence exists.

Portability remains explicit:

- Green workflows can run with the embedded Goflow runtime.
- Yellow workflows still require listed network, credential, service, database,
  or external runtime setup on the destination machine.
- Python Code requires Python 3 because Python is not embedded.
- Red workflows remain blocked from App Builder.
