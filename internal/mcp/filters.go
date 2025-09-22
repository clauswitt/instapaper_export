package mcp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"instapaper-cli/internal/model"
	"instapaper-cli/internal/search"
	"instapaper-cli/internal/export"
	"gopkg.in/yaml.v3"
)

// searchWithFilters performs a search with additional filtering beyond the basic search
func (s *Server) searchWithFilters(opts search.SearchOptions, req SearchRequest) ([]model.SearchResult, error) {
	// Start with basic search
	results, err := s.performBasicSearch(opts)
	if err != nil {
		return nil, err
	}

	// Apply additional filters
	if len(req.Tags) > 0 || len(req.Folders) > 0 || req.DateAfter != "" || req.DateBefore != "" {
		results, err = s.applyAdditionalFilters(results, req)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// performBasicSearch performs the basic search using the existing search functionality
func (s *Server) performBasicSearch(opts search.SearchOptions) ([]model.SearchResult, error) {
	if opts.UseFTS {
		return s.searchFTS(opts)
	}
	return s.searchLike(opts)
}

// searchFTS performs FTS search
func (s *Server) searchFTS(opts search.SearchOptions) ([]model.SearchResult, error) {
	if opts.Query == "" {
		return nil, fmt.Errorf("FTS search requires a query")
	}

	baseQuery := `
		SELECT
			a.id,
			a.url,
			a.title,
			f.path_cache as folder_path,
			GROUP_CONCAT(t.title, ', ') as tags,
			a.synced_at,
			a.failed_count,
			a.status_code,
			a.instapapered_at
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		INNER JOIN articles_fts fts ON a.id = fts.rowid
		WHERE a.obsolete = FALSE
	`

	var whereClause string
	var args []interface{}

	if opts.Field != "" {
		switch opts.Field {
		case "url":
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, "url: "+opts.Query)
		case "title":
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, "title: "+opts.Query)
		case "content":
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, "content: "+opts.Query)
		case "tags":
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, "tags: "+opts.Query)
		case "folder":
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, "folder: "+opts.Query)
		default:
			return nil, fmt.Errorf("invalid field for FTS: %s", opts.Field)
		}
	} else {
		// For multiple keywords, create an AND query for intersection search
		keywords := strings.Fields(strings.TrimSpace(opts.Query))
		if len(keywords) > 1 {
			// Build FTS query with AND operators for intersection
			ftsQuery := strings.Join(keywords, " AND ")
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, ftsQuery)
		} else {
			whereClause = "AND articles_fts MATCH ?"
			args = append(args, opts.Query)
		}
	}

	query := baseQuery + " " + whereClause + `
		GROUP BY a.id
		ORDER BY rank
	`

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	var results []model.SearchResult
	if err := s.db.Select(&results, query, args...); err != nil {
		return nil, err
	}

	return results, nil
}

// searchLike performs LIKE search
func (s *Server) searchLike(opts search.SearchOptions) ([]model.SearchResult, error) {
	baseQuery := `
		SELECT
			a.id,
			a.url,
			a.title,
			f.path_cache as folder_path,
			GROUP_CONCAT(t.title, ', ') as tags,
			a.synced_at,
			a.failed_count,
			a.status_code,
			a.instapapered_at
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE a.obsolete = FALSE
	`

	var whereClause string
	var args []interface{}

	if opts.Field != "" && opts.Query != "" {
		switch opts.Field {
		case "url":
			whereClause = "AND a.url LIKE ?"
		case "title":
			whereClause = "AND a.title LIKE ?"
		case "content":
			whereClause = "AND a.content_md LIKE ?"
		case "tags":
			whereClause = "AND t.title LIKE ?"
		case "folder":
			whereClause = "AND (f.path_cache LIKE ? OR f.title LIKE ?)"
			args = append(args, "%"+opts.Query+"%")
		default:
			return nil, fmt.Errorf("invalid field: %s", opts.Field)
		}
		args = append(args, "%"+opts.Query+"%")
	} else if opts.Query != "" {
		whereClause = `
			AND (a.url LIKE ? OR a.title LIKE ? OR a.content_md LIKE ?
			       OR t.title LIKE ? OR f.path_cache LIKE ?)
		`
		pattern := "%" + opts.Query + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	query := baseQuery + " " + whereClause + `
		GROUP BY a.id
		ORDER BY a.instapapered_at DESC
	`

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	var results []model.SearchResult
	if err := s.db.Select(&results, query, args...); err != nil {
		return nil, err
	}

	return results, nil
}

// applyAdditionalFilters applies date, tag, and folder filters to search results
func (s *Server) applyAdditionalFilters(results []model.SearchResult, req SearchRequest) ([]model.SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Extract article IDs for filtering
	articleIDs := make([]string, len(results))
	for i, result := range results {
		articleIDs[i] = strconv.FormatInt(result.ID, 10)
	}

	// Build filter query
	query := `
		SELECT DISTINCT a.id
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE a.id IN (` + strings.Join(articleIDs, ",") + `)
	`

	var conditions []string
	var args []interface{}

	// Apply date filters
	if req.DateAfter != "" {
		conditions = append(conditions, "a.instapapered_at >= ?")
		args = append(args, req.DateAfter)
	}
	if req.DateBefore != "" {
		conditions = append(conditions, "a.instapapered_at <= ?")
		args = append(args, req.DateBefore)
	}

	// Apply tag filters (must have ALL specified tags)
	if len(req.Tags) > 0 {
		tagPlaceholders := make([]string, len(req.Tags))
		for i, tag := range req.Tags {
			tagPlaceholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf(`
			a.id IN (
				SELECT at2.article_id
				FROM article_tags at2
				JOIN tags t2 ON at2.tag_id = t2.id
				WHERE t2.title IN (%s)
				GROUP BY at2.article_id
				HAVING COUNT(DISTINCT t2.title) = %d
			)
		`, strings.Join(tagPlaceholders, ","), len(req.Tags)))
	}

	// Apply folder filters (must be in ANY specified folder)
	if len(req.Folders) > 0 {
		folderConditions := make([]string, len(req.Folders))
		for i, folder := range req.Folders {
			folderConditions[i] = "f.path_cache = ? OR f.title = ?"
			args = append(args, folder, folder)
		}
		conditions = append(conditions, "("+strings.Join(folderConditions, " OR ")+")")
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Execute filter query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply filters: %w", err)
	}
	defer rows.Close()

	// Collect filtered IDs
	filteredIDs := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		filteredIDs[id] = true
	}

	// Filter original results
	var filteredResults []model.SearchResult
	for _, result := range results {
		if filteredIDs[result.ID] {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

// performAdvancedSearch performs complex search with multiple conditions
func (s *Server) performAdvancedSearch(req AdvancedSearchRequest) ([]model.SearchResult, error) {
	baseQuery := `
		SELECT DISTINCT
			a.id,
			a.url,
			a.title,
			f.path_cache as folder_path,
			GROUP_CONCAT(DISTINCT t.title, ', ') as tags,
			a.synced_at,
			a.failed_count,
			a.status_code,
			a.instapapered_at
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
	`

	var joins []string
	var conditions []string
	var args []interface{}

	// Add FTS join if needed
	if req.UseFTS && req.Query != "" {
		joins = append(joins, "INNER JOIN articles_fts fts ON a.id = fts.rowid")
	}

	// Build conditions
	if req.Query != "" {
		if req.UseFTS {
			// For multiple keywords, create an AND query for intersection search
			keywords := strings.Fields(strings.TrimSpace(req.Query))
			if len(keywords) > 1 {
				// Build FTS query with AND operators for intersection
				ftsQuery := strings.Join(keywords, " AND ")
				conditions = append(conditions, "articles_fts MATCH ?")
				args = append(args, ftsQuery)
			} else {
				conditions = append(conditions, "articles_fts MATCH ?")
				args = append(args, req.Query)
			}
		} else {
			conditions = append(conditions, "(a.url LIKE ? OR a.title LIKE ? OR a.content_md LIKE ? OR t.title LIKE ? OR f.path_cache LIKE ?)")
			pattern := "%" + req.Query + "%"
			args = append(args, pattern, pattern, pattern, pattern, pattern)
		}
	}

	if req.TitleContains != "" {
		conditions = append(conditions, "a.title LIKE ?")
		args = append(args, "%"+req.TitleContains+"%")
	}

	if req.ContentContains != "" {
		conditions = append(conditions, "a.content_md LIKE ?")
		args = append(args, "%"+req.ContentContains+"%")
	}

	if req.URLContains != "" {
		conditions = append(conditions, "a.url LIKE ?")
		args = append(args, "%"+req.URLContains+"%")
	}

	if req.DateAfter != "" {
		conditions = append(conditions, "a.instapapered_at >= ?")
		args = append(args, req.DateAfter)
	}

	if req.DateBefore != "" {
		conditions = append(conditions, "a.instapapered_at <= ?")
		args = append(args, req.DateBefore)
	}

	if req.OnlySynced {
		conditions = append(conditions, "a.content_md IS NOT NULL")
	}

	// Handle tag filters
	if len(req.Tags) > 0 {
		// Must have ALL these tags
		tagPlaceholders := make([]string, len(req.Tags))
		for i, tag := range req.Tags {
			tagPlaceholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf(`
			a.id IN (
				SELECT at2.article_id
				FROM article_tags at2
				JOIN tags t2 ON at2.tag_id = t2.id
				WHERE t2.title IN (%s)
				GROUP BY at2.article_id
				HAVING COUNT(DISTINCT t2.title) = %d
			)
		`, strings.Join(tagPlaceholders, ","), len(req.Tags)))
	}

	if len(req.AnyTags) > 0 {
		// Must have ANY of these tags
		tagPlaceholders := make([]string, len(req.AnyTags))
		for i, tag := range req.AnyTags {
			tagPlaceholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf("t.title IN (%s)", strings.Join(tagPlaceholders, ",")))
	}

	// Handle folder filters
	if len(req.Folders) > 0 {
		folderConditions := make([]string, len(req.Folders))
		for i, folder := range req.Folders {
			folderConditions[i] = "f.path_cache = ? OR f.title = ?"
			args = append(args, folder, folder)
		}
		conditions = append(conditions, "("+strings.Join(folderConditions, " OR ")+")")
	}

	// Build final query
	query := baseQuery
	if len(joins) > 0 {
		query += " " + strings.Join(joins, " ")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY a.id"

	// Add sorting
	orderBy := "a.instapapered_at DESC"
	if req.SortBy != "" {
		sortField := req.SortBy
		sortOrder := "DESC"
		if req.SortOrder == "asc" {
			sortOrder = "ASC"
		}

		switch sortField {
		case "title":
			orderBy = "a.title " + sortOrder
		case "url":
			orderBy = "a.url " + sortOrder
		case "instapapered_at":
			orderBy = "a.instapapered_at " + sortOrder
		default:
			if req.UseFTS && req.Query != "" {
				orderBy = "rank"
			}
		}
	} else if req.UseFTS && req.Query != "" {
		orderBy = "rank"
	}

	query += " ORDER BY " + orderBy

	// Add limit
	if req.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, req.Limit)
	}

	// Execute query
	var results []model.SearchResult
	if err := s.db.Select(&results, query, args...); err != nil {
		return nil, fmt.Errorf("advanced search failed: %w", err)
	}

	return results, nil
}

// findRelatedArticles finds articles related to the given article based on relationship type
func (s *Server) findRelatedArticles(article model.ArticleWithDetails, relationshipType string, maxRelated int) ([]model.ArticleWithDetails, error) {
	var query string
	var args []interface{}

	switch relationshipType {
	case "folder":
		if article.FolderID == nil {
			return []model.ArticleWithDetails{}, nil
		}
		query = `
			SELECT DISTINCT
				a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
				a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
				a.status_text, a.final_url, a.content_md, a.raw_html,
				f.path_cache as folder_path
			FROM articles a
			LEFT JOIN folders f ON a.folder_id = f.id
			WHERE a.folder_id = ? AND a.id != ?
			ORDER BY a.instapapered_at DESC
			LIMIT ?
		`
		args = []interface{}{*article.FolderID, article.ID, maxRelated}

	case "tags":
		query = `
			SELECT DISTINCT
				a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
				a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
				a.status_text, a.final_url, a.content_md, a.raw_html,
				f.path_cache as folder_path
			FROM articles a
			LEFT JOIN folders f ON a.folder_id = f.id
			INNER JOIN article_tags at ON a.id = at.article_id
			INNER JOIN tags t ON at.tag_id = t.id
			WHERE t.title IN (
				SELECT t2.title
				FROM article_tags at2
				JOIN tags t2 ON at2.tag_id = t2.id
				WHERE at2.article_id = ?
			)
			AND a.id != ?
			ORDER BY a.instapapered_at DESC
			LIMIT ?
		`
		args = []interface{}{article.ID, article.ID, maxRelated}

	case "content_similarity":
		// Simple content similarity based on common words (basic implementation)
		if article.ContentMD == nil || *article.ContentMD == "" {
			return []model.ArticleWithDetails{}, nil
		}

		// Extract key words from the content (very basic implementation)
		words := s.extractKeyWords(*article.ContentMD)
		if len(words) == 0 {
			return []model.ArticleWithDetails{}, nil
		}

		// Build LIKE conditions for content similarity
		conditions := make([]string, len(words))
		for i, word := range words {
			conditions[i] = "a.content_md LIKE ?"
			args = append(args, "%"+word+"%")
		}

		query = fmt.Sprintf(`
			SELECT DISTINCT
				a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
				a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
				a.status_text, a.final_url, a.content_md, a.raw_html,
				f.path_cache as folder_path
			FROM articles a
			LEFT JOIN folders f ON a.folder_id = f.id
			WHERE (%s) AND a.id != ? AND a.content_md IS NOT NULL
			ORDER BY a.instapapered_at DESC
			LIMIT ?
		`, strings.Join(conditions, " OR "))
		args = append(args, article.ID, maxRelated)

	default:
		return []model.ArticleWithDetails{}, fmt.Errorf("unknown relationship type: %s", relationshipType)
	}

	var results []model.ArticleWithDetails
	if err := s.db.Select(&results, query, args...); err != nil {
		return nil, fmt.Errorf("failed to find related articles: %w", err)
	}

	// Get tags for each article
	for i := range results {
		tags, err := s.getArticleTags(results[i].ID)
		if err != nil {
			continue
		}
		results[i].Tags = tags
	}

	return results, nil
}

// extractKeyWords extracts key words from content for similarity matching
func (s *Server) extractKeyWords(content string) []string {
	// Very basic implementation - extract words longer than 4 characters
	words := strings.Fields(strings.ToLower(content))
	var keyWords []string
	seen := make(map[string]bool)

	for _, word := range words {
		// Remove common punctuation
		word = strings.Trim(word, ".,!?;:()[]{}\"'")

		// Skip short words, common words, and duplicates
		if len(word) > 4 && !s.isCommonWord(word) && !seen[word] {
			keyWords = append(keyWords, word)
			seen[word] = true

			// Limit to avoid too many conditions
			if len(keyWords) >= 10 {
				break
			}
		}
	}

	return keyWords
}

// isCommonWord checks if a word is too common to be useful for similarity
func (s *Server) isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"that": true, "this": true, "with": true, "from": true, "they": true,
		"have": true, "been": true, "their": true, "said": true, "each": true,
		"which": true, "there": true, "what": true, "would": true, "about": true,
		"could": true, "other": true, "after": true, "first": true, "never": true,
		"these": true, "think": true, "where": true, "being": true, "every": true,
		"great": true, "might": true, "shall": true, "still": true, "those": true,
		"while": true, "should": true, "through": true, "before": true, "around": true,
	}
	return commonWords[word]
}

// getArticleTags gets tags for an article
func (s *Server) getArticleTags(articleID int64) ([]string, error) {
	query := `
		SELECT t.title
		FROM tags t
		JOIN article_tags at ON t.id = at.tag_id
		WHERE at.article_id = ?
		ORDER BY t.title
	`

	var tags []string
	if err := s.db.Select(&tags, query, articleID); err != nil {
		return nil, err
	}

	return tags, nil
}

// buildAdvancedSearchDescription builds a human-readable description of the search
func (s *Server) buildAdvancedSearchDescription(req AdvancedSearchRequest) string {
	var parts []string

	if req.Query != "" {
		parts = append(parts, fmt.Sprintf("general query: '%s'", req.Query))
	}
	if req.TitleContains != "" {
		parts = append(parts, fmt.Sprintf("title contains: '%s'", req.TitleContains))
	}
	if req.ContentContains != "" {
		parts = append(parts, fmt.Sprintf("content contains: '%s'", req.ContentContains))
	}
	if req.URLContains != "" {
		parts = append(parts, fmt.Sprintf("URL contains: '%s'", req.URLContains))
	}
	if len(req.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("must have tags: [%s]", strings.Join(req.Tags, ", ")))
	}
	if len(req.AnyTags) > 0 {
		parts = append(parts, fmt.Sprintf("must have any of tags: [%s]", strings.Join(req.AnyTags, ", ")))
	}
	if len(req.Folders) > 0 {
		parts = append(parts, fmt.Sprintf("in folders: [%s]", strings.Join(req.Folders, ", ")))
	}
	if req.DateAfter != "" {
		parts = append(parts, fmt.Sprintf("after: %s", req.DateAfter))
	}
	if req.DateBefore != "" {
		parts = append(parts, fmt.Sprintf("before: %s", req.DateBefore))
	}

	if len(parts) == 0 {
		return "all articles"
	}

	return strings.Join(parts, ", ")
}

// getArticleWithDetails gets an article with full details including tags
func (s *Server) getArticleWithDetails(id int64) (*model.ArticleWithDetails, error) {
	query := `
		SELECT
			a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
			a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
			a.status_text, a.final_url, a.content_md, a.raw_html,
			f.path_cache as folder_path
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		WHERE a.id = ?
	`

	var article model.ArticleWithDetails
	if err := s.db.Get(&article, query, id); err != nil {
		return nil, err
	}

	tags, err := s.getArticleTags(id)
	if err != nil {
		return nil, err
	}
	article.Tags = tags

	return &article, nil
}

// getArticlesForExport gets articles for export based on options
func (s *Server) getArticlesForExport(opts export.ExportAllOptions) ([]model.ArticleWithDetails, error) {
	if opts.FromSearch != "" {
		return s.getArticlesFromSearch(opts)
	}

	query := `
		SELECT DISTINCT
			a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
			a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
			a.status_text, a.final_url, a.content_md, a.raw_html,
			f.path_cache as folder_path
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
		WHERE 1=1
	`

	var args []interface{}

	if opts.OnlySynced {
		query += " AND a.content_md IS NOT NULL"
	}

	if opts.FolderFilter != "" {
		query += " AND (f.path_cache = ? OR f.title = ?)"
		args = append(args, opts.FolderFilter, opts.FolderFilter)
	}

	if opts.TagFilter != "" {
		query += " AND t.title = ?"
		args = append(args, opts.TagFilter)
	}

	if opts.Since != "" {
		query += " AND a.instapapered_at >= ?"
		args = append(args, opts.Since)
	}

	if opts.Until != "" {
		query += " AND a.instapapered_at <= ?"
		args = append(args, opts.Until)
	}

	query += " ORDER BY a.instapapered_at DESC"

	var articles []model.ArticleWithDetails
	if err := s.db.Select(&articles, query, args...); err != nil {
		return nil, err
	}

	for i := range articles {
		tags, err := s.getArticleTags(articles[i].ID)
		if err != nil {
			return nil, err
		}
		articles[i].Tags = tags
	}

	return articles, nil
}

// getArticlesFromSearch gets articles based on search criteria
func (s *Server) getArticlesFromSearch(opts export.ExportAllOptions) ([]model.ArticleWithDetails, error) {
	baseQuery := `
		SELECT
			a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
			a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
			a.status_text, a.final_url, a.content_md, a.raw_html,
			f.path_cache as folder_path
		FROM articles a
		LEFT JOIN folders f ON a.folder_id = f.id
		LEFT JOIN article_tags at ON a.id = at.article_id
		LEFT JOIN tags t ON at.tag_id = t.id
	`

	var whereClause string
	var args []interface{}

	if opts.SearchFTS {
		baseQuery = `
			SELECT
				a.id, a.url, a.title, a.selection, a.folder_id, a.instapapered_at,
				a.synced_at, a.sync_failed_at, a.failed_count, a.status_code,
				a.status_text, a.final_url, a.content_md, a.raw_html,
				f.path_cache as folder_path
			FROM articles a
			LEFT JOIN folders f ON a.folder_id = f.id
			LEFT JOIN article_tags at ON a.id = at.article_id
			LEFT JOIN tags t ON at.tag_id = t.id
			INNER JOIN articles_fts fts ON a.id = fts.rowid
		`

		if opts.SearchField != "" {
			switch opts.SearchField {
			case "url":
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, "url: "+opts.FromSearch)
			case "title":
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, "title: "+opts.FromSearch)
			case "content":
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, "content: "+opts.FromSearch)
			case "tags":
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, "tags: "+opts.FromSearch)
			case "folder":
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, "folder: "+opts.FromSearch)
			default:
				return nil, fmt.Errorf("invalid field for FTS: %s", opts.SearchField)
			}
		} else {
			// For multiple keywords, create an AND query for intersection search
			keywords := strings.Fields(strings.TrimSpace(opts.FromSearch))
			if len(keywords) > 1 {
				// Build FTS query with AND operators for intersection
				ftsQuery := strings.Join(keywords, " AND ")
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, ftsQuery)
			} else {
				whereClause = "WHERE articles_fts MATCH ?"
				args = append(args, opts.FromSearch)
			}
		}
	} else {
		if opts.SearchField != "" {
			switch opts.SearchField {
			case "url":
				whereClause = "WHERE a.url LIKE ?"
			case "title":
				whereClause = "WHERE a.title LIKE ?"
			case "content":
				whereClause = "WHERE a.content_md LIKE ?"
			case "tags":
				whereClause = "WHERE t.title LIKE ?"
			case "folder":
				whereClause = "WHERE f.path_cache LIKE ? OR f.title LIKE ?"
				args = append(args, "%"+opts.FromSearch+"%")
			default:
				return nil, fmt.Errorf("invalid field: %s", opts.SearchField)
			}
			args = append(args, "%"+opts.FromSearch+"%")
		} else {
			whereClause = `
				WHERE (a.url LIKE ? OR a.title LIKE ? OR a.content_md LIKE ?
				       OR t.title LIKE ? OR f.path_cache LIKE ?)
			`
			pattern := "%" + opts.FromSearch + "%"
			args = append(args, pattern, pattern, pattern, pattern, pattern)
		}
	}

	query := baseQuery + " " + whereClause + `
		GROUP BY a.id
	`

	if opts.SearchFTS {
		query += " ORDER BY rank"
	} else {
		query += " ORDER BY a.instapapered_at DESC"
	}

	if opts.SearchLimit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.SearchLimit)
	}

	var articles []model.ArticleWithDetails
	if err := s.db.Select(&articles, query, args...); err != nil {
		return nil, err
	}

	for i := range articles {
		tags, err := s.getArticleTags(articles[i].ID)
		if err != nil {
			return nil, err
		}
		articles[i].Tags = tags
	}

	return articles, nil
}

// buildMarkdownContent builds markdown content for an article
func (s *Server) buildMarkdownContent(article model.ArticleWithDetails) (string, error) {
	tags := append([]string{"instapaper"}, article.Tags...)

	instapaperedAt, err := time.Parse(time.RFC3339, article.InstapaperedAt)
	if err != nil {
		return "", fmt.Errorf("failed to parse instapapered_at: %w", err)
	}

	frontMatter := model.FrontMatter{
		Title:          article.Title,
		InstapaperedAt: instapaperedAt,
		ExportedAt:     time.Now().UTC(),
		Source:         article.URL,
		Tags:           tags,
	}

	yamlBytes, err := yaml.Marshal(frontMatter)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	var content strings.Builder

	content.WriteString("---\n")
	content.Write(yamlBytes)
	content.WriteString("---\n\n")

	if article.ContentMD != nil && *article.ContentMD != "" {
		content.WriteString(*article.ContentMD)
	} else {
		content.WriteString(fmt.Sprintf("*Article content not yet fetched. Source: %s*\n", article.URL))
	}

	return content.String(), nil
}