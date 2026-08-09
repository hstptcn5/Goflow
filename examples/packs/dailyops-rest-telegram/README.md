# DailyOps REST to Telegram

Experimental first-party example pack ID: `official.dailyops-rest-telegram`.

This pack fetches a vendor-neutral DailyOps JSON document from a configured HTTP(S) endpoint, builds a concise daily report, and sends one Telegram message through an encrypted `TELEGRAM_BOT` credential slot. It is an example pack, not a marketplace listing or a vendor integration.

Automated tests use mock HTTP and Telegram servers. Real Telegram credentials are not required for tests.

## Setup

- `source_url`: absolute HTTP(S) URL returning the normalized JSON contract.
- `chat_id`: Telegram chat, group, or channel identifier. This is not a token.
- `report_title`: non-secret title shown in setup.
- `low_stock_threshold`: optional planning threshold for future adapters.
- `telegram`: encrypted Telegram bot token credential slot.

No bot token, authorization header, database, key file, or credential value belongs in `pack.json`, workflow JSON, this README, or built bundles.

## Normalized Source API Contract

The source endpoint must return a bounded JSON object:

```json
{
  "report_date": "2026-08-09",
  "timezone": "Asia/Bangkok",
  "revenue": 1250000,
  "order_count": 42,
  "cancelled_refunded_count": 3,
  "low_stock_summary": "2 items under threshold",
  "comparison_summary": "Revenue up 8% vs previous day"
}
```

Field meanings:

- `report_date`: report date or timestamp string.
- `timezone`: display timezone string.
- `revenue`: numeric revenue amount in the source system's configured currency.
- `order_count`: number of created/paid orders in the period.
- `cancelled_refunded_count`: combined cancelled or refunded order count.
- `low_stock_summary`: already-normalized concise inventory summary.
- `comparison_summary`: optional prior-period comparison text.

Vendor-specific adapters for systems such as POS, ecommerce, or ERP platforms require a separate pilot and validation pass.

## Local Author Flow

PowerShell:

```powershell
.\goflow.exe pack validate .\examples\packs\dailyops-rest-telegram
.\goflow.exe pack test .\examples\packs\dailyops-rest-telegram --output json
.\goflow.exe pack build .\examples\packs\dailyops-rest-telegram --output .\dist
```

POSIX:

```sh
./goflow pack validate ./examples/packs/dailyops-rest-telegram
./goflow pack test ./examples/packs/dailyops-rest-telegram --output json
./goflow pack build ./examples/packs/dailyops-rest-telegram --output ./dist
```

## Mock Demo Run

Use a local mock source endpoint and the appliance UI. Create a Telegram credential only in the local Goflow data directory. Do not edit the workflow JSON to add a token.

```sh
./goflow pack run ./examples/packs/dailyops-rest-telegram --data-dir ./tmp-dailyops-data --no-open
```

Open the printed URL, fill the mock source URL, assign a Telegram credential, complete setup, and run once.

## Pilot Checklist

- Confirm the source adapter emits exactly the normalized contract.
- Confirm endpoint size, timeout, and error behavior with real sample data.
- Confirm the Telegram target chat and notification text with a non-production chat first.
- Review retry behavior before enabling any schedule.
- Back up `goflow.db` and `goflow.master.key` together before pilot use.
