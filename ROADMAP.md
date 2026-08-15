# Goflow Product Roadmap

Goflow is a local-first workflow automation engine that can turn workflows
into portable, task-specific applications. This is the canonical product phase
roadmap. It defines direction and decision boundaries, not delivery dates or a
promise that every item will ship.

Current product and edition boundaries are defined in
[COMMERCIAL.md](COMMERCIAL.md). A phase is not evidence that its named product
is available; availability must be stated separately in current documentation.

## Roadmap Principles

- Keep the public execution core useful and preserve capabilities already
  released under MIT.
- Separate the Goflow Platform from focused Pack Appliance experiences while
  keeping one execution foundation.
- Prove reliability and trust boundaries before commercial packaging.
- Keep local-first and offline operation deliberate, including any future
  entitlement and update mechanisms.
- Require evidence before vendor, pricing, market, or delivery claims.

## Phase 0: Product & Commercial Reset

Outcome:

- Establish truthful product positioning, current distribution guidance, the
  Free/Pro/Teams/OEM boundary, monetization principles, and this roadmap.

Non-goals:

- Product features, billing integration, proprietary code, licensing changes,
  pricing, releases, installers, or commercial launch.

## Phase 1: Community 1.0

Outcome:

- Define and meet a stable Community release bar for the existing platform and
  Pack lifecycle: documented compatibility, dependable installation and
  upgrade guidance, security review, support boundaries, and trusted release
  evidence.

Non-goals:

- Paid entitlements, no-code Pro Builder, production hosted services, Teams, or
  removal of existing public capabilities.

## Phase 2: Pro Creator Alpha

Outcome:

- Validate a separated Pro Creator prototype for visually building focused
  Pack Appliances, including guided setup schema, branding, launcher metadata,
  and packaging workflows.

Non-goals:

- A generally available paid product, production signing, final pricing,
  automatic updates, Teams, or OEM contracts.

## Phase 3: Pro Creator Beta And Entitlements

Outcome:

- Test the complete Creator workflow and a reviewed local-first entitlement
  design, including offline signed entitlement behavior and rollback-safe
  license-state handling.

Non-goals:

- Embedding provider secrets in binaries, assuming permanent connectivity,
  declaring RevenueCat selected, or claiming production commercial readiness.

## Phase 4: Trusted Windows Distribution

Outcome:

- Establish reproducible Windows packaging, publisher identity, production
  signing, provenance, explicit update channels, rollback UX, and verified
  install/uninstall behavior.

Non-goals:

- Shipping unsigned artifacts as stable releases, silent background updates,
  bypassing operating-system protections, or selecting an unreviewed installer
  toolchain.

## Phase 5: Private Pack Hub

Outcome:

- Provide controlled Pack distribution with publisher identity, signatures,
  compatibility metadata, revocation, review, and incident-response rules.

Non-goals:

- An unrestricted public marketplace, trust-on-first-use, native Pack code
  execution without sandbox review, or treating checksums as authenticity.

## Phase 6: Agent Action Gateway

Outcome:

- Offer policy-controlled agent access to approved workflows and Packs through
  supported interfaces, with scoped identity, auditability, bounded inputs,
  and explicit operator authorization.

Non-goals:

- Letting agents edit or execute arbitrary workflows, bypassing workflow
  security boundaries, or introducing an unbounded hosted agent control plane.

## Phase 7: Teams

Outcome:

- Add shared projects, role-based access, team license administration, audit
  and governance, and shared Pack distribution after the single-user product
  and entitlement boundaries are proven.

Non-goals:

- Claiming enterprise compliance by default, weakening local-first operation,
  or moving existing Community capabilities behind a team plan.

## Phase 8: OEM

Outcome:

- Support negotiated embedding, custom branding and distribution, lifecycle
  responsibilities, and commercial support through explicit agreements and
  clear technical boundaries.

Non-goals:

- Granting automatic trademark rights, retroactively changing MIT grants,
  offering unsupported redistribution, or publishing generic legal terms
  without ownership and counsel review.

## Decisions Intentionally Open

No delivery dates, prices, refund or tax policies, billing provider, installer
toolchain, signing authority, hosted-service plan, or OEM terms are selected by
this roadmap. Those decisions require separate technical, security, legal, and
commercial evidence.
