# Goflow Productization Progress

Overall status: IN_PROGRESS

Allowed checkpoint states: NOT_STARTED, IN_PROGRESS, BLOCKED,
MANUAL_GATE_REQUIRED, DONE.

## Input And Safety Baseline

Date: 2026-08-13

- Repository: `hstptcn5/Goflow`.
- PR #14: draft, open, unmerged, mergeable, no review comments returned.
- PR #14 expected and observed head:
  `417f741fddf9cf9cbe0bff3b0890ea6865fd0b27`.
- Expected and observed `main`:
  `198aad124d5a535084a3e6d9806f70c7879b091c`.
- Expected main is an ancestor of PR #14 head.
- PR #14 CI run `31509466621`: completed successfully.
- Phase 1 branch was created from the exact PR #14 head and will be stacked on
  `feature/goflow-pilot-ux-hardening` while PR #14 remains open.
- Tracked sensitive/binary candidate scan: no database, key, environment,
  executable, installer, or archive candidates; no tracked file over 5 MB.
- Untracked user files intentionally preserved and excluded:
  `GOFLOW_ECOSYSTEM_MASTER_GOAL.md`,
  `GOFLOW_PRODUCTIZATION_MASTER_GOAL.md`, and
  `roseblade-ranger.audit.html`.

Baseline verification:

- `go test -count=1 ./...`: PASS.
- `npm run test`: PASS, 11 files and 56 tests.

## Checkpoint A: Baseline, Architecture, Scheduler Contract

Status: DONE

Branch: `feature/goflow-productization-scheduler`

Pull request: `https://github.com/hstptcn5/Goflow/pull/15`, draft/open/unmerged,
based on `feature/goflow-pilot-ux-hardening`.

Design decisions:

- Keep `TriggerService` as the only workflow start path.
- Store one versioned daily schedule per managed workflow in SQLite.
- Default disabled; IANA timezone; no appliance raw cron; missed runs skip.
- Use an injected clock and deterministic per-instant idempotency.
- Preserve advanced workflow cron behavior independently.

Files:

- `docs/productization/PLAN.md`
- `docs/productization/PROGRESS.md`
- `docs/productization/DECISIONS.md`
- `docs/productization/THREAT_MODEL.md`
- `docs/productization/POST_BETA_ROADMAP.md`

Checkpoint design commit:
`81349053070f3b6718386103dd48d2a8c2fc7c0f`.

CI evidence:

- Exact verified head: `9dbb23fc933300bf43c3101b267985ecbd028633`.
- GitHub Actions run: `31707818103`.
- Conclusion: SUCCESS.
- Passed jobs include backend tests/race/vet/vulnerability, frontend test/build,
  Pack contracts and deterministic scan, appliance Playwright/DailyOps/runner,
  five platform builds, and native Windows DailyOps artifact verification.

Checkpoint verification:

- `go test -count=1 ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `npm run test`: PASS, 11 files and 56 tests.
- `npm run build`: PASS.
- `npm audit --audit-level=moderate`: PASS, 0 vulnerabilities.

Known limitations:

- No scheduler runtime, persistence migration, API, or UI exists yet.
- Local Windows host does not provide the C compiler needed for Go race tests;
  race remains a required GitHub Actions gate.

Manual/external gates: none for Checkpoint A.

## Checkpoint B: Scheduler Backend MVP

Status: DONE

Branch and draft PR remain the Phase 1 branch and PR #15.

Implementation decisions:

- Add transactional SQLite migration `0005_workflow_schedules` with one row per
  managed workflow and optimistic `revision` updates.
- Keep a closed daily/IANA/skip schema and fail corrupt or future rows closed.
- Add a deterministic calendar calculator with explicit DST gap/fold behavior.
- Use a one-minute due grace; older missed periods skip to the next future run.
- Call `TriggerService` with a digest-based scheduled-instant idempotency key;
  only advance schedule metadata after execution admission.
- Start the managed scheduler only in appliance `serverapp`; secondary Pack Run
  launches continue to reuse the primary process.

Primary files:

- `internal/storage/schema.go`
- `internal/storage/workflow_schedule_store.go`
- `internal/scheduler/calculator.go`
- `internal/scheduler/service.go`
- `internal/application/trigger_service.go`
- `internal/api/appliance.go`
- `internal/serverapp/serverapp.go`

Verification:

- Scoped storage/scheduler/application/API/serverapp/Pack Run tests: PASS.
- `go test -count=1 ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- Local race test remains unavailable on this host; GitHub Actions race is the
  required gate. Explicit attempts failed because CGO is disabled by default and
  no `gcc` C compiler is installed when CGO is enabled.

Checkpoint implementation commits:
`cf056d6` (`feat(scheduler): add deterministic appliance backend`) and
`1ae18b4b1318ccc8ab5e10bb7fdb27ce99c53225`
(`docs(productization): record scheduler backend evidence`).

Final CI: GitHub Actions run `31710784254` completed successfully on exact head
`1ae18b4b1318ccc8ab5e10bb7fdb27ce99c53225`, including backend race/vet/tests,
frontend, Playwright, Pack/DailyOps contracts, multi-platform builds, and the
native Windows DailyOps pilot appliance E2E.

## Checkpoint C: Scheduler Appliance UX And DailyOps

Status: DONE

Implemented:

- Add a host-guarded appliance schedule GET/PUT resource with strict daily
  time/IANA timezone validation and optimistic revision conflicts.
- Keep the schedule disabled by default; embed IANA timezone data for portable
  Windows use.
- Add accessible DailyOps setup controls and dashboard status for enabled
  state, timezone, next run, last scheduled result, and manual-run state.
- Reopen setup when Pack configuration changes while preserving encrypted
  credential bindings and the independently persisted schedule.
- Add an internal-only controlled clock/wake seam to the DailyOps test harness;
  no production CLI/env/manifest/API/persisted control was added.
- Extend the real Pack Run/backend/built-UI E2E to prove one scheduled delivery,
  a simultaneous manual `already_running` result, restart without replay, and
  the next scheduled delivery after restart.
- Keep DailyOps Pack version `0.2.0` because its manifest/workflow contract did
  not change; the runtime/UI schedule capability requires no Pack migration.

Verification:

- Scoped and full Go tests: PASS.
- `go vet ./...` and `go build ./...`: PASS.
- UI unit tests: 58/58 PASS; production UI build: PASS.
- Real DailyOps schedule/concurrency/restart E2E: PASS.
- Scoped/full Playwright and runner smoke: PASS.
- Pack validate/offline test, deterministic build, ZIP/extracted verification,
  forbidden runtime-state scan, and text canary/path scan: PASS.
- `govulncheck ./...`: 0 reachable vulnerabilities; npm audit: 0
  vulnerabilities.
- Clean checkout backend/vet/build/vulnerability, frontend 58/58/build/audit,
  and real DailyOps schedule E2E: PASS.

Implementation and CI:

- Main implementation: `93d32a390d235f3a96690c136c4ba2a674c99dbd`.
- Linux restart cleanup hardening: `488010bd1451079834ee1ec5ce2b17bf7ca64aef`
  and `c8491d161b288c527b9862cf8fb1192445f3d1dc`.
- GitHub Actions run `31714389016`: SUCCESS on exact head
  `c8491d161b288c527b9862cf8fb1192445f3d1dc`, including backend race/vet/
  govulncheck, frontend, Linux Playwright DailyOps restart, pack determinism,
  all platform builds, and native Windows DailyOps setup/persistence E2E.
- Draft PR #15 remains open, draft, stacked on
  `feature/goflow-pilot-ux-hardening`, and unmerged.

Manual/external gates: none for Phase 1.

## Phase 2: Lifecycle And Operational Hardening

Status: Checkpoint D in progress

Branch: `feature/goflow-productization-lifecycle`

Stack base: exact Phase 1 closure head
`9af5fa34acffab28a822f6f4e64d88558c3311e8`, whose GitHub Actions run
`31715066909` completed successfully.

Planned phase scope:

- D: versioned Pack setup migration and fail-closed revalidation.
- E: bounded diagnostics, retention, local metrics, and redacted export.
- F: Windows appliance experience and safe offline update/rollback.

## Remaining Checkpoints

| Checkpoint | State |
| --- | --- |
| B: Scheduler backend MVP | DONE |
| C: Scheduler appliance UX and DailyOps | DONE |
| D: Versioned Pack setup migration | NOT_STARTED |
| E: Diagnostics, retention, local metrics | NOT_STARTED |
| F: Windows experience and offline update | NOT_STARTED |
| G: Pack author compatibility toolkit | NOT_STARTED |
| H: Authenticity/signing foundation | NOT_STARTED |
| I: Adapter architecture | NOT_STARTED |
| J: Vendor selection dossier | NOT_STARTED |
| K: DailyOps Beta journey | NOT_STARTED |
| L: Final acceptance suite | NOT_STARTED |
| M: Pilot operations package | NOT_STARTED |
