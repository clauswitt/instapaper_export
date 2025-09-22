package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"instapaper-cli/internal/model"
)

// convertSearchResultToResponse converts a SearchResult to ArticleResponse
func (s *Server) convertSearchResultToResponse(result model.SearchResult) ArticleResponse {
	response := ArticleResponse{
		ID:          result.ID,
		URL:         result.URL,
		Title:       result.Title,
		FailedCount: result.FailedCount,
		StatusCode:  result.StatusCode,
	}

	// Parse the timestamp
	if parsedTime, err := time.Parse(time.RFC3339, result.InstapaperedAt); err == nil {
		response.InstapaperedAt = parsedTime
	}

	// Handle optional fields
	if result.FolderPath != nil {
		response.FolderPath = result.FolderPath
	}

	if result.Tags != nil && *result.Tags != "" {
		tags := strings.Split(*result.Tags, ", ")
		response.Tags = tags
	}

	if result.SyncedAt != nil {
		if parsedTime, err := time.Parse(time.RFC3339, *result.SyncedAt); err == nil {
			response.SyncedAt = &parsedTime
		}
	}

	return response
}

// convertArticleWithDetailsToResponse converts ArticleWithDetails to ArticleResponse
func (s *Server) convertArticleWithDetailsToResponse(article model.ArticleWithDetails, includeContent, includeHTML, includeTags bool) ArticleResponse {
	response := ArticleResponse{
		ID:          article.ID,
		URL:         article.URL,
		Title:       article.Title,
		Selection:   article.Selection,
		FailedCount: article.FailedCount,
		StatusCode:  article.StatusCode,
		StatusText:  article.StatusText,
		FinalURL:    article.FinalURL,
	}

	// Parse timestamps
	if parsedTime, err := time.Parse(time.RFC3339, article.InstapaperedAt); err == nil {
		response.InstapaperedAt = parsedTime
	}

	if article.SyncedAt != nil {
		if parsedTime, err := time.Parse(time.RFC3339, *article.SyncedAt); err == nil {
			response.SyncedAt = &parsedTime
		}
	}

	if article.SyncFailedAt != nil {
		if parsedTime, err := time.Parse(time.RFC3339, *article.SyncFailedAt); err == nil {
			response.SyncFailedAt = &parsedTime
		}
	}

	// Handle optional content
	if includeContent && article.ContentMD != nil {
		response.ContentMD = article.ContentMD
	}

	if includeHTML && article.RawHTML != nil {
		response.RawHTML = article.RawHTML
	}

	if includeTags && len(article.Tags) > 0 {
		response.Tags = article.Tags
	}

	if article.FolderPath != "" {
		response.FolderPath = &article.FolderPath
	}

	return response
}

// formatSearchResponse formats a search response for display
func (s *Server) formatSearchResponse(response SearchResponse) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# Search Results\n\n"))
	output.WriteString(fmt.Sprintf("**Query:** %s\n", response.SearchQuery))
	output.WriteString(fmt.Sprintf("**Results:** %d articles\n", response.TotalCount))
	output.WriteString(fmt.Sprintf("**Search Time:** %s\n\n", response.SearchTime))

	if len(response.Articles) == 0 {
		output.WriteString("No articles found matching the search criteria.\n")
		return output.String()
	}

	for i, article := range response.Articles {
		output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, article.Title))
		output.WriteString(fmt.Sprintf("**ID:** %d\n", article.ID))
		output.WriteString(fmt.Sprintf("**URL:** %s\n", article.URL))

		if article.FolderPath != nil && *article.FolderPath != "" {
			output.WriteString(fmt.Sprintf("**Folder:** %s\n", *article.FolderPath))
		}

		if len(article.Tags) > 0 {
			output.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(article.Tags, ", ")))
		}

		output.WriteString(fmt.Sprintf("**Added:** %s\n", article.InstapaperedAt.Format("2006-01-02 15:04:05")))

		if article.SyncedAt != nil {
			output.WriteString(fmt.Sprintf("**Content Synced:** %s\n", article.SyncedAt.Format("2006-01-02 15:04:05")))
		} else {
			output.WriteString("**Content Synced:** No\n")
		}

		if article.FailedCount > 0 {
			output.WriteString(fmt.Sprintf("**Failed Fetches:** %d\n", article.FailedCount))
		}

		output.WriteString("\n")
	}

	return output.String()
}

// formatArticleResponse formats a single article response
func (s *Server) formatArticleResponse(article ArticleResponse) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# %s\n\n", article.Title))
	output.WriteString(fmt.Sprintf("**ID:** %d\n", article.ID))
	output.WriteString(fmt.Sprintf("**URL:** %s\n", article.URL))

	if article.FinalURL != nil && *article.FinalURL != article.URL {
		output.WriteString(fmt.Sprintf("**Final URL:** %s\n", *article.FinalURL))
	}

	if article.FolderPath != nil && *article.FolderPath != "" {
		output.WriteString(fmt.Sprintf("**Folder:** %s\n", *article.FolderPath))
	}

	if len(article.Tags) > 0 {
		output.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(article.Tags, ", ")))
	}

	output.WriteString(fmt.Sprintf("**Added to Instapaper:** %s\n", article.InstapaperedAt.Format("2006-01-02 15:04:05")))

	if article.SyncedAt != nil {
		output.WriteString(fmt.Sprintf("**Content Synced:** %s\n", article.SyncedAt.Format("2006-01-02 15:04:05")))
	}

	if article.Selection != nil && *article.Selection != "" {
		output.WriteString(fmt.Sprintf("**Selected Text:** %s\n", *article.Selection))
	}

	output.WriteString("\n")

	if article.ContentMD != nil && *article.ContentMD != "" {
		output.WriteString("## Content\n\n")
		output.WriteString(*article.ContentMD)
		output.WriteString("\n")
	} else {
		output.WriteString("*Article content not yet fetched.*\n")
	}

	return output.String()
}

// formatMultipleArticlesResponse formats multiple articles
func (s *Server) formatMultipleArticlesResponse(articles []ArticleResponse) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# Articles (%d)\n\n", len(articles)))

	for i, article := range articles {
		output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, article.Title))
		output.WriteString(fmt.Sprintf("**ID:** %d  \n", article.ID))
		output.WriteString(fmt.Sprintf("**URL:** %s  \n", article.URL))

		if article.FolderPath != nil && *article.FolderPath != "" {
			output.WriteString(fmt.Sprintf("**Folder:** %s  \n", *article.FolderPath))
		}

		if len(article.Tags) > 0 {
			output.WriteString(fmt.Sprintf("**Tags:** %s  \n", strings.Join(article.Tags, ", ")))
		}

		output.WriteString(fmt.Sprintf("**Added:** %s  \n", article.InstapaperedAt.Format("2006-01-02")))

		if article.ContentMD != nil && *article.ContentMD != "" {
			// Show first 200 characters of content
			content := *article.ContentMD
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			output.WriteString(fmt.Sprintf("**Preview:** %s\n", content))
		}

		output.WriteString("\n---\n\n")
	}

	return output.String()
}

// formatFoldersResponse formats the list of folders
func (s *Server) formatFoldersResponse(folders []FolderInfo) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# Folders (%d)\n\n", len(folders)))

	if len(folders) == 0 {
		output.WriteString("No folders found.\n")
		return output.String()
	}

	output.WriteString("| Folder Path | Article Count |\n")
	output.WriteString("|-------------|---------------|\n")

	for _, folder := range folders {
		path := folder.PathCache
		if path == "" {
			path = folder.Title
		}
		output.WriteString(fmt.Sprintf("| %s | %d |\n", path, folder.ArticleCount))
	}

	return output.String()
}

// formatTagsResponse formats the list of tags
func (s *Server) formatTagsResponse(tags []TagInfo) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("# Tags (%d)\n\n", len(tags)))

	if len(tags) == 0 {
		output.WriteString("No tags found.\n")
		return output.String()
	}

	output.WriteString("| Tag | Article Count |\n")
	output.WriteString("|-----|---------------|\n")

	for _, tag := range tags {
		output.WriteString(fmt.Sprintf("| %s | %d |\n", tag.Title, tag.ArticleCount))
	}

	return output.String()
}

// formatArticleContextResponse formats an article with its context
func (s *Server) formatArticleContextResponse(response interface{}) string {
	// Marshal to JSON and then back to get the structure we need
	jsonData, err := json.Marshal(response)
	if err != nil {
		return "Error formatting response"
	}

	var contextResp struct {
		MainArticle      ArticleResponse   `json:"main_article"`
		RelatedArticles  []ArticleResponse `json:"related_articles"`
		RelationshipType string            `json:"relationship_type"`
	}

	if err := json.Unmarshal(jsonData, &contextResp); err != nil {
		return "Error parsing response"
	}

	var output strings.Builder

	// Format main article
	output.WriteString("# Main Article\n\n")
	output.WriteString(s.formatArticleResponse(contextResp.MainArticle))

	// Format related articles
	if len(contextResp.RelatedArticles) > 0 {
		output.WriteString(fmt.Sprintf("\n# Related Articles (%s)\n\n", contextResp.RelationshipType))

		for i, article := range contextResp.RelatedArticles {
			output.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, article.Title))
			output.WriteString(fmt.Sprintf("**ID:** %d  \n", article.ID))
			output.WriteString(fmt.Sprintf("**URL:** %s  \n", article.URL))

			if article.FolderPath != nil && *article.FolderPath != "" {
				output.WriteString(fmt.Sprintf("**Folder:** %s  \n", *article.FolderPath))
			}

			if len(article.Tags) > 0 {
				output.WriteString(fmt.Sprintf("**Tags:** %s  \n", strings.Join(article.Tags, ", ")))
			}

			if article.ContentMD != nil && *article.ContentMD != "" {
				// Show first 300 characters of content for related articles
				content := *article.ContentMD
				if len(content) > 300 {
					content = content[:300] + "..."
				}
				output.WriteString(fmt.Sprintf("**Content Preview:** %s\n", content))
			}

			output.WriteString("\n")
		}
	} else {
		output.WriteString(fmt.Sprintf("\n# Related Articles (%s)\n\n", contextResp.RelationshipType))
		output.WriteString("No related articles found.\n")
	}

	return output.String()
}