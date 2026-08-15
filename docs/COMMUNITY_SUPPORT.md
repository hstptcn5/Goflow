# Community Support Policy

Goflow Community support is best effort. There is no response-time, resolution,
availability, or maintenance SLA.

During stable-candidate preparation, support and compatibility work targets
`1.0.0` and the documented upgrade path from exact tag `v1.0.0-rc.1`. Older
development snapshots, modified binaries, unsupported
database schemas, custom forks, and unlisted platforms may still receive useful
guidance but are outside the acceptance boundary.

For normal bugs, open a GitHub issue at
<https://github.com/hstptcn5/Goflow/issues>. Include:

- `goflow version --output json` output;
- target OS and architecture;
- exact CI run and artifact name when applicable;
- concise reproduction steps and expected/actual behavior;
- sanitized logs and whether a clean temporary data directory reproduces it.

Never attach databases, master keys, tokens, credentials, webhook secrets,
private workflow payloads, or unsanitized logs. Follow the private-handling
instructions in [Security](SECURITY.md) for suspected vulnerabilities.
