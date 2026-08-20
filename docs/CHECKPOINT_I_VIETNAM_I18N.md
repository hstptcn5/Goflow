# Checkpoint I — Internationalization + Vietnam Market Foundation

Checkpoint I adds a bilingual product foundation without forking the Goflow runtime or the flagship Daily Business Report Pack.

## Runtime and Pack foundation

- English and Vietnamese appliance UI with a persistent language selector.
- Validated `locales.json` Pack metadata convention for localized product/setup copy.
- Stable select/config values remain language-neutral; only labels and delivered copy are localized.
- Generic HTTP requests can inject an encrypted bearer/API credential at runtime without placing secrets in Pack JSON.
- JS Code Runner exposes one setup-bound `input` value for small adapter transformations.

## Vietnam launch Packs

1. `official.daily-business-report` v0.10.0
   - same Pack identity as the global flagship
   - English/Vietnamese report output
   - deterministic/OpenAI/DeepSeek parity in both languages

2. `official.low-stock-alert` v0.1.0
   - normalized inventory source contract
   - sends only when `low_stock_count` is non-zero
   - Vietnamese-first, English optional
   - Telegram delivery

3. `official.haravan-zalo-daily-report` v0.1.0
   - authorized Haravan Admin API GET using encrypted bearer injection
   - local order summary
   - posts `{ recipient_id, message }` to an authorized Zalo OA delivery adapter endpoint
   - does not hard-code undocumented Zalo endpoints or bypass provider permissions

## Distribution boundary

Only the bilingual Daily Business Report is promoted to an immutable Windows public beta in this checkpoint. Low Stock Alert and Haravan → Zalo remain validated preview Packs until their real-user adapter contracts are exercised.

The storefront must describe this distinction clearly and must not present preview Packs as downloadable production integrations.
