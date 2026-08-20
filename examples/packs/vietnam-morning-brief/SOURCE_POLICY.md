# Vietnam Morning Brief source policy

Reviewed for the 0.1.0 personal pilot on **2026-08-19**.

This document is a product/compliance boundary for the reference Pack, not legal advice or a permanent license determination.

## Pilot status

The bundled source list is **personal/non-commercial pilot only**. Do not treat inclusion in this repository as permission for commercial redistribution.

Before any commercial release or paid service uses a publisher feed, re-check the publisher's current RSS/Terms of Use and obtain permission where the intended use is not clearly allowed. Remove a source if the publisher requests that use stop or if its terms no longer fit the product.

## Source-specific notes

### VnExpress

- Official RSS directory: `https://vnexpress.net/rss`
- Feed used by the pilot: `https://vnexpress.net/rss/tin-moi-nhat.rss`
- The RSS directory states that feeds are provided free to individuals and non-profit organizations and that VnExpress may request that redistribution stop.
- Commercial use therefore remains **not cleared by this pilot**.

### Tuổi Trẻ

- Official RSS directory: `https://tuoitre.vn/rss.htm`
- Feed used by the pilot: `https://tuoitre.vn/home.rss`
- The RSS directory states that feeds are provided free to individuals and non-profit organizations and that Tuổi Trẻ may request that redistribution stop.
- Commercial use therefore remains **not cleared by this pilot**.

### Thanh Niên

- Official RSS directory: `https://thanhnien.vn/rss.html`
- Feed used by the pilot: `https://thanhnien.vn/rss/home.rss`
- The publisher explicitly provides RSS feed URLs for RSS readers. The 0.1.0 review did not establish a blanket commercial redistribution permission, so commercial use remains **not cleared by this pilot**.

## Technical collection boundary

The Pack reads only publisher-provided RSS/Atom feeds through Goflow's bounded `rssFeedSource` node.

It does not:

- crawl article bodies;
- scrape publisher HTML pages;
- execute publisher JavaScript;
- log in to publisher accounts;
- bypass paywalls, CAPTCHAs, robots/access controls, or rate limits;
- use proxy rotation to evade blocking;
- copy publisher images;
- republish full article text.

For each feed item, the workflow uses only the feed-provided metadata needed for the brief: publisher, title, short feed summary, publication time, category and original URL.

## Output boundary

The output is a concise derived digest. Every delivered story must retain at least one original publisher link. When AI is enabled, the model may select/group/summarize only supplied candidate IDs; it is not trusted to produce source URLs. The final formatter resolves IDs back to original RSS items and constructs source links itself.

If AI output is malformed or contains no valid candidate IDs, Goflow falls back to the deterministic source-linked digest.

If one feed fails, the brief may continue from remaining feeds and notes the partial-source condition. If all feeds fail, the source node fails the run rather than generating an ungrounded brief.
