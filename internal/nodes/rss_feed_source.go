package nodes

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"goflow/internal/sourceprobe"
)

const (
	maxRSSFeeds               = 12
	maxRSSFeedBytes     int64 = 2 << 20
	maxRSSItems               = 200
	maxRSSPerFeedItems        = 100
	maxRSSLookbackHours       = 168
)

var (
	rssHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)
	rssSpacePattern   = regexp.MustCompile(`\s+`)
)

type RSSFeedSourceExecutor struct {
	client *http.Client
	now    func() time.Time
}

type rssFeedSpec struct {
	ID        string `json:"id"`
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
	Category  string `json:"category,omitempty"`
}

type rssNormalizedItem struct {
	SourceID    string
	Publisher   string
	Category    string
	Title       string
	Summary     string
	URL         string
	PublishedAt time.Time
}

type rssXMLItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Date        string `xml:"date"`
	Description string `xml:"description"`
}

type atomXMLLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomXMLEntry struct {
	Title     string        `xml:"title"`
	Links     []atomXMLLink `xml:"link"`
	Published string        `xml:"published"`
	Updated   string        `xml:"updated"`
	Summary   string        `xml:"summary"`
	Content   string        `xml:"content"`
}

type feedXMLDocument struct {
	XMLName xml.Name
	Channel struct {
		Items []rssXMLItem `xml:"item"`
	} `xml:"channel"`
	Items   []rssXMLItem   `xml:"item"`
	Entries []atomXMLEntry `xml:"entry"`
}

func NewRSSFeedSourceExecutor() *RSSFeedSourceExecutor {
	return NewRSSFeedSourceExecutorWithClient(nil, nil)
}

func NewRSSFeedSourceExecutorWithClient(client *http.Client, now func() time.Time) *RSSFeedSourceExecutor {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: sourceprobe.SafeRedirect}
	} else {
		bounded := *client
		if bounded.Timeout <= 0 || bounded.Timeout > 15*time.Second {
			bounded.Timeout = 15 * time.Second
		}
		bounded.CheckRedirect = sourceprobe.SafeRedirect
		client = &bounded
	}
	if now == nil {
		now = time.Now
	}
	return &RSSFeedSourceExecutor{client: client, now: now}
}

func (e *RSSFeedSourceExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	feeds, lookbackHours, maxItems, perFeedMax, err := rssFeedSourceParams(node)
	if err != nil {
		return nil, err
	}

	now := e.now().UTC()
	cutoff := now.Add(-time.Duration(lookbackHours) * time.Hour)
	items := make([]rssNormalizedItem, 0, minInt(maxItems, len(feeds)*perFeedMax))
	sourceErrors := make([]map[string]interface{}, 0)
	succeeded := 0
	skippedUndated := 0

	for _, feed := range feeds {
		feedItems, undated, fetchErr := e.fetchFeed(ctx, feed, now, cutoff, perFeedMax)
		if fetchErr != nil {
			sourceErrors = append(sourceErrors, map[string]interface{}{
				"source_id": feed.ID,
				"publisher": feed.Publisher,
				"error":     fetchErr.Error(),
			})
			continue
		}
		succeeded++
		skippedUndated += undated
		items = append(items, feedItems...)
	}

	if succeeded == 0 {
		return nil, fmt.Errorf("RSS Feed Source could not read any configured feed")
	}

	items = dedupeRSSItems(items)
	sort.SliceStable(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	outputItems := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		outputItems = append(outputItems, map[string]interface{}{
			"source_id":    item.SourceID,
			"publisher":    item.Publisher,
			"category":     item.Category,
			"title":        item.Title,
			"summary":      item.Summary,
			"url":          item.URL,
			"published_at": item.PublishedAt.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"fetched_at":      now.Format(time.RFC3339),
		"lookback_hours":  lookbackHours,
		"source_count":    len(feeds),
		"sources_ok":      succeeded,
		"item_count":      len(outputItems),
		"items":           outputItems,
		"source_errors":   sourceErrors,
		"skipped_undated": skippedUndated,
	}, nil
}

func (e *RSSFeedSourceExecutor) fetchFeed(ctx *ExecutionContext, feed rssFeedSpec, now, cutoff time.Time, perFeedMax int) ([]rssNormalizedItem, int, error) {
	req, err := http.NewRequestWithContext(ctx.Context, http.MethodGet, feed.URL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("could not create feed request")
	}
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "Goflow RSS Reader/1.0 (+https://github.com/hstptcn5/Goflow)")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("feed is unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRSSFeedBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("feed response could not be read")
	}
	if int64(len(data)) > maxRSSFeedBytes {
		return nil, 0, fmt.Errorf("feed exceeds %d byte limit", maxRSSFeedBytes)
	}
	parsed, undated, err := parseFeedXML(data, feed, now, cutoff)
	if err != nil {
		return nil, 0, err
	}
	if len(parsed) > perFeedMax {
		parsed = parsed[:perFeedMax]
	}
	return parsed, undated, nil
}

func (e *RSSFeedSourceExecutor) Validate(node *Node) error {
	_, _, _, _, err := rssFeedSourceParams(node)
	return err
}

func (e *RSSFeedSourceExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeRSSFeedSource, Name: "RSS Feed Source", Icon: "Rss", Category: "ACTION", Retryable: true,
		Description: "Reads bounded public RSS/Atom feeds and returns normalized recent items with source attribution",
		Params: []ParamDefinition{
			{Name: "feeds", Label: "RSS / Atom feeds", Type: "json", Default: []interface{}{}, Required: true, Description: "Array of {id, publisher, url, category?}. Use publisher-provided public feeds only."},
			{Name: "lookback_hours", Label: "Lookback hours", Type: "integer", Default: 24, Required: true, Description: "Only include items published inside this window"},
			{Name: "max_items", Label: "Maximum items", Type: "integer", Default: 60, Required: true, Description: "Maximum normalized items returned after deduplication"},
			{Name: "per_feed_max_items", Label: "Maximum items per feed", Type: "integer", Default: 30, Required: true, Description: "Bound applied independently to each feed"},
		},
	}
}

func rssFeedSourceParams(node *Node) ([]rssFeedSpec, int, int, int, error) {
	feeds, err := parseRSSFeedSpecs(node.Params["feeds"])
	if err != nil {
		return nil, 0, 0, 0, err
	}
	lookback, err := rssPositiveInt(node.Params["lookback_hours"], 24, maxRSSLookbackHours)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("RSS lookback_hours %w", err)
	}
	maxItems, err := rssPositiveInt(node.Params["max_items"], 60, maxRSSItems)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("RSS max_items %w", err)
	}
	perFeed, err := rssPositiveInt(node.Params["per_feed_max_items"], 30, maxRSSPerFeedItems)
	if err != nil {
		return nil, 0, 0, 0, fmt.Errorf("RSS per_feed_max_items %w", err)
	}
	return feeds, lookback, maxItems, perFeed, nil
}

func parseRSSFeedSpecs(raw interface{}) ([]rssFeedSpec, error) {
	if raw == nil {
		return nil, fmt.Errorf("RSS Feed Source requires at least one feed")
	}
	var data []byte
	var err error
	if text, ok := raw.(string); ok {
		data = []byte(text)
	} else {
		data, err = json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("RSS feeds must be a JSON array")
		}
	}
	var feeds []rssFeedSpec
	if err := json.Unmarshal(data, &feeds); err != nil {
		return nil, fmt.Errorf("RSS feeds must be a JSON array of feed objects")
	}
	if len(feeds) == 0 {
		return nil, fmt.Errorf("RSS Feed Source requires at least one feed")
	}
	if len(feeds) > maxRSSFeeds {
		return nil, fmt.Errorf("RSS Feed Source supports at most %d feeds", maxRSSFeeds)
	}
	seen := map[string]struct{}{}
	for i := range feeds {
		feeds[i].ID = strings.TrimSpace(feeds[i].ID)
		feeds[i].Publisher = strings.TrimSpace(feeds[i].Publisher)
		feeds[i].URL = strings.TrimSpace(feeds[i].URL)
		feeds[i].Category = strings.TrimSpace(feeds[i].Category)
		if feeds[i].ID == "" || feeds[i].Publisher == "" || feeds[i].URL == "" {
			return nil, fmt.Errorf("RSS feed %d requires id, publisher, and url", i+1)
		}
		if _, exists := seen[feeds[i].ID]; exists {
			return nil, fmt.Errorf("RSS feed id %q is duplicated", feeds[i].ID)
		}
		seen[feeds[i].ID] = struct{}{}
		if err := sourceprobe.ValidateURL(feeds[i].URL); err != nil {
			return nil, fmt.Errorf("RSS feed %q URL must be absolute http or https", feeds[i].ID)
		}
	}
	return feeds, nil
}

func rssPositiveInt(raw interface{}, fallback, max int) (int, error) {
	if raw == nil {
		return fallback, nil
	}
	value := 0
	switch typed := raw.(type) {
	case int:
		value = typed
	case int64:
		value = int(typed)
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("must be an integer")
		}
		value = int(typed)
	case string:
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &value); err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
	default:
		return 0, fmt.Errorf("must be an integer")
	}
	if value <= 0 || value > max {
		return 0, fmt.Errorf("must be between 1 and %d", max)
	}
	return value, nil
}

func parseFeedXML(data []byte, feed rssFeedSpec, now, cutoff time.Time) ([]rssNormalizedItem, int, error) {
	var doc feedXMLDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("feed is not valid RSS/Atom XML")
	}
	result := make([]rssNormalizedItem, 0)
	undated := 0
	appendRSS := func(item rssXMLItem) {
		published, ok := parseFeedDate(firstNonEmpty(item.PubDate, item.Date))
		if !ok {
			undated++
			return
		}
		if published.Before(cutoff) || published.After(now.Add(10*time.Minute)) {
			return
		}
		link := firstNonEmpty(item.Link, item.GUID)
		resolved := resolveFeedItemURL(feed.URL, link)
		if resolved == "" || strings.TrimSpace(item.Title) == "" {
			return
		}
		result = append(result, rssNormalizedItem{
			SourceID: feed.ID, Publisher: feed.Publisher, Category: feed.Category,
			Title: cleanFeedText(item.Title, 500), Summary: cleanFeedText(item.Description, 1200),
			URL: resolved, PublishedAt: published.UTC(),
		})
	}
	for _, item := range doc.Channel.Items {
		appendRSS(item)
	}
	for _, item := range doc.Items {
		appendRSS(item)
	}
	for _, entry := range doc.Entries {
		published, ok := parseFeedDate(firstNonEmpty(entry.Published, entry.Updated))
		if !ok {
			undated++
			continue
		}
		if published.Before(cutoff) || published.After(now.Add(10*time.Minute)) {
			continue
		}
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Href == "" {
				continue
			}
			if candidate.Rel == "" || strings.EqualFold(candidate.Rel, "alternate") {
				link = candidate.Href
				break
			}
			if link == "" {
				link = candidate.Href
			}
		}
		resolved := resolveFeedItemURL(feed.URL, link)
		if resolved == "" || strings.TrimSpace(entry.Title) == "" {
			continue
		}
		result = append(result, rssNormalizedItem{
			SourceID: feed.ID, Publisher: feed.Publisher, Category: feed.Category,
			Title: cleanFeedText(entry.Title, 500), Summary: cleanFeedText(firstNonEmpty(entry.Summary, entry.Content), 1200),
			URL: resolved, PublishedAt: published.UTC(),
		})
	}
	if len(doc.Channel.Items) == 0 && len(doc.Items) == 0 && len(doc.Entries) == 0 {
		return nil, undated, fmt.Errorf("feed contains no RSS items or Atom entries")
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].PublishedAt.After(result[j].PublishedAt) })
	return result, undated, nil
}

func parseFeedDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano, time.RFC3339,
		time.RFC1123Z, time.RFC1123,
		time.RFC822Z, time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05-0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func resolveFeedItemURL(feedURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	base, err := url.Parse(feedURL)
	if err != nil {
		return ""
	}
	candidate, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(candidate)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	resolved.Fragment = ""
	query := resolved.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			query.Del(key)
		}
	}
	resolved.RawQuery = query.Encode()
	return resolved.String()
}

func dedupeRSSItems(items []rssNormalizedItem) []rssNormalizedItem {
	seenURL := map[string]struct{}{}
	seenTitle := map[string]struct{}{}
	result := make([]rssNormalizedItem, 0, len(items))
	for _, item := range items {
		urlKey := strings.ToLower(strings.TrimSpace(item.URL))
		titleKey := normalizeFeedTitle(item.Title)
		if urlKey != "" {
			if _, exists := seenURL[urlKey]; exists {
				continue
			}
		}
		if titleKey != "" {
			if _, exists := seenTitle[titleKey]; exists {
				continue
			}
		}
		if urlKey != "" {
			seenURL[urlKey] = struct{}{}
		}
		if titleKey != "" {
			seenTitle[titleKey] = struct{}{}
		}
		result = append(result, item)
	}
	return result
}

func normalizeFeedTitle(value string) string {
	value = strings.ToLower(cleanFeedText(value, 600))
	var builder strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space && builder.Len() > 0 {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func cleanFeedText(value string, limit int) string {
	value = rssHTMLTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = rssSpacePattern.ReplaceAllString(value, " ")
	value = strings.TrimSpace(value)
	if limit > 0 && len([]rune(value)) > limit {
		runes := []rune(value)
		value = strings.TrimSpace(string(runes[:limit])) + "…"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
