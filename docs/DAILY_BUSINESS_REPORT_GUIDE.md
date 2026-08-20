# Daily Business Report — Bilingual Public Beta Guide

This guide covers **Daily Business Report** (`official.daily-business-report`, pack version `0.10.0`). The same Pack supports English and Vietnamese output; no separate `-vi` fork is required.

The shortest successful path takes about **5–10 minutes** and does not require an AI provider. Public beta distribution is Windows-first and remains unsigned, so Windows SmartScreen may warn before launch.

## What you need

- Windows 10/11 on amd64.
- A Telegram bot token from BotFather.
- A Telegram chat, group, or channel ID the bot can access.
- A reachable HTTP(S) JSON endpoint containing the daily business snapshot, or use the public sample already filled into the first-run form.

Optional after the deterministic report works: OpenAI or DeepSeek API key for AI commentary.

## 1. Download, verify, extract

Download `Goflow-Daily-Business-Report-Windows-amd64.zip` and `SHA256SUMS.txt` from the official `daily-business-report-v0.10.0-beta.1` GitHub prerelease. Verify the ZIP before running it and do not use a re-hosted copy from an unknown third party.

Extract the complete ZIP to a writable folder and run `goflow.exe`. Goflow starts a loopback-only local web UI. Runtime state, encrypted credential bindings, schedule state and execution history remain outside the portable bundle in the Goflow application-data location.

## 2. Choose your interface and report language

The appliance header has a language selector: **English / Tiếng Việt**. This changes the setup/dashboard interface.

The **Report language / Ngôn ngữ báo cáo** field controls the message that is delivered. Choose `English` or `Tiếng Việt`. Interface language and report language are intentionally separate choices.

## 3. Use the public sample for the fastest first run

The first-run form is prefilled with:

`https://raw.githubusercontent.com/hstptcn5/Goflow/daily-business-report-v0.10.0-beta.1/examples/packs/daily-business-report/sample-source.json`

Leave it unchanged for the first **Test source**. After the first successful report, replace it with your own endpoint.

Your real endpoint must return:

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

`report_date` and `timezone` must be non-empty strings; `revenue` a number; counts non-negative integers; both summary fields strings.

## 4. Connect Telegram

Enter your Telegram chat ID, create the encrypted Telegram bot credential and select **Test Telegram / Kiểm tra Telegram**. The test verifies the bot token and access to the configured destination.

## 5. Keep AI off for the first run

Leave **AI commentary / Nhận xét bằng AI** at `none / Không dùng AI`. The deterministic English and Vietnamese report branches require only the source endpoint and Telegram.

After that path succeeds, optionally add either OpenAI or DeepSeek. The selected provider receives only the business metrics required for the short commentary branch.

## 6. Configure scheduling and run

Scheduling is optional. Enable the daily schedule, choose a local time, and use an IANA timezone such as `Asia/Ho_Chi_Minh`. Complete setup only after the source and Telegram destination pass their tests, then select **Run now / Chạy ngay**.

A successful message contains report date/timezone, revenue, orders, cancelled/refunded count, low-stock summary and comparison summary in the selected language.

## 7. Replace the sample and verify restart persistence

Reopen setup, replace the sample URL with your real endpoint, test the source again and complete setup. Close Goflow cleanly and reopen the same extracted app. Setup, encrypted credentials and schedule should persist, and **Run now / Chạy ngay** should continue to work.

## Support and feedback

Open a GitHub issue beginning `[Daily Business Report]` with the failed stage, exact non-secret error and redacted diagnostics if useful. Never post Telegram tokens, AI keys, master keys or other secrets.

## Public beta boundary

This Windows package is an **unsigned public beta**. Verify the official SHA-256 checksum. No runtime database, master key, saved setup file or credential binding is distributed inside the portable artifact. Payment/licensing and a signed installer remain outside this beta.
