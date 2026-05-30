package main

import (
"context"
"encoding/xml"
"errors"
"flag"
"fmt"
"io"
"net/http"
"os"
"strings"
"time"

"github.com/uiliugang/golang-learning/internal/opml"
)

const maxFeedBytes = 5 << 20

type rssDocument struct {
Channel Channel `xml:"channel"`
}

type Channel struct {
Title       string `xml:"title"`
Link        string `xml:"link"`
Description string `xml:"description"`
Items       []Item `xml:"item"`
}

type Item struct {
Title   string `xml:"title"`
Link    string `xml:"link"`
PubDate string `xml:"pubDate"`
}

func main() {
url := flag.String("url", "", "RSS feed URL (for reading feeds)")
limit := flag.Int("limit", 5, "Number of items to print")
timeout := flag.Duration("timeout", 10*time.Second, "Request timeout (e.g. 10s)")
importOPML := flag.String("import-opml", "", "Import feeds from OPML file and display them")
exportOPML := flag.String("export-opml", "", "Export specified feeds to OPML file")
feedList := flag.String("feeds", "", "Comma-separated list of feeds: 'title1|url1|cat1,title2|url2|cat2'")
flag.Parse()

// Handle OPML import
if *importOPML != "" {
if err := handleImportOPML(*importOPML, *limit, *timeout); err != nil {
fmt.Fprintln(os.Stderr, "import failed:", err)
os.Exit(1)
}
return
}

// Handle OPML export
if *exportOPML != "" {
if err := handleExportOPML(*exportOPML, *feedList); err != nil {
fmt.Fprintln(os.Stderr, "export failed:", err)
os.Exit(1)
}
return
}

// Default: fetch and display a single RSS feed
if *url == "" {
fmt.Fprintln(os.Stderr, "missing -url or -import-opml or -export-opml")
flag.Usage()
os.Exit(1)
}

if *limit < 1 {
fmt.Fprintln(os.Stderr, "limit must be positive")
os.Exit(1)
}

ctx, cancel := context.WithTimeout(context.Background(), *timeout)
defer cancel()

doc, err := fetchFeed(ctx, *url)
if err != nil {
fmt.Fprintln(os.Stderr, "fetch feed failed:", err)
os.Exit(1)
}

printFeed(doc.Channel, *limit)
}

// ParseFeedList parses a feed specification string
// Format: "title1|url1|category1,title2|url2|category2" or "url1,url2"
// Using | instead of : to avoid conflicts with URLs like https://
func ParseFeedList(feedSpec string) []opml.FeedItem {
var feeds []opml.FeedItem

if feedSpec == "" {
return feeds
}

entries := strings.Split(feedSpec, ",")
for _, entry := range entries {
entry = strings.TrimSpace(entry)
if entry == "" {
continue
}

// Try to parse as "title|url|category" format
parts := strings.Split(entry, "|")
if len(parts) >= 2 {
feed := opml.FeedItem{
Title:  strings.TrimSpace(parts[0]),
XMLURL: strings.TrimSpace(parts[1]),
}
if len(parts) >= 3 {
feed.Category = strings.TrimSpace(parts[2])
}
feeds = append(feeds, feed)
} else {
// Just a URL
feeds = append(feeds, opml.FeedItem{
Title:  entry,
XMLURL: entry,
})
}
}

return feeds
}

// handleImportOPML imports feeds from an OPML file and displays them
func handleImportOPML(filename string, limit int, timeout time.Duration) error {
feeds, err := opml.ImportFeeds(filename)
if err != nil {
return err
}

fmt.Printf("Imported %d feeds from %s\n\n", len(feeds), filename)

ctx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()

for i, feed := range feeds {
fmt.Printf("[%d/%d] %s\n", i+1, len(feeds), feed.Title)
if feed.Category != "" {
fmt.Printf("Category: %s\n", feed.Category)
}
fmt.Printf("URL: %s\n", feed.XMLURL)

doc, err := fetchFeed(ctx, feed.XMLURL)
if err != nil {
fmt.Printf("Error fetching: %v\n\n", err)
continue
}

printFeed(doc.Channel, limit)
fmt.Println("---")
}

return nil
}

// handleExportOPML exports feeds to an OPML file
func handleExportOPML(filename, feedList string) error {
if feedList == "" {
return fmt.Errorf("missing -feeds parameter for export\nUsage: -export-opml file.opml -feeds 'title1|url1|cat1,title2|url2|cat2'")
}

feeds := ParseFeedList(feedList)
if len(feeds) == 0 {
return fmt.Errorf("no feeds provided")
}

if err := opml.ExportFeeds(filename, feeds); err != nil {
return err
}

fmt.Printf("Successfully exported %d feeds to %s\n", len(feeds), filename)

// Print exported feeds for verification
for _, feed := range feeds {
fmt.Printf("  - %s (%s)", feed.Title, feed.XMLURL)
if feed.Category != "" {
fmt.Printf(" [%s]", feed.Category)
}
fmt.Println()
}

return nil
}

func fetchFeed(ctx context.Context, url string) (rssDocument, error) {
client := &http.Client{}

req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
if err != nil {
return rssDocument{}, err
}
req.Header.Set("User-Agent", "simple-rss-reader/1.0")

resp, err := client.Do(req)
if err != nil {
return rssDocument{}, err
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
return rssDocument{}, fmt.Errorf("unexpected status: %s", resp.Status)
}

body, err := readWithLimit(resp.Body, maxFeedBytes)
if err != nil {
return rssDocument{}, err
}

var doc rssDocument
if err := xml.Unmarshal(body, &doc); err != nil {
return rssDocument{}, err
}
if doc.Channel.Title == "" && len(doc.Channel.Items) == 0 {
return rssDocument{}, errors.New("unrecognized RSS feed format")
}

return doc, nil
}

func readWithLimit(r io.Reader, limit int64) ([]byte, error) {
data, err := io.ReadAll(io.LimitReader(r, limit+1))
if err != nil {
return nil, err
}
if int64(len(data)) > limit {
return nil, fmt.Errorf("feed exceeds %d bytes", limit)
}
return data, nil
}

func printFeed(channel Channel, limit int) {
fmt.Printf("Feed: %s\n%s\n\n", channel.Title, channel.Link)
if len(channel.Items) == 0 {
fmt.Println("No items found.")
return
}

count := len(channel.Items)
if limit > 0 && limit < count {
count = limit
}

for i := 0; i < count; i++ {
item := channel.Items[i]
fmt.Printf("%d. %s\n", i+1, item.Title)
if item.Link != "" {
fmt.Printf("   %s\n", item.Link)
}
if item.PubDate != "" {
fmt.Printf("   %s\n", item.PubDate)
}
fmt.Println()
}
}
