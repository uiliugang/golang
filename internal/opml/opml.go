package opml

import (
"encoding/xml"
"fmt"
"io"
"os"
"time"
)

// OPML represents an OPML (Outline Processor Markup Language) document
type OPML struct {
XMLName xml.Name `xml:"opml"`
Version string   `xml:"version,attr"`
Head    Head     `xml:"head"`
Body    Body     `xml:"body"`
}

// Head contains metadata about the OPML document
type Head struct {
Title       string `xml:"title"`
DateCreated string `xml:"dateCreated"`
DateModified string `xml:"dateModified"`
OwnerName   string `xml:"ownerName"`
OwnerEmail  string `xml:"ownerEmail"`
Docs        string `xml:"docs"`
}

// Body contains the outline elements
type Body struct {
Outlines []Outline `xml:"outline"`
}

// Outline represents an outline element which can be a category or a feed
type Outline struct {
XMLName xml.Name `xml:"outline"`
Text    string   `xml:"text,attr"`
Title   string   `xml:"title,attr"`
Type    string   `xml:"type,attr"`
XMLURL  string   `xml:"xmlUrl,attr"`
HTMLURL string   `xml:"htmlUrl,attr"`
Children []Outline `xml:"outline"`
}

// FeedItem represents a single RSS feed with its metadata
type FeedItem struct {
Title       string
XMLURL      string
HTMLURL     string
Description string
Category    string
}

// ExportFeeds exports a list of feeds to an OPML file
func ExportFeeds(filename string, feeds []FeedItem) error {
opmlDoc := OPML{
Version: "2.0",
Head: Head{
Title:        "RSS Feeds",
DateCreated:  time.Now().Format(time.RFC3339),
DateModified: time.Now().Format(time.RFC3339),
OwnerName:    "RSS Reader",
Docs:         "http://www.opml.org/spec2.opml",
},
Body: Body{
Outlines: []Outline{},
},
}

// Group feeds by category
categoryMap := make(map[string][]FeedItem)
uncategorized := []FeedItem{}

for _, feed := range feeds {
if feed.Category != "" {
categoryMap[feed.Category] = append(categoryMap[feed.Category], feed)
} else {
uncategorized = append(uncategorized, feed)
}
}

// Add categorized feeds
for category, categoryFeeds := range categoryMap {
outline := Outline{
Text:     category,
Title:    category,
Children: []Outline{},
}

for _, feed := range categoryFeeds {
feedOutline := Outline{
Text:    feed.Title,
Title:   feed.Title,
Type:    "rss",
XMLURL:  feed.XMLURL,
HTMLURL: feed.HTMLURL,
}
outline.Children = append(outline.Children, feedOutline)
}

opmlDoc.Body.Outlines = append(opmlDoc.Body.Outlines, outline)
}

// Add uncategorized feeds
if len(uncategorized) > 0 {
outline := Outline{
Text:     "Uncategorized",
Title:    "Uncategorized",
Children: []Outline{},
}

for _, feed := range uncategorized {
feedOutline := Outline{
Text:    feed.Title,
Title:   feed.Title,
Type:    "rss",
XMLURL:  feed.XMLURL,
HTMLURL: feed.HTMLURL,
}
outline.Children = append(outline.Children, feedOutline)
}

opmlDoc.Body.Outlines = append(opmlDoc.Body.Outlines, outline)
}

// Marshal to XML
xmlData, err := xml.MarshalIndent(opmlDoc, "", "  ")
if err != nil {
return fmt.Errorf("failed to marshal OPML: %w", err)
}

// Write to file with XML declaration
file, err := os.Create(filename)
if err != nil {
return fmt.Errorf("failed to create OPML file: %w", err)
}
defer file.Close()

xmlDeclaration := []byte(xml.Header)
if _, err := file.Write(xmlDeclaration); err != nil {
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

// ImportFeeds imports feeds from an OPML file
func ImportFeeds(filename string) ([]FeedItem, error) {
file, err := os.Open(filename)
if err != nil {
return nil, fmt.Errorf("failed to open OPML file: %w", err)
}
defer file.Close()

return ImportFeedsFromReader(file)
}

// ImportFeedsFromReader imports feeds from an io.Reader
func ImportFeedsFromReader(reader io.Reader) ([]FeedItem, error) {
var opmlDoc OPML
decoder := xml.NewDecoder(reader)

if err := decoder.Decode(&opmlDoc); err != nil {
return nil, fmt.Errorf("failed to parse OPML: %w", err)
}

feeds := []FeedItem{}

// Extract feeds from outlines
for _, outline := range opmlDoc.Body.Outlines {
category := outline.Text

// If it's a feed (has xmlUrl), add it directly
if outline.XMLURL != "" && outline.Type == "rss" {
feeds = append(feeds, FeedItem{
Title:       outline.Title,
XMLURL:      outline.XMLURL,
HTMLURL:     outline.HTMLURL,
Description: "",
Category:    "",
})
}

// If it has children, they are likely feeds
for _, child := range outline.Children {
if child.XMLURL != "" && child.Type == "rss" {
feeds = append(feeds, FeedItem{
Title:       child.Title,
XMLURL:      child.XMLURL,
HTMLURL:     child.HTMLURL,
Description: "",
Category:    category,
})
}
}
}

if len(feeds) == 0 {
return nil, fmt.Errorf("no RSS feeds found in OPML file")
}

return feeds, nil
}
