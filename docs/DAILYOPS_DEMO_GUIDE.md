# DailyOps Reference Pack Demo

Pack ID: `official.dailyops-rest-telegram`

This reference pack fetches vendor-neutral JSON over HTTP(S), formats a short report, and sends it through a Telegram bot credential slot. Automated tests use local mock services only.

## Sample Data Contract

The source URL should return JSON with these fields:

```json
{
  "date": "2026-08-09",
  "sales_total": 1234.56,
  "orders": 42,
  "low_stock": [
    { "sku": "SKU-1", "name": "Sample item", "quantity": 2 }
  ]
}
```

Use non-production data for pilot setup.

## Setup Values

- `source_url`: absolute `http` or `https` URL returning the JSON contract.
- `chat_id`: destination chat ID for the Telegram bot.
- `report_title`: optional display title.
- `low_stock_threshold`: optional threshold used in report text.
- `telegram`: `TELEGRAM_BOT` credential with `telegram_get_me` connection test.

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

4. Complete setup with sample data and a non-production Telegram bot token.
5. Run the workflow from the dashboard.

## Limits

This pack does not claim access to any specific sales vendor. A real vendor adapter requires official API documentation and customer-authorized sandbox credentials.
