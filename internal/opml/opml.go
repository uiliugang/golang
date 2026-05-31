package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// OPML represents an OPML (Outline Processor Markup Language) document.
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

// Head contains metadata about the OPML document.
type Head struct {
	Title        string `xml:"title,omitempty"`
	DateCreated  string `xml:"dateCreated,omitempty"`
	DateModified string `xml:"dateModified,omitempty"`
	OwnerName    string `xml:"ownerName,omitempty"`
	OwnerEmail   string `xml:"ownerEmail,omitempty"`
	Docs         string `xml:"docs,omitempty"`
}

// Body contains the outline elements.
type Body struct {
	Outlines []Outline `xml:"outline"`
}

// Outline represents an outline element which can be a category or a feed.
type Outline struct {
	XMLName  xml.Name  `xml:"outline"`
	Text     string    `xml:"text,attr,omitempty"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Children []Outline `xml:"outline,omitempty"`
}

// FeedItem represents a single RSS feed with its metadata.
type FeedItem struct {
	Title       string
	XMLURL      string
	HTMLURL     string
	Description string
	Category    string
}

// ExportFeeds exports a list of feeds to an OPML file.
func ExportFeeds(filename string, feeds []FeedItem) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("filename is required")
	}
	if len(feeds) == 0 {
		return fmt.Errorf("no feeds provided")
	}
	for i, feed := range feeds {
		if strings.TrimSpace(feed.XMLURL) == "" {
			return fmt.Errorf("feed %d is missing xml URL", i+1)
		}
		if strings.TrimSpace(feed.Title) == "" {
			feeds[i].Title = feed.XMLURL
		}
	}

	opmlDoc := OPML{
		Version: "2.0",
		Head: Head{
			Title:        "RSS Feeds",
			DateCreated:  time.Now().UTC().Format(time.RFC3339),
			DateModified: time.Now().UTC().Format(time.RFC3339),
			OwnerName:    "RSS Reader",
			Docs:         "http://www.opml.org/spec2.opml",
		},
	}

	categorized := make(map[string][]FeedItem)
	var uncategorized []FeedItem

	for _, feed := range feeds {
		category := strings.TrimSpace(feed.Category)
		if category == "" {
			uncategorized = append(uncategorized, feed)
			continue
		}
		categorized[category] = append(categorized[category], feed)
	}

	categoryNames := make([]string, 0, len(categorized))
	for category := range categorized {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		categoryFeeds := categorized[category]
		sortFeeds(categoryFeeds)

		outline := Outline{
			Text:  category,
			Title: category,
		}
		for _, feed := range categoryFeeds {
			outline.Children = append(outline.Children, feedToOutline(feed))
		}
		opmlDoc.Body.Outlines = append(opmlDoc.Body.Outlines, outline)
	}

	sortFeeds(uncategorized)
	for _, feed := range uncategorized {
		opmlDoc.Body.Outlines = append(opmlDoc.Body.Outlines, feedToOutline(feed))
	}

	xmlData, err := xml.MarshalIndent(opmlDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal OPML: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create OPML file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(xml.Header); err != nil {
		return fmt.Errorf("failed to write XML declaration: %w", err)
	}
	if _, err := file.Write(xmlData); err != nil {
		return fmt.Errorf("failed to write OPML data: %w", err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// ImportFeeds imports feeds from an OPML file.
func ImportFeeds(filename string) ([]FeedItem, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open OPML file: %w", err)
	}
	defer file.Close()

	return ImportFeedsFromReader(file)
}

// ImportFeedsFromReader imports feeds from an io.Reader.
func ImportFeedsFromReader(reader io.Reader) ([]FeedItem, error) {
	var opmlDoc OPML
	decoder := xml.NewDecoder(reader)

	if err := decoder.Decode(&opmlDoc); err != nil {
		return nil, fmt.Errorf("failed to parse OPML: %w", err)
	}

	var feeds []FeedItem
	walkOutlines(opmlDoc.Body.Outlines, nil, &feeds)
	if len(feeds) == 0 {
		return nil, fmt.Errorf("no RSS feeds found in OPML file")
	}

	return feeds, nil
}

func feedToOutline(feed FeedItem) Outline {
	return Outline{
		Text:    feed.Title,
		Title:   feed.Title,
		Type:    "rss",
		XMLURL:  feed.XMLURL,
		HTMLURL: feed.HTMLURL,
	}
}

func sortFeeds(feeds []FeedItem) {
	sort.Slice(feeds, func(i, j int) bool {
		if feeds[i].Title != feeds[j].Title {
			return feeds[i].Title < feeds[j].Title
		}
		return feeds[i].XMLURL < feeds[j].XMLURL
	})
}

// DedupFeeds removes duplicate feeds based on XMLURL, keeping the first occurrence.
func DedupFeeds(feeds []FeedItem) []FeedItem {
	seen := make(map[string]bool)
	var result []FeedItem
	for _, feed := range feeds {
		key := strings.TrimSpace(feed.XMLURL)
		if key == "" {
			continue
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, feed)
		}
	}
	return result
}

// ValidateResult contains the result of OPML validation.
type ValidateResult struct {
	Valid  bool
	Errors []string
	Feeds  []FeedItem
}

// Validate checks an OPML document for common issues and attempts auto-repair.
func Validate(data []byte) ValidateResult {
	result := ValidateResult{Valid: true}

	opmlDoc := OPML{}
	if err := xml.Unmarshal(data, &opmlDoc); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid XML: %v", err))
		return result
	}

	if opmlDoc.Version == "" {
		result.Errors = append(result.Errors, "missing opml version attribute, auto-repaired: defaulting to 2.0")
		opmlDoc.Version = "2.0"
	}

	if opmlDoc.Head.Title == "" {
		result.Errors = append(result.Errors, "missing head/title, auto-repaired: defaulting to 'RSS Feeds'")
		opmlDoc.Head.Title = "RSS Feeds"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if opmlDoc.Head.DateCreated == "" {
		result.Errors = append(result.Errors, "missing head/dateCreated, auto-repaired: set to current time")
		opmlDoc.Head.DateCreated = now
	}

	if opmlDoc.Head.DateModified == "" {
		result.Errors = append(result.Errors, "missing head/dateModified, auto-repaired: set to current time")
		opmlDoc.Head.DateModified = now
	}

	var feeds []FeedItem
	walkOutlines(opmlDoc.Body.Outlines, nil, &feeds)
	result.Feeds = feeds

	fixCount := 0
	for i, feed := range feeds {
		if feed.XMLURL == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("feed %d (%q): missing xmlUrl, skipped", i, feed.Title))
			continue
		}
		if feed.Title == "" {
			feeds[i].Title = feed.XMLURL
			fixCount++
		}
	}
	if fixCount > 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("auto-repaired: %d feeds with missing title default to xmlUrl", fixCount))
	}

	if len(feeds) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "no valid RSS feeds found")
	}

	return result
}

// MergeFeeds imports and merges feeds from multiple OPML files with deduplication.
func MergeFeeds(files []string, dedup bool) ([]FeedItem, error) {
	var allFeeds []FeedItem
	seen := make(map[string]bool)

	for _, file := range files {
		feeds, err := ImportFeeds(file)
		if err != nil {
			return nil, fmt.Errorf("failed to import %s: %w", file, err)
		}
		for _, feed := range feeds {
			key := strings.TrimSpace(feed.XMLURL)
			if key == "" {
				continue
			}
			if dedup && seen[key] {
				continue
			}
			seen[key] = true
			allFeeds = append(allFeeds, feed)
		}
	}

	return allFeeds, nil
}

// LoadFeeds loads feeds from an OPML file, returning both feeds and the raw OPML data.
func LoadFeeds(filename string) ([]FeedItem, []byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read OPML file: %w", err)
	}
	feeds, err := ImportFeedsFromReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, nil, err
	}
	return feeds, data, nil
}

func walkOutlines(outlines []Outline, categoryPath []string, feeds *[]FeedItem) {
	for _, outline := range outlines {
		nextCategoryPath := categoryPath
		if outline.Text != "" || outline.Title != "" {
			label := outline.Text
			if label == "" {
				label = outline.Title
			}
			if label != "" && outline.XMLURL == "" {
				nextCategoryPath = append(append([]string(nil), categoryPath...), label)
			}
		}

		if outline.XMLURL != "" {
			category := strings.Join(categoryPath, " / ")
			title := outline.Title
			if title == "" {
				title = outline.Text
			}
			if title == "" {
				title = outline.XMLURL
			}
			*feeds = append(*feeds, FeedItem{
				Title:    title,
				XMLURL:   outline.XMLURL,
				HTMLURL:  outline.HTMLURL,
				Category: category,
			})
		}

		if len(outline.Children) > 0 {
			walkOutlines(outline.Children, nextCategoryPath, feeds)
		}
	}
}
