package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"instapaper-cli/internal/model"
	"instapaper-cli/internal/search"
)

// handleSearchArticles handles the search_articles tool
func (s *Server) handleSearchArticles(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract parameters with defaults
	query, _ := arguments["query"].(string)
	field, _ := arguments["field"].(string)
	since, _ := arguments["since"].(string)
	until, _ := arguments["until"].(string)

	// Default to FTS for better search experience and intersection queries
	useFTS := true
	if val, exists := arguments["use_fts"]; exists {
		if boolVal, ok := val.(bool); ok {
			useFTS = boolVal
		}
	}

	limit := 50
	if l, ok := arguments["limit"].(float64); ok {
		limit = int(l)
	}
	onlySynced, _ := arguments["only_synced"].(bool)

	// Build search options
	searchOpts := search.SearchOptions{
		Query:      query,
		Field:      field,
		UseFTS:     useFTS,
		Limit:      limit,
		JSONOutput: false,
		Since:      since,
		Until:      until,
	}

	// Perform basic search using existing functionality
	var results []model.SearchResult
	var err error

	if useFTS && query != "" {
		results, err = s.searchFTS(searchOpts)
	} else if query != "" {
		results, err = s.searchLike(searchOpts)
	} else if since != "" || until != "" {
		// Handle date-only filtering (like latest command)
		results, err = s.searchLike(searchOpts)
	} else {
		// Return empty results if no query or date filter
		results = []model.SearchResult{}
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	// Filter by synced status if requested
	if onlySynced {
		var filteredResults []model.SearchResult
		for _, result := range results {
			if result.SyncedAt != nil {
				filteredResults = append(filteredResults, result)
			}
		}
		results = filteredResults
	}

	// Format results
	if len(results) == 0 {
		return mcp.NewToolResultText("No articles found matching the search criteria."), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d articles:\n\n", len(results)))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("ID: %d\n", result.ID))
		output.WriteString(fmt.Sprintf("URL: %s\n", result.URL))

		if result.FolderPath != nil && *result.FolderPath != "" {
			output.WriteString(fmt.Sprintf("Folder: %s\n", *result.FolderPath))
		}

		if result.Tags != nil && *result.Tags != "" {
			output.WriteString(fmt.Sprintf("Tags: %s\n", *result.Tags))
		}

		if result.SyncedAt != nil {
			output.WriteString("Content: Available\n")
		} else {
			output.WriteString("Content: Not downloaded\n")
		}

		output.WriteString("\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleGetArticle handles the get_article tool
func (s *Server) handleGetArticle(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Extract article ID
	idFloat, ok := arguments["id"].(float64)
	if !ok {
		return mcp.NewToolResultError("Article ID is required and must be a number"), nil
	}
	id := int64(idFloat)

	includeContent := true
	if ic, ok := arguments["include_content"].(bool); ok {
		includeContent = ic
	}

	includeTags := true
	if it, ok := arguments["include_tags"].(bool); ok {
		includeTags = it
	}

	// Get article with details
	article, err := s.getArticleWithDetails(id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get article: %v", err)), nil
	}

	// Format article
	var output strings.Builder
	output.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	output.WriteString(fmt.Sprintf("**ID:** %d\n", article.ID))
	output.WriteString(fmt.Sprintf("**URL:** %s\n", article.URL))

	if article.FolderPath != "" {
		output.WriteString(fmt.Sprintf("**Folder:** %s\n", article.FolderPath))
	}

	if includeTags && len(article.Tags) > 0 {
		output.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(article.Tags, ", ")))
	}

	if article.Selection != nil && *article.Selection != "" {
		output.WriteString(fmt.Sprintf("**Selected Text:** %s\n", *article.Selection))
	}

	parsedTime, _ := time.Parse(time.RFC3339, article.InstapaperedAt)
	output.WriteString(fmt.Sprintf("**Added:** %s\n", parsedTime.Format("2006-01-02 15:04:05")))

	if article.SyncedAt != nil {
		parsedSyncTime, _ := time.Parse(time.RFC3339, *article.SyncedAt)
		output.WriteString(fmt.Sprintf("**Content Downloaded:** %s\n", parsedSyncTime.Format("2006-01-02 15:04:05")))
	}

	output.WriteString("\n")

	if includeContent && article.ContentMD != nil && *article.ContentMD != "" {
		output.WriteString("## Content\n\n")
		output.WriteString(*article.ContentMD)
	} else {
		output.WriteString("*Article content not yet downloaded.*")
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleListFolders handles the list_folders tool
func (s *Server) handleListFolders(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	query := `
		SELECT f.id, f.title, f.path_cache, COUNT(a.id) as article_count
		FROM folders f
		LEFT JOIN articles a ON f.id = a.folder_id
		GROUP BY f.id, f.title, f.path_cache
		ORDER BY f.path_cache, f.title
	`

	var folders []FolderInfo
	rows, err := s.db.Query(query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to query folders: %v", err)), nil
	}
	defer rows.Close()

	for rows.Next() {
		var folder FolderInfo
		var pathCache *string
		if err := rows.Scan(&folder.ID, &folder.Title, &pathCache, &folder.ArticleCount); err != nil {
			continue
		}
		if pathCache != nil {
			folder.PathCache = *pathCache
		} else {
			folder.PathCache = folder.Title
		}
		folders = append(folders, folder)
	}

	if len(folders) == 0 {
		return mcp.NewToolResultText("No folders found."), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d folders:\n\n", len(folders)))

	for _, folder := range folders {
		output.WriteString(fmt.Sprintf("**%s** (%d articles)\n", folder.PathCache, folder.ArticleCount))
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleListTags handles the list_tags tool
func (s *Server) handleListTags(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	minCount := 0
	if mc, ok := arguments["min_count"].(float64); ok {
		minCount = int(mc)
	}

	query := `
		SELECT t.id, t.title, COUNT(at.article_id) as article_count
		FROM tags t
		LEFT JOIN article_tags at ON t.id = at.tag_id
		GROUP BY t.id, t.title
	`

	var args []interface{}
	if minCount > 0 {
		query += " HAVING COUNT(at.article_id) >= ?"
		args = append(args, minCount)
	}

	query += " ORDER BY article_count DESC, t.title"

	var tags []TagInfo
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to query tags: %v", err)), nil
	}
	defer rows.Close()

	for rows.Next() {
		var tag TagInfo
		if err := rows.Scan(&tag.ID, &tag.Title, &tag.ArticleCount); err != nil {
			continue
		}
		tags = append(tags, tag)
	}

	if len(tags) == 0 {
		return mcp.NewToolResultText("No tags found."), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d tags:\n\n", len(tags)))

	for _, tag := range tags {
		output.WriteString(fmt.Sprintf("**%s** (%d articles)\n", tag.Title, tag.ArticleCount))
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleExportArticles handles the export_articles tool
func (s *Server) handleExportArticles(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	query, _ := arguments["query"].(string)
	limit := 10 // Default limit for exports
	if l, ok := arguments["limit"].(float64); ok {
		limit = int(l)
	}

	onlySynced := true
	if os, ok := arguments["only_synced"].(bool); ok {
		onlySynced = os
	}

	// Get articles based on search
	var articles []model.ArticleWithDetails

	if query != "" {
		// Search for articles first
		searchOpts := search.SearchOptions{
			Query:      query,
			UseFTS:     true,
			Limit:      limit,
			JSONOutput: false,
		}

		results, searchErr := s.searchFTS(searchOpts)
		if searchErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", searchErr)), nil
		}

		// Get full details for each result
		for _, result := range results {
			article, detailErr := s.getArticleWithDetails(result.ID)
			if detailErr != nil {
				continue
			}

			if onlySynced && (article.ContentMD == nil || *article.ContentMD == "") {
				continue
			}

			articles = append(articles, *article)
		}
	} else {
		// Get recent articles
		articlesQuery := `
			SELECT a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
				   a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
				   a.status_text, a.final_url, a.content_md, a.raw_html,
				   f.path_cache as folder_path
			FROM articles a
			LEFT JOIN folders f ON a.folder_id = f.id
			WHERE 1=1
		`

		if onlySynced {
			articlesQuery += " AND a.content_md IS NOT NULL"
		}

		articlesQuery += " ORDER BY a.instapapered_at DESC LIMIT ?"

		if err := s.db.Select(&articles, articlesQuery, limit); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get articles: %v", err)), nil
		}

		// Get tags for each article
		for i := range articles {
			tags, _ := s.getArticleTags(articles[i].ID)
			articles[i].Tags = tags
		}
	}

	if len(articles) == 0 {
		return mcp.NewToolResultText("No articles found matching the criteria."), nil
	}

	// Build combined markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Exported Articles (%d)\n\n", len(articles)))

	for i, article := range articles {
		if i > 0 {
			content.WriteString("\n---\n\n")
		}

		content.WriteString(fmt.Sprintf("## %s\n\n", article.Title))
		content.WriteString(fmt.Sprintf("**Source:** %s\n", article.URL))

		if article.FolderPath != "" {
			content.WriteString(fmt.Sprintf("**Folder:** %s\n", article.FolderPath))
		}

		if len(article.Tags) > 0 {
			content.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(article.Tags, ", ")))
		}

		parsedTime, _ := time.Parse(time.RFC3339, article.InstapaperedAt)
		content.WriteString(fmt.Sprintf("**Added:** %s\n\n", parsedTime.Format("2006-01-02")))

		if article.ContentMD != nil && *article.ContentMD != "" {
			content.WriteString(*article.ContentMD)
		} else {
			content.WriteString("*Content not yet downloaded.*")
		}

		content.WriteString("\n\n")
	}

	return mcp.NewToolResultText(content.String()), nil
}

// handleGetLatestArticles handles the get_latest_articles tool
func (s *Server) handleGetLatestArticles(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	limit := 20
	if l, ok := arguments["limit"].(float64); ok {
		limit = int(l)
	}

	since, _ := arguments["since"].(string)
	until, _ := arguments["until"].(string)
	onlySynced, _ := arguments["only_synced"].(bool)

	// Use search functionality with empty query to get latest articles
	searchOpts := search.SearchOptions{
		Query:      "",
		Field:      "",
		UseFTS:     false,
		Limit:      limit,
		JSONOutput: false,
		Since:      since,
		Until:      until,
	}

	// Get results using search
	results, err := s.searchLike(searchOpts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get latest articles: %v", err)), nil
	}

	// Filter by synced status if requested
	if onlySynced {
		var filteredResults []model.SearchResult
		for _, result := range results {
			if result.SyncedAt != nil {
				filteredResults = append(filteredResults, result)
			}
		}
		results = filteredResults
	}

	// Format results
	if len(results) == 0 {
		return mcp.NewToolResultText("No articles found matching the criteria."), nil
	}

	var output strings.Builder

	// Add context about date filtering
	if since != "" || until != "" {
		output.WriteString("Latest articles")
		if since != "" && until != "" {
			output.WriteString(fmt.Sprintf(" from %s to %s", since, until))
		} else if since != "" {
			output.WriteString(fmt.Sprintf(" since %s", since))
		} else if until != "" {
			output.WriteString(fmt.Sprintf(" until %s", until))
		}
		output.WriteString(fmt.Sprintf(" (%d articles):\n\n", len(results)))
	} else {
		output.WriteString(fmt.Sprintf("Latest %d articles:\n\n", len(results)))
	}

	for i, result := range results {
		output.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("ID: %d\n", result.ID))
		output.WriteString(fmt.Sprintf("URL: %s\n", result.URL))

		// Parse and format the date nicely
		if parsedTime, err := time.Parse(time.RFC3339, result.InstapaperedAt); err == nil {
			output.WriteString(fmt.Sprintf("Added: %s\n", parsedTime.Format("2006-01-02 15:04:05")))
		}

		if result.FolderPath != nil && *result.FolderPath != "" {
			output.WriteString(fmt.Sprintf("Folder: %s\n", *result.FolderPath))
		}

		if result.Tags != nil && *result.Tags != "" {
			output.WriteString(fmt.Sprintf("Tags: %s\n", *result.Tags))
		}

		if result.SyncedAt != nil {
			output.WriteString("Content: Available\n")
		} else {
			output.WriteString("Content: Not downloaded\n")
		}

		output.WriteString("\n")
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleGetUsageExamples provides examples of how to handle common requests
func (s *Server) handleGetUsageExamples(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	examples := `# Common Request Patterns and Tool Usage

## Search with Time Filters

**User Request: "Give me all the Kubernetes articles from the last week"**
Tool: search_articles
Parameters:
- query: "kubernetes"
- since: "1w"

**User Request: "Show me AI articles from today"**
Tool: search_articles
Parameters:
- query: "AI"
- since: "today"

**User Request: "Find Docker articles from last month"**
Tool: search_articles
Parameters:
- query: "docker"
- since: "1m"

**User Request: "Python articles from January 2024"**
Tool: search_articles
Parameters:
- query: "python"
- since: "2024-01-01"
- until: "2024-01-31"

## Recent Articles Without Search

**User Request: "What did I save recently?" / "Show me my recent articles"**
Tool: get_latest_articles
Parameters:
- limit: 10

**User Request: "What did I save last week?"**
Tool: get_latest_articles
Parameters:
- since: "1w"
- limit: 20

**User Request: "Show me articles I saved today"**
Tool: get_latest_articles
Parameters:
- since: "today"

## Date Filter Values

Common date filters to use:
- "today" - Articles from today
- "yesterday" - Articles from yesterday
- "1d" - Last 1 day
- "3d" - Last 3 days
- "1w" - Last 1 week
- "2w" - Last 2 weeks
- "1m" - Last 1 month
- "3m" - Last 3 months
- "1y" - Last 1 year
- "2024-01-15" - Specific date
- "2024-01-01" to "2024-01-31" - Date range (use both since and until)

## Search vs Latest Articles

- Use **search_articles** when user mentions specific topics/keywords + time
- Use **get_latest_articles** when user just wants recent articles by time without topics

## Content vs Metadata

- Most searches return metadata (title, URL, date, tags)
- Use **get_article** with specific ID to get full article content
- Set only_synced=true to only return articles with downloaded content

## Examples in Context

"Show me recent articles about machine learning" → search_articles(query="machine learning", since="1w")
"What have I saved in the past few days?" → get_latest_articles(since="3d")
"Find all React articles from last month" → search_articles(query="react", since="1m")
"Get me the latest 5 articles" → get_latest_articles(limit=5)
"Show me Node.js articles from this year" → search_articles(query="node.js", since="1y")`

	return mcp.NewToolResultText(examples), nil
}