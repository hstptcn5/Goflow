# Goflow Post-Alpha Roadmap

Status: Completed for ecosystem alpha Checkpoint I

This roadmap defines work that must not be inferred as complete during the ecosystem alpha. Each phase requires objective entry gates, exit criteria, security dependencies, and explicit user or business decisions.

## 1. Pilot Validation

Entry gates:

- Ecosystem alpha acceptance suite is green.
- DailyOps mock setup and run journey is verified.
- Pilot guide is complete.

Exit criteria:

- Three real users complete observed setup using non-production sample data.
- Reporting pain, current workflow, and willingness to adopt are documented.
- Setup success rate, time to first run, error rate, and support burden are measured.

Security dependencies:

- No production secrets accepted through chat, issues, email, or committed files.
- Consent and privacy checklist completed for any sample data.

Codex must not infer:

- Customer validation, revenue, vendor access, or product-market fit.

## 2. First Vendor Adapter

Entry gates:

- Pilot users identify one concrete vendor integration need.
- Official API documentation is available.
- Customer-authorized sandbox credentials are provided through a secure channel outside the repository.

Exit criteria:

- One vendor adapter passes contract tests against official sandbox behavior or documented fixtures.
- Rate limits, auth refresh, pagination, retries, error handling, and data minimization are documented.
- No real credentials or customer data appear in repository, logs, tests, or artifacts.

Security dependencies:

- Credential handling reviewed.
- HTTP/SSRF redirect policy enforced.
- Vendor-specific diagnostics redaction added.

Codex must not infer:

- Undocumented API contracts, credentials, terms of service, or customer consent.

## 3. Trust Layer

Entry gates:

- Alpha pack format and artifact pipeline are stable.
- Threat model residual risks for unsigned packs are accepted.

Exit criteria:

- Publisher identity model is defined.
- Offline signature verification exists for bundle metadata.
- Key rotation, revocation, reproducible release evidence, and security review process are documented and tested.

Security dependencies:

- Key custody decision.
- Signing ceremony and storage policy.
- Revocation distribution design.

Codex must not infer:

- Signing keys, certificate procurement, legal identity, or custody policy.

## 4. Remote Registry

Entry gates:

- Trust layer is implemented and reviewed.
- Multiple maintained packs exist with clear ownership.

Exit criteria:

- Signed metadata registry.
- Bounded registry client.
- Explicit user-approved install.
- Rollback and uninstall behavior.
- No remote code execution.

Security dependencies:

- Abuse response process.
- Metadata signing and revocation.
- Client-side size, count, and timeout limits.

Codex must not infer:

- Marketplace policy, publisher approval, or remote install defaults.

## 5. Updates

Entry gates:

- Trust layer and rollback mechanism are implemented.
- Artifact compatibility policy is documented.

Exit criteria:

- User-approved updates.
- Signature verification before applying updates.
- Atomic replacement and rollback.
- Clear failure recovery.

Security dependencies:

- Update metadata signing.
- Downgrade and rollback policy.
- No silent background update.

Codex must not infer:

- Auto-update consent, background update policy, or trust in unsigned artifacts.

## 6. Commercial Layer

Entry gates:

- Pilot validation shows real willingness to pay.
- Legal, tax, support, privacy, and refund decisions are made by the user/business owner.

Exit criteria:

- Licensing/payment flow is designed and reviewed.
- Support and privacy policies exist.
- Commercial code boundary is documented.

Security dependencies:

- Payment provider account and key custody outside repository.
- Privacy and support data handling.

Codex must not infer:

- Pricing validation, legal terms, tax compliance, refund policy, or payment account setup.

## 7. Marketplace

Entry gates:

- Multiple maintained packs.
- Active users.
- Trust layer.
- Abuse response process.
- Publisher governance.

Exit criteria:

- Publisher onboarding.
- Review and takedown flow.
- Signed pack publication.
- User reporting and rollback.
- Clear marketplace policies.

Security dependencies:

- Publisher identity verification.
- Malware and secret scanning.
- Vulnerability response.
- Registry integrity monitoring.

Codex must not infer:

- Marketplace launch readiness, publisher trust, revenue model, or governance decisions.
