# Daily Business Report

`official.daily-business-report` is the flagship Goflow pack for turning one normalized business snapshot into a scheduled Telegram report.

## What it does

1. Fetches one HTTP(S) JSON endpoint.
2. Validates the response before any message is sent.
3. Builds a deterministic daily report by default.
4. Optionally adds concise OpenAI or DeepSeek commentary.
5. Sends exactly one Telegram message on the selected branch.
6. Runs manually or from the host-managed daily schedule.

## Required setup

- A reachable JSON source matching the contract below.
- A Telegram bot token.
- A Telegram chat, group, or channel ID the bot can access.

Optional:

- OpenAI API key when `ai_provider` is `openai`.
- DeepSeek API key when `ai_provider` is `deepseek`.

The safe default is `ai_provider = none`; no AI account or API key is needed for the core report.

## Source contract

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

The source connection test validates all seven fields before setup can complete.

## First run

1. Save the business data URL and Telegram chat ID.
2. Test the source.
3. Create and test the Telegram bot credential.
4. Keep AI commentary set to `none` for the shortest first-run path.
5. Pick a daily time and timezone if scheduled delivery is wanted.
6. Complete setup.
7. Select **Run now** once and confirm the Telegram message arrives.

After a successful first run, restarting the appliance keeps non-secret setup, encrypted credential bindings, schedule state, and execution history outside the portable bundle.

## Optional AI commentary

Change `ai_provider` only after the deterministic report works:

- `openai`: save an OpenAI API key in the optional OpenAI credential slot.
- `deepseek`: save a DeepSeek API key in the optional DeepSeek credential slot.
- `none`: skips all LLM nodes and sends the deterministic report.

Only the selected branch executes. The other AI and Telegram branches are skipped by the workflow engine.

## Security boundary

- No API key or Telegram token belongs in `pack.json` or the workflow JSON.
- Credentials are stored through the Goflow credential store.
- The website catalog does not receive runtime credentials.
- The portable artifact must not contain runtime database, master key, config, setup state, or credential binding files.
