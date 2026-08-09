# Goflow Ecosystem Alpha Threat Model

Status: Completed for ecosystem alpha Checkpoint I

This document tracks the security boundary for the workflow-pack ecosystem alpha. Each later checkpoint must update the relevant threats with implementation evidence and tests.

## Security Boundary

The alpha is a local-first appliance mode for a trusted user running an unsigned development artifact on their own machine. It is not a marketplace, SaaS, signed distribution system, or trust registry.

Trusted components:

- The local Goflow runtime binary the user chose to run.
- The local SQLite database and master key under the per-pack data directory.
- The user's browser on the same loopback origin after appliance bootstrap.

Untrusted or semi-trusted inputs:

- Source pack directories.
- Portable ZIP bundles.
- Extracted bundle contents.
- Pack manifest setup metadata.
- Workflow JSON inside a pack.
- HTTP source responses for reference packs.
- Browser requests that lack same-origin proof.
- Diagnostic export requests.

## Threats

### Malicious Or Tampered Packs

Boundary:

- Pack validation and bundle verification can prove structural validity and local integrity consistency, not author identity.

Mitigations:

- Bounded manifest and workflow reads.
- Portable path validation.
- Symlink escape rejection for source validation.
- Extracted bundles reject symlinks and unexpected controlled files.
- `PACK_INFO` SHA-256 and size verification.
- Pack-only embedded secret scan for known secret-bearing parameters.

Required tests:

- Tampered, missing, extra, duplicate, symlink, oversized, and metadata mismatch rejection.
- Valid legacy Pack Format v1 packs still pass.

Residual risk:

- Unsigned packs can still be malicious workflows. Publisher authenticity requires a future trust layer.

Future signing/registry requirement:

- Signed publisher identity, offline bundle signature verification, revocation, and reviewed pack metadata before any remote install.

### Unsigned PACK_INFO Limitations

Boundary:

- `PACK_INFO` is unsigned metadata inside the bundle.

Mitigations:

- UI and docs must call it integrity metadata, not publisher trust.
- Development artifacts must be labeled unsigned.

Required tests:

- UI and diagnostics wording snapshots or unit tests where practical.
- Documentation review in final gate.

Residual risk:

- A malicious party can rebuild a bundle with matching hashes. Future signing is required.

Future signing/registry requirement:

- `PACK_INFO` must be covered by an external signature and publisher key policy before it is treated as trust evidence.

### Symlink And Path Traversal

Boundary:

- Pack and bundle paths must remain inside the pack or extracted bundle root.

Mitigations:

- Portable slash path validation.
- Windows reserved-name rejection.
- Source pack symlink resolution with containment checks.
- Extracted bundle policy rejects all symlinks.

Required tests:

- Traversal, absolute path, drive path, UNC/device-like path, colon segment, reserved names, and symlink tests.

Residual risk:

- Platform filesystem behavior differences require CI and local smoke coverage.

Future signing/registry requirement:

- Registry ingestion must repeat path and symlink validation before signing pack metadata.

### ZIP Bombs And Bounded Reads

Boundary:

- The runtime must not allocate or read unbounded pack, metadata, workflow, resource, or diagnostic data.

Mitigations:

- Existing pack limits for manifest, workflow, resources, runtime, `PACK_INFO`, and entry count.
- Named limits for setup metadata, API JSON bodies, diagnostics, HTTP response bodies, and recent execution counts.

Required tests:

- Oversized manifest, workflow, resource, `PACK_INFO`, setup metadata, API body, diagnostic data, and HTTP response rejection.

Residual risk:

- Compression-level resource exhaustion must remain bounded by streaming and entry limits.

Future signing/registry requirement:

- Registry-side scanning must enforce the same size, count, and compression limits before publication.

### Embedded Workflow Secrets

Boundary:

- Packs may declare credential slots but must not carry secret values.

Mitigations:

- Existing manifest secret-field rejection.
- Pack-only scan for known secret-bearing workflow parameters.
- Telegram pack workflows must use credential IDs, not literal bot tokens.

Required tests:

- Telegram `bot_token` literal rejected in pack context.
- Error messages do not echo possible secrets.
- Canary secret scan of generated artifacts.

Residual risk:

- Unknown node types or future secret-bearing parameters require maintained metadata.

Future signing/registry requirement:

- Signed pack review must include maintained secret-parameter metadata and automated canary scans.

### Localhost CSRF And DNS Rebinding

Boundary:

- Appliance setup and run APIs mutate local state and must be protected even on loopback.

Mitigations:

- Exact allowed `Origin` check.
- Loopback `Host` validation.
- High-entropy per-process appliance token from same-origin bootstrap.
- No wildcard CORS.

Required tests:

- Missing origin, wrong origin, DNS rebinding host, missing token, wrong token, wrong content type, oversized body.

Residual risk:

- Browser and proxy edge cases require careful host/origin parsing.

Future signing/registry requirement:

- Remote registry or update features must not weaken loopback-only appliance mutation guards.

### Credential Exfiltration

Boundary:

- Credential plaintext may only be decrypted for execution or explicit allowlisted non-mutating connection tests.

Mitigations:

- Existing encrypted credential store.
- Credential slot to credential ID binding only.
- Diagnostics exclude decrypted values and encrypted blobs.
- UI never displays existing decrypted secrets.

Required tests:

- API/UI/diagnostics/logs do not expose canary secrets.
- Credential ID validation checks expected type.

Residual risk:

- Unsafe workflows can still send secrets to remote endpoints if the user authorizes that workflow.

Future signing/registry requirement:

- Signed pack metadata must declare credential use, network destinations where knowable, and reviewer approval for credential-bearing workflows.

### SSRF And Unsafe Redirects

Boundary:

- HTTP Request nodes in reference packs fetch configured URLs.

Mitigations:

- URL scheme validation, method allowlist, request/body/header/response limits.
- Redirect policy that avoids carrying authorization to different origins.
- Pack setup must not accept arbitrary connection-test URLs.

Required tests:

- Invalid scheme, oversized response, redirect to different origin with auth, timeout, cancellation, redacted error body.

Residual risk:

- Generic workflows created by admins retain broad HTTP capability.

Future signing/registry requirement:

- Registry metadata should classify network behavior and require review for broad outbound access.

### Diagnostic And Log Leakage

Boundary:

- Diagnostics are intended for support but must be safe to share.

Mitigations:

- Explicit allowlist of diagnostic fields.
- Redaction of validation errors, execution summaries, URLs, tokens, and canary strings.
- Exclude paths, environment values, DB/key bytes, workflow IO by default, and credential IDs when not necessary.

Required tests:

- Seed canary secrets in config, credentials, workflow params, execution input/output/logs, and verify absence.

Residual risk:

- Redaction patterns require ongoing maintenance.

Future signing/registry requirement:

- Registry and support tooling must reject diagnostic attachments that contain known credential, key, or path patterns.

### Stale Or Cross-Pack Single Instance State

Boundary:

- Different packs sharing a data directory must not reuse each other's server.

Mitigations:

- Existing `RunState.PackID` validation.
- Existing primary launch stale `run-state.json` removal.
- Loopback-only URL check and context-aware health probing.

Required tests:

- Mismatched pack ID, permanent mismatch timeout, opener not called, shared data dir rejection, stale healthy state cleared.

Residual risk:

- Local users with direct data directory write access can disrupt appliance startup.

Future signing/registry requirement:

- Signed pack upgrades must preserve pack ID continuity and reject cross-pack state migration without explicit user action.

### Plugin Or Native Execution

Boundary:

- Packaged native/plugin execution is out of scope for alpha.

Mitigations:

- Existing Pack Run fails early when `plugins` is non-empty.
- Author tools inspect plugins as files only.

Required tests:

- Pack with plugins cannot run appliance mode.
- Inspect/verify/test do not execute plugin files.

Residual risk:

- Future plugin support needs signing, sandboxing, and explicit trust.

Future signing/registry requirement:

- Native/plugin execution requires signed publishers, sandbox policy, user consent, and registry review before support is enabled.

### Supply Chain And CI Artifact Trust

Boundary:

- CI artifacts are unsigned development alpha outputs.

Mitigations:

- Manual artifact workflow only, no release/tag/latest pointer.
- SHA-256 checksums and deterministic metadata.
- Canary scan generated artifacts and bundle members.

Required tests:

- Artifact names include `UNSIGNED-DEVELOPMENT-ALPHA`.
- No release or tag creation.
- Deterministic bundle SHA evidence.

Residual risk:

- GitHub Actions artifacts are not a substitute for publisher signing.

Future signing/registry requirement:

- Production releases require reproducible release evidence, signed artifacts, key rotation, and revocation.

### Pack Upgrade Compatibility

Boundary:

- Pack updates must preserve compatible setup state, credentials, workflow ID, and history.

Mitigations:

- Stable managed workflow ID from pack ID.
- Setup schema version in state.
- Revalidate stored values and credential slot assignments against the current manifest.
- Return to `NEEDS_SETUP` when new required setup is introduced.

Required tests:

- Upgrade preserves config, credentials, workflow ID, history.
- Upgrade with new required field returns to `NEEDS_SETUP`.

Residual risk:

- Incompatible schema changes require clear user action and future migration contracts.

Future signing/registry requirement:

- Registry metadata must carry compatibility and migration contracts signed by the pack publisher.
