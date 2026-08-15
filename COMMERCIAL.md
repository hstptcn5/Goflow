# Goflow Product And Commercial Direction

Goflow is a local-first workflow automation engine that can turn workflows
into portable, task-specific applications.

This document is the canonical commercial-direction document. It distinguishes
what the public repository provides today from possible future editions. Goflow
Pro Creator, Goflow Teams, and Goflow OEM are roadmap directions; they are not
currently available products, subscriptions, or offers.

## Current Product

Goflow currently has two related product paths.

### Goflow Platform

The Platform is the public, self-hosted application. It includes the visual
workflow editor, local execution engine, scheduler, encrypted credentials, CLI,
supported developer interfaces, and the current Pack authoring, validation,
testing, building, verification, and running lifecycle.

It is local-first and uses an embedded Web UI and SQLite by default. It does not
require a hosted Goflow service.

### Pack Appliance

A Pack Appliance packages a workflow with a focused setup and run experience.
It is intended to let a non-technical operator configure and operate a specific
task without editing workflow JSON.

DailyOps is the current reference and demonstration Pack. It is not Goflow's
entire product identity, and it does not establish compatibility with any
particular vendor.

## Edition Direction

| Edition | Current status | Boundary |
| --- | --- | --- |
| Goflow Free / Community | Available in this public repository | All capabilities already published here, including the core engine, editor, scheduler, local credentials, CLI, supported developer interfaces, and current Pack lifecycle |
| Goflow Pro Creator | Planned; not currently sold | No-code Appliance/Pack Builder, guided configuration and credential-schema tools, branding and white-label controls, launcher metadata, installer generation, signed distribution workflows, update channels and rollback UX, commercial distribution convenience, premium adapter tooling or maintained commercial adapters, and priority support where eventually offered |
| Goflow Teams | Future direction; not available | Shared projects, role-based access, team license administration, audit and governance, and shared Pack distribution |
| Goflow OEM | Future direction; not available | Embedding and redistribution agreements, custom branding and distribution, commercial support, and negotiated terms |

### Free / Community Commitment

The public repository remains MIT licensed in this phase. Users may self-host,
use, copy, modify, and redistribute the published code, including for
commercial purposes, under the MIT License.
Capabilities already released publicly will not be removed or artificially
restricted merely to create a paid tier. The execution core should remain
useful without a paid plan.

## Monetization Principles

- Do not paywall capabilities already released under MIT.
- Monetize convenience, packaging, trusted distribution, maintained
  integrations, and support.
- Keep local execution and the public core useful without a paid plan.
- Do not claim customer validation, commercial demand, or willingness to pay
  without evidence.
- Do not invent prices or present roadmap editions as purchasable products.

RevenueCat is one possible future billing and entitlement provider. It is not
an implemented dependency or a selected vendor. Any future entitlement design
must keep provider secrets out of desktop binaries. Offline and local-first use
would require a deliberately designed signed entitlement mechanism rather than
an embedded billing secret or an assumption of continuous network access.

Pricing, refund policy, taxes, terms, support commitments, and final billing or
entitlement provider selection remain undecided.

## Licensing Boundary

Existing published MIT versions remain MIT. Removing or changing the license
for a later version cannot revoke rights already granted for published code.

If proprietary Pro work is created in the future, it should be isolated from
the public core through a clear repository, module, and build boundary. This
milestone does not create a private repository, proprietary module, CLA,
dual-license arrangement, source-available license, or commercial license.

Contributor agreements, copyright ownership, and third-party obligations must
be audited before any future relicensing decision. This document describes
product direction and is not professional legal advice.

The MIT License covers the software. Use of the Goflow name and branding is a
separate matter described in [TRADEMARK.md](TRADEMARK.md).

## Roadmap And Current Limitations

The canonical phase sequence is in [ROADMAP.md](ROADMAP.md). Current operational
limitations remain authoritative in
[docs/BETA_LIMITATIONS.md](docs/BETA_LIMITATIONS.md), including the absence of a
stable Release channel, production signing, installer, hosted control plane,
and production marketplace.
