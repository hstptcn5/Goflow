# Community Release Policy

Goflow Community uses semantic versions. A release candidate such as
`1.0.0-rc.1` is a bounded evaluation build for final compatibility and release
checks; it is not a stable release or a generally available download. A stable
`1.0.0` requires a separate decision and release process.

## Supported RC Targets

- Linux amd64 and arm64
- Windows amd64
- macOS amd64 and arm64

Each CI artifact is named `UNSIGNED-COMMUNITY-RC-goflow-<target>` and contains
one ZIP plus its outer SHA-256 file. The ZIP contains only the runtime,
`COMMUNITY_ARTIFACT.json`, `README.txt`, and `LICENSE`.

The checksum and inner runtime hash establish integrity for identified bytes.
They do not establish publisher authenticity. Community RC artifacts are
unsigned, are not installers, and are not published through a `latest` URL.

## Compatibility

The supported Phase 1 upgrade path is from the exact merged Productization Beta
base identified in the upgrade guide to `1.0.0-rc.1`. The application must be
stopped and the complete external data directory backed up first. Downgrade and
forward-schema compatibility are not automatic guarantees.

An RC may receive a new prerelease version when behavior or artifact bytes
change. An existing RC archive is never silently replaced as if it were the
same accepted build.

## Acceptance Gates

An accepted Community RC candidate requires:

- exact-head backend, frontend, E2E, Pack, and five-target build CI;
- deterministic archive comparison on a canonical target;
- strict metadata, member, hash, executable-header, and architecture checks;
- extracted-runtime health, embedded UI, external-state, and restart tests;
- an exact-base Beta-to-RC persistence test and fail-closed migration fixture;
- documented checksum, unsigned status, support boundary, and upgrade steps.

Passing these gates qualifies a candidate for review. It does not create a tag,
GitHub Release, signature, installer, SLA, or stable-release claim.
