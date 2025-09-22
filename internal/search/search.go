package search

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"
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
}

func New(database *db.DB) *Search {
	return &Search{db: database}
}

func (s *Search) Search(opts SearchOptions) error {
	if opts.Query == "" && opts.Field == "" {
		return fmt.Errorf("search query is required")
	}

	var results []model.SearchResult
	var err error

	if opts.UseFTS {
		results, err = s.searchFTS(opts)
	} else {
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

	if opts.Field != "" && opts.Query != "" {
		switch opts.Field {
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
			args = append(args, "%"+opts.Query+"%")
		default:
			return nil, fmt.Errorf("invalid field: %s", opts.Field)
		}
		args = append(args, "%"+opts.Query+"%")
	} else if opts.Query != "" {
		whereClause = `
			WHERE (a.url LIKE ? OR a.title LIKE ? OR a.content_md LIKE ?
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

	if opts.Field != "" {
		switch opts.Field {
		case "url":
			whereClause = "WHERE articles_fts MATCH ?"
			args = append(args, "url: "+opts.Query)
		case "title":
			whereClause = "WHERE articles_fts MATCH ?"
			args = append(args, "title: "+opts.Query)
		case "content":
			whereClause = "WHERE articles_fts MATCH ?"
			args = append(args, "content: "+opts.Query)
		case "tags":
			whereClause = "WHERE articles_fts MATCH ?"
			args = append(args, "tags: "+opts.Query)
		case "folder":
			whereClause = "WHERE articles_fts MATCH ?"
			args = append(args, "folder: "+opts.Query)
		default:
			return nil, fmt.Errorf("invalid field for FTS: %s", opts.Field)
		}
	} else {
		whereClause = "WHERE articles_fts MATCH ?"
		args = append(args, opts.Query)
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