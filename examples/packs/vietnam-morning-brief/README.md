# Vietnam Morning Brief

`official.vietnam-morning-brief` is a small reference Pack for a daily, source-linked Vietnamese news digest.

## What it does

1. Reads recent items from a small set of publisher-provided public RSS feeds.
2. Keeps only items published in the last 24 hours.
3. Normalizes and removes basic duplicates by canonical URL and normalized title.
4. Builds a deterministic Telegram digest from the feed title/summary/link.
5. Optionally asks OpenAI or DeepSeek to select and summarize up to 10 notable stories.
6. Reattaches publisher names and original URLs from the RSS data after AI output is parsed, so AI never supplies the delivery links.
7. Sends the result to Telegram.

If one source is temporarily unavailable, the workflow continues with the remaining sources. If all configured feeds fail, the run fails instead of producing an ungrounded brief.

## First run

You only need:

- a Telegram bot token;
- the Telegram chat/group/channel ID that should receive the brief.

Keep **AI summarization = none** for the first run. This sends the latest source-linked RSS items without requiring an AI key.

After the deterministic version works, you can optionally choose OpenAI or DeepSeek and add the corresponding encrypted credential. AI is used only to select/group/summarize supplied candidates. The final source links are reconstructed from the original RSS items.

## Daily schedule

After setup and a successful manual run, configure the Pack Run daily schedule. A useful pilot default is around `07:30` in `Asia/Ho_Chi_Minh`, but the actual time is user-controlled.

## Source boundary

This reference Pack intentionally uses RSS only. It does **not**:

- scrape full article HTML;
- automate a browser;
- sign in to publisher accounts;
- bypass paywalls, CAPTCHAs, rate limits, or access controls;
- republish full article bodies or publisher images.

The delivered brief contains a short derived digest and links readers back to the original publisher pages.

See [SOURCE_POLICY.md](SOURCE_POLICY.md) and [sources.json](sources.json). The bundled source list is for a personal/non-commercial pilot while source-specific terms and permissions are evaluated for any future commercial distribution.

## Current pilot sources

- VnExpress — latest-news RSS
- Tuổi Trẻ — home RSS
- Thanh Niên — home RSS

Source availability and terms can change. A future commercial release must re-check the publisher terms and remove/replace any source that does not permit the intended use.
