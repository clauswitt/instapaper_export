package rss

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// ParseRSSFeed fetches and parses an RSS feed from a URL
func ParseRSSFeed(url string) (*RSS, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS feed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RSS feed: %w", err)
	}

	var rss RSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	return &rss, nil
}

// SyncFeed synchronizes articles from an RSS feed, applying feed tags to new articles
func SyncFeed(database *db.DB, feed *model.RSSFeed, feedTags []string) (int, error) {
	// Parse the RSS feed
	rss, err := ParseRSSFeed(feed.URL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	newArticles := 0

	// Process each item in the feed
	for _, item := range rss.Channel.Items {
		// Normalize URL to https
		normalizedURL := normalizeURL(item.Link)

		// Check if article already exists (with normalized URL)
		var existingID int64
		err := database.Get(&existingID, "SELECT id FROM articles WHERE url = ?", normalizedURL)
		if err == nil {
			// Article already exists, skip
			continue
		}

		// Parse publish date
		pubDate, err := parsePubDate(item.PubDate)
		if err != nil {
			// If parsing fails, use current time
			pubDate = time.Now()
		}

		// Insert new article with normalized URL
		result, err := database.Exec(`
			INSERT INTO articles (url, title, instapapered_at)
			VALUES (?, ?, ?)
		`, normalizedURL, item.Title, pubDate.Format(time.RFC3339))
		if err != nil {
			return newArticles, fmt.Errorf("failed to insert article: %w", err)
		}

		articleID, err := result.LastInsertId()
		if err != nil {
			return newArticles, fmt.Errorf("failed to get article ID: %w", err)
		}

		// Add feed tags to the article
		for _, tagTitle := range feedTags {
			tagID, err := database.UpsertTag(tagTitle)
			if err != nil {
				return newArticles, fmt.Errorf("failed to upsert tag: %w", err)
			}

			_, err = database.Exec(`
				INSERT OR IGNORE INTO article_tags (article_id, tag_id)
				VALUES (?, ?)
			`, articleID, tagID)
			if err != nil {
				return newArticles, fmt.Errorf("failed to associate tag: %w", err)
			}
		}

		// Update FTS index for the new article
		if err := database.UpsertArticleFTS(articleID); err != nil {
			return newArticles, fmt.Errorf("failed to update FTS: %w", err)
		}

		newArticles++
	}

	// Update last synced timestamp
	_, err = database.Exec(`
		UPDATE rss_feeds SET last_synced_at = datetime('now') WHERE id = ?
	`, feed.ID)
	if err != nil {
		return newArticles, fmt.Errorf("failed to update sync time: %w", err)
	}

	return newArticles, nil
}

// parsePubDate attempts to parse RSS pubDate in RFC1123 format
func parsePubDate(dateStr string) (time.Time, error) {
	// Try RFC1123 format (common in RSS)
	t, err := time.Parse(time.RFC1123, dateStr)
	if err == nil {
		return t, nil
	}

	// Try RFC1123Z format (with timezone)
	t, err = time.Parse(time.RFC1123Z, dateStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("failed to parse date: %s", dateStr)
}

// normalizeURL converts http:// URLs to https:// for consistency
func normalizeURL(url string) string {
	if strings.HasPrefix(url, "http://") {
		return strings.Replace(url, "http://", "https://", 1)
	}
	return url
}
