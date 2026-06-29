package opml

import (
	"strings"
	"testing"
)

func TestExportFeeds(t *testing.T) {
	feeds := []FeedItem{
		{
			Title:    "BBC News",
			XMLURL:   "https://feeds.bbci.co.uk/news/rss.xml",
			HTMLURL:  "https://www.bbc.com/news",
			Category: "News",
		},
		{
			Title:    "CNN News",
			XMLURL:   "https://feeds.cnn.com/rss/news.rss",
			HTMLURL:  "https://www.cnn.com",
			Category: "News",
		},
		{
			Title:    "TechCrunch",
			XMLURL:   "https://techcrunch.com/feed/",
			HTMLURL:  "https://techcrunch.com",
			Category: "Technology",
		},
		{
			Title:   "Golang Blog",
			XMLURL:  "https://go.dev/blog/feed.atom",
			HTMLURL: "https://go.dev/blog",
		},
	}

	filename := "/tmp/test_feeds.opml"
	err := ExportFeeds(filename, feeds)
	if err != nil {
		t.Fatalf("ExportFeeds failed: %v", err)
	}

	importedFeeds, err := ImportFeeds(filename)
	if err != nil {
		t.Fatalf("ImportFeeds failed: %v", err)
	}

	if len(importedFeeds) != len(feeds) {
		t.Errorf("Expected %d feeds, got %d", len(feeds), len(importedFeeds))
	}

	// Check that all feeds are present
	feedMap := make(map[string]bool)
	for _, feed := range importedFeeds {
		feedMap[feed.XMLURL] = true
	}

	for _, feed := range feeds {
		if !feedMap[feed.XMLURL] {
			t.Errorf("Feed %s not found in imported feeds", feed.Title)
		}
	}
}

func TestImportFeedsFromReader(t *testing.T) {
	opmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>RSS Feeds</title>
    <dateCreated>2026-05-30T18:00:00Z</dateCreated>
    <dateModified>2026-05-30T18:00:00Z</dateModified>
  </head>
  <body>
    <outline text="News" title="News">
      <outline text="BBC News" title="BBC News" type="rss" xmlUrl="https://feeds.bbci.co.uk/news/rss.xml" htmlUrl="https://www.bbc.com/news"/>
      <outline text="CNN News" title="CNN News" type="rss" xmlUrl="https://feeds.cnn.com/rss/news.rss" htmlUrl="https://www.cnn.com"/>
    </outline>
    <outline text="Technology" title="Technology">
      <outline text="TechCrunch" title="TechCrunch" type="rss" xmlUrl="https://techcrunch.com/feed/" htmlUrl="https://techcrunch.com"/>
    </outline>
  </body>
</opml>`

	reader := strings.NewReader(opmlContent)
	feeds, err := ImportFeedsFromReader(reader)
	if err != nil {
		t.Fatalf("ImportFeedsFromReader failed: %v", err)
	}

	if len(feeds) != 3 {
		t.Errorf("Expected 3 feeds, got %d", len(feeds))
	}

	// Check categories
	newsCount := 0
	techCount := 0
	for _, feed := range feeds {
		if feed.Category == "News" {
			newsCount++
		} else if feed.Category == "Technology" {
			techCount++
		}
	}

	if newsCount != 2 {
		t.Errorf("Expected 2 News feeds, got %d", newsCount)
	}
	if techCount != 1 {
		t.Errorf("Expected 1 Technology feed, got %d", techCount)
	}
}

func TestImportFeedsFromReaderInvalid(t *testing.T) {
	invalidContent := `<?xml version="1.0"?>
<opml version="2.0">
  <head><title>Empty</title></head>
  <body></body>
</opml>`

	reader := strings.NewReader(invalidContent)
	_, err := ImportFeedsFromReader(reader)
	if err == nil {
		t.Error("Expected error for empty OPML, got nil")
	}
}

func TestExportAndImportRoundTrip(t *testing.T) {
	originalFeeds := []FeedItem{
		{
			Title:    "Go Blog",
			XMLURL:   "https://go.dev/blog/feed.atom",
			HTMLURL:  "https://go.dev",
			Category: "Programming",
		},
		{
			Title:    "Python Weekly",
			XMLURL:   "https://pythonweekly.com/feed.xml",
			HTMLURL:  "https://pythonweekly.com",
			Category: "Programming",
		},
		{
			Title:   "Hacker News",
			XMLURL:  "https://news.ycombinator.com/rss",
			HTMLURL: "https://news.ycombinator.com",
		},
	}

	// Export to buffer
	filename := "/tmp/roundtrip_test.opml"
	err := ExportFeeds(filename, originalFeeds)
	if err != nil {
		t.Fatalf("ExportFeeds failed: %v", err)
	}

	// Import back
	importedFeeds, err := ImportFeeds(filename)
	if err != nil {
		t.Fatalf("ImportFeeds failed: %v", err)
	}

	if len(importedFeeds) != len(originalFeeds) {
		t.Errorf("Expected %d feeds, got %d", len(originalFeeds), len(importedFeeds))
	}

	// Verify all URLs are present
	importedURLs := make(map[string]bool)
	for _, feed := range importedFeeds {
		importedURLs[feed.XMLURL] = true
	}

	for _, feed := range originalFeeds {
		if !importedURLs[feed.XMLURL] {
			t.Errorf("Feed URL %s not found in imported feeds", feed.XMLURL)
		}
	}
}
