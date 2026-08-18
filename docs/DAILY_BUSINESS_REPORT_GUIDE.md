# Daily Business Report — Windows Pilot Guide

This guide covers the first productized Goflow workflow: **Daily Business Report** (`official.daily-business-report`, pack version `0.9.0`).

The shortest successful path takes about **5–10 minutes** and does not require an AI provider.

## What you need

- Windows 10/11 on amd64.
- A reachable HTTP(S) JSON endpoint containing the daily business snapshot.
- A Telegram bot token from BotFather.
- A Telegram chat, group, or channel ID the bot can access.

Optional after the deterministic report works:

- OpenAI API key for OpenAI commentary.
- DeepSeek API key for DeepSeek commentary.

## 1. Extract and start

Extract the complete Windows artifact to a normal writable folder. Keep the files together.

Run `goflow.exe` inside the extracted application directory. Goflow starts a loopback-only local web UI and opens the Daily Business Report appliance.

The portable bundle does not keep runtime state inside itself. Setup state, encrypted credential bindings, schedule state and execution history are stored under the Goflow application-data location for this pack.

## 2. Prepare the business data endpoint

The endpoint must return JSON matching this contract:

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

Paste the absolute HTTP(S) URL into **Business data URL**, save the configuration, then select **Test source**. Do not continue until the source is marked valid.

## 3. Connect Telegram

Enter the target Telegram chat ID.

Create the Telegram credential using the bot token. Then select **Test Telegram**. The test verifies both the token and access to the configured chat.

If Telegram reports that the chat is inaccessible:

1. Send `/start` to the bot for a direct chat, or add the bot to the target group/channel.
2. Confirm the chat ID.
3. Run **Test Telegram** again.

## 4. Keep AI commentary off for the first run

The default `AI commentary` value is `none`.

Leave it at `none` for the first successful run. The workflow then skips every LLM node and builds the report deterministically from the seven validated source fields.

This means the core product requires only:

- the business-data endpoint; and
- Telegram.

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

A successful deterministic message contains:

- report date and timezone;
- revenue;
- order count;
- cancelled/refunded count;
- low-stock summary; and
- comparison/trend summary.

Check the Telegram destination and confirm exactly one message arrived.

## 7. Verify restart persistence

Close the process cleanly and start `goflow.exe` again from the same extracted application directory.

The appliance should reopen without requiring the source URL, Telegram credential or schedule to be entered again. **Run now** should still work after restart.

## Optional: OpenAI commentary

After the deterministic report succeeds:

1. Create the optional **OpenAI API key** credential.
2. Change `AI commentary` to `openai`.
3. Save configuration and complete/revalidate setup if requested.
4. Run the report again.

Only the OpenAI branch executes. The DeepSeek branch and deterministic-only send branch are skipped.

AI commentary is instructed to stay concise and factual and not invent numbers or causes absent from the source data.

## Optional: DeepSeek commentary

After the deterministic report succeeds:

1. Create the optional **DeepSeek API key** credential.
2. Change `AI commentary` to `deepseek`.
3. Save configuration and complete/revalidate setup if requested.
4. Run the report again.

Only the DeepSeek branch executes. The OpenAI branch and deterministic-only send branch are skipped.

## Troubleshooting

### Source test fails

Confirm the URL is reachable from the machine and returns all seven required fields with the correct JSON types.

### Telegram token is invalid

Create/copy the token again from BotFather and replace the saved Telegram credential.

### Telegram chat is inaccessible

Make sure the bot can access the target destination and verify the chat ID.

### Run now says a report is already running

Daily Business Report allows one active run at a time. Wait for the current execution to finish; the appliance tracks it on the dashboard.

### Scheduled run needs attention

Open the appliance, inspect the schedule state, re-test the current source and Telegram destination if setup changed, and save the schedule again if required.

### Need diagnostics

Use the appliance diagnostics panel to refresh, copy or download the redacted diagnostic payload. Secrets and credential IDs are not included in the public diagnostic view.

## Pilot boundary

The Windows package produced by Checkpoint G is an **unsigned pilot artifact**. Windows may display a SmartScreen warning. Verify the published SHA-256 checksums before running it.

No runtime database, master key, saved setup file or credential binding file should be distributed inside the portable artifact.
