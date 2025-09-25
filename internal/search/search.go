package search

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"
	"instapaper-cli/internal/util"
)

type Search struct {
	db *db.DB
}

type SearchOptions struct {
	Query      string
	Field      string
	UseFTS     bool
	Limit      int
	JSONOutput bool
	Since      string
	Until      string
}

func New(database *db.DB) *Search {
	return &Search{db: database}
}

func (s *Search) Search(opts SearchOptions) error {
	// Allow empty query for latest articles functionality
	if opts.Query == "" && opts.Field == "" && opts.Since == "" && opts.Until == "" {
		return fmt.Errorf("search query or date filter is required")
	}

	var results []model.SearchResult
	var err error

	if opts.UseFTS && opts.Query != "" {
		results, err = s.searchFTS(opts)
	} else if opts.Query != "" {
		results, err = s.searchLike(opts)
	} else {
		// Handle case where we only have date filters (for latest command)
		results, err = s.searchLike(opts)
	}

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if opts.JSONOutput {
		return s.outputJSON(results)
	}

	return s.outputTable(results)
}

func (s *Search) searchLike(opts SearchOptions) ([]model.SearchResult, error) {
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
	`

	var whereClause string
	var args []interface{}

	// Always exclude obsolete articles
	var conditions []string
	conditions = append(conditions, "a.obsolete = FALSE")

	// Add date filtering
	if opts.Since != "" || opts.Until != "" {
		sinceTime, untilTime, err := util.FormatDateRange(opts.Since, opts.Until)
		if err != nil {
			return nil, err
		}

		if sinceTime != nil {
			conditions = append(conditions, "a.instapapered_at >= ?")
			args = append(args, sinceTime.Format("2006-01-02 15:04:05"))
		}

		if untilTime != nil {
			conditions = append(conditions, "a.instapapered_at <= ?")
			args = append(args, untilTime.Format("2006-01-02 15:04:05"))
		}
	}

	if opts.Field != "" && opts.Query != "" {
		switch opts.Field {
		case "url":
			conditions = append(conditions, "a.url LIKE ? COLLATE NOCASE")
		case "title":
			conditions = append(conditions, "a.title LIKE ? COLLATE NOCASE")
		case "content":
			conditions = append(conditions, "a.content_md LIKE ? COLLATE NOCASE")
		case "tags":
			conditions = append(conditions, "t.title LIKE ? COLLATE NOCASE")
		case "folder":
			conditions = append(conditions, "(f.path_cache LIKE ? COLLATE NOCASE OR f.title LIKE ? COLLATE NOCASE)")
			args = append(args, "%"+opts.Query+"%")
		default:
			return nil, fmt.Errorf("invalid field: %s", opts.Field)
		}
		args = append(args, "%"+opts.Query+"%")
	} else if opts.Query != "" {
		conditions = append(conditions, `(a.url LIKE ? COLLATE NOCASE OR a.title LIKE ? COLLATE NOCASE OR a.content_md LIKE ? COLLATE NOCASE
		       OR t.title LIKE ? COLLATE NOCASE OR f.path_cache LIKE ? COLLATE NOCASE)`)
		pattern := "%" + opts.Query + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	whereClause = "WHERE " + strings.Join(conditions, " AND ")

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

func (s *Search) searchFTS(opts SearchOptions) ([]model.SearchResult, error) {
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
	`

	var whereClause string
	var args []interface{}

	// Always exclude obsolete articles
	var conditions []string
	conditions = append(conditions, "a.obsolete = FALSE")

	// Add date filtering
	if opts.Since != "" || opts.Until != "" {
		sinceTime, untilTime, err := util.FormatDateRange(opts.Since, opts.Until)
		if err != nil {
			return nil, err
		}

		if sinceTime != nil {
			conditions = append(conditions, "a.instapapered_at >= ?")
			args = append(args, sinceTime.Format("2006-01-02 15:04:05"))
		}

		if untilTime != nil {
			conditions = append(conditions, "a.instapapered_at <= ?")
			args = append(args, untilTime.Format("2006-01-02 15:04:05"))
		}
	}

	if opts.Field != "" {
		switch opts.Field {
		case "url":
			conditions = append(conditions, "articles_fts MATCH ?")
			args = append(args, "url: "+opts.Query)
		case "title":
			conditions = append(conditions, "articles_fts MATCH ?")
			args = append(args, "title: "+opts.Query)
		case "content":
			conditions = append(conditions, "articles_fts MATCH ?")
			args = append(args, "content: "+opts.Query)
		case "tags":
			conditions = append(conditions, "articles_fts MATCH ?")
			args = append(args, "tags: "+opts.Query)
		case "folder":
			conditions = append(conditions, "articles_fts MATCH ?")
			args = append(args, "folder: "+opts.Query)
		default:
			return nil, fmt.Errorf("invalid field for FTS: %s", opts.Field)
		}
	} else {
		conditions = append(conditions, "articles_fts MATCH ?")
		args = append(args, opts.Query)
	}

	whereClause = "WHERE " + strings.Join(conditions, " AND ")

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

func (s *Search) outputJSON(results []model.SearchResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func (s *Search) outputTable(results []model.SearchResult) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tTITLE\tURL\tFOLDER\tTAGS\tSYNCED\tFAILED")

	for _, result := range results {
		id := fmt.Sprintf("%d", result.ID)

		title := result.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		url := result.URL
		if len(url) > 60 {
			url = url[:57] + "..."
		}

		folder := ""
		if result.FolderPath != nil {
			folder = *result.FolderPath
			if len(folder) > 20 {
				folder = folder[:17] + "..."
			}
		}

		tags := ""
		if result.Tags != nil {
			tags = *result.Tags
			if len(tags) > 30 {
				tags = tags[:27] + "..."
			}
		}

		synced := "No"
		if result.SyncedAt != nil {
			synced = "Yes"
		}

		failed := ""
		if result.FailedCount > 0 {
			failed = fmt.Sprintf("%d", result.FailedCount)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			id, title, url, folder, tags, synced, failed)
	}

	return nil
}