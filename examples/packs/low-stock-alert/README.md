# Low Stock Alert

Vietnam-first preview Pack for a small but recurring retail task: check a normalized inventory endpoint and notify Telegram only when one or more items need attention.

## Source contract

The source endpoint must return JSON with:

- `checked_at` — non-empty string
- `store_name` — non-empty string
- `low_stock_count` — integer >= 0
- `items_summary` — non-empty string

Use `sample-source.json` as the contract example.

## Setup

1. Enter the inventory JSON URL.
2. Enter the Telegram destination and encrypted bot credential.
3. Choose Vietnamese or English output.
4. Test the source and Telegram destination.
5. Complete setup and optionally enable a daily schedule.

A zero `low_stock_count` completes without sending an alert. This preview does not scrape private storefronts or bypass inventory-system access controls.
