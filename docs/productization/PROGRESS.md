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

Status: Checkpoints D-F complete; Phase 2 complete

Branch: `feature/goflow-productization-lifecycle`

Stack base: exact Phase 1 closure head
`9af5fa34acffab28a822f6f4e64d88558c3311e8`, whose GitHub Actions run
`31715066909` completed successfully.

Planned phase scope:

- D: versioned Pack setup migration and fail-closed revalidation.
- E: bounded diagnostics, retention, local metrics, and redacted export.
- F: Windows appliance experience and safe offline update/rollback.

## Checkpoint D: Versioned Pack Setup Migration

Status: DONE

Implemented:

- Add a closed, host-managed, forward-only migration registry with explicit
  revalidation/config/user-review categories.
- Add bounded migration metadata and pre-mutation setup snapshots with sorted
  SHA-256 inventory.
- Apply config transforms in memory, retain credential ID/type references
  without resolution/decryption, validate the destination contract, and
  compensate all setup files on persistence failure.
- Integrate migration before appliance server start; retain stable workflow,
  history, credential, and schedule rows while suspending enabled schedules for
  revalidation.
- Bump DailyOps to `0.3.0` for the lifecycle behavior contract and register
  `0.1.0 -> 0.2.0 -> 0.3.0`.
- Expose only a bounded attention category to setup/status and explain the
  required review without requiring the encrypted Telegram credential again.

Verification:

- Scoped Pack setup/storage/scheduler/Pack Run/API/Pack tests: PASS.
- Migration fixtures cover ordered 0.1/0.2 upgrades, retain/remove/rename,
  rollback injection, idempotency, downgrade/future schema failure, credential
  reference preservation, and no credential reference in migration metadata.
- Local backend, vet, build, `govulncheck`, npm audit, UI unit/build,
  appliance Playwright, and DailyOps E2E gates: PASS.
- Exact implementation commit:
  `7760198601e7be774da2c2ceac454575ab44ef4e`.
- GitHub Actions run `31717261835`: SUCCESS, including native Windows
  DailyOps E2E and artifact verification.

## Checkpoint E: Diagnostics, Retention, And Local Pilot Metrics

Status: DONE

Implemented:

- Normalize internal and legacy failures through a closed public category
  catalog with fixed safe messages.
- Replace the broad diagnostics payload with a stable versioned allowlist for
  app/Pack/platform, setup, schedule, recent status/duration/category, integrity,
  and explicit privacy flags.
- Bound recent diagnostics to ten entries and omit IDs, timestamps, config,
  URLs, chat IDs, payloads, responses, logs, request metadata, and paths.
- Make age/count cleanup transactional, validate finite safe ranges, and retain
  every active `RUNNING` execution.
- Keep export local-only behind explicit Copy/Download actions; add no
  analytics, remote endpoint, fingerprinting, or background telemetry.

Verification:

- Scoped app-error/storage/config/API/Pack Run tests: PASS.
- Golden diagnostics, restart stability, canary redaction, malformed legacy
  error, large history, active-row retention, and concurrent prune/read tests:
  PASS.
- Full Go test/vet/build and `govulncheck`: PASS; zero reachable
  vulnerabilities.
- npm audit: PASS with zero vulnerabilities; UI unit/build: PASS (59/59).
- Appliance Playwright (2/2), DailyOps setup/restart E2E, and runner smoke:
  PASS.
- Pack validate/offline test, deterministic Windows bundle build/verify, and
  extracted text canary/path scan: PASS. Deterministic SHA-256:
  `8a08b1578e70e39008acffdf1a1f4d387f5e452866b4fbc82e74a889032d44a1`.
- Local race execution is unavailable because this Windows Go environment has
  CGO disabled; the required Linux race gate passed in GitHub Actions.
- Exact implementation commit:
  `55607077664c805c48e43ac03935cb8f094320f2`.
- GitHub Actions run `31719917790`: SUCCESS, including Linux race/security,
  appliance Playwright, DailyOps restart, Pack contracts, build matrix, and
  native Windows artifact verification.

## Checkpoint F: Windows Experience And Offline Update

Status: DONE

Design decision:

- ADR-005 compares WiX 4, Inno Setup, MSIX/Windows SDK, and a verified portable
  PowerShell flow across per-user install, uninstall, reproducibility, CI,
  signing, and supply-chain risk.
- `BLOCKED_INSTALLER_TOOLCHAIN`: no installer is added before an exact
  compiler/SDK, dependency lock, provenance policy, uninstall identity, and
  signing verification are selected together.
- Proceed with the required portable fallback: explicit offline candidate
  verification, stopped-instance check, external-data snapshot, retained app
  rollback, startup health gate, compensation, and no download/silent update.

Implementation:

- Add `Update-Goflow.ps1` outside the application directory and include it only
  in the outer Windows artifact with the pilot guide and SHA-256 inventory.
- Require current-runtime archive/extracted verification, candidate self-
  verification, exact Pack/target identity, ordinary non-reparse app/data
  trees, and no active process before mutation.
- Retain the prior application directory and external-data snapshot; compensate
  failures between rename, activation, migration/startup, and local health.
- Fix `pack verify/test --output json` to return a nonzero exit code on failed
  JSON results; updater additionally requires parsed `status=PASS`.
- Rename the exact-head Windows artifact/marker to `UNSIGNED-PILOT-BETA` and
  expose runtime version plus unsigned channel in bootstrap/UI.

Verification:

- CLI/API/Pack Run scoped tests and build/vet: PASS.
- UI unit/build: PASS (60/60); appliance Playwright: PASS (2/2).
- Exact implementation head:
  `0832758b986130a1c9cef8314fae982525d5f52e`.
- Native local portable update acceptance: PASS for valid health-checked
  update, tamper rejection before mutation, unhealthy-start app/data rollback,
  reparse rejection, and active-instance rejection.
- Native exact-head artifact acceptance: PASS, including setup, manual and
  scheduled execution, restart, single-instance reuse, tamper rejection,
  external state, PE AMD64, update compensation, and leakage scans.
- Artifact: `UNSIGNED-PILOT-BETA-goflow-dailyops-windows-amd64`;
  `Goflow-DailyOps-Windows-amd64.zip` SHA-256
  `63a1cee814a48f6d67874f33a4115335825bb084bfbe2b9a478ccd6c76fd440c`;
  `SHA256SUMS.txt` SHA-256
  `d4d29a989a16ff49646c083bc8ba43382c6ac9ce5b3ec5ed314d926105581b6a`.
- Smoke request evidence: source `4`, Telegram `getMe` `2`, `getChat` `2`,
  and `sendMessage` `2` across the manual/scheduled/restart scenarios; no
  duplicate message was accepted for a concurrent request pair.
- UI build output is byte-identical across CRLF and clean LF checkouts; unit
  tests pass 60/60 and the committed production asset hashes are stable.
- GitHub Actions run `31723919989`: SUCCESS, including backend race/vet/
  vulnerability checks, frontend, Playwright, Pack contracts, all target
  builds, and native Windows artifact/update verification.
- `MANUAL_GATE_REQUIRED`: a nontechnical pilot user must still perform the
  documented unsigned Windows install and offline upgrade on a real pilot
  machine. This is not represented as an automated PASS.

## Phase 3: Trust And Pack Author Platform

Status: Checkpoints G-H complete; Phase 3 complete

Branch: `feature/goflow-productization-trust`

Stack base: exact Phase 2 closure head
`9739aed00a96dacf0e6c3989bd1ce6ab212a3fb8`, whose GitHub Actions run
`31724659808` completed successfully.

### Checkpoint G: Pack Author Compatibility Toolkit

Status: DONE

Design decision:

- ADR-006 preserves legacy Pack Format v1 by making capability and offline
  fixture declarations optional.
- Runtime compatibility uses a closed capability allowlist instead of brittle
  runtime version inference; unknown capability fails closed.
- Offline fixtures remain bounded, non-secret author inputs and are excluded
  from built appliance artifacts.
- Schedule and migration remain host-managed; no Pack executable hook is added.

Implementation and verification:

- Exact implementation head:
  `23cbabba29e18a2a7fd15417a32e7da504409434`.
- Pack Format v1 remains compatible when capability/fixture declarations are
  omitted. New capability declarations use a closed allowlist and fail with
  exact manifest locations when unknown, duplicate, malformed, unavailable,
  or inconsistent with bindings/connection tests.
- A bounded strict offline fixture is validated during `pack validate`, uses
  the same config-value validator as persisted setup/migrations, rejects
  unknown keys, future schemas, secrets, traversal, symlinks, oversize, and
  invalid types/ranges, and is excluded from runtime bundles.
- `pack init -> validate -> test -> build -> verify` passes for a generated
  scaffold. Legacy hello-webhook and DailyOps both validate; DailyOps offline
  test passes with fixture evidence and external checks skipped.
- Two native DailyOps builds were byte-identical with SHA-256
  `adb4315d764a6ddbff315fdaf54bf0cc22454edf3aee4ce17a01ed531e52cb63`.
- Full Go test/vet/build: PASS. `govulncheck`: 0 reachable vulnerabilities.
  npm audit: 0 vulnerabilities.
- Clean LF exact-head checkout: full Go test/vet/build and legacy/DailyOps/
  scaffold author acceptance PASS with no tracked worktree changes. The run
  also fixed a pre-existing CRLF-only diagnostics golden comparison.
- GitHub Actions run `31726801228`: SUCCESS, including race/security,
  frontend, Pack contracts, Playwright, build matrix, and native Windows gate.

### Checkpoint H: Authenticity And Signing Foundation

Status: DONE

Design-first evidence:

- `PACK_SIGNING.md` defines strict container bounds, deterministic canonical
  bytes, Ed25519 algorithm agility boundary, key ID semantics, exact identity/
  target/capability binding, verification order, and unsigned policy.
- ADR-007 requires explicit operator trust and forbids embedded/downloaded/TOFU
  keys, production credentials, registry/release behavior, and automatic
  downgrade acceptance.
- Threat model covers modified/malicious/stolen/revoked/replayed/unsigned inputs
  and separates valid signatures from publisher governance.

Implementation and verification:

- Exact implementation head:
  `19fd735c28e1ecfab235c7fd7e34c74570b4deac`.
- Standard-library Ed25519 signs deterministic canonical bytes binding exact
  `PACK_INFO.json`, Pack ID/version/target/capabilities, schema, algorithm, and
  explicit key ID. One bounded strict signature member remains outside
  inventory; self-inventory is rejected.
- `pack sign` accepts one PEM PKCS#8 private key from an explicit local file or
  stdin and atomically creates a different ZIP without overwrite. `pack
  verify-signature` requires a separately supplied matching PEM PKIX public-key
  file and key ID. No embedded/network/TOFU key is trusted.
- Integrity-only `pack verify` reports `UNSIGNED` or `SIGNED_UNVERIFIED`; only
  explicit trust verification reports `VERIFIED`. Extracted signed bundles are
  supported under the same inventory-first order.
- Tests pass for valid/deterministic signatures; wrong key/ID; unsigned policy;
  modified manifest/workflow/asset/runtime/inventory; version/target/capability
  mismatch; unknown schema/algorithm/field; duplicate/oversized/symlink
  metadata; circular inventory; existing-output protection; and private-key
  leakage scans.
- Full Go test/vet/build and clean LF exact-head checkout: PASS. `govulncheck`:
  0 reachable vulnerabilities. npm audit: 0 vulnerabilities.
- Native unsigned Windows beta regression: PASS; artifact remains explicitly
  `UNSIGNED-PILOT-BETA` and no CI signing behavior was added.
- GitHub Actions run `31729541465`: SUCCESS, including backend race/security,
  frontend, Pack contracts, Playwright, build matrix, and native Windows gate.
- Residual external gate: production key governance, publisher identity,
  rotation/revocation, minimum-version policy, certificate/signing authority,
  and signed release remain unimplemented and must not be inferred from this
  offline foundation.

## Phase 4: Adapter Architecture And Commercial Discovery

Status: Checkpoint I in progress

Branch: `feature/goflow-productization-adapters`

Stack base: exact Phase 3 closure head
`e0baf54dd678c329c8dbdf8fd0d4e23fe4f5ca02`, whose GitHub Actions run
`31730282252` completed successfully.

### Checkpoint I: Adapter Architecture

Status: IN_PROGRESS

- ADR-008 selects a reviewed declarative GET-only normalized HTTP source node,
  not arbitrary Pack/native execution.
- Auth, pagination, rate limit, retry/idempotency, normalized output, public
  error mapping, secrets, compatibility, and offline contract tests are defined
  before runtime mutation.
- Future process adapters remain design-only pending a separate security review.

## Remaining Checkpoints

| Checkpoint | State |
| --- | --- |
| B: Scheduler backend MVP | DONE |
| C: Scheduler appliance UX and DailyOps | DONE |
| D: Versioned Pack setup migration | DONE |
| E: Diagnostics, retention, local metrics | DONE |
| F: Windows experience and offline update | DONE |
| G: Pack author compatibility toolkit | DONE |
| H: Authenticity/signing foundation | DONE |
| I: Adapter architecture | IN_PROGRESS |
| J: Vendor selection dossier | NOT_STARTED |
| K: DailyOps Beta journey | NOT_STARTED |
| L: Final acceptance suite | NOT_STARTED |
| M: Pilot operations package | NOT_STARTED |
