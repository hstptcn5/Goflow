# Changelog

## 0.1.0

- Initial personal-pilot Vietnam Morning Brief Pack.
- Reads bounded publisher-provided RSS/Atom feeds through the generic `rssFeedSource` node.
- Filters to a 24-hour lookback and removes basic URL/title duplicates.
- Sends a deterministic source-linked Telegram digest without requiring AI.
- Optional OpenAI or DeepSeek selection/summarization with source URLs reattached from original RSS items.
- Includes source/compliance manifest and explicit no-scraping boundary.
