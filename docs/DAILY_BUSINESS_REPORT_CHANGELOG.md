# Daily Business Report — Product Changelog

## 0.10.0 — Bilingual public beta

- Added one Pack, two report languages: English and Vietnamese.
- Added persistent English/Tiếng Việt appliance UI switcher.
- Added Vietnamese deterministic, OpenAI and DeepSeek report paths.
- Added validated localization metadata and stable locale-neutral config values.
- Kept AI disabled by default and preserved the same normalized source contract.

## 0.9.0 — Checkpoint G/H public beta

First productized public beta of `official.daily-business-report`.

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

- Native Windows amd64 unsigned public beta artifact.
- Deterministic pack build verification.
- SHA-256 checksum manifest.
- Portable artifact state/secret scan.
- Install guide and product changelog included with the artifact.

### Known beta limits

- Artifact is unsigned and may trigger Windows SmartScreen.
- The normalized source endpoint is supplied by the user; vendor adapters remain separate preview Packs.
- Public payment/licensing remains out of scope for this checkpoint.
