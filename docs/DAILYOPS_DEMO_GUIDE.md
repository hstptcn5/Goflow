# DailyOps Reference Pack Demo

Pack ID: `official.dailyops-rest-telegram`

This reference pack fetches vendor-neutral JSON over HTTP(S), formats a short report, and sends it through a Telegram bot credential slot. Automated tests use local mock services only.

## Sample Data Contract

The source URL should return JSON with these fields:

```json
{
  "report_date": "2026-08-09",
  "timezone": "Asia/Bangkok",
  "revenue": 48250.75,
  "order_count": 314,
  "cancelled_refunded_count": 7,
  "low_stock_summary": "3 SKUs below threshold",
  "comparison_summary": "Revenue up 12.4% vs prior day"
}
```

Use non-production data for pilot setup.

## Setup Values

- `source_url`: absolute `http` or `https` URL returning the JSON contract.
- `chat_id`: destination chat ID for the Telegram bot.
- `telegram`: encrypted `TELEGRAM_BOT` credential. Test Telegram verifies both the token and configured chat without sending a message.

## Demo Flow

1. Validate the pack:

   ```bash
   goflow pack validate examples/packs/dailyops-rest-telegram
   ```

2. Run the offline pack test:

   ```bash
   goflow pack test examples/packs/dailyops-rest-telegram --output json
   ```

3. Start appliance mode:

   ```bash
   goflow pack run examples/packs/dailyops-rest-telegram --no-open
   ```

4. Enter the JSON API endpoint, not a website homepage, and require **Test source** to report `Valid`.
5. Enter the chat ID, send `/start` to the bot when using a direct chat, and require **Test Telegram** to report `Valid`.
6. Complete setup with sample data and a non-production Telegram bot token.
7. Run the workflow from the dashboard. The button remains disabled while the single allowed execution is active and status updates without reloading.

## Limits

This pack does not claim access to any specific sales vendor. A real vendor adapter requires official API documentation and customer-authorized sandbox credentials.
