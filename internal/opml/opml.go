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
