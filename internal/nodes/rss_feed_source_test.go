package nodes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRSSFeedSourceNormalizesRSSAndAtomWithPartialFailure(t *testing.T) {
	now := time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rss":
			w.Header().Set("Content-Type", "application/rss+xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>A</title>
			<item><title>Tin A &amp; B</title><link>%s/story?utm_source=rss</link><pubDate>Wed, 19 Aug 2026 06:30:00 +0000</pubDate><description><![CDATA[<p>Tóm tắt <b>RSS</b></p>]]></description></item>
			<item><title>Tin cũ</title><link>%s/old</link><pubDate>Mon, 17 Aug 2026 06:30:00 +0000</pubDate></item>
			</channel></rss>`, server.URL, server.URL)
		case "/atom":
			w.Header().Set("Content-Type", "application/atom+xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
			<entry><title>Tin C</title><link rel="alternate" href="%s/c"/><published>2026-08-19T06:00:00Z</published><summary>Atom summary</summary></entry>
			<entry><title>Tin A &amp; B</title><link href="%s/duplicate"/><updated>2026-08-19T05:00:00Z</updated></entry>
			</feed>`, server.URL, server.URL)
		case "/bad":
			http.Error(w, "nope", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	executor := NewRSSFeedSourceExecutorWithClient(server.Client(), func() time.Time { return now })
	node := &Node{Params: map[string]interface{}{
		"feeds": []interface{}{
			map[string]interface{}{"id": "a", "publisher": "Publisher A", "url": server.URL + "/rss", "category": "news"},
			map[string]interface{}{"id": "b", "publisher": "Publisher B", "url": server.URL + "/atom", "category": "news"},
			map[string]interface{}{"id": "broken", "publisher": "Broken", "url": server.URL + "/bad"},
		},
		"lookback_hours":     24,
		"max_items":          20,
		"per_feed_max_items": 20,
	}}
	result, err := executor.Execute(NewExecutionContextWithContext(context.Background(), "wf", "exec"), node)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	output := result.(map[string]interface{})
	if output["sources_ok"] != 2 {
		t.Fatalf("sources_ok=%v", output["sources_ok"])
	}
	items := output["items"].([]map[string]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 deduplicated recent items, got %d: %#v", len(items), items)
	}
	if items[0]["title"] != "Tin A & B" || items[0]["summary"] != "Tóm tắt RSS" {
		t.Fatalf("unexpected normalized RSS item: %#v", items[0])
	}
	if items[0]["url"] != server.URL+"/story" {
		t.Fatalf("tracking query was not removed: %v", items[0]["url"])
	}
	if len(output["source_errors"].([]map[string]interface{})) != 1 {
		t.Fatalf("expected one source error: %#v", output["source_errors"])
	}
}

func TestRSSFeedSourceRejectsInvalidConfigAndAllSourcesFailing(t *testing.T) {
	executor := NewRSSFeedSourceExecutorWithClient(&http.Client{Timeout: time.Second}, func() time.Time {
		return time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC)
	})
	invalid := &Node{Params: map[string]interface{}{"feeds": []interface{}{
		map[string]interface{}{"id": "x", "publisher": "X", "url": "file:///tmp/feed.xml"},
	}}}
	if err := executor.Validate(invalid); err == nil {
		t.Fatal("expected invalid feed URL to fail validation")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	allFail := &Node{Params: map[string]interface{}{"feeds": []interface{}{
		map[string]interface{}{"id": "x", "publisher": "X", "url": server.URL},
	}}}
	if _, err := executor.Execute(NewExecutionContext("wf", "exec"), allFail); err == nil {
		t.Fatal("expected all feeds failing to return an error")
	}
}

func TestParseFeedDateSupportsCommonPublisherLayouts(t *testing.T) {
	for _, raw := range []string{
		"Wed, 19 Aug 2026 13:30:00 +0700",
		"2026-08-19T13:30:00+07:00",
		"Wed, 19 Aug 26 13:30:00 +0700",
	} {
		if _, ok := parseFeedDate(raw); !ok {
			t.Fatalf("date format was not parsed: %s", raw)
		}
	}
}
