# Daily Business Report v0.10.0 — Bilingual Public Beta

This prerelease extends the validated Daily Business Report public beta without changing its local-first runtime model.

## What is new

- English and Vietnamese report output in one Pack identity.
- `Report language` selector with English as the backward-compatible default.
- Vietnamese deterministic report templates.
- Vietnamese OpenAI and DeepSeek commentary prompts when AI is explicitly enabled.
- English/Vietnamese localization metadata for setup labels and option names.
- Appliance UI language switcher for English and Tiếng Việt.

## Still true

- The first run works with AI set to `none`.
- Runtime credentials remain in the local Goflow data directory, outside the portable bundle.
- Windows artifacts remain unsigned public-beta builds and may trigger SmartScreen.
- Verify `SHA256SUMS.txt` before running the downloaded bundle.
