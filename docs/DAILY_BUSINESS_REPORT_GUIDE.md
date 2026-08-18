# Daily Business Report — Public Beta Guide

This guide covers **Daily Business Report** (`official.daily-business-report`, pack version `0.9.0`).

The shortest successful path takes about **5–10 minutes** and does not require an AI provider. Public beta distribution is Windows-first and remains unsigned, so Windows SmartScreen may warn before launch.

## What you need

- Windows 10/11 on amd64.
- A Telegram bot token from BotFather.
- A Telegram chat, group, or channel ID the bot can access.
- A reachable HTTP(S) JSON endpoint containing the daily business snapshot, **or use the public sample already filled into the first-run form**.

Optional after the deterministic report works:

- OpenAI API key for OpenAI commentary.
- DeepSeek API key for DeepSeek commentary.

## 1. Download, verify, extract

Download `Goflow-Daily-Business-Report-Windows-amd64.zip` from the official Daily Business Report GitHub prerelease.

Download `SHA256SUMS.txt` from the same release and verify the ZIP before running it. Do not use a ZIP re-hosted by an unknown third party.

Extract the complete ZIP to a normal writable folder. Keep the files together.

Run `goflow.exe` inside the extracted application directory. Goflow starts a loopback-only local web UI and opens the Daily Business Report appliance.

The portable bundle does not keep runtime state inside itself. Setup state, encrypted credential bindings, schedule state and execution history are stored under the Goflow application-data location for this pack.

## 2. Use the public sample for the fastest first run

The first-run form is prefilled with a version-pinned public sample URL:

`https://raw.githubusercontent.com/hstptcn5/Goflow/daily-business-report-v0.9.0-beta.1/examples/packs/daily-business-report/sample-source.json`

Leave this URL unchanged for the first test and select **Test source**. The sample contains demo-only values and exists only to prove that the install, source validation and Telegram delivery path works without making you host JSON first.

After the first successful report, replace it with your own endpoint.

Your real endpoint must return JSON matching this contract:

```json
{
  "report_date": "2026-08-18",
  "timezone": "Asia/Ho_Chi_Minh",
  "revenue": 48250.75,
  "order_count": 314,
  "cancelled_refunded_count": 7,
  "low_stock_summary": "3 SKUs below threshold",
  "comparison_summary": "Revenue up 12.4% vs prior day"
}
```

Required field rules:

- `report_date`: non-empty string.
- `timezone`: non-empty string.
- `revenue`: number.
- `order_count`: integer >= 0.
- `cancelled_refunded_count`: integer >= 0.
- `low_stock_summary`: string.
- `comparison_summary`: string.

## 3. Connect Telegram

Enter the target Telegram chat ID.

Create the Telegram credential using the bot token. Then select **Test Telegram**. The test verifies both the token and access to the configured chat.

If Telegram reports that the chat is inaccessible:

1. Send `/start` to the bot for a direct chat, or add the bot to the target group/channel.
2. Confirm the chat ID.
3. Run **Test Telegram** again.

## 4. Keep AI commentary off for the first run

Leave **AI commentary** at `none` for the first successful run. The workflow skips every LLM node and builds the report deterministically from the seven validated source fields.

The core product therefore requires only the business-data endpoint and Telegram.

OpenAI and DeepSeek credentials are optional. Add only the provider you choose later.

## 5. Configure the daily schedule

Scheduling is optional. To enable it:

1. Turn on **Enable scheduled report**.
2. Pick a local daily time.
3. Use an IANA timezone such as `Asia/Ho_Chi_Minh`.
4. Save the schedule.

The schedule is host-managed, persists across restart, and uses the configured timezone.

## 6. Complete setup and run once

Select **Complete setup** only after the current source and Telegram destination pass their tests.

Then select **Run now**.

A successful deterministic message contains the report date/timezone, revenue, order count, cancelled/refunded count, low-stock summary and comparison summary. Confirm exactly one Telegram message arrived.

## 7. Replace the sample with your data

Reopen setup and replace **Business data URL** with your real endpoint. Run **Test source** again before completing setup.

The sample URL is intentionally version-pinned to the public beta release so its contract cannot silently change underneath an installed beta build.

## 8. Verify restart persistence

Close the process cleanly and start `goflow.exe` again from the same extracted application directory.

The appliance should reopen without requiring the source URL, Telegram credential or schedule to be entered again. **Run now** should still work after restart.

## Optional: OpenAI commentary

After the deterministic report succeeds:

1. Create the optional **OpenAI API key** credential.
2. Change **AI commentary** to `openai`.
3. Save/revalidate setup if requested.
4. Run the report again.

Only the OpenAI branch executes. AI commentary is instructed to stay concise and factual and not invent numbers or causes absent from the source data.

## Optional: DeepSeek commentary

After the deterministic report succeeds:

1. Create the optional **DeepSeek API key** credential.
2. Change **AI commentary** to `deepseek`.
3. Save/revalidate setup if requested.
4. Run the report again.

Only the DeepSeek branch executes.

## Support and feedback

Open a GitHub issue in the Goflow repository with a title beginning `[Daily Business Report]`. Include the stage that failed, the exact non-secret error message, and redacted diagnostics if useful.

Before posting diagnostics, use the appliance diagnostics panel and keep the payload redacted. **Never post Telegram bot tokens, OpenAI keys, DeepSeek keys, master keys or other secrets in a GitHub issue.**

## Public beta boundary

This Windows package is an **unsigned public beta**. Verify the official SHA-256 checksum before running it.

No runtime database, master key, saved setup file or credential binding file is distributed inside the portable artifact. Payment/licensing and signed installers remain outside this beta.
