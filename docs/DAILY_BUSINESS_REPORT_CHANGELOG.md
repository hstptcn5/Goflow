# Daily Business Report — Product Changelog

## 0.9.0 — Checkpoint G pilot

First productized pilot of `official.daily-business-report`.

### Product

- Dedicated Daily Business Report pack identity and version.
- 5–10 minute deterministic first-run path.
- Normalized seven-field business snapshot contract.
- Telegram delivery with source and destination connection tests.
- Manual run plus persistent host-managed daily schedule.
- Execution history, bounded concurrency, restart persistence and redacted diagnostics inherited from the Goflow appliance runtime.

### AI commentary

- AI is off by default.
- Optional OpenAI branch.
- Optional DeepSeek branch.
- Only the selected AI branch executes.
- AI prompt explicitly avoids inventing unsupported numbers or causes.

### Distribution

- Native Windows amd64 unsigned pilot artifact.
- Deterministic pack build verification.
- SHA-256 checksum manifest.
- Portable artifact state/secret scan.
- Install guide and product changelog included with the artifact.

### Known pilot limits

- Artifact is unsigned and may trigger Windows SmartScreen.
- The normalized source endpoint is supplied by the user; Checkpoint G does not yet bundle vendor-specific sales adapters.
- Public payment/licensing is intentionally out of scope until the install/run path is validated with external users.
