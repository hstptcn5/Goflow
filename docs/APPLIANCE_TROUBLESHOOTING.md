# Appliance Troubleshooting And Diagnostics

## Startup

If `pack run` exits before printing a URL:

- Run `goflow pack validate <pack-directory>`.
- Check that the pack has no packaged plugins.
- Use a fresh `--data-dir` to rule out stale lock state.
- Choose another `--port` if the printed port is unavailable.

If another instance already owns the data directory, Goflow probes its loopback health endpoint and reuses it only when `run-state.json` matches the same pack ID and a loopback URL.

## Setup Not Ready

The appliance reports logical missing requirements, not submitted values. Common causes:

- Required config field is empty.
- URL config is malformed or not `http`/`https`.
- Credential slot has no assigned credential.
- Assigned credential was deleted or has the wrong type.
- Setup was reopened after completion.

Use the setup screen to re-save config and credentials. Do not edit setup JSON by hand unless you are intentionally recovering test data.

## Connection Tests

Connection tests are closed by credential type and test kind. `TELEGRAM_BOT` supports `telegram_get_me`; unsupported combinations are rejected during pack validation. Empty `test_kind` is allowed and appears as skipped in offline pack tests.

Credential test requests are rate-limited and serialized to avoid repeated secret use.

## Diagnostics

The diagnostics endpoint returns a redacted, bounded support summary:

- Pack identity.
- Workflow ID and readiness state.
- Missing logical requirement keys.
- Managed workflow state.
- Latest execution summary.

It intentionally excludes decrypted credentials, credential IDs, database paths, master key paths, raw workflow input/output, environment values, and source/build paths.

## Browser Issues

Appliance mutation APIs require:

- Exact loopback `Host`.
- Exact `Origin`.
- `X-Goflow-Appliance-Token` from bootstrap.
- JSON content type.

If the UI cannot mutate setup state, reload the loopback page printed by the current `pack run` process. Do not use a copied token from another process or port.
