# Community Release Policy

Goflow Community uses semantic versions. Community `1.0.0` is being prepared as
a stable candidate, but it is not yet a stable GitHub Release or generally
available download. Publishing it remains a separate human decision and release
process.

## Stable Candidate Targets

- Linux amd64 and arm64
- Windows amd64
- macOS amd64 and arm64

Each CI artifact is named `UNSIGNED-COMMUNITY-STABLE-goflow-<target>` and contains
one ZIP plus its outer SHA-256 file. The ZIP contains only the runtime,
`COMMUNITY_ARTIFACT.json`, `README.txt`, and `LICENSE`.

The adjacent checksum binds the exact archive basename and bytes. Inside the
archive, a deterministic inventory in `COMMUNITY_ARTIFACT.json` binds the exact
path, size, and SHA-256 of the runtime, `README.txt`, and `LICENSE`. These checks
establish integrity for identified bytes; they do not establish publisher
authenticity. Community Stable candidate artifacts are unsigned, are not
installers, and are not published through a `latest` URL or automatic update
channel.

## Compatibility

The supported candidate upgrade path is from exact tag `v1.0.0-rc.1` at
`0fdf961ecf67a6ec903d6555b48f67d937728a08` to `1.0.0`. The application must be
stopped and the database and matching master key backed up together first.
Downgrade, hot backup, and forward-schema compatibility are not automatic
guarantees.

An existing RC or stable archive is never silently replaced as if it were the
same accepted build.

## Acceptance Gates

An accepted Community Stable candidate requires:

- exact-head backend, frontend, E2E, Pack, and five-target build CI;
- deterministic archive comparison on a canonical target;
- strict metadata, member, hash, executable-header, and architecture checks;
- extracted-runtime health, embedded UI, external-state, and restart tests;
- an exact RC-to-Stable persistence test, stopped backup/restore test, partial
  restore safety checks, and fail-closed migration fixture;
- documented checksum, unsigned status, support boundary, and upgrade steps.

Passing these gates qualifies a candidate for review. It does not create a tag,
GitHub Release, signature, installer, automatic update channel, SLA, or
stable-release claim.

The previous RC has owner-reported successful workflow, credential-persistence,
and restart evaluation on three anonymous Windows devices. Linux and macOS have
CI verification only. Pro Creator, billing, Teams, OEM, and hosted services
remain future work and are outside this candidate.
