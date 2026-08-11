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

Use the setup screen to re-save config and credentials, then rerun the required source and Telegram tests. A pack version upgrade can intentionally require one revalidation while preserving the encrypted credential. Do not edit setup JSON by hand unless you are intentionally recovering test data.

## Connection Tests

Connection tests are closed by field/credential type and test kind. URL fields may use `http_json_contract`. `TELEGRAM_BOT` supports `telegram_get_me`, which checks both `getMe` and the configured chat with `getChat`; unsupported combinations are rejected during pack validation. Empty `test_kind` is allowed and appears as skipped in offline pack tests.

Source and credential test requests are rate-limited and serialized to avoid repeated network or secret use.

Common categories:

- `source_invalid_url`: enter an absolute `http` or `https` JSON endpoint.
- `source_timeout` or `source_unreachable`: confirm the endpoint is online and reachable from this Windows computer.
- `source_http_error`: fix authentication or the upstream service; Goflow requires HTTP 2xx.
- `source_non_json` or `source_invalid_json`: use the API endpoint, not an HTML website, and fix malformed JSON.
- `source_contract_invalid`: add the required DailyOps fields with the documented types.
- `telegram_unauthorized`: replace the bot token from BotFather.
- `telegram_chat_inaccessible`: send `/start`, add the bot to the destination, and verify the chat ID.
- `already_running`: wait for the current execution; Goflow rejected the duplicate request.
- `internal_error`: refresh once, then collect redacted diagnostics if it repeats.

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
