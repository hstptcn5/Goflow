# Vietnam Morning Brief — Windows Personal Pilot

This artifact is an **unsigned personal/non-commercial pilot** for `official.vietnam-morning-brief` v0.1.0.

It is intended for a small real-use trial before any public or commercial distribution decision.

## What it does

On each run, Goflow:

1. reads the publisher RSS feeds bundled by the pilot Pack;
2. keeps recent items from the configured 24-hour lookback;
3. normalizes and deduplicates stories;
4. prepares a concise Vietnamese morning brief;
5. optionally uses OpenAI or DeepSeek to select/summarize candidate stories;
6. reconstructs final source links from the original RSS items;
7. sends the result to Telegram.

The default first-run path uses **no AI**.

## Important pilot boundary

The bundled source manifest marks every source as `commercial_status: not_cleared` and the distribution mode as `personal_noncommercial_pilot`.

Use this build only for your own pilot. Do not sell, redistribute, or advertise the bundled source configuration as commercially cleared.

The pilot uses publisher RSS only. It does not scrape article pages, bypass access controls, or republish full article text.

## Download contents

The GitHub Actions artifact contains:

- `Goflow-Vietnam-Morning-Brief-Windows-amd64/` — extracted portable application;
- `Goflow-Vietnam-Morning-Brief-Windows-amd64.zip` — verified Pack bundle;
- `SHA256SUMS.txt` — checksums;
- `UNSIGNED-PERSONAL-PILOT.txt` — exact commit and Pack identity;
- `START_HERE.md` — this guide.

## First run

1. Extract the GitHub Actions artifact.
2. Open `Goflow-Vietnam-Morning-Brief-Windows-amd64`.
3. Run `goflow.exe`.
4. Windows may warn because this pilot is unsigned. Inspect the artifact/checksum before continuing.
5. In setup, choose Vietnamese if desired.
6. Enter your Telegram chat ID.
7. Add the Telegram bot token as the required credential.
8. Keep **AI provider = none** for the first successful run.
9. Complete setup.
10. Click **Run Now** once.

Expected first-run result: one Telegram message containing recent Vietnamese stories with original source links.

## Telegram preparation

Create or reuse a Telegram bot and make sure the destination can receive messages from it.

For a private chat, send `/start` to the bot first. For a group/channel, add the bot with the permissions required to post messages and use the correct chat ID or channel identifier.

Never paste the bot token into GitHub issues, screenshots, logs, or source files.

## Optional AI trial

Only after the deterministic no-AI path works:

1. reconfigure the Pack;
2. choose `openai` or `deepseek`;
3. add the corresponding API-key credential;
4. run again;
5. compare usefulness against the deterministic brief.

The AI branch is not trusted to supply source URLs. Final URLs are mapped back to the RSS catalog. Invalid or ungrounded AI output falls back to the deterministic source-linked brief.

## Daily schedule

After one manual run succeeds, enable the existing Goflow daily schedule from the appliance UI.

For this pilot, a useful test is one delivery each morning for 3–7 days. Keep the machine running at the scheduled time.

## What to evaluate during the pilot

Record only a few practical signals:

- Did a brief arrive when expected?
- Were the selected stories actually useful?
- Were important stories obviously missing?
- Did duplicates appear?
- Did source links open the intended original article?
- Was the brief short enough to read in under a few minutes?
- Did AI improve the brief enough to justify enabling it?

The checkpoint is successful only if the brief becomes something you genuinely want to read each morning.

## If a feed temporarily fails

The RSS node tolerates partial source failure. If at least one configured feed remains readable, the brief can continue with the available sources. If all feeds fail, the run fails closed rather than inventing content.

## Data location and secrets

Runtime state and encrypted credentials are stored outside the portable application bundle using Goflow's existing Pack Run state model. The build process rejects common runtime-state and secret files from the downloadable artifact.

## Stop conditions

Do not expand this pilot into HTML scraping, browser automation, paywall/login bypass, Zalo delivery, finance/gold/real-estate Briefs, or commercial distribution until this personal pilot has been evaluated and the source/commercial boundary has been reviewed separately.
