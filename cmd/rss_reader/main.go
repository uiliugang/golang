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
	if len(os.Args) > 1 && os.Args[1] == "opml" {
		handleOPMLSubcommand()
		return
	}

	url := flag.String("url", "", "RSS feed URL (for reading feeds)")
	limit := flag.Int("limit", 5, "Number of items to print")
	timeout := flag.Duration("timeout", 10*time.Second, "Request timeout (e.g. 10s)")
	importOPML := flag.String("import-opml", "", "Import feeds from OPML file and display them")
	exportOPML := flag.String("export-opml", "", "Export specified feeds to OPML file")
	feedList := flag.String("feeds", "", "Comma-separated list of feeds: 'title1|url1|cat1,title2|url2|cat2'")
	dedup := flag.Bool("dedup", false, "Deduplicate feeds by URL on import")
	flag.Parse()

	if *importOPML != "" {
		if err := handleImportOPML(*importOPML, *limit, *timeout, *dedup); err != nil {
			fmt.Fprintln(os.Stderr, "import failed:", err)
			os.Exit(1)
		}
		return
	}

	if *exportOPML != "" {
		if err := handleExportOPML(*exportOPML, *feedList); err != nil {
			fmt.Fprintln(os.Stderr, "export failed:", err)
			os.Exit(1)
		}
		return
	}

	if *url == "" {
		fmt.Fprintln(os.Stderr, "missing -url or -import-opml or -export-opml")
		fmt.Fprintln(os.Stderr, "subcommands: opml add|list|remove|update|validate|merge")
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

// handleOPMLSubcommand dispatches opml subcommands: add, list, remove, update, validate, merge.
func handleOPMLSubcommand() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml <command> [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  add       Add a feed to an OPML file")
		fmt.Fprintln(os.Stderr, "  list      List feeds in an OPML file")
		fmt.Fprintln(os.Stderr, "  remove    Remove a feed from an OPML file by index")
		fmt.Fprintln(os.Stderr, "  update    Update a feed in an OPML file by index")
		fmt.Fprintln(os.Stderr, "  validate  Validate an OPML file")
		fmt.Fprintln(os.Stderr, "  merge     Merge multiple OPML files into one")
		os.Exit(1)
	}

	subcmd := os.Args[2]
	switch subcmd {
	case "add":
		handleOPMLAdd()
	case "list":
		handleOPMLList()
	case "remove":
		handleOPMLRemove()
	case "update":
		handleOPMLUpdate()
	case "validate":
		handleOPMLValidate()
	case "merge":
		handleOPMLMerge()
	default:
		fmt.Fprintf(os.Stderr, "unknown opml subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func handleOPMLAdd() {
	fs := flag.NewFlagSet("opml add", flag.ExitOnError)
	file := fs.String("file", "", "OPML file path")
	title := fs.String("title", "", "Feed title")
	url := fs.String("url", "", "Feed XML URL")
	htmlURL := fs.String("html-url", "", "Feed HTML URL")
	category := fs.String("category", "", "Feed category")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml add -file feeds.opml -title BBC -url https://... [-category News]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *file == "" || *url == "" {
		fmt.Fprintln(os.Stderr, "error: -file and -url are required")
		fs.Usage()
		os.Exit(1)
	}

	feeds, _, err := opml.LoadFeeds(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load failed:", err)
		os.Exit(1)
	}

	newTitle := *title
	if newTitle == "" {
		newTitle = *url
	}
	feeds = append(feeds, opml.FeedItem{
		Title:   newTitle,
		XMLURL:  *url,
		HTMLURL: *htmlURL,
		Category: *category,
	})

	if err := opml.ExportFeeds(*file, feeds); err != nil {
		fmt.Fprintln(os.Stderr, "export failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Added feed: %s (%s)\n", newTitle, *url)
}

func handleOPMLList() {
	fs := flag.NewFlagSet("opml list", flag.ExitOnError)
	file := fs.String("file", "", "OPML file path")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml list -file feeds.opml")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: -file is required")
		fs.Usage()
		os.Exit(1)
	}

	feeds, _, err := opml.LoadFeeds(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load failed:", err)
		os.Exit(1)
	}

	if len(feeds) == 0 {
		fmt.Println("No feeds found.")
		return
	}

	fmt.Printf("Found %d feeds in %s:\n\n", len(feeds), *file)
	for i, feed := range feeds {
		fmt.Printf("[%d] %s\n", i+1, feed.Title)
		fmt.Printf("    URL: %s\n", feed.XMLURL)
		if feed.HTMLURL != "" {
			fmt.Printf("    Web: %s\n", feed.HTMLURL)
		}
		if feed.Category != "" {
			fmt.Printf("    Category: %s\n", feed.Category)
		}
		fmt.Println()
	}
}

func handleOPMLRemove() {
	fs := flag.NewFlagSet("opml remove", flag.ExitOnError)
	file := fs.String("file", "", "OPML file path")
	index := fs.Int("id", -1, "Feed index (1-based) to remove")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml remove -file feeds.opml -id 3")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Use 'rss_reader opml list -file feeds.opml' to see feed indices.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *file == "" || *index < 1 {
		fmt.Fprintln(os.Stderr, "error: -file and -id (>= 1) are required")
		fs.Usage()
		os.Exit(1)
	}

	feeds, _, err := opml.LoadFeeds(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load failed:", err)
		os.Exit(1)
	}

	idx := *index - 1
	if idx >= len(feeds) {
		fmt.Fprintf(os.Stderr, "error: index %d out of range (max: %d)\n", *index, len(feeds))
		os.Exit(1)
	}

	removed := feeds[idx]
	feeds = append(feeds[:idx], feeds[idx+1:]...)

	if err := opml.ExportFeeds(*file, feeds); err != nil {
		fmt.Fprintln(os.Stderr, "export failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Removed feed: [%d] %s (%s)\n", *index, removed.Title, removed.XMLURL)
}

func handleOPMLUpdate() {
	fs := flag.NewFlagSet("opml update", flag.ExitOnError)
	file := fs.String("file", "", "OPML file path")
	index := fs.Int("id", -1, "Feed index (1-based) to update")
	title := fs.String("title", "", "New feed title")
	url := fs.String("url", "", "New feed XML URL")
	htmlURL := fs.String("html-url", "", "New feed HTML URL")
	category := fs.String("category", "", "New feed category")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml update -file feeds.opml -id 3 -title NewTitle [-url new-url]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *file == "" || *index < 1 {
		fmt.Fprintln(os.Stderr, "error: -file and -id (>= 1) are required")
		fs.Usage()
		os.Exit(1)
	}

	feeds, _, err := opml.LoadFeeds(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load failed:", err)
		os.Exit(1)
	}

	idx := *index - 1
	if idx >= len(feeds) {
		fmt.Fprintf(os.Stderr, "error: index %d out of range (max: %d)\n", *index, len(feeds))
		os.Exit(1)
	}

	feed := &feeds[idx]
	if *title != "" {
		feed.Title = *title
	}
	if *url != "" {
		feed.XMLURL = *url
	}
	if *htmlURL != "" {
		feed.HTMLURL = *htmlURL
	}
	if *category != "" {
		feed.Category = *category
	}

	if err := opml.ExportFeeds(*file, feeds); err != nil {
		fmt.Fprintln(os.Stderr, "export failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Updated feed [%d]: %s (%s)\n", *index, feed.Title, feed.XMLURL)
}

func handleOPMLValidate() {
	fs := flag.NewFlagSet("opml validate", flag.ExitOnError)
	file := fs.String("file", "", "OPML file path")
	fix := fs.Bool("fix", false, "Attempt to auto-repair issues and rewrite the file")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml validate -file feeds.opml [-fix]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: -file is required")
		fs.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read failed:", err)
		os.Exit(1)
	}

	result := opml.Validate(data)

	if result.Valid {
		fmt.Println("OPML file is valid.")
	} else {
		fmt.Println("OPML file has errors:")
	}

	for _, e := range result.Errors {
		fmt.Printf("  - %s\n", e)
	}

	if *fix && len(result.Feeds) > 0 {
		if err := opml.ExportFeeds(*file, result.Feeds); err != nil {
			fmt.Fprintln(os.Stderr, "fix failed:", err)
			os.Exit(1)
		}
		fmt.Printf("\nFixed: rewrote %s with %d feeds.\n", *file, len(result.Feeds))
	}

	if !result.Valid {
		os.Exit(1)
	}
}

func handleOPMLMerge() {
	fs := flag.NewFlagSet("opml merge", flag.ExitOnError)
	output := fs.String("output", "", "Output OPML file path")
	dedup := fs.Bool("dedup", true, "Deduplicate feeds by URL")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rss_reader opml merge -output merged.opml file1.opml file2.opml ...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[3:]); err != nil {
		os.Exit(1)
	}

	if *output == "" {
		fmt.Fprintln(os.Stderr, "error: -output is required")
		fs.Usage()
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: at least one input OPML file is required")
		fs.Usage()
		os.Exit(1)
	}

	feeds, err := opml.MergeFeeds(fs.Args(), *dedup)
	if err != nil {
		fmt.Fprintln(os.Stderr, "merge failed:", err)
		os.Exit(1)
	}

	if len(feeds) == 0 {
		fmt.Fprintln(os.Stderr, "no feeds found in input files")
		os.Exit(1)
	}

	if err := opml.ExportFeeds(*output, feeds); err != nil {
		fmt.Fprintln(os.Stderr, "export failed:", err)
		os.Exit(1)
	}

	fmt.Printf("Merged %d feeds from %d files into %s\n", len(feeds), len(fs.Args()), *output)
	for i, f := range fs.Args() {
		fmt.Printf("  Input %d: %s\n", i+1, f)
	}
}

// ParseFeedList parses a feed specification string.
// Format: "title1|url1|category1,title2|url2|category2" or "url1,url2".
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

		parts := strings.Split(entry, "|")
		switch {
		case len(parts) >= 2:
			title := strings.TrimSpace(parts[0])
			xmlURL := strings.TrimSpace(parts[1])
			feed := opml.FeedItem{
				Title:  title,
				XMLURL: xmlURL,
			}
			if feed.Title == "" {
				feed.Title = feed.XMLURL
			}
			if len(parts) >= 3 {
				feed.Category = strings.TrimSpace(parts[2])
			}
			feeds = append(feeds, feed)
		default:
			feeds = append(feeds, opml.FeedItem{
				Title:  entry,
				XMLURL: entry,
			})
		}
	}

	return feeds
}

func handleImportOPML(filename string, limit int, timeout time.Duration, dedup bool) error {
	feeds, err := opml.ImportFeeds(filename)
	if err != nil {
		return err
	}

	if dedup {
		before := len(feeds)
		feeds = opml.DedupFeeds(feeds)
		if removed := before - len(feeds); removed > 0 {
			fmt.Printf("Removed %d duplicate feeds\n", removed)
		}
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
