package model

import (
	"time"
)

type Article struct {
	ID             int64   `db:"id" json:"id"`
	URL            string  `db:"url" json:"url"`
	Title          string  `db:"title" json:"title"`
	Selection      *string `db:"selection" json:"selection,omitempty"`
	FolderID       *int64  `db:"folder_id" json:"folder_id,omitempty"`
	InstapaperedAt string  `db:"instapapered_at" json:"instapapered_at"`
	SyncedAt       *string `db:"synced_at" json:"synced_at,omitempty"`
	SyncFailedAt   *string `db:"sync_failed_at" json:"sync_failed_at,omitempty"`
	FailedCount    int     `db:"failed_count" json:"failed_count"`
	StatusCode     *int    `db:"status_code" json:"status_code,omitempty"`
	StatusText     *string `db:"status_text" json:"status_text,omitempty"`
	FinalURL       *string `db:"final_url" json:"final_url,omitempty"`
	ContentMD      *string `db:"content_md" json:"content_md,omitempty"`
	RawHTML        *string `db:"raw_html" json:"raw_html,omitempty"`
}

type Folder struct {
	ID        int64   `db:"id" json:"id"`
	Title     string  `db:"title" json:"title"`
	ParentID  *int64  `db:"parent_id" json:"parent_id,omitempty"`
	PathCache *string `db:"path_cache" json:"path_cache,omitempty"`
}

type Tag struct {
	ID    int64  `db:"id" json:"id"`
	Title string `db:"title" json:"title"`
}

type ArticleTag struct {
	ArticleID int64 `db:"article_id" json:"article_id"`
	TagID     int64 `db:"tag_id" json:"tag_id"`
}

type ArticleWithDetails struct {
	Article
	FolderPath string   `db:"folder_path" json:"folder_path,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

type CSVRecord struct {
	URL       string `csv:"URL"`
	Title     string `csv:"Title"`
	Selection string `csv:"Selection"`
	Folder    string `csv:"Folder"`
	Timestamp int64  `csv:"Timestamp"`
	Tags      string `csv:"Tags"`
}

type FrontMatter struct {
	Title          string    `yaml:"title"`
	InstapaperedAt time.Time `yaml:"instapapered_at"`
	ExportedAt     time.Time `yaml:"exported_at"`
	Source         string    `yaml:"source"`
	Tags           []string  `yaml:"tags"`
}

type SearchResult struct {
	ID             int64   `db:"id" json:"id"`
	URL            string  `db:"url" json:"url"`
	Title          string  `db:"title" json:"title"`
	FolderPath     *string `db:"folder_path" json:"folder_path,omitempty"`
	Tags           *string `db:"tags" json:"tags,omitempty"`
	SyncedAt       *string `db:"synced_at" json:"synced_at,omitempty"`
	FailedCount    int     `db:"failed_count" json:"failed_count"`
	StatusCode     *int    `db:"status_code" json:"status_code,omitempty"`
	InstapaperedAt string  `db:"instapapered_at" json:"instapapered_at"`
}

type RSSFeed struct {
	ID           int64   `db:"id" json:"id"`
	URL          string  `db:"url" json:"url"`
	Name         string  `db:"name" json:"name"`
	CreatedAt    string  `db:"created_at" json:"created_at"`
	LastSyncedAt *string `db:"last_synced_at" json:"last_synced_at,omitempty"`
	Active       bool    `db:"active" json:"active"`
}

type RSSFeedWithTags struct {
	RSSFeed
	Tags []string `json:"tags,omitempty"`
}

type RSSFeedTag struct {
	FeedID int64 `db:"feed_id" json:"feed_id"`
	TagID  int64 `db:"tag_id" json:"tag_id"`
}