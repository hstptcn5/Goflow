# Goflow Ecosystem Alpha Progress

Status: IN_PROGRESS

Allowed status values: NOT_STARTED, IN_PROGRESS, BLOCKED, MANUAL_VERIFICATION_REQUIRED, DONE.

## Foundation Preflight

Date: 2026-08-09

Starting branch: `feature/goflow-pack-run-mvp`

Expected foundation commit: `8dd397a21f00cb8828753d177069e97f09db8a90`

Observed foundation commit: `8dd397a21f00cb8828753d177069e97f09db8a90`

PR #2: `https://github.com/hstptcn5/Goflow/pull/2`

PR #2 state:

- Open: yes.
- Draft: yes.
- Merged: no.
- Mergeable state: clean.
- CI: run #55 completed successfully.
- Reviews: none returned by GitHub API.
- Issue comments: none returned by GitHub API.

Repository state:

- Branch was not behind `origin/main`: ahead 10, behind 0.
- No tracked worktree modifications before Checkpoint A.
- Untracked and intentionally excluded: `GOFLOW_ECOSYSTEM_MASTER_GOAL.md`, `roseblade-ranger.audit.html`.
- Tracked artifact scan for binaries, ZIPs, databases, keys, secrets, and temp paths: no matches.

Baseline verification before alpha branch:

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `cd ui && npm ci`: PASS install, with existing audit report of 1 moderate and 1 high vulnerability.
- `cd ui && npm run test`: PASS, 10 files and 48 tests.
- `cd ui && npm run build`: PASS.

Final-alpha dependency gate:

- Existing frontend npm audit findings from `npm ci`: 1 moderate and 1 high vulnerability.
- Before final acceptance, this must be resolved by a compatible upgrade, removed by dependency changes, or documented with verified non-reachability and accepted residual risk.

Alpha branch:

- Created branch: `feature/goflow-ecosystem-alpha`.
- Branch base: `8dd397a21f00cb8828753d177069e97f09db8a90`.

## Checkpoint A - Baseline, Architecture, and Contracts

Status: IN_PROGRESS

Starting commit: `8dd397a21f00cb8828753d177069e97f09db8a90`

Architectural decision:

- Keep one Goflow binary and extend the existing Pack Format v1 with optional setup metadata.
- Add appliance behavior through an explicit startup context from `packrun` to `serverapp` to `api.NewRouter`.
- Keep ordinary `goflow serve` isolated from appliance routes and state.
- Store setup state in the external per-pack data directory, never in the source pack or extracted bundle.
- Keep `TriggerService` as the only workflow execution path for appliance run-now.

Files changed:

- `docs/ECOSYSTEM_ALPHA_PLAN.md`
- `docs/ECOSYSTEM_ALPHA_PROGRESS.md`
- `docs/ECOSYSTEM_THREAT_MODEL.md`
- `docs/POST_ALPHA_ROADMAP.md`

Tests run and result:

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `cd ui && npm run test`: PASS, 10 files and 48 tests.
- `cd ui && npm run build`: PASS.

Security considerations:

- No production behavior changes in this checkpoint.
- Threat model skeleton records known risks and required test mappings before code changes.
- `PACK_INFO` remains unsigned integrity metadata only.
- Packaged plugin execution remains unsupported.

Commit SHA:

- `1388efc` (`docs: plan ecosystem alpha checkpoints`).

Remaining work:

- Finish Checkpoint A verification.
- Commit and push Checkpoint A.
- Begin Checkpoint B setup metadata validation.

Blockers:

- None.

## Checkpoint B - Backwards-Compatible Pack Setup Metadata

Status: IN_PROGRESS

Starting commit: `88c49dfcfacc7477450a12ddd981e2308dda19bb`

Architectural decision:

- Keep Pack Format v1 at `schema_version: 1` and add optional known fields only.
- Keep legacy `required_credentials`; structured `credential_requirements` are the appliance setup contract when present.
- Validate bindings during pack load against the entry workflow and built-in node parameter definitions.
- Treat `display_only: true` as the explicit marker for required setup items that are intentionally not bound.
- Reject literal secret-bearing workflow parameters only in pack context; generic workflow validation remains unchanged.

Files changed:

- `internal/pack/pack.go`
- `internal/pack/pack_test.go`
- `internal/pack/build_test.go`
- `docs/PACKS.md`
- `docs/ECOSYSTEM_ALPHA_PROGRESS.md`

Tests run and result:

- `go test ./internal/pack`: PASS.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `govulncheck ./...`: PASS, 0 reachable vulnerabilities.
- `go test -race ./...` on Windows: BLOCKED because this Go toolchain requires CGO for race.
- `wsl.exe sh -lc 'cd /mnt/d/build2026/Goflow && export PATH="$HOME/.cache/codex-go/go/bin:$PATH" && go test -race ./...'`: PASS.
- `cd ui && npm run test`: PASS, 10 files and 48 tests.
- `cd ui && npm run build`: PASS.

Security considerations:

- New config metadata rejects secret-like keys, labels, descriptions, defaults, and options.
- Credential requirements describe slots only and allowlist credential types and connection test kinds.
- Credential bindings can target only `credential` parameters.
- Config bindings cannot target credential or secret-like parameters.
- Pack workflow literal secret-bearing params, including Telegram `bot_token`, are rejected before pack load succeeds.
- Validation errors name fields and logical items without echoing secret-looking values.

Commit SHA:

- `a4caa3e` (`feat: validate pack setup metadata`).

Remaining work:

- Push Checkpoint B.
- Wait for CI on PR #3.

Blockers:

- None.

## Checkpoint B.1 - Pack Setup Correctness Hardening

Status: IN_PROGRESS

Starting commit: `a13d36bfbdd5881f77d0665bf2052342eba83d5f`

Architectural decision:

- Keep binding fan-out from one source to multiple distinct targets.
- Reject any second binding to the same `node_id.param` target, regardless of source.
- Keep connection tests as a closed compatibility matrix between credential type and test kind.
- Restrict URL setup defaults to absolute `http` and `https` URLs because those are the only safe schemes required for current Goflow pack setup.

Files changed:

- `internal/pack/pack.go`
- `internal/pack/pack_test.go`
- `docs/PACKS.md`
- `docs/ECOSYSTEM_ALPHA_PROGRESS.md`

Tests run and result:

- `go test ./internal/pack`: PASS.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `govulncheck ./...`: first run invalid because it overlapped with `npm ci` mutating `ui/node_modules`; rerun after install completed: PASS, 0 reachable vulnerabilities.
- `wsl.exe sh -lc 'cd /mnt/d/build2026/Goflow && export PATH="$HOME/.cache/codex-go/go/bin:$PATH" && go test -race ./...'`: PASS.
- `cd ui && npm ci`: PASS install, with existing audit report of 1 moderate and 1 high vulnerability.
- `cd ui && npm run test`: PASS, 10 files and 48 tests.
- `cd ui && npm run build`: PASS.

Security considerations:

- Prevents ambiguous parameter ownership in setup bindings.
- Prevents impossible or misleading credential test declarations.
- Prevents file/custom-scheme URL defaults from being introduced through pack metadata.

Commit SHA:

- `1492f7e` (`fix: harden pack setup metadata validation`).

Remaining work:

- Push, update PR #3, and wait for CI.

Blockers:

- None.

## Checkpoint C - Safe Runtime Configuration and Parameter Resolution

Status: DONE

Starting commit: `788b5470ae17bd1441c864a87ee2f15630048f92`

Architectural decision:

- Add a dedicated `internal/packsetup` package for runtime setup state instead of expanding `packrun`.
- Store non-secret config in `pack-config.json` under the per-pack data directory.
- Store credential slot assignments in `pack-credentials.json` under the per-pack data directory.
- Include pack ID and config schema version in the stored file.
- Revalidate known fields against the current manifest before applying them.
- Retain safe stale fields on disk but keep them out of the applied runtime config map.
- Reject unsafe stale fields that look like retained secret material.
- Validate credential assignments through an injectable resolver before setup is considered ready.
- Apply setup bindings to a cloned workflow by copying config values and credential IDs only.
- Provide a pack-specific path-only resolver for `input.*`, `nodes.*`, and `pack.config.*` expressions.
- Harden HTTP Request and Telegram network behavior with context-aware requests, bounded bodies, URL/method/header validation, redirect policy, and Telegram redaction.

Files changed:

- `internal/packsetup/config.go`
- `internal/packsetup/config_test.go`
- `internal/packsetup/credentials.go`
- `internal/packsetup/credentials_test.go`
- `internal/packsetup/bindings.go`
- `internal/packsetup/bindings_test.go`
- `internal/packsetup/resolve.go`
- `internal/packsetup/resolve_test.go`
- `internal/nodes/http_request.go`
- `internal/nodes/http_request_test.go`
- `internal/nodes/telegram_bot.go`
- `internal/nodes/telegram_bot_test.go`
- `docs/PACKS.md`
- `docs/ECOSYSTEM_ALPHA_PROGRESS.md`

Tests run and result:

- `go test ./internal/packsetup`: PASS.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `govulncheck ./...`: PASS, 0 reachable vulnerabilities.
- `wsl.exe sh -lc 'cd /mnt/d/build2026/Goflow && export PATH="$HOME/.cache/codex-go/go/bin:$PATH" && go test -race ./...'`: PASS.
- `cd ui && npm run test`: PASS, 10 files and 48 tests.
- `cd ui && npm run build`: PASS.
- GitHub Actions CI run #62 for `8bbdd52`: PASS. Jobs: Frontend build, Backend tests, Build linux-amd64, Build linux-arm64, Build darwin-amd64, Build darwin-arm64, Build windows-amd64.

Security considerations:

- Config storage rejects pack ID mismatches, unsupported schema versions, oversized files, bad URL values, wrong scalar types, missing required values, and unsafe stale fields.
- Config writes are atomic and use `0600` mode where supported.
- Credential-slot storage rejects pack ID mismatches, unsupported schema versions, oversized files, undeclared slots, missing required slots, missing credentials, wrong credential types, and unsafe stale slot metadata.
- Credential-slot files store credential IDs and expected types only; decrypted values are never read or written by this package.
- Binding application mutates only a cloned workflow value and writes credential IDs, never decrypted values.
- The pack resolver has no credential, environment, filesystem, function, JavaScript, loop, reflection, or command access.
- Missing expression errors name bounded expressions without echoing resolved data values.
- HTTP Request rejects malformed, relative, or unsupported-scheme URLs; unsupported methods; bad or oversized headers; oversized request bodies; and oversized responses.
- HTTP Request does not automatically forward `Authorization` or `Cookie` headers across origin-changing redirects.
- Telegram uses execution context cancellation, bounded/redacted error bodies, injectable base URL for tests, and credential ID precedence over literal token fallback.

Commit SHA:

- `e6f8aae` (`feat: add pack config storage`).
- `ad46eb3` (`feat: add pack credential slot storage`).
- `3f4ec23` (`feat: add pack setup binding resolver`).
- `69243e4` (`fix: harden pack network nodes`).
- `bc5091d` (`docs: mark checkpoint c complete`).

Remaining work:

- Integrate setup storage and binding primitives with appliance backend in Checkpoint D.
- Build appliance UI in Checkpoint E.

## Checkpoint D - Appliance Backend and Security Boundary

Status: IN_PROGRESS

Starting commit: `137322a565169139a8214e14cc37f0662b66e3a1`

Architectural decision:

- Add explicit appliance context passed from `packrun` into `serverapp` and the API router.
- Generate an in-memory per-process appliance session token during Pack Run.
- Mount `/api/appliance/*` only when appliance context is present.
- Keep generic `goflow serve` isolated: appliance routes return 404 when no appliance context exists.
- Require exact Host validation for appliance routes and exact Origin plus session-token header for mutations.
- Add setup schema/current readiness, config save, and credential-slot save endpoints using the Checkpoint C setup storage primitives.
- Return redacted credential readiness without decrypted values or credential IDs.

Files changed:

- `internal/api/appliance.go`
- `internal/api/router.go`
- `internal/api/router_test.go`
- `internal/serverapp/serverapp.go`
- `internal/packrun/run.go`
- `docs/ECOSYSTEM_ALPHA_PROGRESS.md`

Tests run and result:

- `go test ./internal/api ./internal/serverapp ./internal/packrun`: PASS.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go build ./...`: PASS.
- `govulncheck ./...`: PASS, 0 reachable vulnerabilities.
- `wsl.exe sh -lc 'cd /mnt/d/build2026/Goflow && export PATH="$HOME/.cache/codex-go/go/bin:$PATH" && go test -race ./...'`: PASS.
- GitHub Actions CI run #64 for `6d18ef3`: PASS. Jobs: Frontend build, Backend tests, Build linux-amd64, Build linux-arm64, Build darwin-amd64, Build darwin-arm64, Build windows-amd64.

Security considerations:

- Appliance session token is generated from 32 random bytes and is not stored in pack files or data files.
- Appliance routes are absent in generic mode.
- Mutation requests require JSON content type, exact allowed Origin, and the appliance token header.
- Host mismatch returns 404 to reduce DNS-rebinding exposure.
- Current diagnostics skeleton includes only pack ID/version, workflow ID, and readiness state.
- Setup config writes use bounded strict JSON bodies and the existing non-secret config validator.
- Credential slot writes validate credential existence/type and return assigned/type status without credential IDs or decrypted values.

Commit SHA:

- `e229fe5` (`feat: add appliance backend context`).

Remaining work:

- Credential creation/selection/replacement and allowlisted connection tests.
- Complete/reopen setup and setup state machine.
- Server/workflow status, run-now, latest execution, recent executions, and diagnostics redaction tests.
- Oversized-body, rate-limit, bad-host, cross-origin, missing-token, and generic-mode coverage across all appliance mutations.

## Checkpoint E - Nontechnical Appliance UI

Status: NOT_STARTED

Planned scope:

- Appliance first-run wizard.
- Appliance dashboard.
- Unit/component and Playwright coverage for setup, run, diagnostics, and generic isolation.

## Checkpoint F - Pack Author Toolkit

Status: NOT_STARTED

Planned scope:

- `goflow pack init`
- `goflow pack inspect`
- `goflow pack test`
- `goflow pack verify`
- Author tutorial and CLI golden tests.

## Checkpoint G - DailyOps Reference Pack

Status: NOT_STARTED

Planned scope:

- Experimental `official.dailyops-rest-telegram` reference pack.
- Mock sales source and mock Telegram tests.
- Deterministic build and extracted verification.

## Checkpoint H - Development Artifact Pipeline

Status: NOT_STARTED

Planned scope:

- Extend PR CI for appliance E2E, pack CLI contracts, DailyOps mock E2E, determinism, and canary scans.
- Add manual unsigned development artifact workflow without release/tag/signature.

## Checkpoint I - Security Review, Documentation, and Pilot Handoff

Status: NOT_STARTED

Planned scope:

- Complete threat model.
- Appliance quickstart, troubleshooting, backup/restore, credential rotation, author guide, DailyOps guide, artifact verification.
- Pilot guide and post-alpha roadmap.

## Final Acceptance Suite

Status: NOT_STARTED

Required evidence:

- Backend, frontend, pack contracts, security, and user journey gates from `GOFLOW_ECOSYSTEM_MASTER_GOAL.md`.
- Clean checkout evidence.
- Draft PR or stacked draft PRs open, draft, unmerged, and green.
