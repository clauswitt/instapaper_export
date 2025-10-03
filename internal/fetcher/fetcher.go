package fetcher

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"instapaper-cli/internal/db"
	"instapaper-cli/internal/model"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-shiori/go-readability"
)

type Fetcher struct {
	db     *db.DB
	client *http.Client
	logger *log.Logger
}

type FetchOptions struct {
	Order           string
	SearchPhrase    string
	Limit           int
	PreferExtracted bool
	StoreRaw        bool
	LogPath         string
}

func New(database *db.DB) *Fetcher {
	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			DisableCompression: false,
		},
	}

	return &Fetcher{
		db:     database,
		client: client,
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (f *Fetcher) FetchArticles(opts FetchOptions) error {
	if opts.LogPath != "" {
		logFile, err := os.OpenFile(opts.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()
		f.logger = log.New(logFile, "", log.LstdFlags)
	}

	articles, err := f.getCandidateArticles(opts)
	if err != nil {
		return fmt.Errorf("failed to get candidate articles: %w", err)
	}

	f.logger.Printf("Found %d articles to fetch", len(articles))

	for i, article := range articles {
		f.logger.Printf("Fetching article %d/%d: %s", i+1, len(articles), article.URL)

		if err := f.fetchSingleArticle(article, opts); err != nil {
			f.logger.Printf("Failed to fetch article %d: %v", article.ID, err)
			continue
		}

		time.Sleep(500 * time.Millisecond)
	}

	f.logger.Printf("Fetch completed")
	return nil
}

func (f *Fetcher) getCandidateArticles(opts FetchOptions) ([]model.Article, error) {
	query := `
		SELECT id, url, title, instapapered_at
		FROM articles
		WHERE synced_at IS NULL
		AND failed_count < 5
		AND (sync_failed_at IS NULL OR sync_failed_at <= datetime('now', '-1 hour'))
		AND obsolete = FALSE
	`

	args := []interface{}{}

	if opts.SearchPhrase != "" {
		query += ` AND (url LIKE ? OR title LIKE ?)`
		searchPattern := "%" + opts.SearchPhrase + "%"
		args = append(args, searchPattern, searchPattern)
	}

	switch opts.Order {
	case "newest":
		query += ` ORDER BY instapapered_at DESC`
	default:
		query += ` ORDER BY instapapered_at ASC`
	}

	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	var articles []model.Article
	if err := f.db.Select(&articles, query, args...); err != nil {
		return nil, err
	}

	return articles, nil
}

func (f *Fetcher) fetchSingleArticle(article model.Article, opts FetchOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", article.URL, nil)
	if err != nil {
		return f.recordFailure(article.ID, 0, fmt.Sprintf("RequestError: %v", err))
	}

	req.Header.Set("User-Agent", "instapaper-cli/1.0 (+https://github.com/user/instapaper-cli)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := f.client.Do(req)
	if err != nil {
		return f.recordFailure(article.ID, 0, fmt.Sprintf("NetworkError: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return f.recordFailure(article.ID, resp.StatusCode, resp.Status)
	}

	readabilityResult, err := readability.FromReader(resp.Body, resp.Request.URL)
	if err != nil {
		return f.recordFailure(article.ID, resp.StatusCode, fmt.Sprintf("ReadabilityError: %v", err))
	}

	converter := md.NewConverter("", true, nil)
	markdown, err := converter.ConvertString(readabilityResult.Content)
	if err != nil {
		return f.recordFailure(article.ID, resp.StatusCode, fmt.Sprintf("MarkdownError: %v", err))
	}

	markdown = f.prettifyMarkdown(markdown)

	title := article.Title
	if opts.PreferExtracted && readabilityResult.Title != "" {
		title = readabilityResult.Title
	}

	var rawHTML *string
	if opts.StoreRaw {
		rawHTML = &readabilityResult.Content
	}

	now := time.Now().UTC().Format(time.RFC3339)
	finalURL := resp.Request.URL.String()

	_, err = f.db.Exec(`
		UPDATE articles
		SET synced_at = ?, content_md = ?, raw_html = ?, title = ?, final_url = ?,
		    status_code = ?, status_text = ?, failed_count = 0, sync_failed_at = NULL
		WHERE id = ?
	`, now, markdown, rawHTML, title, finalURL, resp.StatusCode, "OK", article.ID)

	if err != nil {
		return fmt.Errorf("failed to update article: %w", err)
	}

	// Update FTS table
	if err := f.db.UpsertArticleFTS(article.ID); err != nil {
		f.logger.Printf("Warning: failed to update FTS for article %d: %v", article.ID, err)
	}

	f.logger.Printf("Successfully fetched article %d: %s", article.ID, article.Title)
	return nil
}

func (f *Fetcher) recordFailure(articleID int64, statusCode int, statusText string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := f.db.Exec(`
		UPDATE articles
		SET sync_failed_at = ?, failed_count = failed_count + 1,
		    status_code = ?, status_text = ?
		WHERE id = ?
	`, now, statusCode, statusText, articleID)

	if err != nil {
		f.logger.Printf("Failed to record failure for article %d: %v", articleID, err)
	} else {
		f.logger.Printf("Recorded failure for article %d: %s", articleID, statusText)
	}

	return fmt.Errorf("fetch failed: %s", statusText)
}

func (f *Fetcher) prettifyMarkdown(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			if len(cleaned) > 0 && cleaned[len(cleaned)-1] != "" {
				cleaned = append(cleaned, "")
			}
			continue
		}

		if strings.Contains(line, "facebook.com/tr") ||
			strings.Contains(line, "google-analytics") ||
			strings.Contains(line, "gtag") ||
			strings.Contains(line, "googletagmanager") {
			continue
		}

		cleaned = append(cleaned, line)
	}

	result := strings.Join(cleaned, "\n")

	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")

	return strings.TrimSpace(result)
}