package mcp

import (
	"time"
)

// SearchRequest represents parameters for searching articles
type SearchRequest struct {
	Query         string   `json:"query,omitempty"`
	Field         string   `json:"field,omitempty"`         // url, title, content, tags, folder
	UseFTS        bool     `json:"use_fts,omitempty"`       // Use full-text search
	Limit         int      `json:"limit,omitempty"`
	Tags          []string `json:"tags,omitempty"`          // Filter by tags
	Folders       []string `json:"folders,omitempty"`       // Filter by folders
	DateAfter     string   `json:"date_after,omitempty"`    // ISO 8601 date
	DateBefore    string   `json:"date_before,omitempty"`   // ISO 8601 date
	OnlySynced    bool     `json:"only_synced,omitempty"`   // Only articles with content
	IncludeUnsync bool     `json:"include_unsynced,omitempty"`
}

// GetArticleRequest represents parameters for getting a single article
type GetArticleRequest struct {
	ID             int64 `json:"id"`
	IncludeContent bool  `json:"include_content,omitempty"` // Include full markdown content
	IncludeHTML    bool  `json:"include_html,omitempty"`    // Include raw HTML
	IncludeTags    bool  `json:"include_tags,omitempty"`    // Include tags array
}

// GetArticlesByIDsRequest represents parameters for getting multiple articles
type GetArticlesByIDsRequest struct {
	IDs            []int64 `json:"ids"`
	IncludeContent bool    `json:"include_content,omitempty"`
	IncludeHTML    bool    `json:"include_html,omitempty"`
	IncludeTags    bool    `json:"include_tags,omitempty"`
}

// ExportRequest represents parameters for exporting articles
type ExportRequest struct {
	SearchRequest                    // Embed search parameters for filtering
	Format           string `json:"format,omitempty"`           // markdown, json
	IncludeMetadata  bool   `json:"include_metadata,omitempty"` // Include YAML frontmatter
	OutputToStdout   bool   `json:"output_to_stdout,omitempty"` // Return content instead of file paths
	SeparateFiles    bool   `json:"separate_files,omitempty"`   // Whether to create separate files (ignored for stdout)
}

// AdvancedSearchRequest represents complex search with multiple conditions
type AdvancedSearchRequest struct {
	Query             string            `json:"query,omitempty"`
	TitleContains     string            `json:"title_contains,omitempty"`
	ContentContains   string            `json:"content_contains,omitempty"`
	URLContains       string            `json:"url_contains,omitempty"`
	Tags              []string          `json:"tags,omitempty"`              // Must have ALL these tags
	AnyTags           []string          `json:"any_tags,omitempty"`          // Must have ANY of these tags
	Folders           []string          `json:"folders,omitempty"`           // Must be in ANY of these folders
	DateAfter         string            `json:"date_after,omitempty"`        // ISO 8601 date
	DateBefore        string            `json:"date_before,omitempty"`       // ISO 8601 date
	OnlySynced        bool              `json:"only_synced,omitempty"`
	Limit             int               `json:"limit,omitempty"`
	UseFTS            bool              `json:"use_fts,omitempty"`
	SortBy            string            `json:"sort_by,omitempty"`           // instapapered_at, title, url
	SortOrder         string            `json:"sort_order,omitempty"`        // asc, desc
	CustomFilters     map[string]string `json:"custom_filters,omitempty"`    // Key-value pairs for custom filtering
}

// GetArticleContextRequest represents parameters for getting article with context
type GetArticleContextRequest struct {
	ID                 int64 `json:"id"`
	IncludeRelated     bool  `json:"include_related,omitempty"`     // Include related articles
	MaxRelated         int   `json:"max_related,omitempty"`         // Max number of related articles
	RelationshipType   string `json:"relationship_type,omitempty"`  // folder, tags, content_similarity
	IncludeContent     bool  `json:"include_content,omitempty"`
}

// ArticleResponse represents an article in API responses
type ArticleResponse struct {
	ID             int64     `json:"id"`
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	Selection      *string   `json:"selection,omitempty"`
	FolderPath     *string   `json:"folder_path,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	InstapaperedAt time.Time `json:"instapapered_at"`
	SyncedAt       *time.Time `json:"synced_at,omitempty"`
	SyncFailedAt   *time.Time `json:"sync_failed_at,omitempty"`
	FailedCount    int       `json:"failed_count"`
	StatusCode     *int      `json:"status_code,omitempty"`
	StatusText     *string   `json:"status_text,omitempty"`
	FinalURL       *string   `json:"final_url,omitempty"`
	ContentMD      *string   `json:"content_md,omitempty"`
	RawHTML        *string   `json:"raw_html,omitempty"`
}

// SearchResponse represents the result of a search operation
type SearchResponse struct {
	Articles    []ArticleResponse `json:"articles"`
	TotalCount  int               `json:"total_count"`
	SearchTime  string            `json:"search_time"`
	SearchQuery string            `json:"search_query"`
}

// ExportResponse represents the result of an export operation
type ExportResponse struct {
	Articles      []ArticleResponse `json:"articles,omitempty"`     // When output_to_stdout=true
	Content       string            `json:"content,omitempty"`      // Combined markdown content
	FilePaths     []string          `json:"file_paths,omitempty"`   // When separate_files=true
	ExportedCount int               `json:"exported_count"`
	ExportTime    string            `json:"export_time"`
}

// FolderInfo represents folder information
type FolderInfo struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	PathCache    string `json:"path_cache"`
	ArticleCount int    `json:"article_count"`
}

// TagInfo represents tag information
type TagInfo struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	ArticleCount int    `json:"article_count"`
}