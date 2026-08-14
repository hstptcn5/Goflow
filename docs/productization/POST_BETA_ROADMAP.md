# Goflow Post-Beta Roadmap

This file records deferred work only. None of these items is authorized by the
productization master goal.

## Commercial Adapter

Prerequisites:

- at least three consenting pilot users and evidence of retained value;
- user-approved vendor selection;
- complete official API documentation and legal/terms review;
- authorized sandbox credentials and non-production test data;
- reusable adapter contract tests passing against the sandbox.

Only then should a vendor-specific adapter be implemented in a separate PR.

## Production Signing And Releases

Prerequisites:

- reviewed signing specification and trust-store policy;
- production key custody, rotation, revocation, and incident procedures;
- reproducible build and provenance evidence;
- certificate/private-key authority supplied outside the repository;
- manual Windows acceptance and release approval.

Checksums remain integrity metadata and are not publisher authenticity.

## User-Approved Remote Update

Prerequisites:

- signed update metadata and authenticated artifacts;
- anti-rollback policy;
- atomic installation, health verification, and tested rollback;
- select and pin one Windows installer toolchain together with dependency
  provenance, uninstall identity, and production code-signing verification;
- explicit user consent and no silent/background install;
- privacy and support documentation.

## Curated Pack Registry And Marketplace

Prerequisites:

- multiple maintained Packs and active users;
- publisher identity, signing, revocation, and review governance;
- malware/secret/path/network scanning and incident response;
- compatibility and migration policy;
- legal, privacy, tax, refund, payment, and support decisions.

## Hosted And Enterprise Features

Hosted control planes, multi-tenant orchestration, remote telemetry, enterprise
RBAC/SSO, Kubernetes deployment, mobile applications, and AI workflow generation
require separate product evidence, threat models, and user authorization. They
are not implied by the local-first beta candidate.
